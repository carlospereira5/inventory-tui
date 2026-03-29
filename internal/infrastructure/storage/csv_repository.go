package storage

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"inventory-tui/internal/domain/entity"
	"inventory-tui/internal/domain/repository"
	"io"
	"os"
	"strings"
	"time"
)

// CSVStorage gestiona la persistencia y lectura de archivos CSV.
type CSVStorage struct {
	productRepo repository.ProductRepository
}

// NewCSVStorage crea una nueva instancia de almacenamiento CSV.
func NewCSVStorage(repo repository.ProductRepository) *CSVStorage {
	return &CSVStorage{productRepo: repo}
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
