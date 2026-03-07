package runtime

import (
	"strings"
	"testing"
)

func assert(t *testing.T, condition bool, msgs ...string) {
	t.Helper()
	if !condition {
		msg := "assertion failed"
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		t.Fatal(msg)
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func TestRuntime_Registry(t *testing.T) {
	// Default should be claude
	d := Default()
	assert(t, d != nil, "default runtime should not be nil")
	assert(t, d.Name() == "claude", "default should be claude")

	// All registered runtimes
	names := []string{"claude", "codex", "gemini", "aider", "goose", "openhands", "cursor", "opencode", "custom", "http", "openclaw"}
	for _, name := range names {
		rt := Get(name)
		assert(t, rt != nil, "missing runtime: "+name)
		assert(t, rt.Name() == name, "runtime name mismatch for "+name)
	}

	// Runtimes with images (custom and http have empty images by design)
	for _, name := range []string{"claude", "codex", "gemini", "aider", "goose", "openhands", "cursor", "opencode"} {
		rt := Get(name)
		assert(t, rt.Image() != "", "runtime image should not be empty for "+name)
	}

	// Unknown returns nil
	assert(t, Get("nonexistent") == nil, "unknown runtime should return nil")
}

func TestRuntime_List(t *testing.T) {
	list := List()
	assert(t, len(list) >= 11, "should have at least 11 runtimes")

	found := map[string]bool{}
	for _, info := range list {
		found[info.Name] = true
		assert(t, info.Name != "", "runtime info name should not be empty")
	}

	assert(t, found["claude"], "list should contain claude")
	assert(t, found["codex"], "list should contain codex")
}

func TestRuntime_ClaudeBuildArgs(t *testing.T) {
	rt := Get("claude")

	args := rt.BuildExecArgs("hello world", false)
	assert(t, contains(args, "claude"), "should contain claude command")
	assert(t, contains(args, "-p"), "should contain -p flag")
	assert(t, contains(args, "--dangerously-skip-permissions"), "should contain --dangerously-skip-permissions")
	assert(t, contains(args, "--output-format"), "should contain --output-format")
	assert(t, contains(args, "stream-json"), "should contain stream-json")
	assert(t, contains(args, "hello world"), "should contain message")
	assert(t, !contains(args, "--continue"), "should not contain --continue when not continued")

	args = rt.BuildExecArgs("hello", true)
	assert(t, contains(args, "--continue"), "should contain --continue when continued")
}

func TestRuntime_CodexBuildArgs(t *testing.T) {
	rt := Get("codex")

	args := rt.BuildExecArgs("test task", false)
	assert(t, contains(args, "sh"), "codex should use sh")
	assert(t, contains(args, "-c"), "codex should use -c")
	// The third arg should contain "codex exec"
	found := false
	for _, arg := range args {
		if len(arg) > 10 && arg[len(arg)-1] != 0 {
			// Check if it contains codex exec
			if containsStr(arg, "codex exec") {
				found = true
			}
		}
	}
	assert(t, found, "codex args should contain 'codex exec' in shell command")
}

func TestRuntime_ClaudeParseStreamLine(t *testing.T) {
	rt := Get("claude")

	// Assistant event
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`
	token, result, done := rt.ParseStreamLine(line)
	assert(t, token == "Hello", "should parse assistant text")
	assert(t, result == "", "result should be empty")
	assert(t, !done, "should not be done")

	// Result event
	line = `{"type":"result","result":"Final answer"}`
	token, result, done = rt.ParseStreamLine(line)
	assert(t, token == "", "token should be empty for result")
	assert(t, result == "Final answer", "should parse result")
	assert(t, done, "should be done")

	// Unknown event
	line = `{"type":"system","message":"init"}`
	token, result, done = rt.ParseStreamLine(line)
	assert(t, token == "", "unknown event should return empty token")
	assert(t, result == "", "unknown event should return empty result")
	assert(t, !done, "unknown event should not be done")

	// Invalid JSON
	token, result, done = rt.ParseStreamLine("not json")
	assert(t, token == "", "invalid json should return empty token")
	assert(t, !done, "invalid json should not be done")
}

func TestRuntime_CodexParseStreamLine(t *testing.T) {
	rt := Get("codex")

	// item.completed with agent_message
	line := `{"type":"item.completed","item":{"type":"agent_message","text":"Done!"}}`
	token, _, done := rt.ParseStreamLine(line)
	assert(t, token == "Done!", "should parse agent message text")
	assert(t, !done, "item.completed should not be done")

	// turn.completed
	line = `{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}`
	_, _, done = rt.ParseStreamLine(line)
	assert(t, done, "turn.completed should be done")

	// error event
	line = `{"type":"error","message":"something went wrong"}`
	_, result, done := rt.ParseStreamLine(line)
	assert(t, done, "error should be done")
	assert(t, containsStr(result, "Error"), "error result should contain Error")
}

func TestRuntime_GeminiParseStreamLine(t *testing.T) {
	rt := Get("gemini")

	token, _, done := rt.ParseStreamLine("Hello from Gemini")
	assert(t, token == "Hello from Gemini\n", "should return line with newline")
	assert(t, !done, "should not be done")

	token, _, done = rt.ParseStreamLine("")
	assert(t, token == "", "empty line should return empty token")
}

func TestRuntime_EnvKeys(t *testing.T) {
	tests := map[string][]string{
		"claude": {"ANTHROPIC_API_KEY"},
		"codex":  {"OPENAI_API_KEY"},
		"gemini": {"GEMINI_API_KEY"},
	}
	for name, expected := range tests {
		rt := Get(name)
		assert(t, rt != nil, "runtime should exist: "+name)
		keys := rt.EnvKeys()
		assert(t, len(keys) == len(expected), "env keys length mismatch for "+name)
		for i, k := range keys {
			assert(t, k == expected[i], "env key mismatch for "+name+": got "+k+", want "+expected[i])
		}
	}
}

func TestRuntime_SetupCommands(t *testing.T) {
	// Claude, codex, gemini should have nil/empty setup commands
	for _, name := range []string{"claude", "codex", "gemini"} {
		rt := Get(name)
		cmds := rt.SetupCommands()
		assert(t, len(cmds) == 0, name+" should have no setup commands")
	}
}

// containsStr checks if s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRuntime_OpenClawBuildArgs(t *testing.T) {
	rt := Get("openclaw")
	assert(t, rt != nil, "openclaw runtime not found")
	args := rt.BuildExecArgs("hello", false)
	assert(t, len(args) == 3, "should be sh -c 'curl ...'")
	assert(t, args[0] == "sh", "first arg should be sh")
	assert(t, strings.Contains(args[2], "OPENCLAW_GATEWAY_URL"), "should reference gateway URL env")
	assert(t, strings.Contains(args[2], "chat/completions"), "should call chat/completions endpoint")
}

func TestRuntime_OpenClawParseStreamLine(t *testing.T) {
	rt := Get("openclaw")
	assert(t, rt != nil, "openclaw runtime not found")

	// SSE delta
	token, _, done := rt.ParseStreamLine(`data: {"choices":[{"delta":{"content":"Hi"}}]}`)
	assert(t, token == "Hi", "should parse delta content")
	assert(t, !done, "delta should not be done")

	// SSE done
	_, _, done = rt.ParseStreamLine("data: [DONE]")
	assert(t, done, "DONE marker should be done")

	// Non-streaming full response
	_, result, done := rt.ParseStreamLine(`data: {"choices":[{"message":{"content":"Full response"}}]}`)
	assert(t, result == "Full response", "should parse full message content")
	assert(t, done, "full message should be done")

	// Empty line
	token, _, done = rt.ParseStreamLine("")
	assert(t, token == "", "empty line should return empty token")
	assert(t, !done, "empty line should not be done")

	// Non-data line
	token, _, done = rt.ParseStreamLine("event: message")
	assert(t, token == "", "non-data line should return empty token")
	assert(t, !done, "non-data line should not be done")

	// finish_reason stop
	_, _, done = rt.ParseStreamLine(`data: {"choices":[{"delta":{"content":""},"finish_reason":"stop"}]}`)
	assert(t, done, "finish_reason stop should be done")
}

func TestRuntime_OpenClawEnvKeys(t *testing.T) {
	rt := Get("openclaw")
	assert(t, rt != nil, "openclaw runtime not found")
	keys := rt.EnvKeys()
	assert(t, len(keys) == 3, "should have 3 env keys")
	assert(t, keys[0] == "OPENCLAW_GATEWAY_URL", "first key should be OPENCLAW_GATEWAY_URL")
	assert(t, keys[1] == "OPENCLAW_GATEWAY_TOKEN", "second key should be OPENCLAW_GATEWAY_TOKEN")
	assert(t, keys[2] == "OPENCLAW_AGENT_ID", "third key should be OPENCLAW_AGENT_ID")
}

func TestRuntime_OpenClawSetupCommands(t *testing.T) {
	rt := Get("openclaw")
	assert(t, rt != nil, "openclaw runtime not found")
	cmds := rt.SetupCommands()
	assert(t, len(cmds) == 1, "should have 1 setup command")
	assert(t, strings.Contains(cmds[0], "curl"), "setup command should install curl")
}
