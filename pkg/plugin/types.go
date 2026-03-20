package plugin

import (
	"context"
	"encoding/json"

	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/storage"
	"go.zoe.im/agentbox/internal/store"
)

// PluginType identifies the category of a plugin.
type PluginType string

const (
	PluginTypeExecutor PluginType = "executor"
	PluginTypeChannel  PluginType = "channel"
	PluginTypeStorage  PluginType = "storage"
	PluginTypeTool     PluginType = "tool"
)

// Plugin is the base interface all plugins must implement.
type Plugin interface {
	Name() string
	Version() string
	Type() PluginType
	Init(config json.RawMessage) error
}

// ExecutorPlugin extends executor.Executor for plugin-based executors.
type ExecutorPlugin interface {
	Plugin
	executor.Executor
}

// ChannelPlugin extends channel.Channel for plugin-based channels.
type ChannelPlugin interface {
	Plugin
	channel.Channel
}

// StoragePlugin extends storage.Storage for plugin-based storage backends.
type StoragePlugin interface {
	Plugin
	storage.Storage
}

// StorePlugin extends store.Store for plugin-based store backends.
type StorePlugin interface {
	Plugin
	store.Store
}

// ToolPlugin provides a generic tool execution interface.
type ToolPlugin interface {
	Plugin
	Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

// Manifest describes a plugin package loaded from plugin.yaml.
type Manifest struct {
	Name       string `json:"name" yaml:"name"`
	Version    string `json:"version" yaml:"version"`
	Type       string `json:"type" yaml:"type"`
	Entrypoint string `json:"entrypoint" yaml:"entrypoint"`
}

// PluginConfig describes how to load a single plugin.
type PluginConfig struct {
	Name   string          `json:"name" yaml:"name"`
	Type   string          `json:"type" yaml:"type"`
	Path   string          `json:"path,omitempty" yaml:"path"`
	URL    string          `json:"url,omitempty" yaml:"url"`
	Config json.RawMessage `json:"config,omitempty" yaml:"config"`
}
