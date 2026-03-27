package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waTypes "go.mau.fi/whatsmeow/types"
	waEvents "go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

const channelName = "whatsapp"

type clientSeam interface {
	Send(context.Context, string, string) error
	Start(context.Context, func(rawInboundMessage) error) error
	Stop(context.Context) error
	Login(context.Context, func(loginUpdate) error) error
}

type Adapter struct {
	seam clientSeam

	mu     sync.RWMutex
	status channel.Status

	allowedChatIDs   map[string]struct{}
	enforceAllowlist bool

	lastConnectedAt time.Time
	lastMessageAt   time.Time
	reconnectCount  int
}

type rawInboundMessage struct {
	ChatID  string
	Content string
}

type loginUpdate struct {
	State  string
	Detail string
}

var newWhatsAppSeamFactory = func(cfg config.WhatsAppChannelConfig) (clientSeam, error) {
	return newWhatsmeowSeam(cfg)
}

func NewAdapter(seam clientSeam) *Adapter {
	return &Adapter{
		seam:   seam,
		status: channel.Status{State: "disconnected"},
	}
}

func NewProductionAdapter(cfg config.WhatsAppChannelConfig) (*Adapter, error) {
	if strings.TrimSpace(cfg.StorePath) == "" {
		return nil, errors.New("whatsapp store path is required")
	}
	seam, err := newWhatsAppSeamFactory(cfg)
	if err != nil {
		return nil, err
	}
	adapter := NewAdapter(seam)
	adapter.enforceAllowlist = true
	adapter.allowedChatIDs = normalizeAllowedChatIDs(cfg.AllowedChatIDs)
	return adapter, nil
}

func (a *Adapter) Name() string {
	return channelName
}

func (a *Adapter) Start(ctx context.Context, handler channel.Handler) error {
	if handler == nil {
		return errors.New("whatsapp handler is required")
	}
	if a.seam == nil {
		return errors.New("whatsapp client seam is required")
	}
	if a.enforceAllowlist && len(a.allowedChatIDs) == 0 {
		log.Printf("[whatsapp] inbound allowlist empty; all inbound WhatsApp messages will be ignored")
	}
	a.updateStatus("connecting", "")
	log.Printf("[whatsapp] adapter starting...")
	err := a.seam.Start(ctx, func(raw rawInboundMessage) error {
		msg := raw.normalize()
		if !a.isAllowedChat(msg.ChatID) {
			log.Printf("[whatsapp] dropping inbound from disallowed chat %q", msg.ChatID)
			return nil
		}
		log.Printf("[whatsapp] raw inbound: chatID=%q content=%q", msg.ChatID, msg.Content)
		handler(ctx, msg)
		return nil
	})
	if err != nil {
		log.Printf("[whatsapp] adapter start failed: %v", err)
		a.updateStatus("error", strings.TrimSpace(err.Error()))
		return err
	}
	log.Printf("[whatsapp] adapter started successfully")
	a.updateStatus("connected", "")
	return nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	if a.seam == nil {
		return nil
	}
	err := a.seam.Stop(ctx)
	if err != nil {
		a.updateStatus("error", strings.TrimSpace(err.Error()))
		return err
	}
	a.updateStatus("disconnected", "")
	return nil
}

func (a *Adapter) Send(ctx context.Context, msg channel.OutboundMessage) error {
	if a.seam == nil {
		return errors.New("whatsapp client seam is required")
	}
	if err := a.seam.Send(ctx, strings.TrimSpace(msg.ChatID), msg.Content); err != nil {
		return fmt.Errorf("whatsapp send: %w", err)
	}
	return nil
}

func (a *Adapter) Status(context.Context) (channel.Status, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status, nil
}

func (a *Adapter) LastConnectedAt() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastConnectedAt
}

func (a *Adapter) ReconnectCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.reconnectCount
}

func (a *Adapter) LastMessageAt() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastMessageAt
}

func (a *Adapter) recordMessage() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastMessageAt = time.Now()
}

func (a *Adapter) recordReconnect() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reconnectCount++
}

func (a *Adapter) updateStatus(state, detail string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = channel.Status{State: state, Detail: detail}
	if state == "connected" {
		a.lastConnectedAt = time.Now()
		a.reconnectCount = 0
	}
}

func (a *Adapter) Login(ctx context.Context) error {
	return a.LoginWithUpdates(ctx, nil)
}

func (a *Adapter) LoginWithUpdates(ctx context.Context, report func(channel.Status) error) error {
	if a.seam == nil {
		return errors.New("whatsapp client seam is required")
	}
	err := a.seam.Login(ctx, func(update loginUpdate) error {
		status := update.normalize()
		a.updateStatus(status.State, status.Detail)
		if report != nil {
			return report(status)
		}
		return nil
	})
	if err != nil {
		a.updateStatus("error", strings.TrimSpace(err.Error()))
	}
	return err
}

