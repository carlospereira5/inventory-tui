package storage

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"inventory-tui/internal/domain/entity"
	"inventory-tui/internal/domain/repository"
)

// CSVStorage gestiona la persistencia y lectura de archivos CSV.
type CSVStorage struct {
	productRepo repository.ProductRepository
	groupRepo   repository.CustomGroupRepository
}

// NewCSVStorage crea una nueva instancia de almacenamiento CSV.
func NewCSVStorage(productRepo repository.ProductRepository, groupRepo repository.CustomGroupRepository) *CSVStorage {
	return &CSVStorage{productRepo: productRepo, groupRepo: groupRepo}
}

// FindCSVInRoot busca el único archivo .csv presente en la raíz del proyecto.
func (s *CSVStorage) FindCSVInRoot() (string, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return "", err
	}

	var csvFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".csv") {
			csvFiles = append(csvFiles, f.Name())
		}
	}

	if len(csvFiles) == 0 {
		return "", fmt.Errorf("no se encontró ningún archivo .csv en la raíz")
	}
	if len(csvFiles) > 1 {
		return "", fmt.Errorf("múltiples archivos .csv encontrados, debe haber solo uno")
	}

	return csvFiles[0], nil
}

// FindGroupsCSV busca el archivo de grupos personalizados (grupos.csv) en la raíz.
func (s *CSVStorage) FindGroupsCSV() (string, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return "", err
	}

	for _, f := range files {
		if !f.IsDir() && strings.ToLower(f.Name()) == "grupos.csv" {
			return f.Name(), nil
		}
	}

	return "", fmt.Errorf("no se encontró grupos.csv en la raíz")
}

// ImportProducts lee un CSV e inserta los productos en la base de datos usando concurrencia.
func (s *CSVStorage) ImportProducts(ctx context.Context, db *sql.DB, filePath string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	if _, err = reader.Read(); err != nil { // Saltar cabecera.
		return 0, err
	}

	// Usamos un patrón productor-consumidor para optimizar la inserción masiva.
	type record struct{ barcode, name string }
	records := make(chan record, 100)
	errChan := make(chan error, 1)
	countChan := make(chan int)

	go func() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			errChan <- err
			return
		}
		defer tx.Rollback()

		count := 0
		for rec := range records {
			p := &entity.Product{Barcode: rec.barcode, Name: rec.name}
			// Reutilizamos la lógica del repo de productos para el Upsert.
			if err := s.productRepo.Upsert(ctx, p); err != nil {
				errChan <- err
				return
			}
			count++
		}
		if err := tx.Commit(); err != nil {
			errChan <- err
			return
		}
		countChan <- count
	}()

	var parseErr error
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			parseErr = err
			break
		}
		if len(line) >= 14 && line[13] != "" {
			records <- record{barcode: line[13], name: line[2]}
		}
	}
	close(records)

	select {
	case err := <-errChan:
		return 0, err
	case count := <-countChan:
		return count, parseErr
	}
}

// ExportSession guarda el historial de una sesión en un nuevo archivo CSV.
func (s *CSVStorage) ExportSession(sessionName string, records []entity.Record) (string, error) {
	cleanName := strings.ReplaceAll(strings.ToLower(sessionName), " ", "_")
	fileName := fmt.Sprintf("export_%s_%s.csv", cleanName, time.Now().Format("20060102_150405"))

	f, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	_ = writer.Write([]string{"Codigo de barras", "Nombre", "Cantidad"})
	for _, r := range records {
		_ = writer.Write([]string{r.Barcode, r.Name, fmt.Sprintf("%d", r.Quantity)})
	}
	return fileName, nil
}

// ImportGroups lee un CSV de grupos e inserta los grupos en la base de datos.
// Formato esperado: group_name,barcode (una fila por producto-grupo)
func (s *CSVStorage) ImportGroups(ctx context.Context, filePath string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return 0, err
	}

	// Validar header
	if len(header) < 2 || header[0] != "group_name" || header[1] != "barcode" {
		return 0, fmt.Errorf("formato de CSV inválido: se espera 'group_name,barcode'")
	}

	// Agrupar barcodes por nombre de grupo
	groupMap := make(map[string][]string)
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		if len(line) >= 2 {
			groupName := strings.TrimSpace(line[0])
			barcode := strings.TrimSpace(line[1])
			if groupName != "" && barcode != "" {
				groupMap[groupName] = append(groupMap[groupName], barcode)
			}
		}
	}

	// Insertar cada grupo
	groupCount := 0
	for groupName, barcodes := range groupMap {
		// Obtener IDs de productos por barcode
		var productIDs []int
		for _, barcode := range barcodes {
			p, err := s.productRepo.FindByBarcode(ctx, barcode)
			if err != nil {
				return 0, fmt.Errorf("error al buscar producto %s: %w", barcode, err)
			}
			if p != nil {
				productIDs = append(productIDs, p.ID)
			}
		}

		if len(productIDs) > 0 {
			if err := s.groupRepo.CreateGroup(ctx, groupName, productIDs); err != nil {
				return 0, fmt.Errorf("error al crear grupo %s: %w", groupName, err)
			}
			groupCount++
		}
	}

	return groupCount, nil
}
