package runtime

func init() {
	Register("openhands", &OpenHands{})
}

// OpenHands implements the OpenHands (formerly OpenDevin) runtime.
type OpenHands struct{}

func (o *OpenHands) Name() string  { return "openhands" }
func (o *OpenHands) Image() string { return "agentbox-sandbox:openhands" }

func (o *OpenHands) BuildExecArgs(message string, continued bool) []string {
	return []string{"python", "-m", "openhands.core.main", "-t", message}
}

func (o *OpenHands) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (o *OpenHands) EnvKeys() []string       { return []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"} }
func (o *OpenHands) SetupCommands() []string { return nil }
