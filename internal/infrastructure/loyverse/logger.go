package loyverse

import (
	"fmt"
	"os"
	"time"
)

// fileLogger escribe a un archivo de log para no interferir con la TUI (que secuestra stdout).
// Usar con: tail -f loyverse.log mientras corre la aplicación.
type fileLogger struct{ path string }

func (l fileLogger) log(format string, args ...any) {
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	prefix := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] "+format+"\n", append([]any{prefix}, args...)...)
}
