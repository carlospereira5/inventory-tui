package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"inventory-tui/internal/application/service"
	"inventory-tui/internal/infrastructure/loyverse"
	"inventory-tui/internal/ui/tui/styles"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SyncState representa el estado de la pantalla de sincronización.
type SyncState int

const (
	SyncIdle SyncState = iota
	SyncSyncing
	SyncCompleted
	SyncError
)

// SyncCompletedMsg indica que la sincronización terminó exitosamente.
type SyncCompletedMsg struct{ Result *loyverse.SyncResult }

// SyncErrorMsg indica que la sincronización falló.
type SyncErrorMsg struct{ Err error }

// SyncModel contiene el estado de la pantalla de sincronización con Loyverse.
type SyncModel struct {
	Service  *service.InventoryService
	State    SyncState
	Spinner  spinner.Model
	Progress progress.Model
	Result   *loyverse.SyncResult
	Err      error
	Width    int
	Help     string
}

// NewSyncModel crea un nuevo modelo de sincronización.
func NewSyncModel(svc *service.InventoryService) SyncModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	p := progress.New(progress.WithDefaultGradient())
	p.Width = 40

	return SyncModel{
		Service:  svc,
		State:    SyncIdle,
		Spinner:  s,
		Progress: p,
		Help:     "enter: Iniciar Sync | esc: Volver",
	}
}

// Update procesa mensajes y actualiza el estado del sync.
func (m SyncModel) Update(msg tea.Msg) (SyncModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case SyncCompletedMsg:
		m.State = SyncCompleted
		m.Result = msg.Result
		m.Help = "esc: Volver"
		return m, nil

	case SyncErrorMsg:
		m.State = SyncError
		m.Err = msg.Err
		m.Help = "enter: Reintentar | esc: Volver"
		return m, nil

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Progress.Width = msg.Width - 10
		return m, nil
	}

	return m, nil
}

// View renderiza la pantalla de sincronización.
func (m SyncModel) View() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Sincronización con Loyverse"))
	b.WriteString("\n\n")

	switch m.State {
	case SyncIdle:
		b.WriteString(styles.Gray.Render("Presiona Enter para iniciar la sincronización."))
		b.WriteString("\n\n")
		b.WriteString(styles.Gray.Render("Se obtendrá el catálogo de Loyverse, se calculará el stock local y se enviarán las actualizaciones."))

	case SyncSyncing:
		b.WriteString(m.Spinner.View() + " Sincronizando con Loyverse...")
		b.WriteString("\n\n")
		b.WriteString(m.Progress.ViewAs(0.5))

	case SyncCompleted:
		b.WriteString(m.viewCompleted())

	case SyncError:
		b.WriteString(styles.ErrorStyle.Render("✗ Error en la sincronización"))
		b.WriteString("\n\n")
		b.WriteString(styles.ErrorStyle.Render(m.Err.Error()))
		b.WriteString("\n\n")
		b.WriteString(styles.Gray.Render("Presiona Enter para reintentar o Esc para volver."))
	}

	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render(m.Help))

	return b.String()
}

func (m SyncModel) viewCompleted() string {
	var b strings.Builder
	b.WriteString(styles.SuccessStyle.Render("✓ Sincronización completada"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Total: %d productos", m.Result.Total))
	b.WriteString("\n")
	b.WriteString(styles.SuccessStyle.Render(fmt.Sprintf("Exitosos: %d", m.Result.Success)))
	if m.Result.Failed > 0 {
		b.WriteString("\n")
		b.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("Fallidos: %d", m.Result.Failed)))
	}
	if len(m.Result.Errors) > 0 {
		b.WriteString("\n\nErrores:\n")
		for _, e := range m.Result.Errors {
			b.WriteString(styles.ErrorStyle.Render(fmt.Sprintf("  • %s: %s", e.ProductName, e.Error)))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(styles.Gray.Render("Presiona Esc para volver."))
	return b.String()
}

// CmdSync ejecuta la sincronización con Loyverse en background.
func (m SyncModel) CmdSync() tea.Cmd {
	return func() tea.Msg {
		slog.Info("sync: comando CmdSync iniciado")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result, err := m.Service.SyncWithLoyverse(ctx)
		if err != nil {
			slog.Error("sync: CmdSync fallido", "err", err)
			return SyncErrorMsg{Err: err}
		}
		slog.Info("sync: CmdSync completado",
			"total", result.Total,
			"success", result.Success,
			"failed", result.Failed,
		)
		return SyncCompletedMsg{Result: result}
	}
}

// CmdSyncTick genera ticks periódicos para animar el spinner y progress bar.
func CmdSyncTick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return spinner.TickMsg{Time: t}
	})
}
