package database

import (
	"context"
	"database/sql"
	"inventory-tui/internal/domain/entity"
)

// SQLiteInventoryRepository gestiona los conteos de inventario individuales en SQLite.
type SQLiteInventoryRepository struct {
	db *sql.DB
}

// NewSQLiteInventoryRepository crea una nueva instancia del repositorio de inventario.
func NewSQLiteInventoryRepository(db *sql.DB) *SQLiteInventoryRepository {
	return &SQLiteInventoryRepository{db: db}
}

// IncrementCount aumenta la cantidad en 1 para un código de barras en una sesión.
func (r *SQLiteInventoryRepository) IncrementCount(ctx context.Context, sessionID int, barcode string) error {
	query := `
		INSERT INTO inventory_counts (session_id, barcode, quantity)
		VALUES (?, ?, 1)
		ON CONFLICT(session_id, barcode) DO UPDATE SET quantity = quantity + 1;
	`
	_, err := r.db.ExecContext(ctx, query, sessionID, barcode)
	return err
}

// GetRecord obtiene el registro actual de un producto en una sesión específica.
func (r *SQLiteInventoryRepository) GetRecord(ctx context.Context, sessionID int, barcode string) (*entity.Record, error) {
	var rec entity.Record
	query := `
		SELECT c.id, c.session_id, c.barcode, p.name, c.quantity
		FROM inventory_counts c
		JOIN products p ON c.barcode = p.barcode
		WHERE c.session_id = ? AND c.barcode = ?`

	err := r.db.QueryRowContext(ctx, query, sessionID, barcode).
		Scan(&rec.ID, &rec.SessionID, &rec.Barcode, &rec.Name, &rec.Quantity)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// GetSessionHistory recupera todos los registros contados en una sesión.
func (r *SQLiteInventoryRepository) GetSessionHistory(ctx context.Context, sessionID int) ([]entity.Record, error) {
	query := `
		SELECT c.id, c.session_id, c.barcode, p.name, c.quantity
		FROM inventory_counts c
		JOIN products p ON c.barcode = p.barcode
		WHERE c.session_id = ?
		ORDER BY c.id DESC`

	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []entity.Record
	for rows.Next() {
		var rec entity.Record
		if err := rows.Scan(&rec.ID, &rec.SessionID, &rec.Barcode, &rec.Name, &rec.Quantity); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

// DeleteRecord elimina un registro de conteo específico.
func (r *SQLiteInventoryRepository) DeleteRecord(ctx context.Context, id int) error {
	query := "DELETE FROM inventory_counts WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
