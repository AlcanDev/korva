package api

import (
	"fmt"
	"strings"

	"github.com/alcandev/korva/vault/internal/store"
)

// Field length limits. Values are chosen to be generous for all legitimate
// vault payloads while still providing a meaningful guard against abuse.
const (
	maxShortField  = 500    // project, team, country, agent, author, name, tags
	maxMediumField = 5_000  // goal, title
	maxContentLen  = 1 << 19 // 512 KiB — half the body limit, safe for all prompts/observations
	maxTagsCount   = 50
	maxTagLen      = 200
)

// validateObservation rejects observations with unknown types or oversized fields.
// Called before Save to enforce data quality at the API boundary.
func validateObservation(obs *store.Observation) error {
	if obs.Type == "" {
		return fmt.Errorf("type is required")
	}
	valid := false
	for _, t := range store.AllObservationTypes {
		if string(obs.Type) == t {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unknown observation type %q — valid types: %s",
			obs.Type, strings.Join(store.AllObservationTypes, ", "))
	}
	if len(obs.Project) > maxShortField {
		return fmt.Errorf("project exceeds %d characters", maxShortField)
	}
	if len(obs.Team) > maxShortField {
		return fmt.Errorf("team exceeds %d characters", maxShortField)
	}
	if len(obs.Country) > maxShortField {
		return fmt.Errorf("country exceeds %d characters", maxShortField)
	}
	if len(obs.Author) > maxShortField {
		return fmt.Errorf("author exceeds %d characters", maxShortField)
	}
	if len(obs.Title) > maxMediumField {
		return fmt.Errorf("title exceeds %d characters", maxMediumField)
	}
	if len(obs.Content) > maxContentLen {
		return fmt.Errorf("content exceeds %d characters", maxContentLen)
	}
	if len(obs.Tags) > maxTagsCount {
		return fmt.Errorf("too many tags — maximum is %d", maxTagsCount)
	}
	for _, tag := range obs.Tags {
		if len(tag) > maxTagLen {
			return fmt.Errorf("tag %q exceeds %d characters", tag, maxTagLen)
		}
	}
	return nil
}

// validateSession rejects session-start requests with oversized fields.
func validateSession(project, team, country, agent, goal string) error {
	if len(project) > maxShortField {
		return fmt.Errorf("project exceeds %d characters", maxShortField)
	}
	if len(team) > maxShortField {
		return fmt.Errorf("team exceeds %d characters", maxShortField)
	}
	if len(country) > maxShortField {
		return fmt.Errorf("country exceeds %d characters", maxShortField)
	}
	if len(agent) > maxShortField {
		return fmt.Errorf("agent exceeds %d characters", maxShortField)
	}
	if len(goal) > maxMediumField {
		return fmt.Errorf("goal exceeds %d characters", maxMediumField)
	}
	return nil
}

// validatePrompt rejects prompt save requests with missing or oversized fields.
func validatePrompt(name, content string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > maxShortField {
		return fmt.Errorf("name exceeds %d characters", maxShortField)
	}
	if len(content) > maxContentLen {
		return fmt.Errorf("content exceeds %d characters", maxContentLen)
	}
	return nil
}
