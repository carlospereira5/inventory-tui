package main

import (
	"fmt"
	"inventory-tui/internal/application/service"
	"inventory-tui/internal/infrastructure/database"
	"inventory-tui/internal/infrastructure/loyverse"
	"inventory-tui/internal/infrastructure/storage"
	"inventory-tui/internal/logging"
	"inventory-tui/internal/ui/tui"
	"log"
	"log/slog"
	"net/http"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Configurar logging a archivo antes que nada.
	cleanup, err := logging.SetupFileLogger()
	if err != nil {
		log.Fatalf("No se pudo iniciar el logger: %v", err)
	}
	defer cleanup()

	slog.Info("arrancando inventory-tui")

	db, err := database.NewSQLiteDB("inventory.db")
	if err != nil {
		slog.Error("fallo al iniciar base de datos", "err", err)
		log.Fatalf("No se pudo iniciar la base de datos: %v", err)
	}
	defer db.Close()

	productRepo := database.NewSQLiteProductRepository(db.Conn)
	sessionRepo := database.NewSQLiteSessionRepository(db.Conn)
	invRepo := database.NewSQLiteInventoryRepository(db.Conn)
	loyverseRepo := database.NewSQLiteLoyverseEventRepository(db.Conn)
	groupRepo := database.NewSQLiteCustomGroupRepository(db.Conn)
	csvStore := storage.NewCSVStorage(productRepo, groupRepo)

	svc := service.NewInventoryService(db.Conn, productRepo, sessionRepo, invRepo, loyverseRepo, groupRepo, csvStore)

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
		slog.Info("webhook escuchando", "port", port, "path", "/loyverse/webhook")
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			slog.Error("webhook server error", "err", err)
		}
	}()

	m := tui.NewModel(svc)
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Conectar el programa Bubble Tea al webhook para notificaciones.
	webhook.SetProgram(p)

	slog.Info("TUI iniciada")

	if _, err := p.Run(); err != nil {
		slog.Error("error al ejecutar la TUI", "err", err)
		fmt.Fprintf(os.Stderr, "Error al ejecutar la TUI: %v\n", err)
		os.Exit(1)
	}

	slog.Info("TUI cerrada correctamente")
}
