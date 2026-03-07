package runtime

func init() {
	Register("gemini", &GeminiCLI{})
}

// GeminiCLI implements the Gemini CLI runtime.
type GeminiCLI struct{}

func (c *GeminiCLI) Name() string  { return "gemini" }
func (c *GeminiCLI) Image() string { return "agentbox-sandbox:gemini" }

func (c *GeminiCLI) BuildExecArgs(message string, continued bool) []string {
	return []string{"gemini", "-p", message}
}

func (c *GeminiCLI) ParseStreamLine(line string) (string, string, bool) {
	// Gemini CLI in -p mode outputs plain text to stdout.
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (c *GeminiCLI) EnvKeys() []string       { return []string{"GEMINI_API_KEY"} }
func (c *GeminiCLI) SetupCommands() []string { return nil }
