package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oklog/ulid/v2"
	"gopkg.in/yaml.v3"

	"github.com/alcandev/korva/internal/config"
)

// sentinelRule is the API/JSON shape for one Sentinel custom rule.
// It mirrors the YAML representation in sentinel/validator/internal/rules but
// lives in vault to avoid cross-module internal imports.
type sentinelRule struct {
	ID           string   `yaml:"id" json:"id"`
	Description  string   `yaml:"description,omitempty" json:"description,omitempty"`
	Severity     string   `yaml:"severity,omitempty" json:"severity,omitempty"`
	Pattern      string   `yaml:"pattern" json:"pattern"`
	PathsInclude []string `yaml:"paths_include,omitempty" json:"paths_include,omitempty"`
	PathsExclude []string `yaml:"paths_exclude,omitempty" json:"paths_exclude,omitempty"`
	Message      string   `yaml:"message,omitempty" json:"message,omitempty"`
}

// sentinelRulesFile is the wire shape of `.korva/sentinel-rules.yaml`.
type sentinelRulesFile struct {
	Version int             `yaml:"version" json:"version"`
	Profile string          `yaml:"profile,omitempty" json:"profile,omitempty"`
	Rules   []*sentinelRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

var sentinelRuleIDPattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,30}$`)

// Validate ensures the rule fields are well-formed.
func (r *sentinelRule) Validate() error {
	if !sentinelRuleIDPattern.MatchString(r.ID) {
		return fmt.Errorf("rule id %q must match [A-Z][A-Z0-9-]{2,30}", r.ID)
	}
	if r.Pattern == "" {
		return fmt.Errorf("rule %q: pattern is required", r.ID)
	}
	if _, err := regexp.Compile(r.Pattern); err != nil {
		return fmt.Errorf("rule %q: invalid regex pattern: %w", r.ID, err)
	}
	if r.Severity != "" {
		switch r.Severity {
		case "error", "warning", "info":
		default:
			return fmt.Errorf("rule %q: severity must be error|warning|info (got %q)", r.ID, r.Severity)
		}
	}
	return nil
}

// builtinSentinelRules describes the 10 hardcoded rules so the UI can show
// them next to the user's custom rules. The source-of-truth lives in
// sentinel/validator/internal/rules — these descriptions must be kept in sync.
var builtinSentinelRules = []map[string]any{
	{"id": "HEX-001", "description": "Domain layer must not import from infrastructure or application", "severity": "error"},
	{"id": "HEX-002", "description": "Application layer must not import from infrastructure", "severity": "error"},
	{"id": "HEX-003", "description": "No console.log inside src/", "severity": "error"},
	{"id": "HEX-004", "description": "Adapters must not be instantiated with `new` outside .module.ts", "severity": "warning"},
	{"id": "HEX-005", "description": "No `any` type without // korva-ignore comment", "severity": "warning"},
	{"id": "NAM-001", "description": "DTO classes must use uppercase DTO suffix", "severity": "warning"},
	{"id": "NAM-002", "description": "Port tokens must be SCREAMING_SNAKE_CASE", "severity": "warning"},
	{"id": "NAM-003", "description": "Adapter files must follow *.adapter.*.ts pattern", "severity": "warning"},
	{"id": "SEC-001", "description": "No hardcoded secrets in source files", "severity": "error"},
	{"id": "TEST-001", "description": "Spec files must be co-located with their source", "severity": "warning"},
}

// rulesFilePath resolves the YAML path:
//   - cfg.Sentinel.RulesPath when set (relative to the project root)
//   - .korva/sentinel-rules.yaml as a default
func rulesFilePath(cfg *config.KorvaConfig, configDir string) string {
	rel := ".korva/sentinel-rules.yaml"
	if cfg != nil && cfg.Sentinel.RulesPath != "" {
		rel = cfg.Sentinel.RulesPath
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	if configDir == "" {
		return rel
	}
	return filepath.Join(configDir, rel)
}

// adminGetSentinelRules handles GET /admin/sentinel/rules.
func adminGetSentinelRules(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := loadConfigOrEmpty(configPath)
		rulesPath := rulesFilePath(&cfg, filepath.Dir(configPath))
		file := readSentinelRulesFile(rulesPath)

		writeJSON(w, http.StatusOK, map[string]any{
			"profile":    file.Profile,
			"rules_path": rulesPath,
			"builtin":    builtinSentinelRules,
			"custom":     file.Rules,
		})
	}
}

// putSentinelRulesRequest is the wire shape for PUT /admin/sentinel/rules.
type putSentinelRulesRequest struct {
	Profile     string          `json:"profile"`
	CustomRules []*sentinelRule `json:"custom_rules"`
}

// adminPutSentinelRules handles PUT /admin/sentinel/rules: validate, write atomically.
func adminPutSentinelRules(configPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req putSentinelRulesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate first — never write a file with broken regexes.
		seen := make(map[string]bool, len(req.CustomRules))
		for i, rule := range req.CustomRules {
			if rule == nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("rule at index %d is nil", i))
				return
			}
			if seen[rule.ID] {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("duplicate rule id %q", rule.ID))
				return
			}
			seen[rule.ID] = true
			if err := rule.Validate(); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}

		cfg := loadConfigOrEmpty(configPath)
		rulesPath := rulesFilePath(&cfg, filepath.Dir(configPath))

		file := &sentinelRulesFile{
			Version: 1,
			Profile: req.Profile,
			Rules:   req.CustomRules,
		}
		data, err := yaml.Marshal(file)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "marshal yaml: "+err.Error())
			return
		}

		if err := os.MkdirAll(filepath.Dir(rulesPath), 0o755); err != nil {
			writeError(w, http.StatusInternalServerError, "ensuring rules dir: "+err.Error())
			return
		}

		tmp := rulesPath + ".tmp." + ulid.Make().String()
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, "writing tmp: "+err.Error())
			return
		}
		if err := os.Rename(tmp, rulesPath); err != nil {
			_ = os.Remove(tmp)
			writeError(w, http.StatusInternalServerError, "rename: "+err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status":      "saved",
			"rules_path":  rulesPath,
			"rules_count": len(req.CustomRules),
		})
	}
}

// testSentinelRuleRequest is the wire shape for POST /admin/sentinel/test.
type testSentinelRuleRequest struct {
	Rule     sentinelRule `json:"rule"`
	Code     string       `json:"code"`
	FilePath string       `json:"file_path"`
}

// testSentinelRuleMatch is one match returned by POST /admin/sentinel/test.
type testSentinelRuleMatch struct {
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	MatchedText string `json:"matched_text"`
	Message     string `json:"message"`
}

// adminTestSentinelRule handles POST /admin/sentinel/test: dry-run a single rule
// against a code snippet without touching disk.
func adminTestSentinelRule() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req testSentinelRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := req.Rule.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Path filter — when no include is set, the rule applies everywhere.
		if !ruleAppliesToPath(&req.Rule, req.FilePath) {
			writeJSON(w, http.StatusOK, map[string]any{"matches": []testSentinelRuleMatch{}, "applies": false})
			return
		}

		re := regexp.MustCompile(req.Rule.Pattern) // Validate already compiled it
		msg := req.Rule.Message
		if msg == "" {
			msg = "matched pattern"
		}

		matches := []testSentinelRuleMatch{}
		for i, line := range strings.Split(req.Code, "\n") {
			loc := re.FindStringIndex(line)
			if loc == nil {
				continue
			}
			matches = append(matches, testSentinelRuleMatch{
				Line:        i + 1,
				Column:      loc[0] + 1,
				MatchedText: line[loc[0]:loc[1]],
				Message:     msg,
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"matches": matches,
			"applies": true,
		})
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func loadConfigOrEmpty(path string) config.KorvaConfig {
	if path == "" {
		return config.KorvaConfig{}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.KorvaConfig{}
	}
	return cfg
}

func readSentinelRulesFile(path string) *sentinelRulesFile {
	out := &sentinelRulesFile{Version: 1}
	if path == "" {
		return out
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var file sentinelRulesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return out // best-effort: hide parse errors from this read endpoint
	}
	if file.Version == 0 {
		file.Version = 1
	}
	return &file
}

// ruleAppliesToPath duplicates the path-filter logic so the test endpoint can
// run without depending on the sentinel package. Stays small.
func ruleAppliesToPath(r *sentinelRule, path string) bool {
	if path == "" {
		return true
	}
	clean := filepath.ToSlash(path)
	if matchesAnyGlob(clean, r.PathsExclude) {
		return false
	}
	if len(r.PathsInclude) == 0 {
		return true
	}
	return matchesAnyGlob(clean, r.PathsInclude)
}

func matchesAnyGlob(path string, globs []string) bool {
	for _, g := range globs {
		matched, _ := regexp.MatchString(globToRegex(g), path)
		if matched {
			return true
		}
	}
	return false
}

// globToRegex mirrors the helper in sentinel/validator/internal/rules. Tiny
// duplication to avoid a cross-module dependency.
func globToRegex(glob string) string {
	const (
		dsSlash = "\x00DSS\x00"
		ds      = "\x00DS\x00"
		single  = "\x00S\x00"
	)
	g := strings.ReplaceAll(filepath.ToSlash(glob), "**/", dsSlash)
	g = strings.ReplaceAll(g, "**", ds)
	g = strings.ReplaceAll(g, "*", single)

	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(g); i++ {
		c := g[i]
		switch c {
		case '?':
			b.WriteByte('.')
		case '.', '+', '(', ')', '{', '}', '[', ']', '^', '$', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString("$")
	out := b.String()
	out = strings.ReplaceAll(out, dsSlash, "(?:.*/)?")
	out = strings.ReplaceAll(out, ds, ".*")
	out = strings.ReplaceAll(out, single, "[^/]*")
	return out
}

// errors package import retained for future expansion (e.g. structured errors).
var _ = errors.New
