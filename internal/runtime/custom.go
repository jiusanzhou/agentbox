package runtime

import "os"

func init() {
	Register("custom", &Custom{})
}

// Custom implements a user-provided script/agent runtime.
// The script is expected at /workspace/agent.sh or configured via AGENTBOX_CUSTOM_SCRIPT env.
type Custom struct{}

func (c *Custom) Name() string  { return "custom" }
func (c *Custom) Image() string { return "" } // user must specify

func (c *Custom) BuildExecArgs(message string, continued bool) []string {
	script := os.Getenv("AGENTBOX_CUSTOM_SCRIPT")
	if script == "" {
		script = "/workspace/agent.sh"
	}
	return []string{script, message}
}

func (c *Custom) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (c *Custom) EnvKeys() []string       { return nil }
func (c *Custom) SetupCommands() []string { return nil }
