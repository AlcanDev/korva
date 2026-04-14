// Package tui provides the Bubbletea terminal UI for Korva Vault.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/alcandev/korva/vault/internal/store"
)

// tab represents a TUI tab.
type tab int

const (
	tabDashboard tab = iota
	tabExplorer
	tabSessions
)

var tabLabels = []string{"Dashboard", "Explorer", "Sessions"}

// Model is the root Bubbletea model.
type Model struct {
	store     *store.Store
	activeTab tab
	width     int
	height    int
	loading   bool
	err       error

	// Dashboard state
	stats *store.VaultStats

	// Explorer state
	searchInput    textinput.Model
	searchResults  []store.Observation
	searchCursor   int
	searchExecuted bool

	// Sessions state
	sessions       []store.Session
	sessionsCursor int

	// Shared
	spinner spinner.Model
}

// New creates a new TUI Model backed by the given store.
func New(s *store.Store) Model {
	ti := textinput.New()
	ti.Placeholder = "Search observations…"
	ti.CharLimit = 200
	ti.Width = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorAccent)

	return Model{
		store:       s,
		activeTab:   tabDashboard,
		searchInput: ti,
		spinner:     sp,
		loading:     true,
	}
}

// --- tea.Model interface ---

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadStats(m.store),
		m.spinner.Tick,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "1":
			m.activeTab = tabDashboard
			m.loading = true
			return m, loadStats(m.store)
		case "2":
			m.activeTab = tabExplorer
			m.searchInput.Focus()
		case "3":
			m.activeTab = tabSessions
			m.loading = true
			return m, loadSessions(m.store)
		case "tab":
			m.activeTab = (m.activeTab + 1) % tab(len(tabLabels))
			switch m.activeTab {
			case tabDashboard:
				m.loading = true
				return m, loadStats(m.store)
			case tabExplorer:
				m.searchInput.Focus()
			case tabSessions:
				m.loading = true
				return m, loadSessions(m.store)
			}
		case "enter":
			if m.activeTab == tabExplorer && m.searchInput.Focused() {
				query := strings.TrimSpace(m.searchInput.Value())
				if query != "" {
					m.loading = true
					m.searchExecuted = true
					return m, searchObservations(m.store, query)
				}
			}
		case "up", "k":
			if m.activeTab == tabExplorer && len(m.searchResults) > 0 {
				if m.searchCursor > 0 {
					m.searchCursor--
				}
			}
			if m.activeTab == tabSessions && len(m.sessions) > 0 {
				if m.sessionsCursor > 0 {
					m.sessionsCursor--
				}
			}
		case "down", "j":
			if m.activeTab == tabExplorer && len(m.searchResults) > 0 {
				if m.searchCursor < len(m.searchResults)-1 {
					m.searchCursor++
				}
			}
			if m.activeTab == tabSessions && len(m.sessions) > 0 {
				if m.sessionsCursor < len(m.sessions)-1 {
					m.sessionsCursor++
				}
			}
		case "esc":
			if m.activeTab == tabExplorer {
				m.searchInput.Blur()
				m.searchResults = nil
				m.searchExecuted = false
				m.searchInput.SetValue("")
			}
		}

	case statsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.stats = msg.stats
		}

	case searchResultsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.searchResults = msg.results
			m.searchCursor = 0
		}

	case sessionsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.sessions = msg.sessions
		}

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Forward keys to text input when explorer tab is active
	if m.activeTab == tabExplorer {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}

	header := m.renderHeader()
	tabs := m.renderTabs()
	body := m.renderBody()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)
}

// --- sub-renders ---

func (m Model) renderHeader() string {
	title := styleTitle.Render("◈ Korva Vault")
	status := styleStatusOnline.Render("● online")
	space := strings.Repeat(" ", max(0, m.width-lipgloss.Width(title)-lipgloss.Width(status)-4))
	return styleHeader.Width(m.width).Render(title + space + status)
}

func (m Model) renderTabs() string {
	var tabs []string
	for i, label := range tabLabels {
		numbered := fmt.Sprintf("[%d] %s", i+1, label)
		if tab(i) == m.activeTab {
			tabs = append(tabs, styleTabActive.Render(numbered))
		} else {
			tabs = append(tabs, styleTabInactive.Render(numbered))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

func (m Model) renderBody() string {
	contentHeight := m.height - 6 // header + tabs + footer
	if contentHeight < 1 {
		contentHeight = 1
	}

	if m.err != nil {
		return styleBox.Width(m.width - 4).Height(contentHeight).
			Render(styleStatusOffline.Render("Error: " + m.err.Error()))
	}

	if m.loading {
		return styleBox.Width(m.width - 4).Height(contentHeight).
			Render(m.spinner.View() + " Loading…")
	}

	switch m.activeTab {
	case tabDashboard:
		return m.renderDashboard(contentHeight)
	case tabExplorer:
		return m.renderExplorer(contentHeight)
	case tabSessions:
		return m.renderSessions(contentHeight)
	}
	return ""
}

func (m Model) renderFooter() string {
	hints := "q:quit  tab:next  1-3:view  ↑↓:navigate"
	if m.activeTab == tabExplorer {
		hints = "enter:search  esc:clear  ↑↓:navigate  q:quit"
	}
	return styleHelp.Width(m.width).Render(hints)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
