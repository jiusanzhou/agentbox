package runtime

import (
	"encoding/json"
	"fmt"
)

func init() {
	Register("codex", &OpenAICodex{})
}

// OpenAICodex implements the OpenAI Codex CLI runtime.
// Uses `codex exec --json --full-auto -` with prompt piped via stdin.
type OpenAICodex struct{}

func (c *OpenAICodex) Name() string  { return "codex" }
func (c *OpenAICodex) Image() string { return "agentbox-sandbox:codex" }

func (c *OpenAICodex) BuildExecArgs(message string, continued bool) []string {
	return []string{
		"sh", "-c",
		fmt.Sprintf(`printf '%%s' %s | codex exec --json --full-auto -`, shellQuote(message)),
	}
}

func (c *OpenAICodex) ParseStreamLine(line string) (string, string, bool) {
	var event struct {
		Type string `json:"type"`
		Item struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"item"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", "", false
	}

	switch event.Type {
	case "item.completed":
		if event.Item.Type == "agent_message" && event.Item.Text != "" {
			return event.Item.Text, "", false
		}
	case "turn.completed":
		return "", "", true
	case "error", "turn.failed":
		msg := event.Message
		if msg == "" {
			msg = event.Error.Message
		}
		return "", "Error: " + msg, true
	}
	return "", "", false
}

func (c *OpenAICodex) EnvKeys() []string       { return []string{"OPENAI_API_KEY"} }
func (c *OpenAICodex) SetupCommands() []string { return nil }
