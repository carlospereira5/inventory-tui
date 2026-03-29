package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func findCSVInRoot() (string, error) {
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
		return "", fmt.Errorf("se encontraron múltiples archivos .csv: %s. Solo debe haber uno", strings.Join(csvFiles, ", "))
	}

	return csvFiles[0], nil
}

type csvRecord struct {
	barcode string
	name    string
}

func importCSV(db *sql.DB, filePath string) (int, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	// Skip header
	_, err = reader.Read()
	if err != nil {
		return 0, err
	}

	// Producer-Consumer with a channel
	records := make(chan csvRecord, 100)
	errChan := make(chan error, 1)
	countChan := make(chan int)

	// Worker goroutine: handles database insertions
	go func() {
		tx, err := db.Begin()
		if err != nil {
			errChan <- err
			return
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare(`
			INSERT INTO products (barcode, name) 
			VALUES (?, ?) 
			ON CONFLICT(barcode) DO UPDATE SET name = excluded.name;
		`)
		if err != nil {
			errChan <- err
			return
		}
		defer stmt.Close()

		c := 0
		for rec := range records {
			_, err = stmt.Exec(rec.barcode, rec.name)
			if err != nil {
				errChan <- err
				return
			}
			c++
		}

		if err := tx.Commit(); err != nil {
			errChan <- err
			return
		}
		countChan <- c
	}()

	// Producer (main loop): reads the CSV and sends records to the channel
	var parseErr error
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			parseErr = err
			break
		}

		if len(record) < 14 {
			continue
		}

		name := record[2]
		barcode := record[13]

		if barcode == "" {
			continue
		}

		records <- csvRecord{barcode: barcode, name: name}
	}
	close(records)

	// Wait for worker to finish or for an error
	select {
	case err := <-errChan:
		return 0, err
	case count := <-countChan:
		if parseErr != nil {
			return count, parseErr
		}
		return count, nil
	}
}

func exportSessionToCSV(db *sql.DB, sessionID int, sessionName string) (string, error) {
	records, err := getHistoryInSession(db, sessionID)
	if err != nil {
		return "", err
	}

	// Nombre del archivo basado en la sesión y la fecha
	cleanName := strings.ReplaceAll(strings.ToLower(sessionName), " ", "_")
	fileName := fmt.Sprintf("export_%s_%s.csv",
		cleanName,
		time.Now().Format("20060102_150405"))

	f, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	// Cabecera del archivo
	_ = writer.Write([]string{"Codigo de barras", "Nombre", "Cantidad"})

	for _, r := range records {
		_ = writer.Write([]string{r.Barcode, r.Name, fmt.Sprintf("%d", r.Quantity)})
	}

	return fileName, nil
}
