package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/alcandev/korva/vault/internal/store"
)

func (m Model) renderExplorer(height int) string {
	var lines []string

	// Search bar
	searchBar := lipgloss.JoinHorizontal(lipgloss.Center,
		styleDim.Render("Search: "),
		m.searchInput.View(),
		styleDim.Render("  [enter] search  [esc] clear"),
	)
	lines = append(lines, searchBar, "")

	if !m.searchExecuted {
		lines = append(lines,
			styleDim.Render("  Type a query and press Enter to search the vault."),
			styleDim.Render("  Supports full-text search across titles and content."),
		)
	} else if len(m.searchResults) == 0 {
		lines = append(lines, styleDim.Render("  No results found."))
	} else {
		// Result count
		lines = append(lines,
			styleCardLabel.Render(fmt.Sprintf("  %d result(s)", len(m.searchResults))),
			"",
		)

		// Result list
		maxVisible := height - 6
		start := 0
		if m.searchCursor >= maxVisible {
			start = m.searchCursor - maxVisible + 1
		}

		for i := start; i < len(m.searchResults) && i < start+maxVisible; i++ {
			obs := m.searchResults[i]
			line := renderObservationRow(obs, i == m.searchCursor, m.width-8)
			lines = append(lines, line)
		}

		// Detail panel for selected observation
		if m.searchCursor < len(m.searchResults) {
			lines = append(lines, "", renderObservationDetail(m.searchResults[m.searchCursor], m.width-8))
		}
	}

	content := strings.Join(lines, "\n")
	return styleBox.Width(m.width - 4).Height(height).Render(content)
}

func renderObservationRow(obs store.Observation, selected bool, width int) string {
	badge := styleBadge.Render(string(obs.Type))
	title := obs.Title
	maxTitle := width - lipgloss.Width(badge) - 20
	if len(title) > maxTitle {
		title = title[:maxTitle-1] + "…"
	}

	age := formatAge(obs.CreatedAt)
	row := fmt.Sprintf("%s %s %s", badge, title, styleDim.Render(age))

	if selected {
		return styleSelected.Width(width).Render(row)
	}
	return styleNormal.Width(width).Render(row)
}

func renderObservationDetail(obs store.Observation, width int) string {
	detail := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Width(width)

	content := obs.Content
	if len(content) > 300 {
		content = content[:297] + "…"
	}

	var parts []string
	parts = append(parts, styleTitle.Render(obs.Title))
	if obs.Project != "" {
		parts = append(parts, styleDim.Render("project: ")+obs.Project)
	}
	if len(obs.Tags) > 0 {
		parts = append(parts, styleDim.Render("tags: ")+strings.Join(obs.Tags, ", "))
	}
	parts = append(parts, "")
	parts = append(parts, content)

	return detail.Render(strings.Join(parts, "\n"))
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
