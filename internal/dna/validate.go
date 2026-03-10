package dna

import (
	"fmt"
	"strings"

	"go.zoe.im/agentbox/internal/model"

	semver "github.com/Masterminds/semver/v3"
)

// ValidationError collects multiple validation issues.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return "validation failed:\n  - " + strings.Join(e.Errors, "\n  - ")
}

func (e *ValidationError) add(msg string) {
	e.Errors = append(e.Errors, msg)
}

// Validate checks that an AgentDNA has all required fields for publishing.
func Validate(agent *model.AgentDNA) error {
	ve := &ValidationError{}

	// Identity is required
	if agent.Identity == nil {
		ve.add("identity (AGENT.md) is required")
	} else {
		if agent.Identity.Name == "" {
			ve.add("identity.name is required (use # heading in AGENT.md)")
		}
	}

	// Manifest is required
	if agent.Manifest == nil {
		ve.add("manifest (manifest.yaml) is required")
	} else {
		if agent.Manifest.Version == "" {
			ve.add("manifest.version is required")
		} else if _, err := semver.NewVersion(agent.Manifest.Version); err != nil {
			ve.add(fmt.Sprintf("manifest.version %q is not valid semver: %v", agent.Manifest.Version, err))
		}

		if agent.Manifest.Author == "" {
			ve.add("manifest.author is required")
		}

		if agent.Manifest.Runtime == "" {
			ve.add("manifest.runtime is required")
		} else {
			validRuntimes := map[string]bool{
				"claude": true, "codex": true, "gemini": true, "grok": true,
				"ollama": true, "openai": true, "custom": true,
			}
			if !validRuntimes[agent.Manifest.Runtime] {
				ve.add(fmt.Sprintf("manifest.runtime %q is not a supported runtime", agent.Manifest.Runtime))
			}
		}
	}

	// Version should match manifest version
	if agent.Version != "" && agent.Manifest != nil && agent.Manifest.Version != "" {
		if agent.Version != agent.Manifest.Version {
			ve.add(fmt.Sprintf("version mismatch: agent.version=%q, manifest.version=%q", agent.Version, agent.Manifest.Version))
		}
	}

	if len(ve.Errors) > 0 {
		return ve
	}
	return nil
}

// BumpVersion increments the patch version of a semver string.
// If prev is empty, returns "0.1.0".
func BumpVersion(prev string) (string, error) {
	if prev == "" {
		return "0.1.0", nil
	}
	v, err := semver.NewVersion(prev)
	if err != nil {
		return "", fmt.Errorf("invalid semver %q: %w", prev, err)
	}
	next := v.IncPatch()
	return next.String(), nil
}
