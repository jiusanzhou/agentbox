package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	goplugin "plugin"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader loads plugins from disk or remote sources.
type Loader struct {
	registry *Registry
	logger   *slog.Logger
}

// NewLoader creates a new plugin loader attached to the given registry.
func NewLoader(registry *Registry, logger *slog.Logger) *Loader {
	return &Loader{
		registry: registry,
		logger:   logger,
	}
}

// LoadFromDir scans a directory for Go plugin shared objects (.so files).
// Each .so must export a "NewPlugin" symbol: func() Plugin.
func (l *Loader) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			l.logger.Debug("plugin directory does not exist", "dir", dir)
			return nil
		}
		return fmt.Errorf("read plugin dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".so") {
			continue
		}
		path := filepath.Join(dir, name)
		if err := l.loadSharedObject(path); err != nil {
			l.logger.Warn("failed to load plugin", "path", path, "err", err)
			continue
		}
	}
	return nil
}

// LoadFromConfig loads a plugin based on a PluginConfig entry.
// Supports local .so files (via Path) and remote plugins (via URL, not yet implemented).
func (l *Loader) LoadFromConfig(cfg PluginConfig) error {
	if cfg.Path != "" {
		// Check for plugin.yaml manifest alongside the plugin
		manifestPath := filepath.Join(filepath.Dir(cfg.Path), "plugin.yaml")
		if _, err := os.Stat(manifestPath); err == nil {
			manifest, err := loadManifest(manifestPath)
			if err != nil {
				l.logger.Warn("failed to load plugin manifest", "path", manifestPath, "err", err)
			} else {
				l.logger.Debug("loaded plugin manifest", "name", manifest.Name, "version", manifest.Version)
			}
		}

		if err := l.loadSharedObject(cfg.Path); err != nil {
			return err
		}

		// Initialize with config if provided
		if cfg.Config != nil {
			p, ok := l.registry.Get(cfg.Name)
			if ok {
				if err := p.Init(cfg.Config); err != nil {
					return fmt.Errorf("init plugin %s: %w", cfg.Name, err)
				}
			}
		}
		return nil
	}

	if cfg.URL != "" {
		// Remote plugin loading via HTTP (gRPC/JSON-RPC) is a future extension.
		return fmt.Errorf("remote plugin loading not yet implemented for %s (url: %s)", cfg.Name, cfg.URL)
	}

	return fmt.Errorf("plugin %s: must specify either path or url", cfg.Name)
}

func (l *Loader) loadSharedObject(path string) error {
	p, err := goplugin.Open(path)
	if err != nil {
		return fmt.Errorf("open plugin %s: %w", path, err)
	}

	sym, err := p.Lookup("NewPlugin")
	if err != nil {
		return fmt.Errorf("plugin %s: missing NewPlugin symbol: %w", path, err)
	}

	newFn, ok := sym.(func() Plugin)
	if !ok {
		return fmt.Errorf("plugin %s: NewPlugin has wrong signature (want func() Plugin)", path)
	}

	plug := newFn()
	return l.registry.Register(plug)
}

func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
