package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent  = lipgloss.Color("#7C6AF7")
	colorMuted   = lipgloss.Color("#555577")
	colorSuccess = lipgloss.Color("#50FA7B")
	colorWarning = lipgloss.Color("#FFB86C") //nolint:unused // reserved for warning state UIs
	colorError   = lipgloss.Color("#FF5555")
	colorFg      = lipgloss.Color("#F8F8F2")
	colorBg      = lipgloss.Color("#1A1A2E")
	colorBorder  = lipgloss.Color("#333355")

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			PaddingRight(2)

	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorAccent).
			PaddingLeft(2).
			PaddingRight(2)

	styleTabInactive = lipgloss.NewStyle().
				Foreground(colorMuted).
				PaddingLeft(2).
				PaddingRight(2)

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	styleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 2).
			Width(20)

	styleCardLabel = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	styleCardValue = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorFg).
			Foreground(colorAccent)

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorFg).
			Background(colorBg).
			PaddingLeft(1).
			PaddingRight(1)

	styleStatusOnline = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)

	styleStatusOffline = lipgloss.NewStyle().
				Foreground(colorError).
				Bold(true)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	styleSelected = lipgloss.NewStyle().
			Foreground(colorBg).
			Background(colorAccent).
			PaddingLeft(1).
			PaddingRight(1)

	styleNormal = lipgloss.NewStyle().
			Foreground(colorFg).
			PaddingLeft(1).
			PaddingRight(1)

	styleDim = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleBadge = lipgloss.NewStyle().
			Foreground(colorBg).
			Background(colorMuted).
			PaddingLeft(1).
			PaddingRight(1).
			Bold(true)
)
