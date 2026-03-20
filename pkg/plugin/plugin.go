package plugin

import (
	"fmt"
	"log/slog"
	"sync"
)

// Registry holds all registered plugins keyed by name.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	logger  *slog.Logger
}

// NewRegistry creates a new plugin registry.
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
		logger:  logger,
	}
}

// Register adds a plugin to the registry.
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := p.Name()
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin %q already registered", name)
	}
	r.plugins[name] = p
	r.logger.Info("plugin registered", "name", name, "type", p.Type(), "version", p.Version())
	return nil
}

// Get returns a plugin by name.
func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List returns all registered plugins.
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}

// ListByType returns all plugins of a given type.
func (r *Registry) ListByType(t PluginType) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Plugin
	for _, p := range r.plugins {
		if p.Type() == t {
			result = append(result, p)
		}
	}
	return result
}

// GetExecutor returns a registered executor plugin by name.
func (r *Registry) GetExecutor(name string) (ExecutorPlugin, error) {
	p, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("executor plugin %q not found", name)
	}
	ep, ok := p.(ExecutorPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %q is not an executor plugin", name)
	}
	return ep, nil
}

// GetChannel returns a registered channel plugin by name.
func (r *Registry) GetChannel(name string) (ChannelPlugin, error) {
	p, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("channel plugin %q not found", name)
	}
	cp, ok := p.(ChannelPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %q is not a channel plugin", name)
	}
	return cp, nil
}

// GetTool returns a registered tool plugin by name.
func (r *Registry) GetTool(name string) (ToolPlugin, error) {
	p, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool plugin %q not found", name)
	}
	tp, ok := p.(ToolPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %q is not a tool plugin", name)
	}
	return tp, nil
}
