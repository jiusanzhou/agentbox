package runtime

func init() {
	Register("aider", &Aider{})
}

// Aider implements the aider-chat CLI runtime.
type Aider struct{}

func (a *Aider) Name() string  { return "aider" }
func (a *Aider) Image() string { return "agentbox-sandbox:aider" }

func (a *Aider) BuildExecArgs(message string, continued bool) []string {
	return []string{"aider", "--yes-always", "--no-auto-commits", "--no-git", "--message", message}
}

func (a *Aider) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (a *Aider) EnvKeys() []string       { return []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"} }
func (a *Aider) SetupCommands() []string { return nil }
