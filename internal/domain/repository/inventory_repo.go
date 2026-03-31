package repository

import (
	"context"
	"inventory-tui/internal/domain/entity"
)

// ProductRepository define las operaciones para el catálogo maestro de productos.
type ProductRepository interface {
	// FindByBarcode busca un producto por su código de barras único.
	FindByBarcode(ctx context.Context, barcode string) (*entity.Product, error)

	// FindByName busca un producto por su nombre exacto.
	// Usado por el webhook de Loyverse, que no expone barcode en line_items.
	FindByName(ctx context.Context, name string) (*entity.Product, error)

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
	// AddScan registra un nuevo evento de escaneo (o ajuste de stock) en la sesión.
	AddScan(ctx context.Context, sessionID int, barcode string, delta int, source string) error

	// GetRecord obtiene el total acumulado para un producto específico en una sesión.
	GetRecord(ctx context.Context, sessionID int, barcode string) (*entity.Record, error)

	// GetSessionHistory recupera el historial de TODOS los escaneos individuales en la sesión.
	GetSessionHistory(ctx context.Context, sessionID int) ([]entity.Record, error)

	// DeleteScan elimina un registro de escaneo específico del historial.
	DeleteScan(ctx context.Context, scanID int) error

	// GetSessionTotals devuelve el total acumulado por producto en una sesión.
	GetSessionTotals(ctx context.Context, sessionID int) ([]entity.SessionTotals, error)

	// GetLoyverseEvents devuelve solo los eventos de Loyverse (ventas y refunds) de una sesión.
	GetLoyverseEvents(ctx context.Context, sessionID int) ([]entity.LoyverseEvent, error)
}

// CustomGroupRepository gestiona los grupos personalizados de productos para descuentos de Loyverse.
type CustomGroupRepository interface {
	// CreateGroup crea un nuevo grupo con los IDs de productos especificados.
	CreateGroup(ctx context.Context, groupName string, productIDs []int) error

	// GetGroupByName busca un grupo por su nombre.
	GetGroupByName(ctx context.Context, groupName string) (*entity.CustomGroup, error)

	// GetAllGroups devuelve todos los grupos existentes.
	GetAllGroups(ctx context.Context) ([]entity.CustomGroup, error)

	// DeleteGroup elimina un grupo por su ID.
	DeleteGroup(ctx context.Context, groupID int) error

	// IsProductInGroup verifica si un producto pertenece a un grupo específico.
	IsProductInGroup(ctx context.Context, groupID int, productID int) (bool, error)

	// GetGroupsForProduct devuelve todos los grupos que contienen un producto.
	GetGroupsForProduct(ctx context.Context, productID int) ([]entity.CustomGroup, error)
}
