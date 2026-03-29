package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Colores corporativos.
	purpleColor = lipgloss.Color("#7D56F4")
	grayColor   = lipgloss.Color("#626262")
	whiteColor  = lipgloss.Color("#FAFAFA")
	blueColor   = lipgloss.Color("#01BEFE")
	greenColor  = lipgloss.Color("#04B575")
	redColor    = lipgloss.Color("#FF0000")

	// Estilos basados en colores.
	Purple = lipgloss.NewStyle().Foreground(purpleColor)
	Gray   = lipgloss.NewStyle().Foreground(grayColor)
	Blue   = lipgloss.NewStyle().Foreground(blueColor)
	Green  = lipgloss.NewStyle().Foreground(greenColor)
	Red    = lipgloss.NewStyle().Foreground(redColor)

	// Estilo de la ventana principal.
	WindowStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(purpleColor).
			Padding(1, 2)

	// Estilo para títulos y cabeceras.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(whiteColor).
			Background(purpleColor).
			Padding(0, 1)

	// Estilo para la línea de separación.
	HeaderStyle = lipgloss.NewStyle().
			MarginBottom(1).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(grayColor)

	// Estilo para elementos seleccionados o destacados.
	SelectedStyle = lipgloss.NewStyle().
			Foreground(blueColor).
			Bold(true)

	// Estilo para mensajes de ayuda en el pie de página.
	HelpStyle = lipgloss.NewStyle().
			Foreground(grayColor).
			Italic(true).
			MarginTop(1).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(grayColor).
			PaddingTop(1)

	// Estilo para errores y éxito.
	ErrorStyle   = lipgloss.NewStyle().Foreground(redColor).Italic(true)
	SuccessStyle = lipgloss.NewStyle().Foreground(greenColor).Italic(true)
)
