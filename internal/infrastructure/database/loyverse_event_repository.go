package database

import (
	"context"
	"database/sql"
	"inventory-tui/internal/domain/entity"
)

// SQLiteLoyverseEventRepository implementa la interfaz LoyverseEventRepository usando SQLite.
type SQLiteLoyverseEventRepository struct {
	db *sql.DB
}

// NewSQLiteLoyverseEventRepository crea una nueva instancia del repositorio de eventos de Loyverse.
func NewSQLiteLoyverseEventRepository(db *sql.DB) *SQLiteLoyverseEventRepository {
	return &SQLiteLoyverseEventRepository{db: db}
}

// AddEvent registra un nuevo evento de Loyverse en la sesión.
func (r *SQLiteLoyverseEventRepository) AddEvent(ctx context.Context, sessionID int, barcode string, delta int, source string) error {
	query := `INSERT INTO loyverse_events (session_id, barcode, quantity_delta, source) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, sessionID, barcode, delta, source)
	return err
}

// GetEvents devuelve todos los eventos de Loyverse de una sesión.
func (r *SQLiteLoyverseEventRepository) GetEvents(ctx context.Context, sessionID int) ([]entity.LoyverseEvent, error) {
	query := `
		SELECT le.id, le.session_id, p.name, COALESCE(cg.group_name, ''), le.quantity_delta, le.source, le.created_at
		FROM loyverse_events le
		JOIN products p ON le.barcode = p.barcode
		LEFT JOIN custom_group_products cgp ON p.id = cgp.product_id
		LEFT JOIN custom_groups cg ON cgp.group_id = cg.id
		WHERE le.session_id = ?
		ORDER BY le.created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []entity.LoyverseEvent
	for rows.Next() {
		var e entity.LoyverseEvent
		if err := rows.Scan(&e.ID, &e.SessionID, &e.Name, &e.GroupName, &e.Quantity, &e.Source, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// DeleteEvent elimina un evento de Loyverse por su ID.
func (r *SQLiteLoyverseEventRepository) DeleteEvent(ctx context.Context, eventID int) error {
	query := "DELETE FROM loyverse_events WHERE id = ?"
	_, err := r.db.ExecContext(ctx, query, eventID)
	return err
}
