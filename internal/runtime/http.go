package runtime

import "fmt"

func init() {
	Register("http", &HTTPAdapter{})
}

// HTTPAdapter implements an HTTP webhook adapter for remote agents (e.g. OpenClaw).
// The endpoint URL and optional bearer token are configured via
// ABOX_HTTP_ENDPOINT and ABOX_HTTP_TOKEN environment variables.
type HTTPAdapter struct{}

func (h *HTTPAdapter) Name() string  { return "http" }
func (h *HTTPAdapter) Image() string { return "" } // no container needed

func (h *HTTPAdapter) BuildExecArgs(message string, continued bool) []string {
	return []string{
		"sh", "-c",
		fmt.Sprintf(
			`curl -s -X POST "$ABOX_HTTP_ENDPOINT" -H "Content-Type: application/json" -H "Authorization: Bearer $ABOX_HTTP_TOKEN" -d %s`,
			shellQuote(fmt.Sprintf(`{"message":%q}`, message)),
		),
	}
}

func (h *HTTPAdapter) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (h *HTTPAdapter) EnvKeys() []string       { return []string{"ABOX_HTTP_ENDPOINT"} }
func (h *HTTPAdapter) SetupCommands() []string { return nil }
