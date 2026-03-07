package runtime

func init() {
	Register("cursor", &CursorCLI{})
}

// CursorCLI implements the Cursor agent CLI runtime.
type CursorCLI struct{}

func (c *CursorCLI) Name() string  { return "cursor" }
func (c *CursorCLI) Image() string { return "agentbox-sandbox:cursor" }

func (c *CursorCLI) BuildExecArgs(message string, continued bool) []string {
	return []string{"cursor", "agent", "--message", message}
}

func (c *CursorCLI) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (c *CursorCLI) EnvKeys() []string       { return []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"} }
func (c *CursorCLI) SetupCommands() []string { return nil }
