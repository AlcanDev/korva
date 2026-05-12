package rules

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// CustomRulesFile is the wire shape of `.korva/sentinel-rules.yaml`.
type CustomRulesFile struct {
	Version int           `yaml:"version" json:"version"`
	Profile string        `yaml:"profile,omitempty" json:"profile,omitempty"`
	Rules   []*CustomRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

// LoadCustomRulesFile reads the YAML file at `path` and returns its parsed
// representation. A missing file returns an empty CustomRulesFile and a nil
// error — operators may not have created the file yet.
func LoadCustomRulesFile(path string) (*CustomRulesFile, error) {
	if path == "" {
		return &CustomRulesFile{Version: 1}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CustomRulesFile{Version: 1}, nil
		}
		return nil, fmt.Errorf("reading sentinel rules: %w", err)
	}
	var file CustomRulesFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing sentinel rules YAML: %w", err)
	}

	// Validate each rule so the validator never starts with a broken regex.
	seen := make(map[string]bool, len(file.Rules))
	for i, rule := range file.Rules {
		if rule == nil {
			return nil, fmt.Errorf("rule at index %d is nil", i)
		}
		if seen[rule.IDValue] {
			return nil, fmt.Errorf("duplicate rule id %q", rule.IDValue)
		}
		seen[rule.IDValue] = true
		if err := rule.Validate(); err != nil {
			return nil, err
		}
	}

	if file.Version == 0 {
		file.Version = 1
	}
	return &file, nil
}

// LoadRulesFromYAML returns each CustomRule from `path` as a slice of Rule.
// This is the helper the validator main wires when SentinelConfig.RulesPath
// is non-empty, mixing user rules with the profile's built-ins.
func LoadRulesFromYAML(path string) ([]Rule, error) {
	file, err := LoadCustomRulesFile(path)
	if err != nil {
		return nil, err
	}
	out := make([]Rule, 0, len(file.Rules))
	for _, r := range file.Rules {
		out = append(out, r)
	}
	return out, nil
}

// SaveCustomRulesFile writes the file atomically. The caller is responsible
// for validation; this function refuses to write a file that fails Validate.
func SaveCustomRulesFile(path string, file *CustomRulesFile) error {
	if file == nil {
		return fmt.Errorf("save: nil file")
	}
	if file.Version == 0 {
		file.Version = 1
	}
	for i, r := range file.Rules {
		if r == nil {
			return fmt.Errorf("rule at index %d is nil", i)
		}
		if err := r.Validate(); err != nil {
			return err
		}
	}

	data, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename tmp: %w", err)
	}
	return nil
}
