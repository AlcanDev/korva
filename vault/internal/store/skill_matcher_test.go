package store

import (
	"testing"
)

// seedSkill inserts a skill with auto_load enabled and the given triggers JSON.
func seedSkill(t *testing.T, s *Store, teamID, name, body, triggersJSON string) string {
	t.Helper()
	id := newID()
	_, err := s.db.Exec(`
		INSERT INTO skills (id, team_id, name, body, tags, scope, version, updated_by,
		                   triggers, auto_load, created_at, updated_at)
		VALUES (?, ?, ?, ?, '[]', 'team', 1, '', ?, 1, datetime('now'), datetime('now'))`,
		id, teamID, name, body, triggersJSON)
	if err != nil {
		t.Fatalf("insert skill: %v", err)
	}
	return id
}

func TestMatchSkills_KeywordMatch(t *testing.T) {
	s := newTestStore(t)

	seedSkill(t, s, "team-1", "JWT Auth Pattern", "Use JWT with RS256.",
		`{"keywords":["auth","jwt"],"priority":3}`)
	seedSkill(t, s, "team-1", "Caching Pattern", "Use Redis with TTL.",
		`{"keywords":["cache","redis"],"priority":2}`)

	matches, err := s.MatchSkills(SkillMatchInput{
		TeamID: "team-1",
		Prompt: "I need to implement authentication for the API using JWT tokens",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}
	if matches[0].Name != "JWT Auth Pattern" {
		t.Errorf("expected JWT skill first, got %q", matches[0].Name)
	}
	if matches[0].Score == 0 {
		t.Error("expected non-zero score")
	}
	if matches[0].Reason == "" {
		t.Error("expected non-empty match reason")
	}
}

func TestMatchSkills_FilePatternBeatsKeyword(t *testing.T) {
	s := newTestStore(t)

	seedSkill(t, s, "team-1", "Auth Files Skill", "Auth file conventions.",
		`{"file_patterns":["auth/*.go"],"priority":1}`)
	seedSkill(t, s, "team-1", "Auth Keyword Skill", "Auth keyword skill.",
		`{"keywords":["auth"],"priority":1}`)

	matches, err := s.MatchSkills(SkillMatchInput{
		TeamID:    "team-1",
		Prompt:    "implement auth",
		FilePaths: []string{"auth/jwt.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) < 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	// File-pattern skill should win (higher weight).
	if matches[0].Name != "Auth Files Skill" {
		t.Errorf("expected file-pattern skill first, got %q", matches[0].Name)
	}
}

func TestMatchSkills_PriorityWinsTies(t *testing.T) {
	s := newTestStore(t)

	seedSkill(t, s, "team-1", "Low Priority", "low body.",
		`{"keywords":["foo"],"priority":1}`)
	seedSkill(t, s, "team-1", "High Priority", "high body.",
		`{"keywords":["foo"],"priority":10}`)

	matches, _ := s.MatchSkills(SkillMatchInput{
		TeamID: "team-1",
		Prompt: "do foo",
	})
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Name != "High Priority" {
		t.Errorf("expected High Priority first, got %q", matches[0].Name)
	}
}

func TestMatchSkills_ProjectMatch(t *testing.T) {
	s := newTestStore(t)

	seedSkill(t, s, "team-1", "Project Specific", "Specific to api-server.",
		`{"projects":["api-server"]}`)

	matches, _ := s.MatchSkills(SkillMatchInput{
		TeamID:  "team-1",
		Project: "api-server",
		Prompt:  "any work",
	})
	if len(matches) == 0 {
		t.Fatal("expected match for project")
	}

	// Same skill should NOT match for a different project.
	matches2, _ := s.MatchSkills(SkillMatchInput{
		TeamID:  "team-1",
		Project: "other-project",
		Prompt:  "any work",
	})
	if len(matches2) != 0 {
		t.Errorf("expected 0 matches for non-listed project, got %d", len(matches2))
	}
}

func TestMatchSkills_SkipsAutoLoadOff(t *testing.T) {
	s := newTestStore(t)

	id := newID()
	s.db.Exec(`INSERT INTO skills (id, team_id, name, body, tags, scope, version, updated_by,
	                                triggers, auto_load, created_at, updated_at)
	          VALUES (?, ?, ?, ?, '[]', 'team', 1, '', ?, 0, datetime('now'), datetime('now'))`,
		id, "team-1", "Manual Only", "manual body.", `{"keywords":["any"]}`)

	matches, _ := s.MatchSkills(SkillMatchInput{
		TeamID: "team-1",
		Prompt: "any keyword",
	})
	if len(matches) != 0 {
		t.Errorf("auto_load=0 skills must not be auto-matched, got %d", len(matches))
	}
}

func TestMatchSkills_RespectsLimit(t *testing.T) {
	s := newTestStore(t)

	for i := range 10 {
		seedSkill(t, s, "team-1", "skill-"+intToString(i), "body", `{"keywords":["foo"]}`)
	}

	matches, _ := s.MatchSkills(SkillMatchInput{
		TeamID: "team-1",
		Prompt: "foo",
		Limit:  3,
	})
	if len(matches) != 3 {
		t.Errorf("expected 3 matches (limit), got %d", len(matches))
	}
}

func TestLogSkillActivation(t *testing.T) {
	s := newTestStore(t)
	skillID := seedSkill(t, s, "team-1", "Test", "body", `{"keywords":["test"]}`)

	s.LogSkillActivation(skillID, "team-1", "proj-a", "abc123", "prompt mentions test", 0.7)

	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM skill_activations WHERE skill_id = ?`, skillID).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 activation row, got %d", count)
	}
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
