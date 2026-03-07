package runtime

import "encoding/json"

func init() {
	Register("gemini", &GeminiCLI{})
}

// GeminiCLI implements the Gemini CLI runtime.
type GeminiCLI struct{}

func (c *GeminiCLI) Name() string  { return "gemini" }
func (c *GeminiCLI) Image() string { return "agentbox-sandbox:gemini" }

func (c *GeminiCLI) BuildExecArgs(message string, continued bool) []string {
	args := []string{"gemini", "-p", message}
	return args
}

func (c *GeminiCLI) ParseStreamLine(line string) (string, string, bool) {
	// Skeleton: parse gemini CLI output when format is documented
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
