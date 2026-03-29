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
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS inventory_counts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER,
			barcode TEXT,
			quantity INTEGER DEFAULT 0,
			UNIQUE(session_id, barcode),
			FOREIGN KEY(session_id) REFERENCES inventory_sessions(id) ON DELETE CASCADE
		);`,
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
