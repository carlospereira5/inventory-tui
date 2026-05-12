package database

import (
	"context"
	"database/sql"
	"inventory-tui/internal/domain/entity"
)

// SQLiteProductRepository implementa la interfaz ProductRepository usando SQLite.
type SQLiteProductRepository struct {
	db *sql.DB
}

// NewSQLiteProductRepository crea una nueva instancia del repositorio.
func NewSQLiteProductRepository(db *sql.DB) *SQLiteProductRepository {
	return &SQLiteProductRepository{db: db}
}

// FindByBarcode busca un producto por su código de barras en la tabla products.
func (r *SQLiteProductRepository) FindByBarcode(ctx context.Context, barcode string) (*entity.Product, error) {
	var p entity.Product
	query := "SELECT id, barcode, name, COALESCE(category_id, '') FROM products WHERE barcode = ?"
	err := r.db.QueryRowContext(ctx, query, barcode).Scan(&p.ID, &p.Barcode, &p.Name, &p.CategoryID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByName busca un producto por nombre exacto.
// La búsqueda es case-sensitive; el nombre debe coincidir exactamente con el del catálogo.
func (r *SQLiteProductRepository) FindByName(ctx context.Context, name string) (*entity.Product, error) {
	var p entity.Product
	query := "SELECT id, barcode, name, COALESCE(category_id, '') FROM products WHERE name = ?"
	err := r.db.QueryRowContext(ctx, query, name).Scan(&p.ID, &p.Barcode, &p.Name, &p.CategoryID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Upsert inserta un nuevo producto o actualiza nombre y categoría si el barcode ya existe.
func (r *SQLiteProductRepository) Upsert(ctx context.Context, p *entity.Product) error {
	query := `
		INSERT INTO products (barcode, name, category_id)
		VALUES (?, ?, ?)
		ON CONFLICT(barcode) DO UPDATE SET name = excluded.name, category_id = excluded.category_id;
	`
	_, err := r.db.ExecContext(ctx, query, p.Barcode, p.Name, p.CategoryID)
	return err
}
