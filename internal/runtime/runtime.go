package runtime

import "sync"

// Runtime abstracts agent CLI differences across providers.
type Runtime interface {
	Name() string
	Image() string
	BuildExecArgs(message string, continued bool) []string
	ParseStreamLine(line string) (token string, result string, done bool)
	EnvKeys() []string       // required env var names (e.g. ["ANTHROPIC_API_KEY"])
	SetupCommands() []string // commands to run on first exec (e.g. pip install)
}

var (
	mu       sync.RWMutex
	registry = map[string]Runtime{}
)

// Register adds a runtime to the global registry.
func Register(name string, rt Runtime) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = rt
}

// Get returns a runtime by name. Returns nil if not found.
func Get(name string) Runtime {
	mu.RLock()
	defer mu.RUnlock()
	return registry[name]
}

// Default returns the default runtime ("claude").
func Default() Runtime {
	return Get("claude")
}

// RuntimeInfo describes a registered runtime for API responses.
type RuntimeInfo struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	EnvKeys []string `json:"env_keys"`
}

// List returns info about all registered runtimes.
func List() []RuntimeInfo {
	mu.RLock()
	defer mu.RUnlock()
	var list []RuntimeInfo
	for _, rt := range registry {
		list = append(list, RuntimeInfo{
			Name:    rt.Name(),
			Image:   rt.Image(),
			EnvKeys: rt.EnvKeys(),
		})
	}
	return list
}
