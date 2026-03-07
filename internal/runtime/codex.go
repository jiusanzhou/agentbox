package runtime

import "encoding/json"

func init() {
	Register("codex", &OpenAICodex{})
}

// OpenAICodex implements the OpenAI Codex CLI runtime.
type OpenAICodex struct{}

func (c *OpenAICodex) Name() string  { return "codex" }
func (c *OpenAICodex) Image() string { return "agentbox-sandbox:codex" }

func (c *OpenAICodex) BuildExecArgs(message string, continued bool) []string {
	args := []string{"codex", "--full-auto"}
	args = append(args, message)
	return args
}

func (c *OpenAICodex) ParseStreamLine(line string) (string, string, bool) {
	// Skeleton: parse codex CLI output when format is documented
	var event struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", "", false
	}
	if event.Text != "" {
		return event.Text, "", false
	}
	return "", "", false
}
