package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sessionState int

const (
	stateSessionList sessionState = iota
	stateSessionCreate
	stateScanning
	stateHistory
)

var (
	// Colores de la paleta
	purple = lipgloss.Color("#7D56F4")
	gray   = lipgloss.Color("#626262")
	white  = lipgloss.Color("#FAFAFA")
	blue   = lipgloss.Color("#01BEFE")
	green  = lipgloss.Color("#04B575")
	red    = lipgloss.Color("#FF0000")

	// Estilos de contenedores
	windowStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(white).
			Background(purple).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			MarginBottom(1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(gray)

	itemStyle = lipgloss.NewStyle().
			Foreground(purple).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(gray)

	csvLoadedStyle = lipgloss.NewStyle().
			Foreground(green).
			Italic(true)

	csvErrorStyle = lipgloss.NewStyle().
			Foreground(red).
			Italic(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(blue).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(gray).
			Italic(true).
			MarginTop(1).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(gray).
			PaddingTop(1)
)

type model struct {
	state          sessionState
	db             *sql.DB
	textInput      textinput.Model
	sessionInput   textinput.Model
	activeSession  *InventorySession
	lastScanned    *InventoryRecord
	statusMsg      string
	csvStatus      string
	csvIsError     bool
	err            error
	sessions       []InventorySession
	history        []InventoryRecord
	cursor         int
	width          int
	height         int
}

func formatTime(sqliteTime string) string {
	cleanTime := strings.Replace(sqliteTime, "T", " ", 1)
	cleanTime = strings.Replace(cleanTime, "Z", "", 1)
	if len(cleanTime) > 19 {
		cleanTime = cleanTime[:19]
	}

	layout := "2006-01-02 15:04:05"
	t, err := time.Parse(layout, cleanTime)
	if err != nil {
		return sqliteTime
	}

	t = t.Local()
	now := time.Now()

	meses := map[string]string{
		"Jan": "Ene", "Feb": "Feb", "Mar": "Mar", "Apr": "Abr",
		"May": "May", "Jun": "Jun", "Jul": "Jul", "Aug": "Ago",
		"Sep": "Sep", "Oct": "Oct", "Nov": "Nov", "Dec": "Dic",
	}

	if now.Format("2006-01-02") == t.Format("2006-01-02") {
		return fmt.Sprintf("Hoy  |  %s", t.Format("15:04"))
	}
	if now.AddDate(0, 0, -1).Format("2006-01-02") == t.Format("2006-01-02") {
		return fmt.Sprintf("Ayer |  %s", t.Format("15:04"))
	}

	mesEsp := meses[t.Format("Jan")]
	return fmt.Sprintf("%02d de %s  |  %s", t.Day(), mesEsp, t.Format("15:04"))
}

func initialModel(db *sql.DB) model {
	ti := textinput.New()
	ti.Placeholder = "Escanea el código de barras..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 30

	si := textinput.New()
	si.Placeholder = "Nombre de la sesión (ej. Almacén 1)..."
	si.CharLimit = 50
	si.Width = 40

	sessions, _ := getSessions(db)

	return model{
		state:        stateSessionList,
		db:           db,
		textInput:    ti,
		sessionInput: si,
		sessions:     sessions,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEsc:
			if m.state == stateScanning || m.state == stateHistory {
				m.state = stateSessionList
				m.sessions, _ = getSessions(m.db)
				m.cursor = 0
				return m, nil
			}
			if m.state == stateSessionCreate {
				m.state = stateSessionList
				return m, nil
			}
		}

		switch m.state {
		case stateSessionList:
			switch msg.String() {
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.sessions)-1 {
					m.cursor++
				}
			case "enter":
				if len(m.sessions) > 0 {
					m.activeSession = &m.sessions[m.cursor]
					m.state = stateScanning
					m.textInput.Focus()
					m.lastScanned = nil
					m.statusMsg = ""
				}
			case "e":
				if len(m.sessions) > 0 {
					s := m.sessions[m.cursor]
					fileName, err := exportSessionToCSV(m.db, s.ID, s.Name)
					if err != nil {
						m.err = err
					} else {
						m.statusMsg = fmt.Sprintf("Exportado: %s", fileName)
					}
				}
			case "n":
				m.state = stateSessionCreate
				m.sessionInput.Focus()
				m.sessionInput.SetValue("")
			case "d":
				if len(m.sessions) > 0 {
					deleteSession(m.db, m.sessions[m.cursor].ID)
					m.sessions, _ = getSessions(m.db)
					if m.cursor >= len(m.sessions) && m.cursor > 0 {
						m.cursor--
					}
				}
			case "q":
				return m, tea.Quit
			}

		case stateSessionCreate:
			switch msg.Type {
			case tea.KeyEnter:
				name := m.sessionInput.Value()
				if name != "" {
					id, err := createSession(m.db, name)
					if err == nil {
						m.sessions, _ = getSessions(m.db)
						for i, s := range m.sessions {
							if s.ID == id {
								m.activeSession = &m.sessions[i]
								break
							}
						}
						m.state = stateScanning
						m.textInput.Focus()
					}
				}
				return m, nil
			}
			m.sessionInput, cmd = m.sessionInput.Update(msg)
			return m, cmd

		case stateScanning:
			switch msg.Type {
			case tea.KeyTab:
				m.state = stateHistory
				m.history, _ = getHistoryInSession(m.db, m.activeSession.ID)
				m.cursor = 0
				return m, nil
			case tea.KeyEnter:
				barcode := m.textInput.Value()
				if barcode == "" {
					return m, nil
				}

				// Limpieza de duplicados: Si el código tiene longitud par y la primera mitad
				// es idéntica a la segunda, probablemente el escáner lo envió dos veces.
				if len(barcode) > 0 && len(barcode)%2 == 0 {
					half := len(barcode) / 2
					if barcode[:half] == barcode[half:] {
						barcode = barcode[:half]
					}
				}

				p, err := getProductByBarcode(m.db, barcode)
				if err != nil {
					m.err = err
					return m, nil
				}

				if p == nil {
					m.statusMsg = fmt.Sprintf("Producto no encontrado: %s", barcode)
					m.lastScanned = nil
				} else {
					err = incrementCountInSession(m.db, m.activeSession.ID, barcode)
					if err != nil {
						m.err = err
						return m, nil
					}
					rec, _ := getRecordInSession(m.db, m.activeSession.ID, barcode)
					m.lastScanned = rec
					m.statusMsg = fmt.Sprintf("¡Escaneado: %s!", p.Name)
				}
				m.textInput.SetValue("")
				return m, nil
			}
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd

		case stateHistory:
			switch msg.String() {
			case "tab":
				m.state = stateScanning
				m.textInput.Focus()
			case "e":
				fileName, err := exportSessionToCSV(m.db, m.activeSession.ID, m.activeSession.Name)
				if err != nil {
					m.err = err
				} else {
					m.statusMsg = fmt.Sprintf("Exportado: %s", fileName)
				}
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.history)-1 {
					m.cursor++
				}
			case "d", "backspace":
				if len(m.history) > 0 {
					deleteHistoryRecordInSession(m.db, m.history[m.cursor].ID)
					m.history, _ = getHistoryInSession(m.db, m.activeSession.ID)
					if m.cursor >= len(m.history) && m.cursor > 0 {
						m.cursor--
					}
				}
			}
		}

	case error:
		m.err = msg
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	var body string
	var header string
	var footer string

	// HEADER
	csvStatus := m.csvStatus
	if m.csvIsError {
		csvStatus = csvErrorStyle.Render(fmt.Sprintf("[%s]", csvStatus))
	} else {
		csvStatus = csvLoadedStyle.Render(fmt.Sprintf("[%s]", csvStatus))
	}
	header = headerStyle.Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			titleStyle.Render("Inventario TUI"),
			" ",
			csvStatus,
		),
	)

	// BODY & FOOTER
	switch m.state {
	case stateSessionList:
		body = selectedStyle.Render("MENÚ DE SESIONES") + "\n\n"
		if m.statusMsg != "" {
			body += statusStyle.Render(m.statusMsg) + "\n\n"
		}
		body += "Selecciona o crea una sesión de conteo:\n\n"
		if len(m.sessions) == 0 {
			body += " No hay sesiones. Presiona 'N' para crear una.\n\n"
		} else {
			for i, ses := range m.sessions {
				cursor := " "
				if m.cursor == i {
					cursor = ">"
					body += selectedStyle.Render(fmt.Sprintf("%s %s ( %s )", cursor, ses.Name, formatTime(ses.CreatedAt))) + "\n"
				} else {
					body += fmt.Sprintf("%s %s ( %s )", cursor, ses.Name, formatTime(ses.CreatedAt)) + "\n"
				}
			}
			body += "\n"
		}
		footer = helpStyle.Render("N: Nueva • Enter: Entrar • E: Exportar • D: Borrar • Q: Salir")

	case stateSessionCreate:
		body = selectedStyle.Render("NUEVA SESIÓN") + "\n\n"
		body += "Introduce el nombre para la nueva sesión:\n\n"
		body += m.sessionInput.View() + "\n\n"
		footer = helpStyle.Render("Enter: Crear • Esc: Cancelar")

	case stateScanning:
		body = itemStyle.Render(fmt.Sprintf("SESIÓN ACTIVA: %s", m.activeSession.Name)) + "\n\n"
		body += m.textInput.View() + "\n\n"

		if m.lastScanned != nil {
			body += "Último producto escaneado:\n"
			body += itemStyle.Render(m.lastScanned.Name) + "\n"
			body += fmt.Sprintf("Código: %s | Cantidad: %d\n\n", m.lastScanned.Barcode, m.lastScanned.Quantity)
		}

		if m.statusMsg != "" {
			body += statusStyle.Render(m.statusMsg) + "\n\n"
		}
		footer = helpStyle.Render("Tab: Historial • E: Exportar • Esc: Menú")

	case stateHistory:
		body = itemStyle.Render(fmt.Sprintf("HISTORIAL DE SESIÓN: %s", m.activeSession.Name)) + "\n\n"
		if m.statusMsg != "" {
			body += statusStyle.Render(m.statusMsg) + "\n\n"
		}
		if len(m.history) == 0 {
			body += " No hay escaneos en esta sesión.\n\n"
		} else {
			for i, r := range m.history {
				cursor := " "
				if m.cursor == i {
					cursor = ">"
					body += selectedStyle.Render(fmt.Sprintf("%s %s: %d (Bar: %s)", cursor, r.Name, r.Quantity, r.Barcode)) + "\n"
				} else {
					body += fmt.Sprintf("%s %s: %d (Bar: %s)", cursor, r.Name, r.Quantity, r.Barcode) + "\n"
				}
			}
			body += "\n"
		}
		footer = helpStyle.Render("Tab: Escaneo • E: Exportar • D: Borrar • Esc: Menú")
	}

	if m.err != nil {
		body += "\n" + csvErrorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	// COMPOSITE VIEW
	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		body,
		footer,
	)

	return windowStyle.Copy().
		Width(m.width - 6).
		Height(m.height - 3).
		Render(content)
}

func main() {
	dbPath := "inventory.db"
	db, err := initDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	m := initialModel(db)

	// CSV Dynamic Loading
	csvFile, err := findCSVInRoot()
	if err != nil {
		m.csvStatus = err.Error()
		m.csvIsError = true
	} else {
		count, err := importCSV(db, csvFile)
		if err != nil {
			m.csvStatus = fmt.Sprintf("Error CSV: %v", err)
			m.csvIsError = true
		} else {
			m.csvStatus = fmt.Sprintf("CSV cargado: %s (%d productos)", csvFile, count)
			m.csvIsError = false
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
