package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/x"
)

func init() {
	channel.Register("telegram", func(cfg x.TypedLazyConfig, opts ...any) (channel.Channel, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		return New(c)
	})
}

// Config for the Telegram channel.
type Config struct {
	Token string `json:"token" yaml:"token"`
}

// Telegram implements channel.Channel using long polling.
type Telegram struct {
	bot       *tgbotapi.BotAPI
	handler   channel.Handler
	cbHandler channel.CallbackHandler
	logger    *slog.Logger
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// New creates a Telegram channel.
func New(cfg Config) (*Telegram, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram: token is required")
	}

	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("telegram: %w", err)
	}

	return &Telegram{
		bot:    bot,
		logger: slog.Default(),
	}, nil
}

func (t *Telegram) Name() string { return "telegram" }

// Start begins long polling and dispatches messages to the handler.
func (t *Telegram) Start(ctx context.Context, handler channel.Handler) error {
	t.handler = handler

	ctx, t.cancel = context.WithCancel(ctx)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := t.bot.GetUpdatesChan(u)

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.logger.Info("telegram polling started", "bot", t.bot.Self.UserName)

		for {
			select {
			case <-ctx.Done():
				return
			case update, ok := <-updates:
				if !ok {
					return
				}

				// Handle callback queries (button clicks).
				if update.CallbackQuery != nil {
					t.handleCallbackQuery(ctx, update.CallbackQuery)
					continue
				}

				if update.Message == nil {
					continue
				}

				msg := &channel.Message{
					ID:       strconv.Itoa(update.Message.MessageID),
					ChatID:   strconv.FormatInt(update.Message.Chat.ID, 10),
					UserID:   strconv.FormatInt(update.Message.From.ID, 10),
					Username: update.Message.From.UserName,
					Text:     update.Message.Text,
					Extra:    map[string]string{"channel": "telegram"},
				}

				if update.Message.ReplyToMessage != nil {
					msg.ReplyToID = strconv.Itoa(update.Message.ReplyToMessage.MessageID)
				}

				if err := t.handler(ctx, msg); err != nil {
					t.logger.Error("handle message failed", "chat_id", msg.ChatID, "err", err)
				}
			}
		}
	}()

	return nil
}

func (t *Telegram) handleCallbackQuery(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	// Acknowledge the callback to remove the "loading" state.
	callback := tgbotapi.NewCallback(cq.ID, "")
	t.bot.Request(callback)

	if t.cbHandler == nil {
		return
	}

	chatID := ""
	msgID := ""
	if cq.Message != nil {
		chatID = strconv.FormatInt(cq.Message.Chat.ID, 10)
		msgID = strconv.Itoa(cq.Message.MessageID)
	}

	cb := &channel.Callback{
		ID:        cq.Data,
		ChatID:    chatID,
		UserID:    strconv.FormatInt(cq.From.ID, 10),
		MessageID: msgID,
		Extra:     map[string]string{"channel": "telegram"},
	}

	if err := t.cbHandler(ctx, cb); err != nil {
		t.logger.Error("handle callback failed", "data", cq.Data, "err", err)
	}
}

// Send sends a message to a Telegram chat.
func (t *Telegram) Send(ctx context.Context, chatID string, text string, opts *channel.SendOptions) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat_id %q: %w", chatID, err)
	}

	msg := tgbotapi.NewMessage(id, text)

	if opts != nil {
		if opts.ReplyToID != "" {
			if replyID, err := strconv.Atoi(opts.ReplyToID); err == nil {
				msg.ReplyToMessageID = replyID
			}
		}
		if opts.ParseMode == "markdown" {
			msg.ParseMode = tgbotapi.ModeMarkdown
		} else if opts.ParseMode == "html" {
			msg.ParseMode = tgbotapi.ModeHTML
		}
	}

	_, err = t.bot.Send(msg)
	return err
}

// EditMessage edits an existing Telegram message.
func (t *Telegram) EditMessage(ctx context.Context, chatID string, messageID string, text string, opts *channel.SendOptions) error {
	cid, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat_id %q: %w", chatID, err)
	}
	mid, err := strconv.Atoi(messageID)
	if err != nil {
		return fmt.Errorf("telegram: invalid message_id %q: %w", messageID, err)
	}

	edit := tgbotapi.NewEditMessageText(cid, mid, text)
	if opts != nil {
		if opts.ParseMode == "markdown" {
			edit.ParseMode = tgbotapi.ModeMarkdown
		} else if opts.ParseMode == "html" {
			edit.ParseMode = tgbotapi.ModeHTML
		}
	}

	_, err = t.bot.Send(edit)
	return err
}

// SendWithButtons sends a message with inline keyboard buttons.
func (t *Telegram) SendWithButtons(ctx context.Context, chatID string, text string, buttons []channel.Button, opts *channel.SendOptions) (string, error) {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("telegram: invalid chat_id %q: %w", chatID, err)
	}

	msg := tgbotapi.NewMessage(id, text)
	if opts != nil {
		if opts.ReplyToID != "" {
			if replyID, err := strconv.Atoi(opts.ReplyToID); err == nil {
				msg.ReplyToMessageID = replyID
			}
		}
		if opts.ParseMode == "markdown" {
			msg.ParseMode = tgbotapi.ModeMarkdown
		} else if opts.ParseMode == "html" {
			msg.ParseMode = tgbotapi.ModeHTML
		}
	}

	// Build inline keyboard.
	var row []tgbotapi.InlineKeyboardButton
	for _, b := range buttons {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(b.Text, b.ID))
	}
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row)

	sent, err := t.bot.Send(msg)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(sent.MessageID), nil
}

// OnCallback registers a handler for button click callbacks.
func (t *Telegram) OnCallback(handler channel.CallbackHandler) {
	t.cbHandler = handler
}

// Stop gracefully stops polling.
func (t *Telegram) Stop(ctx context.Context) error {
	if t.cancel != nil {
		t.cancel()
	}
	t.bot.StopReceivingUpdates()
	t.wg.Wait()
	t.logger.Info("telegram channel stopped")
	return nil
}
