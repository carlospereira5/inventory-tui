package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SQLiteDB centraliza la conexión a la base de datos SQLite y su inicialización.
type SQLiteDB struct {
	Conn *sql.DB
}

// NewSQLiteDB abre una conexión SQLite y aplica configuraciones de rendimiento.
func NewSQLiteDB(path string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("error al abrir sqlite: %w", err)
	}

	// Optimizaciones de rendimiento para SQLite.
	_, _ = db.Exec("PRAGMA journal_mode = WAL")
	_, _ = db.Exec("PRAGMA synchronous = NORMAL")
	_, _ = db.Exec("PRAGMA foreign_keys = ON")
	_, _ = db.Exec("PRAGMA cache_size = -2000") // 2MB de caché.

	// busy_timeout: esperar hasta 5s antes de devolver SQLITE_BUSY.
	// Crítico en Android/Termux donde el filesystem tiene locks más agresivos.
	_, _ = db.Exec("PRAGMA busy_timeout = 5000")

	// WAL mode permite lectores concurrentes con un escritor.
	// Con busy_timeout, las escrituras concurrentes se resuelven con reintentos automáticos.
	db.SetMaxOpenConns(4) // Suficiente para concurrencia sin saturar
	db.SetMaxIdleConns(4)

	if err := initSchema(db); err != nil {
		return nil, err
	}

	return &SQLiteDB{Conn: db}, nil
}

// initSchema crea las tablas necesarias si no existen.
func initSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			barcode TEXT UNIQUE,
			name TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS inventory_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			status TEXT DEFAULT 'active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS inventory_scans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER,
			barcode TEXT,
			quantity_delta INTEGER DEFAULT 1,
			source TEXT DEFAULT 'SCAN', -- SCAN, LOYVERSE_SALE, MANUAL
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(session_id) REFERENCES inventory_sessions(id) ON DELETE CASCADE
		);`,
		// Tabla separada para eventos de Loyverse (ventas y refunds).
		// Separada de inventory_scans para que el borrado en una pantalla no afecte la otra.
		`CREATE TABLE IF NOT EXISTS loyverse_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			barcode TEXT NOT NULL,
			quantity_delta INTEGER NOT NULL,
			source TEXT NOT NULL DEFAULT 'LOYVERSE_SALE', -- LOYVERSE_SALE, LOYVERSE_REFUND
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(session_id) REFERENCES inventory_sessions(id) ON DELETE CASCADE
		);`,
		// Tabla para grupos personalizados de productos (descuentos Loyverse).
		`CREATE TABLE IF NOT EXISTS custom_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_name TEXT NOT NULL UNIQUE
		);`,
		// Tabla junction para relacionar grupos con productos.
		`CREATE TABLE IF NOT EXISTS custom_group_products (
			group_id INTEGER NOT NULL,
			product_id INTEGER NOT NULL,
			PRIMARY KEY (group_id, product_id),
			FOREIGN KEY(group_id) REFERENCES custom_groups(id) ON DELETE CASCADE,
			FOREIGN KEY(product_id) REFERENCES products(id) ON DELETE CASCADE
		);`,
		// Vista para ver totales por sesión/producto si es necesario para compatibilidad o reportes rápidos.
		`CREATE VIEW IF NOT EXISTS inventory_totals AS
		SELECT session_id, barcode, SUM(quantity_delta) as total
		FROM inventory_scans
		GROUP BY session_id, barcode;`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("error al ejecutar migración: %w", err)
		}
	}
	return nil
}

// Close cierra la conexión a la base de datos de forma segura.
func (db *SQLiteDB) Close() error {
	return db.Conn.Close()
}
