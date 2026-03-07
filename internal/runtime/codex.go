package runtime

func init() {
	Register("codex", &OpenAICodex{})
}

// OpenAICodex implements the OpenAI Codex CLI runtime.
type OpenAICodex struct{}

func (c *OpenAICodex) Name() string  { return "codex" }
func (c *OpenAICodex) Image() string { return "agentbox-sandbox:codex" }

func (c *OpenAICodex) BuildExecArgs(message string, continued bool) []string {
	return []string{"codex", "--full-auto", "-q", message}
}

func (c *OpenAICodex) ParseStreamLine(line string) (string, string, bool) {
	// Codex in --full-auto mode outputs plain text to stdout.
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (c *OpenAICodex) EnvKeys() []string       { return []string{"OPENAI_API_KEY"} }
func (c *OpenAICodex) SetupCommands() []string { return nil }
