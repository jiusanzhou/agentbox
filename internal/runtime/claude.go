package runtime

import "encoding/json"

func init() {
	Register("claude", &ClaudeCode{})
}

// ClaudeCode implements the Claude Code CLI runtime.
type ClaudeCode struct{}

func (c *ClaudeCode) Name() string  { return "claude" }
func (c *ClaudeCode) Image() string { return "agentbox-sandbox:latest" }

func (c *ClaudeCode) BuildExecArgs(message string, continued bool) []string {
	args := []string{
		"claude", "-p", "--dangerously-skip-permissions",
		"--output-format", "stream-json", "--verbose",
	}
	if continued {
		args = append(args, "--continue")
	}
	args = append(args, message)
	return args
}

func (c *ClaudeCode) EnvKeys() []string       { return []string{"ANTHROPIC_API_KEY"} }
func (c *ClaudeCode) SetupCommands() []string { return nil }

func (c *ClaudeCode) ParseStreamLine(line string) (string, string, bool) {
	var event struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", "", false
	}

	switch event.Type {
	case "assistant":
		for _, c := range event.Message.Content {
			if c.Type == "text" && c.Text != "" {
				return c.Text, "", false
			}
		}
	case "result":
		return "", event.Result, true
	}
	return "", "", false
}
