package slack

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/x"
)

func init() {
	channel.Register("slack", func(cfg x.TypedLazyConfig, opts ...any) (channel.Channel, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		return New(c)
	})
}

// Config for the Slack channel.
type Config struct {
	BotToken string `json:"bot_token" yaml:"bot_token"` // xoxb-...
	AppToken string `json:"app_token" yaml:"app_token"` // xapp-... (Socket Mode)
}

// Slack implements channel.Channel using Socket Mode.
type Slack struct {
	api    *slack.Client
	cfg    Config
	handler channel.Handler
	logger  *slog.Logger
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	botID   string
}

// New creates a Slack channel.
func New(cfg Config) (*Slack, error) {
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("slack: bot_token is required")
	}
	if cfg.AppToken == "" {
		return nil, fmt.Errorf("slack: app_token is required")
	}

	api := slack.New(cfg.BotToken, slack.OptionAppLevelToken(cfg.AppToken))

	return &Slack{
		api:    api,
		cfg:    cfg,
		logger: slog.Default(),
	}, nil
}

func (s *Slack) Name() string { return "slack" }

// Start connects via Socket Mode and dispatches messages.
func (s *Slack) Start(ctx context.Context, handler channel.Handler) error {
	s.handler = handler

	ctx, s.cancel = context.WithCancel(ctx)

	// Get bot user ID for mention stripping.
	auth, err := s.api.AuthTest()
	if err != nil {
		return fmt.Errorf("slack: auth test: %w", err)
	}
	s.botID = auth.UserID

	sm := socketmode.New(s.api)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-sm.Events:
				if !ok {
					return
				}
				s.handleEvent(ctx, sm, evt)
			}
		}
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := sm.RunContext(ctx); err != nil && ctx.Err() == nil {
			s.logger.Error("slack socket mode error", "err", err)
		}
	}()

	s.logger.Info("slack channel started", "bot", auth.User)
	return nil
}

func (s *Slack) handleEvent(ctx context.Context, sm *socketmode.Client, evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeEventsAPI:
		eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}
		sm.Ack(*evt.Request)

		switch ev := eventsAPI.InnerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			s.dispatch(ctx, ev.Channel, ev.User, s.stripMention(ev.Text), ev.TimeStamp, ev.ThreadTimeStamp)
		case *slackevents.MessageEvent:
			// Only handle DMs (im channel type).
			if ev.ChannelType == "im" && ev.BotID == "" {
				s.dispatch(ctx, ev.Channel, ev.User, ev.Text, ev.TimeStamp, ev.ThreadTimeStamp)
			}
		}
	}
}

func (s *Slack) dispatch(ctx context.Context, chID, userID, text, ts, threadTS string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	msg := &channel.Message{
		ID:       ts,
		ChatID:   chID,
		UserID:   userID,
		Username: userID, // Slack events don't include display name directly
		Text:     text,
		Extra:    map[string]string{"channel": "slack"},
	}
	if threadTS != "" {
		msg.ReplyToID = threadTS
	}

	if err := s.handler(ctx, msg); err != nil {
		s.logger.Error("handle message failed", "chat_id", chID, "err", err)
	}
}

func (s *Slack) stripMention(text string) string {
	// Slack mentions look like <@U12345>
	mention := fmt.Sprintf("<@%s>", s.botID)
	text = strings.ReplaceAll(text, mention, "")
	return strings.TrimSpace(text)
}

// Send posts a message to a Slack channel.
func (s *Slack) Send(ctx context.Context, chatID string, text string, opts *channel.SendOptions) error {
	options := []slack.MsgOption{slack.MsgOptionText(text, false)}

	if opts != nil && opts.ReplyToID != "" {
		options = append(options, slack.MsgOptionTS(opts.ReplyToID))
	}

	_, _, err := s.api.PostMessage(chatID, options...)
	return err
}

// Stop gracefully disconnects from Slack.
func (s *Slack) Stop(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.logger.Info("slack channel stopped")
	return nil
}
