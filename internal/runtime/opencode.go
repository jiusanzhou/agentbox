package runtime

func init() {
	Register("opencode", &OpenCode{})
}

// OpenCode implements the OpenCode (Crush) CLI runtime.
type OpenCode struct{}

func (c *OpenCode) Name() string  { return "opencode" }
func (c *OpenCode) Image() string { return "agentbox-sandbox:opencode" }

func (c *OpenCode) BuildExecArgs(message string, continued bool) []string {
	return []string{"opencode", "--non-interactive", "--message", message}
}

func (c *OpenCode) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (c *OpenCode) EnvKeys() []string {
	return []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY"}
}

func (c *OpenCode) SetupCommands() []string { return nil }
