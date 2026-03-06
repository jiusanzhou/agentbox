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

// Handler is called for each incoming message.
type Handler func(ctx context.Context, msg *Message) error

// Channel is a pluggable IM platform adapter.
type Channel interface {
	Name() string
	Start(ctx context.Context, handler Handler) error
	Send(ctx context.Context, chatID string, text string, opts *SendOptions) error
	Stop(ctx context.Context) error
}

// New creates a Channel from config using the factory.
func New(cfg x.TypedLazyConfig) (Channel, error) {
	return Create(cfg)
}
