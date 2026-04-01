// Package logging proporciona logging estructurado a archivo para la TUI.
// La TUI secuestra stdout/stderr, por lo que los logs van a tui_logs/.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// SetupFileLogger configura slog para escribir a un archivo en tui_logs/.
// Retorna la función de cleanup que debe llamarse al cerrar la app.
func SetupFileLogger() (cleanup func(), err error) {
	logDir := "tui_logs"
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	logFile := filepath.Join(logDir, "inventory-tui.log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))

	slog.Info("logger inicializado", "log_file", logFile)

	cleanup = func() {
		_ = f.Close()
	}

	return cleanup, nil
}

// With retorna un logger hijo con atributos adicionales.
func With(attrs ...any) *slog.Logger {
	return slog.With(attrs...)
}

// DiscardLogger retorna un io.Discard y un logger que no escribe a ningún lado.
// Útil para tests o cuando se quiere silenciar el logging.
func DiscardLogger() (*slog.Logger, io.Writer) {
	return slog.New(slog.NewTextHandler(io.Discard, nil)), io.Discard
}
