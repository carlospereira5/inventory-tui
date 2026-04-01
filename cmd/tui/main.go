package main

import (
	"fmt"
	"inventory-tui/internal/application/service"
	"inventory-tui/internal/infrastructure/database"
	"inventory-tui/internal/infrastructure/loyverse"
	"inventory-tui/internal/infrastructure/storage"
	"inventory-tui/internal/ui/tui"
	"log"
	"net/http"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	db, err := database.NewSQLiteDB("inventory.db")
	if err != nil {
		log.Fatalf("No se pudo iniciar la base de datos: %v", err)
	}
	defer db.Close()

	productRepo := database.NewSQLiteProductRepository(db.Conn)
	sessionRepo := database.NewSQLiteSessionRepository(db.Conn)
	invRepo := database.NewSQLiteInventoryRepository(db.Conn)
	groupRepo := database.NewSQLiteCustomGroupRepository(db.Conn)
	csvStore := storage.NewCSVStorage(productRepo, groupRepo)

	svc := service.NewInventoryService(db.Conn, productRepo, sessionRepo, invRepo, groupRepo, csvStore)

	// Webhook de Loyverse.
	secret := os.Getenv("LOYVERSE_SECRET")
	webhook := loyverse.NewLoyverseWebhook(invRepo, secret)
	webhook.SetScanSale(svc.ScanLoyverseSale)
	svc.SetOnSessionActivate(webhook.SetTUIActiveSession)

	// Webhook HTTP server en background.
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		http.Handle("/loyverse/webhook", webhook)
		log.Printf("Webhook escuchando en :%s/loyverse/webhook", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Printf("webhook server error: %v", err)
		}
	}()

	m := tui.NewModel(svc)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error al ejecutar la TUI: %v\n", err)
		os.Exit(1)
	}
}
