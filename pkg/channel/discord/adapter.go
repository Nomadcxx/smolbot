package discord

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/bwmarrin/discordgo"
)

const channelName = "discord"

type clientSeam interface {
	Start(context.Context, func(rawInboundMessage) error) error
	Stop(context.Context) error
	Send(context.Context, string, string) error
	Identify(context.Context) (string, error)
}

type rawInboundMessage struct {
	ChannelID string
	AuthorID  string
	Content   string
}

type Adapter struct {
	seam clientSeam

	mu         sync.RWMutex
	status     channel.Status
	channelIDs map[string]struct{}
	enforce    bool
}

var newDiscordSeamFactory = func(cfg config.DiscordChannelConfig) (clientSeam, error) {
	return newDiscordGoSeam(cfg)
}

func NewAdapter(seam clientSeam) *Adapter {
	return &Adapter{
		seam:   seam,
		status: channel.Status{State: "disconnected"},
	}
}

func NewProductionAdapter(cfg config.DiscordChannelConfig) (*Adapter, error) {
	token, err := loadDiscordToken(cfg)
	if err != nil {
		return nil, err
	}
	cfg.BotToken = token

	seam, err := newDiscordSeamFactory(cfg)
	if err != nil {
		return nil, err
	}

	adapter := NewAdapter(seam)
	adapter.channelIDs = normalizeAllowedChannelIDs(cfg.AllowedChannelIDs)
	adapter.enforce = len(adapter.channelIDs) > 0
	return adapter, nil
}

func (a *Adapter) Name() string {
	return channelName
}

func (a *Adapter) Start(ctx context.Context, handler channel.Handler) error {
	if handler == nil {
		return errors.New("discord handler is required")
	}
	if a.seam == nil {
		return errors.New("discord client seam is required")
	}

	a.updateStatus(channel.Status{State: "connecting"})
	err := a.seam.Start(ctx, func(raw rawInboundMessage) error {
		channelID := strings.TrimSpace(raw.ChannelID)
		if !a.isAllowedChannel(channelID) {
			return nil
		}
		if strings.TrimSpace(raw.Content) == "" {
			return nil
		}
		handler(ctx, channel.InboundMessage{
			Channel: channelName,
			ChatID:  channelID,
			Content: strings.TrimSpace(raw.Content),
		})
		return nil
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

	if err := a.seam.Stop(ctx); err != nil {
		a.updateStatus(channel.Status{State: "error", Detail: strings.TrimSpace(err.Error())})
		return err
	}

	a.updateStatus(channel.Status{State: "disconnected"})
	return nil
}

func (a *Adapter) Send(ctx context.Context, msg channel.OutboundMessage) error {
	if a.seam == nil {
		return errors.New("discord client seam is required")
	}

	channelID := strings.TrimSpace(msg.ChatID)
	if channelID == "" {
		return errors.New("discord channel ID is required")
	}

	for _, chunk := range channel.ChunkMessage(msg.Content, 2000) {
		if err := a.seam.Send(ctx, channelID, chunk); err != nil {
			return fmt.Errorf("discord send: %w", err)
		}
	}
	return nil
}

func (a *Adapter) Status(context.Context) (channel.Status, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status, nil
}

func (a *Adapter) Login(ctx context.Context) error {
	if a.seam == nil {
		return errors.New("discord client seam is required")
	}

	connecting := channel.Status{State: "connecting", Detail: "Validating bot token..."}
	a.updateStatus(connecting)

	name, err := a.seam.Identify(ctx)
	if err != nil {
		status := channel.Status{State: "auth-required", Detail: fmt.Sprintf("Invalid token: %v", err)}
		a.updateStatus(status)
		return err
	}

	connected := channel.Status{State: "connected", Detail: fmt.Sprintf("Bot: @%s", strings.TrimSpace(name))}
	a.updateStatus(connected)
	return nil
}

func (a *Adapter) updateStatus(status channel.Status) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = status
}

func (a *Adapter) isAllowedChannel(channelID string) bool {
	if !a.enforce {
		return true
	}
	_, ok := a.channelIDs[strings.TrimSpace(channelID)]
	return ok
}

func normalizeAllowedChannelIDs(channelIDs []string) map[string]struct{} {
	if len(channelIDs) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(channelIDs))
	for _, channelID := range channelIDs {
		channelID = strings.TrimSpace(channelID)
		if channelID == "" {
			continue
		}
		allowed[channelID] = struct{}{}
	}

	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func loadDiscordToken(cfg config.DiscordChannelConfig) (string, error) {
	token := strings.TrimSpace(cfg.BotToken)
	if token == "" && strings.TrimSpace(cfg.TokenFile) != "" {
		data, err := os.ReadFile(cfg.TokenFile)
		if err != nil {
			return "", fmt.Errorf("read discord token file: %w", err)
		}
		token = strings.TrimSpace(string(data))
	}
	if token == "" {
		return "", errors.New("discord bot token required")
	}
	return token, nil
}

type discordGoSeam struct {
	session *discordgo.Session
}

func newDiscordGoSeam(cfg config.DiscordChannelConfig) (clientSeam, error) {
	session, err := discordgo.New("Bot " + cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}
	session.Identify.Intents = discordgo.MakeIntent(session.Identify.Intents | discordgo.IntentMessageContent)
	return &discordGoSeam{session: session}, nil
}

func (s *discordGoSeam) Start(ctx context.Context, handle func(rawInboundMessage) error) error {
	if s.session == nil {
		return errors.New("discord session is required")
	}

	s.session.AddHandler(func(_ *discordgo.Session, m *discordgo.MessageCreate) {
		if m == nil || m.Author == nil || m.Author.Bot {
			return
		}
		_ = handle(rawInboundMessage{
			ChannelID: m.ChannelID,
			AuthorID:  m.Author.ID,
			Content:   m.Content,
		})
	})

	if err := s.session.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}

	go func() {
		<-ctx.Done()
		_ = s.Stop(context.Background())
	}()
	return nil
}

func (s *discordGoSeam) Stop(context.Context) error {
	if s.session == nil {
		return nil
	}
	err := s.session.Close()
	s.session = nil
	return err
}

func (s *discordGoSeam) Send(_ context.Context, channelID, content string) error {
	if s.session == nil {
		return errors.New("discord session is required")
	}
	_, err := s.session.ChannelMessageSend(channelID, content)
	return err
}

func (s *discordGoSeam) Identify(_ context.Context) (string, error) {
	if s.session == nil {
		return "", errors.New("discord session is required")
	}
	user, err := s.session.User("@me")
	if err != nil {
		return "", err
	}
	return user.Username, nil
}
