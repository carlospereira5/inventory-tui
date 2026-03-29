package main

import (
	"fmt"
	"inventory-tui/internal/application/service"
	"inventory-tui/internal/infrastructure/database"
	"inventory-tui/internal/infrastructure/storage"
	"inventory-tui/internal/ui/tui"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// 1. Inicializar la base de datos SQLite.
	db, err := database.NewSQLiteDB("inventory.db")
	if err != nil {
		log.Fatalf("No se pudo iniciar la base de datos: %v", err)
	}
	defer db.Close()

	// 2. Instanciar los repositorios (Infraestructura).
	productRepo := database.NewSQLiteProductRepository(db.Conn)
	sessionRepo := database.NewSQLiteSessionRepository(db.Conn)
	invRepo := database.NewSQLiteInventoryRepository(db.Conn)
	csvStore := storage.NewCSVStorage(productRepo)

	// 3. Crear el servicio de aplicación que orquesta la lógica.
	svc := service.NewInventoryService(db.Conn, productRepo, sessionRepo, invRepo, csvStore)

	// 4. Iniciar la interfaz de usuario con Bubble Tea.
	m := tui.NewModel(svc)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error al ejecutar la aplicación: %v", err)
		os.Exit(1)
	}
}
