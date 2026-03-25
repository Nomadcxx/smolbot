package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
		client:    client,
	}, nil
}

func (l *WhatsAppLinker) IsLinked() bool {
	return l.client.Store.ID != nil
}

type QRCallback func(string)
type StatusCallback func(string)

func (l *WhatsAppLinker) StartLinking(onQR QRCallback, onStatus StatusCallback) error {
	ctx, cancel := context.WithTimeout(context.Background(), linkTimeout)
	defer cancel()

	if l.client.Store.ID != nil {
		onStatus("Already linked")
		return nil
	}

	qrChan, err := l.client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("get qr channel: %w", err)
	}

	if err := l.client.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer l.client.Disconnect()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout")
		case item, ok := <-qrChan:
			if !ok {
				if l.client.IsLoggedIn() {
					onStatus("Linked successfully!")
					return nil
				}
				return fmt.Errorf("qr channel closed")
			}

			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				onQR(item.Code)
				onStatus("Scan QR with WhatsApp")
			case whatsmeow.QRChannelSuccess.Event:
				onStatus("Linked successfully!")
				return nil
			case whatsmeow.QRChannelTimeout.Event:
				return fmt.Errorf("qr timed out")
			case whatsmeow.QRChannelEventError:
				if item.Error != nil {
					return item.Error
				}
				return fmt.Errorf("link error")
			}
		}
	}
}

func sqliteStoreDSN(path string) string {
	return "file:" + path + "?_foreign_keys=on"
}
