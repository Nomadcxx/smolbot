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
	return NewAdapter(seam), nil
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
	a.setStatus(channel.Status{State: "connecting"})
	err := a.seam.Start(ctx, func(raw rawInboundMessage) error {
		handler(ctx, raw.normalize())
		return nil
	})
	if err != nil {
		a.setStatus(channel.Status{State: "error", Detail: strings.TrimSpace(err.Error())})
		return err
	}
	a.setStatus(channel.Status{State: "connected"})
	return nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	if a.seam == nil {
		return nil
	}
	err := a.seam.Stop(ctx)
	if err != nil {
		a.setStatus(channel.Status{State: "error", Detail: strings.TrimSpace(err.Error())})
		return err
	}
	a.setStatus(channel.Status{State: "disconnected"})
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

func (a *Adapter) Login(ctx context.Context) error {
	return a.LoginWithUpdates(ctx, nil)
}

func (a *Adapter) LoginWithUpdates(ctx context.Context, report func(channel.Status) error) error {
	if a.seam == nil {
		return errors.New("whatsapp client seam is required")
	}
	err := a.seam.Login(ctx, func(update loginUpdate) error {
		status := update.normalize()
		a.setStatus(status)
		if report != nil {
			return report(status)
		}
		return nil
	})
	if err != nil {
		a.setStatus(channel.Status{State: "error", Detail: strings.TrimSpace(err.Error())})
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

type whatsmeowSeam struct {
	client *whatsmeow.Client

	mu        sync.Mutex
	started   bool
	handlerID uint32
}

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

	return &whatsmeowSeam{client: client}, nil
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

func (s *whatsmeowSeam) ensureConnected(ctx context.Context) error {
	if s.client.Store.ID == nil {
		return errors.New("whatsapp login required; run `smolbot channels login whatsapp`")
	}
	if s.client.IsConnected() {
		return nil
	}
	if err := s.client.Connect(); err != nil {
		return fmt.Errorf("whatsapp connect: %w", err)
	}
	return nil
}

func (s *whatsmeowSeam) handleEvent(evt any, handle func(rawInboundMessage) error) {
	switch typed := evt.(type) {
	case *waEvents.Message:
		if typed == nil || typed.Info.IsFromMe {
			return
		}
		content := extractMessageText(typed.Message)
		if strings.TrimSpace(content) == "" {
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
