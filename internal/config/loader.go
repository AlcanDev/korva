package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Load reads korva.config.json from the given path.
// If the file does not exist, it returns a DefaultConfig.
func Load(configPath string) (KorvaConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return KorvaConfig{}, fmt.Errorf("reading config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return KorvaConfig{}, fmt.Errorf("parsing config file %s: %w", configPath, err)
	}

	return cfg, nil
}

// Save writes a KorvaConfig to disk as JSON.
func Save(cfg KorvaConfig, configPath string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config file %s: %w", configPath, err)
	}

	return nil
}

// LoadTeamProfile reads a team-profile.json from the given path.
func LoadTeamProfile(profilePath string) (TeamProfile, error) {
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return TeamProfile{}, fmt.Errorf("reading team profile: %w", err)
	}

	var profile TeamProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return TeamProfile{}, fmt.Errorf("parsing team profile: %w", err)
	}

	return profile, nil
}
