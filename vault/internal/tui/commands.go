package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcandev/korva/vault/internal/store"
)

// --- messages ---

type statsMsg struct {
	stats *store.VaultStats
	err   error
}

type searchResultsMsg struct {
	results []store.Observation
	err     error
}

type sessionsMsg struct {
	sessions []store.Session
	err      error
}

// --- commands ---

func loadStats(s *store.Store) tea.Cmd {
	return func() tea.Msg {
		stats, err := s.Stats()
		return statsMsg{stats: stats, err: err}
	}
}

func searchObservations(s *store.Store, query string) tea.Cmd {
	return func() tea.Msg {
		results, err := s.Search(query, store.SearchFilters{Limit: 50})
		return searchResultsMsg{results: results, err: err}
	}
}

func loadSessions(s *store.Store) tea.Cmd {
	return func() tea.Msg {
		sessions, err := s.ListSessions(20)
		return sessionsMsg{sessions: sessions, err: err}
	}
}
