package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const channelName = "telegram"

type clientSeam interface {
	Start(ctx context.Context, handler func(chatID int64, text string)) error
	Stop(ctx context.Context) error
	Send(ctx context.Context, chatID int64, text string) error
	GetMe(ctx context.Context) (botName string, err error)
}

type telegramBot interface {
	Start(ctx context.Context)
	Close(ctx context.Context) (bool, error)
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	GetMe(ctx context.Context) (*models.User, error)
}

var newTelegramSeam = func(token string) (clientSeam, error) {
	return &telegramSeam{token: token}, nil
}

var newTelegramBot = func(token string, opts ...bot.Option) (telegramBot, error) {
	return bot.New(token, opts...)
}

type Adapter struct {
	seam     clientSeam
	mu       sync.RWMutex
	status   channel.Status
	chatIDs  map[string]struct{}
	enforce  bool
	recent   map[string]time.Time
	recentMu sync.Mutex
}

func NewAdapter(seam clientSeam) *Adapter {
	return &Adapter{
		seam:    seam,
		status:  channel.Status{State: "disconnected"},
		chatIDs: make(map[string]struct{}),
		recent:  make(map[string]time.Time),
	}
}

func NewProductionAdapter(cfg config.TelegramChannelConfig) (*Adapter, error) {
	token := cfg.BotToken
	if token == "" && cfg.TokenFile != "" {
		data, err := os.ReadFile(cfg.TokenFile)
		if err != nil {
			return nil, fmt.Errorf("read telegram token file: %w", err)
		}
		token = strings.TrimSpace(string(data))
	}
	if token == "" {
		return nil, errors.New("telegram bot token required")
	}

	seam, err := newTelegramSeam(token)
	if err != nil {
		return nil, err
	}

	adapter := NewAdapter(seam)
	adapter.chatIDs = normalizeAllowedChatIDs(cfg.AllowedChatIDs)
	adapter.enforce = len(adapter.chatIDs) > 0
	return adapter, nil
}

func (a *Adapter) Name() string {
	return channelName
}

func (a *Adapter) Start(ctx context.Context, handler channel.Handler) error {
	if handler == nil {
		return errors.New("telegram handler is required")
	}
	if a.seam == nil {
		return errors.New("telegram client seam is required")
	}
	a.updateStatus(channel.Status{State: "connecting"})
	err := a.seam.Start(ctx, func(chatID int64, text string) {
		chatIDStr := strconv.FormatInt(chatID, 10)

		if !a.isAllowedChat(chatIDStr) {
			return
		}

		if a.isDuplicateInbound(chatIDStr, text) {
			return
		}

		handler(ctx, channel.InboundMessage{
			Channel: channelName,
			ChatID:  chatIDStr,
			Content: text,
		})
	})
	if err != nil {
		a.updateStatus(channel.Status{State: "error", Detail: strings.TrimSpace(err.Error())})
		return err
	}
	a.updateStatus(channel.Status{State: "connected"})
	return nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	if a.seam == nil {
		a.updateStatus(channel.Status{State: "disconnected"})
		return nil
	}
	err := a.seam.Stop(ctx)
	if err != nil {
		a.updateStatus(channel.Status{State: "error", Detail: strings.TrimSpace(err.Error())})
		return err
	}
	a.updateStatus(channel.Status{State: "disconnected"})
	return nil
}

func (a *Adapter) Send(ctx context.Context, msg channel.OutboundMessage) error {
	if a.seam == nil {
		return errors.New("telegram client seam is required")
	}
	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid telegram chat ID %q: %w", msg.ChatID, err)
	}

	chunks := channel.ChunkMessage(msg.Content, 4096)
	for _, chunk := range chunks {
		if err := a.seam.Send(ctx, chatID, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) Status(context.Context) (channel.Status, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status, nil
}

func (a *Adapter) LoginWithUpdates(ctx context.Context, report func(channel.Status) error) error {
	if a.seam == nil {
		return errors.New("telegram client seam is required")
	}
	connecting := channel.Status{State: "connecting", Detail: "Validating bot token..."}
	a.updateStatus(connecting)
	if err := reportStatus(report, connecting); err != nil {
		return err
	}

	name, err := a.seam.GetMe(ctx)
	if err != nil {
		status := channel.Status{State: "auth-required", Detail: fmt.Sprintf("Invalid token: %v", err)}
		a.updateStatus(status)
		if reportErr := reportStatus(report, status); reportErr != nil {
			return reportErr
		}
		return err
	}

	connected := channel.Status{State: "connected", Detail: fmt.Sprintf("Bot: @%s", name)}
	a.updateStatus(connected)
	return reportStatus(report, connected)
}

type telegramSeam struct {
	bot   telegramBot
	token string
}

func (s *telegramSeam) Start(ctx context.Context, handler func(chatID int64, text string)) error {
	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			if update.Message == nil || update.Message.Text == "" {
				return
			}
			handler(update.Message.Chat.ID, update.Message.Text)
		}),
	}

	b, err := newTelegramBot(s.token, opts...)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}
	s.bot = b

	go b.Start(ctx)
	return nil
}

func (s *telegramSeam) Stop(ctx context.Context) error {
	botClient := s.bot
	s.bot = nil
	if botClient != nil {
		_, err := botClient.Close(ctx)
		return err
	}
	return nil
}

func (s *telegramSeam) Send(ctx context.Context, chatID int64, text string) error {
	if s.bot == nil {
		return errors.New("telegram bot not started")
	}
	_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
	return err
}

func (s *telegramSeam) GetMe(ctx context.Context) (string, error) {
	if s.bot == nil {
		b, err := newTelegramBot(s.token)
		if err != nil {
			return "", fmt.Errorf("create telegram bot: %w", err)
		}
		s.bot = b
	}

	me, err := s.bot.GetMe(ctx)
	if err != nil {
		return "", err
	}
	return me.Username, nil
}

func (a *Adapter) updateStatus(status channel.Status) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = status
}

func (a *Adapter) isAllowedChat(chatID string) bool {
	if !a.enforce {
		return true
	}
	_, ok := a.chatIDs[chatID]
	return ok
}

func (a *Adapter) isDuplicateInbound(chatID, text string) bool {
	dedupKey := fmt.Sprintf("%s:%s", chatID, text)
	now := time.Now()

	a.recentMu.Lock()
	defer a.recentMu.Unlock()

	if _, dup := a.recent[dedupKey]; dup {
		return true
	}

	a.recent[dedupKey] = now
	for k, v := range a.recent {
		if now.Sub(v) > 5*time.Minute {
			delete(a.recent, k)
		}
	}
	return false
}

func reportStatus(report func(channel.Status) error, status channel.Status) error {
	if report == nil {
		return nil
	}
	return report(status)
}

func normalizeAllowedChatIDs(chatIDs []string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, chatID := range chatIDs {
		trimmed := strings.TrimSpace(chatID)
		if trimmed == "" {
			continue
		}
		out[trimmed] = struct{}{}
	}
	return out
}
