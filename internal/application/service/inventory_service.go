package service

import (
	"context"
	"database/sql"
	"inventory-tui/internal/domain/entity"
	"inventory-tui/internal/domain/repository"
	"inventory-tui/internal/infrastructure/storage"
)

// InventoryService coordina los procesos de negocio relacionados con el inventario.
type InventoryService struct {
	db          *sql.DB
	products    repository.ProductRepository
	sessions    repository.SessionRepository
	inventory   repository.InventoryRepository
	csvStorage  *storage.CSVStorage
}

// NewInventoryService crea una instancia del servicio de inventario.
func NewInventoryService(
	db *sql.DB,
	p repository.ProductRepository,
	s repository.SessionRepository,
	i repository.InventoryRepository,
	csv *storage.CSVStorage,
) *InventoryService {
	return &InventoryService{db: db, products: p, sessions: s, inventory: i, csvStorage: csv}
}

// LoadCatalog busca e importa el archivo CSV en la base de datos de productos.
func (s *InventoryService) LoadCatalog(ctx context.Context) (int, string, error) {
	file, err := s.csvStorage.FindCSVInRoot()
	if err != nil {
		return 0, "", err
	}
	count, err := s.csvStorage.ImportProducts(ctx, s.db, file)
	return count, file, err
}

// GetSessions devuelve todas las sesiones de inventario existentes.
func (s *InventoryService) GetSessions(ctx context.Context) ([]entity.Session, error) {
	return s.sessions.GetAll(ctx)
}

// CreateSession crea una nueva sesión y la devuelve con su ID generado.
func (s *InventoryService) CreateSession(ctx context.Context, name string) (int, error) {
	return s.sessions.Create(ctx, name)
}

// ScanProduct busca el producto, incrementa el conteo y devuelve el registro actualizado.
func (s *InventoryService) ScanProduct(ctx context.Context, sessionID int, barcode string) (*entity.Product, *entity.Record, error) {
	p, err := s.products.FindByBarcode(ctx, barcode)
	if err != nil || p == nil {
		return p, nil, err
	}
	if err := s.inventory.IncrementCount(ctx, sessionID, barcode); err != nil {
		return p, nil, err
	}
	rec, err := s.inventory.GetRecord(ctx, sessionID, barcode)
	return p, rec, err
}

// GetHistory obtiene el historial completo de conteos para una sesión.
func (s *InventoryService) GetHistory(ctx context.Context, sessionID int) ([]entity.Record, error) {
	return s.inventory.GetSessionHistory(ctx, sessionID)
}

// ExportSession descarga los datos de la sesión a un archivo CSV.
func (s *InventoryService) ExportSession(ctx context.Context, id int, name string) (string, error) {
	records, err := s.inventory.GetSessionHistory(ctx, id)
	if err != nil {
		return "", err
	}
	return s.csvStorage.ExportSession(name, records)
}

// DeleteSession elimina la sesión por completo.
func (s *InventoryService) DeleteSession(ctx context.Context, id int) error {
	return s.sessions.Delete(ctx, id)
}

// DeleteRecord elimina un registro individual de conteo.
func (s *InventoryService) DeleteRecord(ctx context.Context, id int) error {
	return s.inventory.DeleteRecord(ctx, id)
}
