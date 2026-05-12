package styles

import "github.com/charmbracelet/lipgloss"

// Paleta de colores.
var (
	PurpleColor = lipgloss.Color("#7D56F4")
	GrayColor   = lipgloss.Color("#626262")
	WhiteColor  = lipgloss.Color("#FAFAFA")
	BlueColor   = lipgloss.Color("#01BEFE")
	GreenColor  = lipgloss.Color("#04B575")
	RedColor    = lipgloss.Color("#FF5555") // suavizado: rojo coral en vez de rojo puro
	YellowColor = lipgloss.Color("#F5A623") // advertencias / pending delete
)

// Estilos de texto base.
var (
	Purple = lipgloss.NewStyle().Foreground(PurpleColor)
	Gray   = lipgloss.NewStyle().Foreground(GrayColor)
	Blue   = lipgloss.NewStyle().Foreground(BlueColor)
	Green  = lipgloss.NewStyle().Foreground(GreenColor)
	Red    = lipgloss.NewStyle().Foreground(RedColor)
	Yellow = lipgloss.NewStyle().Foreground(YellowColor)
)

// WindowStyle es el contenedor principal de la app — borde por defecto purple.
// Para borde dinámico por estado, usá WindowStyle.Copy().BorderForeground(color).
var WindowStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(PurpleColor).
	Padding(1, 2)

// TitleStyle — texto blanco sobre fondo purple, usado en títulos de sección.
var TitleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(WhiteColor).
	Background(PurpleColor).
	Padding(0, 1)

// HeaderStyle — línea separadora bajo el header.
var HeaderStyle = lipgloss.NewStyle().
	MarginBottom(1).
	Border(lipgloss.NormalBorder(), false, false, true, false).
	BorderForeground(GrayColor)

// SelectedStyle — ítem seleccionado en listas.
var SelectedStyle = lipgloss.NewStyle().
	Foreground(BlueColor).
	Bold(true)

// HelpStyle — pie de página con atajos de teclado.
var HelpStyle = lipgloss.NewStyle().
	Foreground(GrayColor).
	Italic(true).
	MarginTop(1).
	Border(lipgloss.NormalBorder(), true, false, false, false).
	BorderForeground(GrayColor).
	PaddingTop(1)

// WarnStyle — confirmaciones de borrado y advertencias.
var WarnStyle = lipgloss.NewStyle().
	Foreground(YellowColor).
	Bold(true)

// BreadcrumbSep — separador visual del breadcrumb.
var BreadcrumbSep = Gray.Render(" › ")

// Estilos de estado (éxito / error).
var (
	ErrorStyle   = lipgloss.NewStyle().Foreground(RedColor).Italic(true)
	SuccessStyle = lipgloss.NewStyle().Foreground(GreenColor).Italic(true)
)
