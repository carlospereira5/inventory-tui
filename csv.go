package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
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

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}

		// Handle,REF,Nombre,Categoria,Descripción,Vendido por peso,Opción 1 nombre,Opción 1 valor,Opción 2 nombre,Opción 2 valor,Opción 3 nombre,Opción 3 valor,Coste,Codigo de barras,...
		// Nombre is at index 2
		// Barcode is at index 13
		if len(record) < 14 {
			continue
		}

		name := record[2]
		barcode := record[13]

		if barcode == "" {
			continue
		}

		err = upsertProduct(db, barcode, name)
		if err != nil {
			return count, err
		}
		count++
	}

	return count, nil
}
