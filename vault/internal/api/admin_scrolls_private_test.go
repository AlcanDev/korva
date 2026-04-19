package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alcandev/korva/vault/internal/store"
)

func newTestStoreScrolls(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:", nil)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func scrollsAdminReq(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyBytes = b
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// --- List ---

func TestAdminListPrivateScrolls_Empty(t *testing.T) {
	s := newTestStoreScrolls(t)
	h := adminListPrivateScrolls(s)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/admin/scrolls/private", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if int(resp["count"].(float64)) != 0 {
		t.Errorf("want count=0, got %v", resp["count"])
	}
}

func TestAdminListPrivateScrolls_WithScrolls(t *testing.T) {
	s := newTestStoreScrolls(t)

	// Seed two scrolls directly.
	s.DB().Exec(`INSERT INTO private_scrolls(id, name, content) VALUES ('s1','Alpha','alpha content')`)
	s.DB().Exec(`INSERT INTO private_scrolls(id, name, content) VALUES ('s2','Beta','beta content')`)

	h := adminListPrivateScrolls(s)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/admin/scrolls/private", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp struct {
		Scrolls []privateScrollRow `json:"scrolls"`
		Count   int                `json:"count"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Count != 2 {
		t.Errorf("want count=2, got %d", resp.Count)
	}
	// Should be sorted by name ASC: Alpha first, then Beta.
	if resp.Scrolls[0].Name != "Alpha" {
		t.Errorf("want first name Alpha, got %q", resp.Scrolls[0].Name)
	}
}

// --- Save (create) ---

func TestAdminSavePrivateScroll_Create(t *testing.T) {
	s := newTestStoreScrolls(t)
	h := adminSavePrivateScroll(s, "admin")

	req := scrollsAdminReq(t, "POST", "/admin/scrolls/private",
		map[string]string{"name": "my-scroll", "content": "# Hello"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("response should contain id")
	}
	if resp["status"] != "created" {
		t.Errorf("want status=created, got %q", resp["status"])
	}

	// Verify it was persisted.
	var count int
	s.DB().QueryRow(`SELECT COUNT(*) FROM private_scrolls WHERE name='my-scroll'`).Scan(&count)
	if count != 1 {
		t.Errorf("scroll should be in DB, count=%d", count)
	}
}

// --- Save (update / upsert) ---

func TestAdminSavePrivateScroll_Update(t *testing.T) {
	s := newTestStoreScrolls(t)
	h := adminSavePrivateScroll(s, "admin")

	// Create first.
	req1 := scrollsAdminReq(t, "POST", "/admin/scrolls/private",
		map[string]string{"name": "upsert-scroll", "content": "v1"})
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d", w1.Code)
	}

	// Save again with same name — should update.
	req2 := scrollsAdminReq(t, "POST", "/admin/scrolls/private",
		map[string]string{"name": "upsert-scroll", "content": "v2 updated"})
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d — body: %s", w2.Code, w2.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w2.Body).Decode(&resp)
	if resp["status"] != "updated" {
		t.Errorf("want status=updated, got %q", resp["status"])
	}

	// Verify content was updated and there's only one row.
	var content string
	var rowCount int
	s.DB().QueryRow(`SELECT content FROM private_scrolls WHERE name='upsert-scroll'`).Scan(&content)
	s.DB().QueryRow(`SELECT COUNT(*) FROM private_scrolls WHERE name='upsert-scroll'`).Scan(&rowCount)
	if content != "v2 updated" {
		t.Errorf("content should be updated, got: %q", content)
	}
	if rowCount != 1 {
		t.Errorf("should have exactly 1 row, got %d", rowCount)
	}
}

func TestAdminSavePrivateScroll_MissingName(t *testing.T) {
	s := newTestStoreScrolls(t)
	h := adminSavePrivateScroll(s, "admin")

	req := scrollsAdminReq(t, "POST", "/admin/scrolls/private",
		map[string]string{"content": "no name here"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

// --- Delete ---

func TestAdminDeletePrivateScroll_Success(t *testing.T) {
	s := newTestStoreScrolls(t)
	s.DB().Exec(`INSERT INTO private_scrolls(id, name, content) VALUES ('del-id','to-delete','content')`)

	h := adminDeletePrivateScroll(s, "admin")
	req := httptest.NewRequest("DELETE", "/admin/scrolls/private/del-id", nil)
	req.SetPathValue("scroll_id", "del-id")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var count int
	s.DB().QueryRow(`SELECT COUNT(*) FROM private_scrolls WHERE id='del-id'`).Scan(&count)
	if count != 0 {
		t.Error("scroll should have been deleted from DB")
	}
}

func TestAdminDeletePrivateScroll_NotFound(t *testing.T) {
	s := newTestStoreScrolls(t)
	h := adminDeletePrivateScroll(s, "admin")

	req := httptest.NewRequest("DELETE", "/admin/scrolls/private/nonexistent", nil)
	req.SetPathValue("scroll_id", "nonexistent")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}
