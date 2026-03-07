package discord

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/x"
)

var mentionRe = regexp.MustCompile(`<@!?\d+>`)

func init() {
	channel.Register("discord", func(cfg x.TypedLazyConfig, opts ...any) (channel.Channel, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		return New(c)
	})
}

// Config for the Discord channel.
type Config struct {
	Token   string `json:"token" yaml:"token"`
	GuildID string `json:"guild_id" yaml:"guild_id"` // optional: limit to one guild
}

// Discord implements channel.Channel using discordgo WebSocket.
type Discord struct {
	cfg     Config
	session *discordgo.Session
	handler channel.Handler
	logger  *slog.Logger
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// New creates a Discord channel.
func New(cfg Config) (*Discord, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("discord: token is required")
	}

	s, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("discord: %w", err)
	}

	return &Discord{
		cfg:     cfg,
		session: s,
		logger:  slog.Default(),
	}, nil
}

func (d *Discord) Name() string { return "discord" }

// Start connects to Discord and dispatches messages to the handler.
func (d *Discord) Start(ctx context.Context, handler channel.Handler) error {
	d.handler = handler

	ctx, d.cancel = context.WithCancel(ctx)

	d.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Ignore own messages.
		if m.Author.ID == s.State.User.ID {
			return
		}
		// Ignore bots.
		if m.Author.Bot {
			return
		}
		// Optional guild filter.
		if d.cfg.GuildID != "" && m.GuildID != "" && m.GuildID != d.cfg.GuildID {
			return
		}

		// Only respond to DMs or mentions.
		isDM := m.GuildID == ""
		isMentioned := false
		for _, u := range m.Mentions {
			if u.ID == s.State.User.ID {
				isMentioned = true
				break
			}
		}
		if !isDM && !isMentioned {
			return
		}

		// Strip bot mention from text.
		text := strings.TrimSpace(mentionRe.ReplaceAllString(m.Content, ""))
		if text == "" {
			return
		}

		msg := &channel.Message{
			ID:       m.ID,
			ChatID:   m.ChannelID,
			UserID:   m.Author.ID,
			Username: m.Author.Username,
			Text:     text,
			Extra:    map[string]string{"channel": "discord", "guild_id": m.GuildID},
		}

		if ref := m.MessageReference; ref != nil {
			msg.ReplyToID = ref.MessageID
		}

		if err := d.handler(ctx, msg); err != nil {
			d.logger.Error("handle message failed", "chat_id", msg.ChatID, "err", err)
		}
	})

	d.session.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentMessageContent

	if err := d.session.Open(); err != nil {
		return fmt.Errorf("discord: open session: %w", err)
	}

	d.logger.Info("discord channel started", "user", d.session.State.User.Username)

	// Wait for context cancellation to close session.
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		<-ctx.Done()
		d.session.Close()
	}()

	return nil
}

// Send sends a message to a Discord channel.
func (d *Discord) Send(ctx context.Context, chatID string, text string, opts *channel.SendOptions) error {
	if opts != nil && opts.ReplyToID != "" {
		_, err := d.session.ChannelMessageSendReply(chatID, text, &discordgo.MessageReference{
			MessageID: opts.ReplyToID,
			ChannelID: chatID,
		})
		return err
	}
	_, err := d.session.ChannelMessageSend(chatID, text)
	return err
}

// Stop gracefully closes the Discord connection.
func (d *Discord) Stop(ctx context.Context) error {
	if d.cancel != nil {
		d.cancel()
	}
	d.wg.Wait()
	d.logger.Info("discord channel stopped")
	return nil
}
