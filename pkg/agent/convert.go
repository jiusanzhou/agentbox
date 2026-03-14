package agent

import (
	"encoding/json"

	"go.zoe.im/agentbox/internal/model"
)

// ToDNA converts a Manifest to the internal AgentDNA model for storage.
// It maps agent.yaml fields to the existing ABox data model.
func (m *Manifest) ToDNA() *model.AgentDNA {
	dna := &model.AgentDNA{
		Slug:    m.ID,
		Version: m.Version,
		Identity: &model.AgentIdentity{
			Name:        m.Name,
			Description: m.Description,
		},
		Manifest: &model.AgentManifest{
			Version: m.Version,
			Author:  m.Author,
			License: m.License,
		},
	}

	// Map persona to Soul
	if m.Persona != nil {
		dna.Soul = &model.AgentSoul{
			Personality:        m.Persona.Style,
			Tone:               m.Persona.Tone,
			CommunicationStyle: m.Persona.Tone,
			Values:             m.Persona.Principles,
		}
	}

	// Map skills to Skills JSON
	if len(m.Skills) > 0 {
		if data, err := json.Marshal(m.Skills); err == nil {
			dna.Skills = data
		}
	}

	// Map adapters to Tools JSON
	if m.Adapters != nil {
		if data, err := json.Marshal(m.Adapters); err == nil {
			dna.Tools = data
		}
	}

	// Map experience to Memory JSON
	if m.Experience != nil {
		if data, err := json.Marshal(m.Experience); err == nil {
			dna.Memory = data
		}
	}

	// Map marketplace tags
	if m.Marketplace != nil {
		dna.Manifest.Tags = m.Marketplace.Tags

		if m.Marketplace.Pricing != nil {
			dna.Manifest.PricingModel = m.Marketplace.Pricing.Model
			dna.Manifest.Currency = "USD"
		}
	}

	// Map preferred framework to runtime
	if rt := m.PreferredFramework(); rt != "" {
		dna.Manifest.Runtime = rt
	}

	// Map model requirements
	if m.Model != nil {
		if dna.Manifest.Requirements == nil {
			dna.Manifest.Requirements = make(map[string]string)
		}
		if m.Model.Minimum != "" {
			dna.Manifest.Requirements["model_minimum"] = m.Model.Minimum
		}
		if m.Model.Recommended != "" {
			dna.Manifest.Requirements["model_recommended"] = m.Model.Recommended
		}
		if m.Model.ContextWindow != "" {
			dna.Manifest.Requirements["context_window"] = m.Model.ContextWindow
		}
	}

	// Map runtime requirements
	if m.Runtime != nil {
		if dna.Manifest.Requirements == nil {
			dna.Manifest.Requirements = make(map[string]string)
		}
		if m.Runtime.Sandbox != "" {
			dna.Manifest.Requirements["sandbox"] = m.Runtime.Sandbox
		}
	}

	return dna
}

// FromDNA converts an AgentDNA back to a Manifest (best-effort reconstruction).
func FromDNA(dna *model.AgentDNA) *Manifest {
	m := &Manifest{
		ID:      dna.Slug,
		Version: dna.Version,
	}

	if dna.Identity != nil {
		m.Name = dna.Identity.Name
		m.Description = dna.Identity.Description
	}

	if dna.Manifest != nil {
		m.Author = dna.Manifest.Author
		m.License = dna.Manifest.License

		if dna.Manifest.Runtime != "" {
			m.Adapters = &Adapters{
				Frameworks: []FrameworkRef{
					{Name: dna.Manifest.Runtime, Native: true},
				},
			}
		}

		if len(dna.Manifest.Tags) > 0 {
			m.Marketplace = &Marketplace{
				Tags: dna.Manifest.Tags,
			}
			if dna.Manifest.PricingModel != "" {
				m.Marketplace.Pricing = &Pricing{
					Model: dna.Manifest.PricingModel,
				}
			}
		}

		// Reconstruct model requirements
		if reqs := dna.Manifest.Requirements; reqs != nil {
			m.Model = &ModelRequirements{
				Minimum:       reqs["model_minimum"],
				Recommended:   reqs["model_recommended"],
				ContextWindow: reqs["context_window"],
			}
		}
	}

	if dna.Soul != nil {
		m.Persona = &Persona{
			Style:      dna.Soul.Personality,
			Tone:       dna.Soul.Tone,
			Principles: dna.Soul.Values,
		}
	}

	// Reconstruct skills from JSON
	if len(dna.Skills) > 0 {
		_ = json.Unmarshal(dna.Skills, &m.Skills)
	}

	// Reconstruct experience from JSON
	if len(dna.Memory) > 0 {
		m.Experience = &Experience{}
		_ = json.Unmarshal(dna.Memory, m.Experience)
	}

	// Reconstruct adapters from JSON (if not already set from runtime)
	if len(dna.Tools) > 0 && m.Adapters == nil {
		m.Adapters = &Adapters{}
		_ = json.Unmarshal(dna.Tools, m.Adapters)
	}

	// Stats from registry metadata
	if dna.Downloads > 0 || dna.Rating > 0 {
		if m.Marketplace == nil {
			m.Marketplace = &Marketplace{}
		}
		m.Marketplace.Stats = &Stats{
			Users:  int(dna.Downloads),
			Rating: dna.Rating,
		}
	}

	return m
}