func (m rawInboundMessage) normalize() channel.InboundMessage {
	return channel.InboundMessage{
		Channel: channelName,
		ChatID:  strings.TrimSpace(m.ChatID),
		Content: strings.TrimSpace(m.Content),
	}
}

func (u loginUpdate) normalize() channel.Status {
	if u.State == "" {
		return channel.Status{State: "disconnected"}
	}
	return channel.Status{State: u.State, Detail: strings.TrimSpace(u.Detail)}
}

func (a *Adapter) setStatus(status channel.Status) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = status
}

func (a *Adapter) isAllowedChat(chatID string) bool {
	if !a.enforceAllowlist {
		return true
	}
	if len(a.allowedChatIDs) == 0 {
		return false
	}
	_, ok := a.allowedChatIDs[strings.TrimSpace(chatID)]
	return ok
}

func normalizeAllowedChatIDs(chatIDs []string) map[string]struct{} {
	if len(chatIDs) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(chatIDs))
	for _, chatID := range chatIDs {
		chatID = strings.TrimSpace(chatID)
		if chatID == "" {
			continue
		}
		allowed[chatID] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

type whatsmeowSeam struct {
	client *whatsmeow.Client

	mu        sync.Mutex
	started   bool
	handlerID uint32

	recentMessages map[string]time.Time
	recentMu       sync.Mutex
}

const dedupWindow = 5 * time.Minute

func newWhatsmeowSeam(cfg config.WhatsAppChannelConfig) (*whatsmeowSeam, error) {
	storePath := strings.TrimSpace(cfg.StorePath)
	if storePath == "" {
		return nil, errors.New("whatsapp store path is required")
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		return nil, err
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", sqliteStoreDSN(storePath), waLog.Noop)
	if err != nil {
		return nil, fmt.Errorf("whatsapp store: %w", err)
	}
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("whatsapp device store: %w", err)
	}
	if deviceName := strings.TrimSpace(cfg.DeviceName); deviceName != "" {
		deviceStore.PushName = deviceName
	}

	client := whatsmeow.NewClient(deviceStore, waLog.Noop)
	client.EnableAutoReconnect = true

	return &whatsmeowSeam{
		client:         client,
		recentMessages: make(map[string]time.Time),
	}, nil
}

func (s *whatsmeowSeam) Send(ctx context.Context, chatID, content string) error {
	if err := s.ensureConnected(ctx); err != nil {
		return err
	}
	jid, err := waTypes.ParseJID(strings.TrimSpace(chatID))
	if err != nil {
		return fmt.Errorf("parse jid %q: %w", chatID, err)
	}
	_, err = s.client.SendMessage(ctx, jid, &waProto.Message{
		Conversation: proto.String(content),
	})
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

func (s *whatsmeowSeam) Start(ctx context.Context, handle func(rawInboundMessage) error) error {
	if handle == nil {
		return errors.New("whatsapp inbound handler is required")
	}
	if s.client.Store.ID == nil {
		return errors.New("whatsapp login required; run `smolbot channels login whatsapp`")
	}

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.handlerID = s.client.AddEventHandler(func(evt any) {
		s.handleEvent(evt, handle)
	})
	s.started = true
	s.mu.Unlock()

	if err := s.ensureConnected(ctx); err != nil {
		_ = s.Stop(context.Background())
		return err
	}
	return nil
}

func (s *whatsmeowSeam) Stop(context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	handlerID := s.handlerID
	s.started = false
	s.handlerID = 0
	s.mu.Unlock()

	s.client.RemoveEventHandler(handlerID)
	s.client.Disconnect()
	return nil
}

func (s *whatsmeowSeam) Login(ctx context.Context, report func(loginUpdate) error) error {
	if report == nil {
		return errors.New("whatsapp login reporter is required")
	}

	if s.client.Store.ID != nil {
		if err := s.ensureConnected(ctx); err != nil {
			return err
		}
		defer s.client.Disconnect()
		return report(loginUpdate{State: "connected"})
	}

	qrChan, err := s.client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("whatsapp qr channel: %w", err)
	}
	if err := s.client.Connect(); err != nil {
		return fmt.Errorf("whatsapp connect: %w", err)
	}
	defer s.client.Disconnect()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-qrChan:
			if !ok {
				if s.client.IsLoggedIn() {
					return report(loginUpdate{State: "connected"})
				}
				return nil
			}
			if err := report(loginUpdateFromQR(item)); err != nil {
				return err
			}
			switch item.Event {
			case whatsmeow.QRChannelEventError:
				if item.Error != nil {
					return item.Error
				}
				return errors.New("whatsapp login failed")
			case whatsmeow.QRChannelSuccess.Event:
				if err := report(loginUpdate{State: "connected"}); err != nil {
					return err
				}
				return nil
			case whatsmeow.QRChannelTimeout.Event:
				return errors.New("whatsapp login timed out")
			}
		}
	}
}

