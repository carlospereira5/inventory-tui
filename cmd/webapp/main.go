package main

import (
	"context"
	"inventory-tui/internal/application/service"
	"inventory-tui/internal/infrastructure/database"
	"inventory-tui/internal/infrastructure/loyverse"
	"inventory-tui/internal/infrastructure/storage"
	webui "inventory-tui/ui"
	"log"
	"net/http"
	"os"
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

	// Cargar catálogo CSV al iniciar (no hay TUI que lo haga en background).
	if count, file, err := svc.LoadCatalog(context.Background()); err != nil {
		log.Printf("Advertencia — catálogo CSV no cargado: %v", err)
	} else {
		log.Printf("Catálogo listo: %s (%d productos)", file, count)
	}

	// Webhook de Loyverse.
	secret := os.Getenv("LOYVERSE_SECRET")
	webhook := loyverse.NewLoyverseWebhook(invRepo, secret)

	// Web server.
	webServer := webui.NewServer(svc)
	webhook.SetOnSale(webServer.NotifySale)
	webhook.SetScanSale(svc.ScanLoyverseSale)
	webServer.SetTracker(webhook)

	// Webhook HTTP server en background.
	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		http.Handle("/loyverse/webhook", webhook)
		log.Printf("Webhook escuchando en :%s/loyverse/webhook", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("webhook server error: %v", err)
		}
	}()

	// Web server (blocking).
	webPort := os.Getenv("WEB_PORT")
	if webPort == "" {
		webPort = "9090"
	}
	log.Printf("Web server escuchando en :%s", webPort)
	if err := webServer.Run(":" + webPort); err != nil {
		log.Fatalf("web server error: %v", err)
	}
}
