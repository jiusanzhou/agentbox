package runtime

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Runtime abstracts agent CLI differences across providers.
type Runtime interface {
	Name() string
	Image() string
	BuildExecArgs(message string, continued bool) []string
	ParseStreamLine(line string) (token string, result string, done bool)
	EnvKeys() []string       // required env var names (e.g. ["ANTHROPIC_API_KEY"])
	SetupCommands() []string // commands to run on first exec (e.g. pip install)
	BinaryName() string      // binary name for local detection (e.g. "claude")
	InstallCommand() string  // shell command to install this runtime
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

// RuntimeStatus describes the availability of a runtime on the local system.
type RuntimeStatus struct {
	Name           string `json:"name"`
	BinaryName     string `json:"binary_name"`
	InstallCommand string `json:"install_command"`
	Available      bool   `json:"available"`
	Version        string `json:"version,omitempty"`
	Error          string `json:"error,omitempty"`
}

// IsChinaEnv returns whether China mirrors should be used.
func IsChinaEnv() bool { return isChinaEnv() }

// CheckRuntime checks if a specific runtime binary is available locally.
func CheckRuntime(name string) *RuntimeStatus {
	mu.RLock()
	rt := registry[name]
	mu.RUnlock()

	if rt == nil {
		return &RuntimeStatus{
			Name:  name,
			Error: "unknown runtime",
		}
	}

	status := &RuntimeStatus{
		Name:           rt.Name(),
		BinaryName:     rt.BinaryName(),
		InstallCommand: rt.InstallCommand(),
	}

	bin := rt.BinaryName()
	if bin == "" {
		// Not a locally installable runtime (custom, http, etc.)
		status.Available = true
		return status
	}

	// Check if binary exists
	path, err := exec.LookPath(bin)
	if err != nil {
		status.Available = false
		status.Error = bin + " not found in PATH"
		return status
	}
	status.Available = true

	// Try to get version
	cmd := exec.Command(path, "--version")
	out, err := cmd.Output()
	if err == nil {
		version := strings.TrimSpace(string(out))
		// Take first line only
		if idx := strings.IndexByte(version, '\n'); idx > 0 {
			version = version[:idx]
		}
		status.Version = version
	}

	return status
}

// CheckAll checks availability of all registered runtimes.
func CheckAll() []RuntimeStatus {
	mu.RLock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	mu.RUnlock()

	results := make([]RuntimeStatus, 0, len(names))
	for _, name := range names {
		results = append(results, *CheckRuntime(name))
	}
	return results
}


// npmMirrorRegistry is the China npm mirror.
const npmMirrorRegistry = "https://registry.npmmirror.com"

// pipMirrorIndex is the China pip mirror.
const pipMirrorIndex = "https://pypi.tuna.tsinghua.edu.cn/simple"

// goMirrorProxy is the China Go module proxy.
const goMirrorProxy = "https://goproxy.cn,direct"

// isChinaEnv detects if the system is likely in a Chinese network environment
// by checking timezone and locale settings.
func isChinaEnv() bool {
	// Check TZ env or system timezone
	tz := os.Getenv("TZ")
	if tz == "" {
		if loc, err := time.LoadLocation("Local"); err == nil {
			tz = loc.String()
		}
	}
	chinaZones := []string{"Asia/Shanghai", "Asia/Chongqing", "Asia/Chungking", "Asia/Harbin", "Asia/Urumqi", "PRC"}
	for _, z := range chinaZones {
		if strings.EqualFold(tz, z) {
			return true
		}
	}

	// Check LANG
	lang := os.Getenv("LANG")
	if strings.HasPrefix(lang, "zh_CN") {
		return true
	}

	// Check explicit env override
	if os.Getenv("ABOX_CHINA_MIRROR") == "1" || os.Getenv("ABOX_CHINA_MIRROR") == "true" {
		return true
	}

	return false
}

// WrapInstallCommand adjusts an install command for China mirror sources if needed.
func WrapInstallCommand(cmd string) string {
	if cmd == "" || !isChinaEnv() {
		return cmd
	}

	// npm install → npm install --registry=...
	if strings.Contains(cmd, "npm install") {
		cmd = strings.Replace(cmd, "npm install", "npm install --registry="+npmMirrorRegistry, 1)
	}

	// pip install → pip install -i ...
	if strings.Contains(cmd, "pip install") {
		cmd = strings.Replace(cmd, "pip install", "pip install -i "+pipMirrorIndex, 1)
	}

	// go install → GOPROXY=... go install
	if strings.HasPrefix(cmd, "go install") {
		cmd = "GOPROXY=" + goMirrorProxy + " " + cmd
	}

	return cmd
}
