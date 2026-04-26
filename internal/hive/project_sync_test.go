package hive

import (
	"testing"

	_ "modernc.org/sqlite"
)

func TestIsProjectSyncEnabled_AbsentRow_DefaultsTrue(t *testing.T) {
	outbox := NewOutbox(newTestDB(t))
	enabled, err := outbox.IsProjectSyncEnabled("unknown-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enabled {
		t.Error("absent row should default to sync_enabled=true")
	}
}

func TestPauseAndResume_Project(t *testing.T) {
	outbox := NewOutbox(newTestDB(t))
	project := "home-api"

	// Pause
	if err := outbox.PauseProjectSync(project, "admin@test.com", "maintenance"); err != nil {
		t.Fatalf("PauseProjectSync: %v", err)
	}
	enabled, err := outbox.IsProjectSyncEnabled(project)
	if err != nil {
		t.Fatalf("IsProjectSyncEnabled after pause: %v", err)
	}
	if enabled {
		t.Error("expected sync_enabled=false after pause")
	}

	// Resume
	if err := outbox.ResumeProjectSync(project, "admin@test.com"); err != nil {
		t.Fatalf("ResumeProjectSync: %v", err)
	}
	enabled, err = outbox.IsProjectSyncEnabled(project)
	if err != nil {
		t.Fatalf("IsProjectSyncEnabled after resume: %v", err)
	}
	if !enabled {
		t.Error("expected sync_enabled=true after resume")
	}
}

func TestListProjectSyncControls_Empty(t *testing.T) {
	outbox := NewOutbox(newTestDB(t))
	controls, err := outbox.ListProjectSyncControls()
	if err != nil {
		t.Fatalf("ListProjectSyncControls: %v", err)
	}
	if len(controls) != 0 {
		t.Errorf("expected 0 controls, got %d", len(controls))
	}
}

func TestListProjectSyncControls_WithData(t *testing.T) {
	outbox := NewOutbox(newTestDB(t))

	outbox.PauseProjectSync("proj-a", "alice", "client data")
	outbox.ResumeProjectSync("proj-b", "bob")

	controls, err := outbox.ListProjectSyncControls()
	if err != nil {
		t.Fatalf("ListProjectSyncControls: %v", err)
	}
	if len(controls) != 2 {
		t.Fatalf("expected 2 controls, got %d", len(controls))
	}

	for _, c := range controls {
		switch c.Project {
		case "proj-a":
			if c.SyncEnabled {
				t.Error("proj-a should be paused")
			}
			if c.PausedBy != "alice" {
				t.Errorf("paused_by = %q, want %q", c.PausedBy, "alice")
			}
			if c.Reason != "client data" {
				t.Errorf("reason = %q, want %q", c.Reason, "client data")
			}
		case "proj-b":
			if !c.SyncEnabled {
				t.Error("proj-b should be enabled after resume")
			}
		default:
			t.Errorf("unexpected project %q", c.Project)
		}
	}
}

func TestEnqueueForProject_PausedSkips(t *testing.T) {
	db := newTestDB(t)
	outbox := NewOutbox(db)

	outbox.PauseProjectSync("paused-proj", "admin", "test")

	enqueued, err := outbox.EnqueueForProject("obs-001", "paused-proj", []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enqueued {
		t.Error("expected enqueued=false for paused project")
	}

	counts, _ := outbox.Status()
	if counts.Pending != 0 {
		t.Errorf("pending = %d, want 0 for paused project", counts.Pending)
	}
}

func TestEnqueueForProject_EnabledEnqueues(t *testing.T) {
	db := newTestDB(t)
	outbox := NewOutbox(db)

	enqueued, err := outbox.EnqueueForProject("obs-002", "active-proj", []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !enqueued {
		t.Error("expected enqueued=true for active project")
	}

	counts, _ := outbox.Status()
	if counts.Pending != 1 {
		t.Errorf("pending = %d, want 1 for active project", counts.Pending)
	}
}

func TestPauseProjectSync_Idempotent(t *testing.T) {
	outbox := NewOutbox(newTestDB(t))
	for i := 0; i < 3; i++ {
		if err := outbox.PauseProjectSync("proj", "admin", "reason"); err != nil {
			t.Fatalf("PauseProjectSync call %d: %v", i+1, err)
		}
	}
	enabled, _ := outbox.IsProjectSyncEnabled("proj")
	if enabled {
		t.Error("project should remain paused after repeated pause calls")
	}
}
