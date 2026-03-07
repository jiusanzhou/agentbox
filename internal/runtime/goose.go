package runtime

func init() {
	Register("goose", &Goose{})
}

// Goose implements the block/goose CLI runtime.
type Goose struct{}

func (g *Goose) Name() string  { return "goose" }
func (g *Goose) Image() string { return "agentbox-sandbox:goose" }

func (g *Goose) BuildExecArgs(message string, continued bool) []string {
	return []string{"goose", "run", "--text", message, "--no-confirm"}
}

func (g *Goose) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (g *Goose) EnvKeys() []string       { return []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"} }
func (g *Goose) SetupCommands() []string { return nil }
