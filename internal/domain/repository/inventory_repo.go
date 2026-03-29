package repository

import (
	"context"
	"inventory-tui/internal/domain/entity"
)

// ProductRepository define las operaciones para el catálogo maestro de productos.
type ProductRepository interface {
	// FindByBarcode busca un producto por su código de barras único.
	FindByBarcode(ctx context.Context, barcode string) (*entity.Product, error)

	// Upsert inserta un producto o lo actualiza si ya existe (basado en el código de barras).
	Upsert(ctx context.Context, product *entity.Product) error
}

// SessionRepository gestiona las sesiones de conteo de inventario.
type SessionRepository interface {
	// Create crea una nueva sesión y devuelve su ID generado.
	Create(ctx context.Context, name string) (int, error)

	// GetAll devuelve todas las sesiones de inventario existentes, ordenadas por fecha de creación.
	GetAll(ctx context.Context) ([]entity.Session, error)

	// Delete elimina una sesión (y por cascada todos sus conteos asociados).
	Delete(ctx context.Context, id int) error
}

// InventoryRepository gestiona los conteos individuales dentro de las sesiones.
type InventoryRepository interface {
	// IncrementCount aumenta en 1 la cantidad contada de un código de barras en una sesión específica.
	IncrementCount(ctx context.Context, sessionID int, barcode string) error

	// GetRecord busca el registro de conteo específico para un código de barras en una sesión.
	GetRecord(ctx context.Context, sessionID int, barcode string) (*entity.Record, error)

	// GetSessionHistory devuelve todos los registros de conteo para una sesión de inventario dada.
	GetSessionHistory(ctx context.Context, sessionID int) ([]entity.Record, error)

	// DeleteRecord elimina un registro de conteo específico por su ID único.
	DeleteRecord(ctx context.Context, id int) error
}
