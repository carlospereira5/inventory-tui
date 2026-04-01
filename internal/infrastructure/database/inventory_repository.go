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

// AddScan registra un nuevo evento de escaneo (o ajuste de stock) en la sesión.
func (r *SQLiteInventoryRepository) AddScan(ctx context.Context, sessionID int, barcode string, delta int, source string) error {
	query := `INSERT INTO inventory_scans (session_id, barcode, quantity_delta, source) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, sessionID, barcode, delta, source)
	return err
}

// GetRecord obtiene el total acumulado para un producto específico en una sesión.
func (r *SQLiteInventoryRepository) GetRecord(ctx context.Context, sessionID int, barcode string) (*entity.Record, error) {
	var rec entity.Record
	query := `
		SELECT 0, s.session_id, s.barcode, p.name, SUM(s.quantity_delta) as total
		FROM inventory_scans s
		JOIN products p ON s.barcode = p.barcode
		WHERE s.session_id = ? AND s.barcode = ?
		GROUP BY s.session_id, s.barcode`

	err := r.db.QueryRowContext(ctx, query, sessionID, barcode).
		Scan(&rec.ID, &rec.SessionID, &rec.Barcode, &rec.Name, &rec.Quantity)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &rec, err
}

// GetSessionHistory recupera el historial de TODOS los escaneos individuales en la sesión.
func (r *SQLiteInventoryRepository) GetSessionHistory(ctx context.Context, sessionID int) ([]entity.Record, error) {
	query := `
		SELECT s.id, s.session_id, s.barcode, p.name, s.quantity_delta, s.source
		FROM inventory_scans s
		JOIN products p ON s.barcode = p.barcode
		WHERE s.session_id = ?
		ORDER BY s.created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []entity.Record
	for rows.Next() {
		var rec entity.Record
		if err := rows.Scan(&rec.ID, &rec.SessionID, &rec.Barcode, &rec.Name, &rec.Quantity, &rec.Source); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

// DeleteScan elimina un registro de escaneo específico del historial.
func (r *SQLiteInventoryRepository) DeleteScan(ctx context.Context, scanID int) error {
	query := "DELETE FROM inventory_scans WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, scanID)
	return err
}

// GetSessionTotals devuelve el total acumulado por producto en una sesión.
// Incluye tanto escaneos manuales como eventos de Loyverse.
func (r *SQLiteInventoryRepository) GetSessionTotals(ctx context.Context, sessionID int) ([]entity.SessionTotals, error) {
	query := `
		SELECT barcode, name, SUM(total_delta) as total
		FROM (
			SELECT s.barcode, p.name, s.quantity_delta as total_delta
			FROM inventory_scans s
			JOIN products p ON s.barcode = p.barcode
			WHERE s.session_id = ?
			UNION ALL
			SELECT le.barcode, p.name, le.quantity_delta as total_delta
			FROM loyverse_events le
			JOIN products p ON le.barcode = p.barcode
			WHERE le.session_id = ?
		)
		GROUP BY barcode
		ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query, sessionID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var totals []entity.SessionTotals
	for rows.Next() {
		var t entity.SessionTotals
		if err := rows.Scan(&t.Barcode, &t.Name, &t.Quantity); err != nil {
			return nil, err
		}
		totals = append(totals, t)
	}
	return totals, nil
}
