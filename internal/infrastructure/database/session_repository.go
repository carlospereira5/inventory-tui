package database

import (
	"context"
	"database/sql"
	"inventory-tui/internal/domain/entity"
)

// SQLiteSessionRepository gestiona las sesiones de inventario en la base de datos SQLite.
type SQLiteSessionRepository struct {
	db *sql.DB
}

// NewSQLiteSessionRepository crea una nueva instancia del repositorio de sesiones.
func NewSQLiteSessionRepository(db *sql.DB) *SQLiteSessionRepository {
	return &SQLiteSessionRepository{db: db}
}

// Create inserta una nueva sesión y devuelve su ID generado.
func (r *SQLiteSessionRepository) Create(ctx context.Context, name string) (int, error) {
	query := "INSERT INTO inventory_sessions (name) VALUES (?)"
	res, err := r.db.ExecContext(ctx, query, name)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

// GetAll recupera todas las sesiones ordenadas por fecha de creación descendente.
func (r *SQLiteSessionRepository) GetAll(ctx context.Context) ([]entity.Session, error) {
	query := "SELECT id, name, created_at FROM inventory_sessions ORDER BY created_at DESC"
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []entity.Session
	for rows.Next() {
		var s entity.Session
		if err := rows.Scan(&s.ID, &s.Name, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// Delete elimina una sesión específica por su ID.
func (r *SQLiteSessionRepository) Delete(ctx context.Context, id int) error {
	query := "DELETE FROM inventory_sessions WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
