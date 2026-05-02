package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/alcandev/korva/internal/admin"
)

// TestLoreExport_RequiresSession verifies that GET /team/lore/export returns 401
// when no session token is provided.
func TestLoreExport_RequiresSession(t *testing.T) {
	env := newTeamTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/team/lore/export", nil)
	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestLoreExport_MemberCanExport verifies that a valid session token allows
// exporting scrolls scoped to the authenticated team.
func TestLoreExport_MemberCanExport(t *testing.T) {
	env := newTeamTestEnv(t)

	// Seed a private scroll for the team.
	db := env.store.DB()
	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO private_scrolls(id, team_id, name, content, created_by, updated_at)
		VALUES(?,?,?,?,?,?)`,
		newID(), env.teamID, "auth-guide", "Use JWT with RS256", "admin@corp.com", now)

	req := httptest.NewRequest(http.MethodGet, "/team/lore/export", nil)
	req.Header.Set("X-Session-Token", env.memberToken)
	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	notes, ok := resp["notes"].([]any)
	if !ok {
		t.Fatalf("expected notes array, got %T", resp["notes"])
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}

	// Verify the team_id in the response is the authenticated team's ID, not a
	// user-controlled value.
	if resp["team_id"] != env.teamID {
		t.Errorf("team_id mismatch: got %v, want %v", resp["team_id"], env.teamID)
	}
}

// TestLoreExport_CannotEnumerateOtherTeam verifies that the team_id in the response
// is always the authenticated team, even if a different team_id is passed as a query param.
func TestLoreExport_CannotEnumerateOtherTeam(t *testing.T) {
	env := newTeamTestEnv(t)

	// Create a second team and seed a scroll for it.
	db := env.store.DB()
	now := time.Now().UTC().Format(time.RFC3339)
	otherTeamID := "team-other-999"
	db.Exec(`INSERT INTO teams(id, name, owner, created_at) VALUES(?,?,?,?)`,
		otherTeamID, "Other Corp", "other@corp.com", now)
	db.Exec(`INSERT INTO private_scrolls(id, team_id, name, content, created_by, updated_at)
		VALUES(?,?,?,?,?,?)`,
		newID(), otherTeamID, "secret-scroll", "SECRET CONTENT", "other@corp.com", now)

	// Request team/lore/export with a team_id query param pointing at the other team.
	url := fmt.Sprintf("/team/lore/export?team_id=%s", otherTeamID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("X-Session-Token", env.memberToken)
	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck

	// The response must be scoped to the authenticated team, not the requested one.
	if resp["team_id"] == otherTeamID {
		t.Error("SECURITY: endpoint returned other team's data — team_id isolation failed")
	}

	notes := resp["notes"].([]any)
	if len(notes) != 0 {
		t.Errorf("expected 0 notes for authenticated team (no scrolls), got %d", len(notes))
	}
}

// TestLoreExport_Pagination verifies limit and total fields in the response.
func TestLoreExport_Pagination(t *testing.T) {
	env := newTeamTestEnv(t)

	db := env.store.DB()
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range 5 {
		db.Exec(`INSERT INTO private_scrolls(id, team_id, name, content, created_by, updated_at)
			VALUES(?,?,?,?,?,?)`,
			newID(), env.teamID, fmt.Sprintf("scroll-%d", i), "content", "admin@corp.com", now)
	}

	req := httptest.NewRequest(http.MethodGet, "/team/lore/export?limit=2&offset=0", nil)
	req.Header.Set("X-Session-Token", env.memberToken)
	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck

	notes := resp["notes"].([]any)
	if len(notes) != 2 {
		t.Errorf("expected 2 notes (limit=2), got %d", len(notes))
	}

	total := int(resp["total"].(float64))
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}

	pagination := resp["pagination"].(map[string]any)
	if !pagination["has_more"].(bool) {
		t.Error("expected has_more=true for offset=0, limit=2, total=5")
	}
}

// TestLoreExport_SinceFilter verifies that the ?since= parameter filters by updated_at.
func TestLoreExport_SinceFilter(t *testing.T) {
	env := newTeamTestEnv(t)

	db := env.store.DB()
	old := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	recent := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)

	db.Exec(`INSERT INTO private_scrolls(id, team_id, name, content, created_by, updated_at)
		VALUES(?,?,?,?,?,?)`, newID(), env.teamID, "old-scroll", "old", "a@b.com", old)
	db.Exec(`INSERT INTO private_scrolls(id, team_id, name, content, created_by, updated_at)
		VALUES(?,?,?,?,?,?)`, newID(), env.teamID, "new-scroll", "new", "a@b.com", recent)

	since := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/team/lore/export?since="+since, nil)
	req.Header.Set("X-Session-Token", env.memberToken)
	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck

	notes := resp["notes"].([]any)
	if len(notes) != 1 {
		t.Errorf("expected 1 note after since filter, got %d", len(notes))
	}
	if resp["incremental"] != true {
		t.Error("expected incremental=true when since is set")
	}
}

// TestAdminLoreExport_RequiresAdminKey verifies that GET /admin/lore/export
// returns 401 without an X-Admin-Key header.
func TestAdminLoreExport_RequiresAdminKey(t *testing.T) {
	env := newTeamTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/lore/export?team_id="+env.teamID, nil)
	w := httptest.NewRecorder()
	env.handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestAdminLoreExport_AllTeams verifies that the admin endpoint returns scrolls
// across all teams when no team_id filter is provided.
func TestAdminLoreExport_AllTeams(t *testing.T) {
	env := newTeamTestEnv(t)

	keyPath := filepath.Join(t.TempDir(), "admin.key")
	cfg, err := admin.Generate(keyPath, "test@korva.dev", false)
	if err != nil {
		t.Fatalf("admin.Generate: %v", err)
	}
	h := Router(context.Background(), env.store, RouterConfig{AdminKeyPath: keyPath})

	db := env.store.DB()
	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec(`INSERT INTO private_scrolls(id, team_id, name, content, created_by, updated_at)
		VALUES(?,?,?,?,?,?)`, newID(), env.teamID, "scroll-a", "content-a", "a@b.com", now)

	req := httptest.NewRequest(http.MethodGet, "/admin/lore/export", nil)
	req.Header.Set("X-Admin-Key", cfg.Key)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck

	notes := resp["notes"].([]any)
	if len(notes) == 0 {
		t.Error("expected at least 1 note for admin bulk export")
	}
}
