package database

import (
	"context"
	"database/sql"
)

// SQLiteActiveCategoryRepository gestiona las categorías activas para filtrado de webhook.
type SQLiteActiveCategoryRepository struct {
	db *sql.DB
}

// NewSQLiteActiveCategoryRepository crea una nueva instancia del repositorio.
func NewSQLiteActiveCategoryRepository(db *sql.DB) *SQLiteActiveCategoryRepository {
	return &SQLiteActiveCategoryRepository{db: db}
}

// GetAll devuelve un mapa category_id → true para todas las categorías activas.
func (r *SQLiteActiveCategoryRepository) GetAll(ctx context.Context) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT category_id FROM active_categories")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
}

// IsActive informa si una categoría específica está en la lista de activas.
func (r *SQLiteActiveCategoryRepository) IsActive(ctx context.Context, categoryID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM active_categories WHERE category_id = ?", categoryID).Scan(&count)
	return count > 0, err
}

// SetActive activa (INSERT) o desactiva (DELETE) una categoría.
func (r *SQLiteActiveCategoryRepository) SetActive(ctx context.Context, categoryID string, active bool) error {
	if active {
		_, err := r.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO active_categories (category_id) VALUES (?)", categoryID)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM active_categories WHERE category_id = ?", categoryID)
	return err
}
