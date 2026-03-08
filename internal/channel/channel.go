package channel

import (
	"context"

	"go.zoe.im/x"
	"go.zoe.im/x/factory"
)

var (
	channelFactory = factory.NewFactory[Channel, any]()
	Create         = channelFactory.Create
	Register       = channelFactory.Register
)

// Message represents an incoming message from an IM platform.
type Message struct {
	ID        string            // platform message ID
	ChatID    string            // chat/group identifier
	UserID    string            // sender ID
	Username  string            // sender display name
	Text      string            // message content
	ReplyToID string            // for threading
	Extra     map[string]string // platform-specific metadata
}

// SendOptions controls how a message is sent.
type SendOptions struct {
	ReplyToID string
	ParseMode string // "markdown", "html", ""
}

// Button represents an inline button attached to a message.
type Button struct {
	ID   string // callback data identifier
	Text string // display label
}

// Callback represents a button click from a user.
type Callback struct {
	ID        string // button ID / callback data
	ChatID    string
	UserID    string
	MessageID string
	Extra     map[string]string
}

// CallbackHandler is called for each button click.
type CallbackHandler func(ctx context.Context, cb *Callback) error

// Handler is called for each incoming message.
type Handler func(ctx context.Context, msg *Message) error

// Channel is a pluggable IM platform adapter.
type Channel interface {
	Name() string
	Start(ctx context.Context, handler Handler) error
	Send(ctx context.Context, chatID string, text string, opts *SendOptions) error
	Stop(ctx context.Context) error

	// EditMessage edits an existing message. Returns error if unsupported.
	EditMessage(ctx context.Context, chatID string, messageID string, text string, opts *SendOptions) error

	// SendWithButtons sends a message with inline buttons.
	// Returns the platform message ID.
	SendWithButtons(ctx context.Context, chatID string, text string, buttons []Button, opts *SendOptions) (string, error)

	// OnCallback registers a handler for button click callbacks.
	OnCallback(handler CallbackHandler)
}

// New creates a Channel from config using the factory.
func New(cfg x.TypedLazyConfig, opts ...any) (Channel, error) {
	return Create(cfg, opts...)
}
