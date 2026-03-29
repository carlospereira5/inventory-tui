package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Product struct {
	ID      int
	Barcode string
	Name    string
}

type InventorySession struct {
	ID        int
	Name      string
	CreatedAt string
}

type InventoryRecord struct {
	ID        int
	SessionID int
	Barcode   string
	Name      string
	Quantity  int
}

func initDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Performance optimizations
	_, _ = db.Exec("PRAGMA journal_mode = WAL")
	_, _ = db.Exec("PRAGMA synchronous = NORMAL")
	_, _ = db.Exec("PRAGMA foreign_keys = ON")
	_, _ = db.Exec("PRAGMA cache_size = -2000") // 2MB cache

	// Master list of products
	queryProducts := `
	CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		barcode TEXT UNIQUE,
		name TEXT
	);`

	// Inventory sessions
	querySessions := `
	CREATE TABLE IF NOT EXISTS inventory_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	// Inventory counts within sessions
	queryCounts := `
	CREATE TABLE IF NOT EXISTS inventory_counts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER,
		barcode TEXT,
		quantity INTEGER DEFAULT 0,
		UNIQUE(session_id, barcode),
		FOREIGN KEY(session_id) REFERENCES inventory_sessions(id) ON DELETE CASCADE
	);`

	// Ejecutar creaciones básicas
	if _, err := db.Exec(queryProducts); err != nil {
		return nil, err
	}
	if _, err := db.Exec(querySessions); err != nil {
		return nil, err
	}

	// COMPROBACIÓN CRÍTICA: ¿Existe la columna session_id?
	// Si la tabla existe pero no tiene la columna, hay que recrearla.
	if tableExists(db, "inventory_counts") {
		if !columnExists(db, "inventory_counts", "session_id") {
			fmt.Println("Migrando tabla inventory_counts (esquema antiguo detectado)...")
			_, _ = db.Exec("DROP TABLE inventory_counts")
		}
	}

	// Crear la tabla (ya sea nueva o recreada tras el DROP)
	if _, err := db.Exec(queryCounts); err != nil {
		return nil, err
	}

	return db, nil
}

func tableExists(db *sql.DB, name string) bool {
	var n string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n)
	return err == nil
}

func columnExists(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, dtype string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &dtype, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == column {
			return true
		}
	}
	return false
}

func getProductByBarcode(db *sql.DB, barcode string) (*Product, error) {
	var p Product
	err := db.QueryRow("SELECT id, barcode, name FROM products WHERE barcode = ?", barcode).
		Scan(&p.ID, &p.Barcode, &p.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func createSession(db *sql.DB, name string) (int, error) {
	res, err := db.Exec("INSERT INTO inventory_sessions (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func getSessions(db *sql.DB) ([]InventorySession, error) {
	rows, err := db.Query("SELECT id, name, created_at FROM inventory_sessions ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []InventorySession
	for rows.Next() {
		var s InventorySession
		if err := rows.Scan(&s.ID, &s.Name, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func deleteSession(db *sql.DB, id int) error {
	_, err := db.Exec("PRAGMA foreign_keys = ON") // ensure cascade
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM inventory_sessions WHERE id = ?", id)
	return err
}

func incrementCountInSession(db *sql.DB, sessionID int, barcode string) error {
	_, err := db.Exec(`
		INSERT INTO inventory_counts (session_id, barcode, quantity)
		VALUES (?, ?, 1)
		ON CONFLICT(session_id, barcode) DO UPDATE SET quantity = quantity + 1;
	`, sessionID, barcode)
	return err
}

func getHistoryInSession(db *sql.DB, sessionID int) ([]InventoryRecord, error) {
	rows, err := db.Query(`
		SELECT c.id, c.session_id, c.barcode, p.name, c.quantity
		FROM inventory_counts c
		JOIN products p ON c.barcode = p.barcode
		WHERE c.session_id = ?
		ORDER BY c.id DESC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []InventoryRecord
	for rows.Next() {
		var r InventoryRecord
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Barcode, &r.Name, &r.Quantity); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

func getRecordInSession(db *sql.DB, sessionID int, barcode string) (*InventoryRecord, error) {
	var r InventoryRecord
	err := db.QueryRow(`
		SELECT c.id, c.session_id, c.barcode, p.name, c.quantity
		FROM inventory_counts c
		JOIN products p ON c.barcode = p.barcode
		WHERE c.session_id = ? AND c.barcode = ?`, sessionID, barcode).
		Scan(&r.ID, &r.SessionID, &r.Barcode, &r.Name, &r.Quantity)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func deleteHistoryRecordInSession(db *sql.DB, id int) error {
	_, err := db.Exec("DELETE FROM inventory_counts WHERE id = ?", id)
	return err
}

// Queryer defines a common interface for sql.DB and sql.Tx
type Queryer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func upsertProduct(db Queryer, barcode, name string) error {
	_, err := db.Exec(`
		INSERT INTO products (barcode, name)
		VALUES (?, ?)
		ON CONFLICT(barcode) DO UPDATE SET name = excluded.name;
	`, barcode, name)
	return err
}
