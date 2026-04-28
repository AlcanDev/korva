package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderDashboard(height int) string {
	if m.stats == nil {
		return styleBox.Width(m.width - 4).Height(height).Render("No data yet.")
	}

	s := m.stats

	// Top row: 3 summary cards
	cardObs := styleCard.Render(
		styleCardLabel.Render("observations") + "\n" +
			styleCardValue.Render(fmt.Sprintf("%d", s.TotalObservations)),
	)
	cardSess := styleCard.Render(
		styleCardLabel.Render("sessions") + "\n" +
			styleCardValue.Render(fmt.Sprintf("%d", s.TotalSessions)),
	)
	cardPrompts := styleCard.Render(
		styleCardLabel.Render("prompts") + "\n" +
			styleCardValue.Render(fmt.Sprintf("%d", s.TotalPrompts)),
	)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, cardObs, "  ", cardSess, "  ", cardPrompts)

	// By type breakdown
	byType := renderBreakdown("By type", s.ByType, 30)

	// By project breakdown
	byProject := renderBreakdown("By project", s.ByProject, 30)

	// By team breakdown
	byTeam := renderBreakdown("By team", s.ByTeam, 30)

	breakdowns := lipgloss.JoinHorizontal(lipgloss.Top, byType, "  ", byProject, "  ", byTeam)

	content := lipgloss.JoinVertical(lipgloss.Left,
		topRow,
		"",
		breakdowns,
	)

	return styleBox.Width(m.width - 4).Height(height).Render(content)
}

func renderBreakdown(title string, data map[string]int, width int) string {
	header := styleTitle.Render(title)

	if len(data) == 0 {
		return lipgloss.NewStyle().Width(width).Render(header + "\n" + styleDim.Render("  (empty)"))
	}

	// Sort keys for stable output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return data[keys[i]] > data[keys[j]]
	})

	var rows []string
	for _, k := range keys {
		label := k
		if len(label) > 14 {
			label = label[:13] + "…"
		}
		bar := strings.Repeat("▪", min(data[k], 10))
		rows = append(rows, fmt.Sprintf("  %-15s %s %d", label, styleDim.Render(bar), data[k]))
	}

	return lipgloss.NewStyle().Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, rows...)...),
	)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
