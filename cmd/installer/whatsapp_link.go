package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

const linkTimeout = 5 * time.Minute

type WhatsAppLinker struct {
	storePath string
	client   *whatsmeow.Client
	
	// Polling state
	mu       sync.Mutex
	qrCode   string
	status   string
	done     bool
	linkErr  error
	started  bool
	qrChan   <-chan whatsmeow.QRChannelItem
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewWhatsAppLinker(storePath string) (*WhatsAppLinker, error) {
	if err := os.MkdirAll(filepath.Dir(storePath), 0755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", sqliteStoreDSN(storePath), waLog.Noop)
	if err != nil {
		return nil, fmt.Errorf("create store: %w", err)
	}

	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	client := whatsmeow.NewClient(device, waLog.Noop)

	return &WhatsAppLinker{
		storePath: storePath,
		client:   client,
	}, nil
}

func (l *WhatsAppLinker) IsLinked() bool {
	return l.client.Store.ID != nil
}

func (l *WhatsAppLinker) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.started {
		return nil
	}

	if l.client.Store.ID != nil {
		l.status = "Already linked"
		l.done = true
		return nil
	}

	l.ctx, l.cancel = context.WithTimeout(context.Background(), linkTimeout)

	qrChan, err := l.client.GetQRChannel(l.ctx)
	if err != nil {
		return fmt.Errorf("get qr channel: %w", err)
	}
	l.qrChan = qrChan

	if err := l.client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	l.started = true
	l.status = "Connecting..."
	
	// Start goroutine to process QR channel
	go l.processQRChannel()
	
	return nil
}

func (l *WhatsAppLinker) processQRChannel() {
	for {
		select {
		case <-l.ctx.Done():
			l.mu.Lock()
			if !l.done {
				l.status = "Timeout"
				l.linkErr = fmt.Errorf("timeout")
				l.done = true
			}
			l.mu.Unlock()
			return
		case item, ok := <-l.qrChan:
			if !ok {
				l.mu.Lock()
				if l.client.IsLoggedIn() {
					l.status = "Linked!"
					l.done = true
				} else {
					l.status = "QR channel closed"
					l.linkErr = fmt.Errorf("qr channel closed")
					l.done = true
				}
				l.mu.Unlock()
				return
			}

			l.mu.Lock()
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				l.qrCode = item.Code
				l.status = "Scan QR with WhatsApp"
			case whatsmeow.QRChannelSuccess.Event:
				l.status = "Linked successfully!"
				l.done = true
				l.client.Disconnect()
			case whatsmeow.QRChannelTimeout.Event:
				l.status = "QR timed out"
				l.linkErr = fmt.Errorf("qr timed out")
				l.done = true
				l.client.Disconnect()
			case whatsmeow.QRChannelEventError:
				if item.Error != nil {
					l.status = fmt.Sprintf("Error: %v", item.Error)
					l.linkErr = item.Error
				} else {
					l.status = "Link error"
					l.linkErr = fmt.Errorf("link error")
				}
				l.done = true
				l.client.Disconnect()
			}
			l.mu.Unlock()
		}
	}
}

func (l *WhatsAppLinker) Poll() (qr string, status string, done bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.qrCode, l.status, l.done, l.linkErr
}

func (l *WhatsAppLinker) IsDone() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.done
}

func (l *WhatsAppLinker) Error() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.linkErr
}

func (l *WhatsAppLinker) Cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		l.cancel()
	}
	if l.client != nil {
		l.client.Disconnect()
	}
}

func sqliteStoreDSN(path string) string {
	return "file:" + path + "?_foreign_keys=on"
}
