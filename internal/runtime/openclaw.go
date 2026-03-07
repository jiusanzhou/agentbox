package runtime

import (
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	Register("openclaw", &OpenClaw{})
}

// OpenClaw implements the Runtime interface for OpenClaw Gateway's
// OpenAI-compatible Chat Completions API. It uses curl to call the
// gateway endpoint, configured via environment variables.
type OpenClaw struct{}

func (o *OpenClaw) Name() string  { return "openclaw" }
func (o *OpenClaw) Image() string { return "" } // uses base image, just needs curl

func (o *OpenClaw) BuildExecArgs(message string, continued bool) []string {
	// Always use streaming so ParseStreamLine works with SSE output
	return []string{
		"sh", "-c",
		fmt.Sprintf(`curl -sS "${OPENCLAW_GATEWAY_URL}/v1/chat/completions" `+
			`-H "Authorization: Bearer ${OPENCLAW_GATEWAY_TOKEN}" `+
			`-H "Content-Type: application/json" `+
			`-d %s`,
			shellQuote(fmt.Sprintf(`{"model":"openclaw:${OPENCLAW_AGENT_ID:-main}","messages":[{"role":"user","content":%q}],"stream":true}`, message)),
		),
	}
}

func (o *OpenClaw) ParseStreamLine(line string) (string, string, bool) {
	if line == "data: [DONE]" {
		return "", "", true
	}

	if !strings.HasPrefix(line, "data: ") {
		return "", "", false
	}

	data := line[6:] // strip "data: " prefix

	var event struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return "", "", false
	}

	if len(event.Choices) == 0 {
		return "", "", false
	}

	choice := event.Choices[0]

	// Streaming: delta content
	if choice.Delta.Content != "" {
		return choice.Delta.Content, "", false
	}

	// Non-streaming: full message
	if choice.Message.Content != "" {
		return "", choice.Message.Content, true
	}

	// Check finish reason
	if choice.FinishReason != nil && *choice.FinishReason == "stop" {
		return "", "", true
	}

	return "", "", false
}

func (o *OpenClaw) EnvKeys() []string {
	return []string{"OPENCLAW_GATEWAY_URL", "OPENCLAW_GATEWAY_TOKEN", "OPENCLAW_AGENT_ID"}
}

func (o *OpenClaw) SetupCommands() []string {
	return []string{
		`which curl > /dev/null 2>&1 || (apk add --no-cache curl 2>/dev/null || apt-get install -y curl 2>/dev/null)`,
	}
}
