package tui

import (
	"fmt"
	"strings"

	"github.com/alcandev/korva/vault/internal/store"
)

func (m Model) renderSessions(height int) string {
	var lines []string

	if len(m.sessions) == 0 {
		return styleBox.Width(m.width - 4).Height(height).
			Render(styleDim.Render("  No sessions yet. Use vault_session_start to begin a session."))
	}

	lines = append(lines,
		styleCardLabel.Render(fmt.Sprintf("  %d session(s)  [↑↓] navigate", len(m.sessions))),
		"",
	)

	// Column headers
	lines = append(lines,
		styleDim.Render(fmt.Sprintf("  %-12s  %-16s  %-10s  %-8s  %s",
			"started", "project", "agent", "status", "goal")),
		styleDim.Render("  "+strings.Repeat("─", m.width-12)),
	)

	maxVisible := height - 7
	start := 0
	if m.sessionsCursor >= maxVisible {
		start = m.sessionsCursor - maxVisible + 1
	}

	for i := start; i < len(m.sessions) && i < start+maxVisible; i++ {
		sess := m.sessions[i]
		row := renderSessionRow(sess, i == m.sessionsCursor, m.width-8)
		lines = append(lines, row)
	}

	// Detail for selected session
	if m.sessionsCursor < len(m.sessions) {
		lines = append(lines, "", renderSessionDetail(m.sessions[m.sessionsCursor], m.width-8))
	}

	content := strings.Join(lines, "\n")
	return styleBox.Width(m.width - 4).Height(height).Render(content)
}

func renderSessionRow(sess store.Session, selected bool, width int) string {
	started := sess.StartedAt.Format("Jan 02 15:04")

	status := styleDim.Render("open")
	if sess.EndedAt != nil {
		status = styleStatusOnline.Render("done")
	}

	goal := sess.Goal
	maxGoal := width - 55
	if maxGoal < 10 {
		maxGoal = 10
	}
	if len(goal) > maxGoal {
		goal = goal[:maxGoal-1] + "…"
	}

	project := sess.Project
	if len(project) > 14 {
		project = project[:13] + "…"
	}

	agent := sess.Agent
	if len(agent) > 8 {
		agent = agent[:7] + "…"
	}

	row := fmt.Sprintf("  %-12s  %-16s  %-10s  %-8s  %s",
		started, project, agent, status, goal)

	if selected {
		return styleSelected.Width(width).Render(row)
	}
	return styleNormal.Width(width).Render(row)
}

func renderSessionDetail(sess store.Session, width int) string {
	detail := styleBox.Width(width)

	var lines []string
	lines = append(lines, styleTitle.Render("Session: "+sess.ID[:8]+"…"))
	lines = append(lines, styleDim.Render("project: ")+sess.Project)
	if sess.Team != "" {
		lines = append(lines, styleDim.Render("team: ")+sess.Team)
	}
	lines = append(lines, styleDim.Render("agent: ")+sess.Agent)
	lines = append(lines, styleDim.Render("goal: ")+sess.Goal)

	if sess.Summary != "" {
		summary := sess.Summary
		if len(summary) > 200 {
			summary = summary[:197] + "…"
		}
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("summary: ")+summary)
	}

	return detail.Render(strings.Join(lines, "\n"))
}