func (s *whatsmeowSeam) isOwnDevice(sender waTypes.JID) bool {
	ownID := s.ownID()
	if ownID == nil {
		return false
	}
	return sender.User == ownID.User && sender.Device == ownID.Device
}

func (s *whatsmeowSeam) ownID() *waTypes.JID {
	if s == nil || s.client == nil || s.client.Store == nil {
		return nil
	}
	return s.client.Store.ID
}

func (s *whatsmeowSeam) ensureConnected(ctx context.Context) error {
	if s.client.Store.ID == nil {
		return errors.New("whatsapp login required; run `smolbot channels login whatsapp`")
	}
	if s.client.IsConnected() {
		return nil
	}
	if err := s.client.Connect(); err != nil {
		category, retryable := categorizeError(err)
		log.Printf("[whatsapp] connect failed (category=%s, retryable=%v): %v", category, retryable, err)
		return fmt.Errorf("whatsapp connect [%s]: %w", category, err)
	}
	return nil
}

func categorizeError(err error) (category string, retryable bool) {
	if err == nil {
		return "unknown", false
	}
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "logged out"), strings.Contains(errStr, "auth"):
		return "auth", false
	case strings.Contains(errStr, "connection"), strings.Contains(errStr, "disconnected"):
		return "connection", true
	case strings.Contains(errStr, "timeout"):
		return "timeout", true
	case strings.Contains(errStr, "rate limit"):
		return "rate", true
	default:
		return "unknown", true
	}
}

func (s *whatsmeowSeam) handleEvent(evt any, handle func(rawInboundMessage) error) {
	switch typed := evt.(type) {
	case *waEvents.Message:
		if typed == nil {
			return
		}
		log.Printf("[whatsapp] message event: from=%s device=%d isFromMe=%v chat=%s",
			typed.Info.Sender.String(), typed.Info.Sender.Device, typed.Info.IsFromMe, typed.Info.Chat.String())
		if shouldIgnoreInboundMessage(typed.Info, s.ownID()) {
			return
		}

		msgID := string(typed.Info.ID)
		if msgID == "" {
			return
		}

		s.recentMu.Lock()
		if recvTime, ok := s.recentMessages[msgID]; ok {
			if time.Since(recvTime) < dedupWindow {
				s.recentMu.Unlock()
				return
			}
		}
		s.recentMessages[msgID] = time.Now()
		now := time.Now()
		for k, v := range s.recentMessages {
			if now.Sub(v) > dedupWindow {
				delete(s.recentMessages, k)
			}
		}
		s.recentMu.Unlock()

		content := extractMessageText(typed.Message)
		if content == "" {
			return
		}
		if err := handle(rawInboundMessage{
			ChatID:  typed.Info.Chat.String(),
			Content: content,
		}); err != nil {
			log.Printf("[whatsapp] handle event: %v", err)
		}
	}
}

func shouldIgnoreInboundMessage(info waTypes.MessageInfo, ownID *waTypes.JID) bool {
	if info.Sender.IsBot() {
		return true
	}
	if info.IsFromMe && ownID != nil {
		return info.Sender.User == ownID.User && info.Sender.Device == ownID.Device
	}
	return false
}

func extractMessageText(msg *waProto.Message) string {
	switch {
	case msg == nil:
		return ""
	case msg.GetConversation() != "":
		return msg.GetConversation()
	case msg.GetExtendedTextMessage().GetText() != "":
		return msg.GetExtendedTextMessage().GetText()
	case msg.GetImageMessage().GetCaption() != "":
		return msg.GetImageMessage().GetCaption()
	case msg.GetVideoMessage().GetCaption() != "":
		return msg.GetVideoMessage().GetCaption()
	case msg.GetDocumentMessage().GetCaption() != "":
		return msg.GetDocumentMessage().GetCaption()
	default:
		return ""
	}
}

func loginUpdateFromQR(item whatsmeow.QRChannelItem) loginUpdate {
	switch item.Event {
	case whatsmeow.QRChannelEventCode:
		return loginUpdate{State: "qr", Detail: strings.TrimSpace(item.Code)}
	case whatsmeow.QRChannelSuccess.Event:
		return loginUpdate{State: "device-link", Detail: "paired"}
	case whatsmeow.QRChannelTimeout.Event:
		return loginUpdate{State: "auth-required", Detail: "timed out"}
	case whatsmeow.QRChannelEventError:
		if item.Error != nil {
			return loginUpdate{State: "error", Detail: strings.TrimSpace(item.Error.Error())}
		}
		return loginUpdate{State: "error", Detail: "login error"}
	default:
		return loginUpdate{State: "device-link", Detail: strings.TrimSpace(item.Event)}
	}
}

func sqliteStoreDSN(path string) string {
	return "file:" + path + "?_foreign_keys=on"
}
