package agentfile

import (
	"bufio"
	"os"
	"strings"

	"go.zoe.im/agentbox/internal/model"
)

// Parse extracts agent definition from AGENTS.md content.
func Parse(content string) (*model.Agent, error) {
	agent := &model.Agent{}
	scanner := bufio.NewScanner(strings.NewReader(content))

	var section string
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") {
			agent.Name = strings.TrimPrefix(trimmed, "# ")
			continue
		}

		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(trimmed, "## ") {
			switch {
			case strings.Contains(lower, "instruction") || strings.Contains(lower, "description"):
				section = "description"
			case strings.Contains(lower, "workflow"):
				section = "workflow"
			case strings.Contains(lower, "guideline"):
				section = "guidelines"
			case strings.Contains(lower, "skill"):
				section = "skills"
			default:
				section = ""
			}
			continue
		}

		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			item := strings.TrimLeft(trimmed, "-* ")
			switch section {
			case "workflow":
				agent.Workflow = append(agent.Workflow, item)
			case "guidelines":
				agent.Guidelines = append(agent.Guidelines, item)
			case "skills":
				agent.Skills = append(agent.Skills, item)
			}
			continue
		}

		if section == "description" && trimmed != "" {
			if agent.Description != "" {
				agent.Description += "\n"
			}
			agent.Description += trimmed
		}
	}

	return agent, scanner.Err()
}

// ParseFile reads and parses an AGENTS.md file.
func ParseFile(path string) (*model.Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}
