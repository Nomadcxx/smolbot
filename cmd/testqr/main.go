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
	"github.com/skip2/go-qrcode"
)

func main() {
	storePath := filepath.Join(os.Getenv("HOME"), ".smolbot", "test_qr.db")
	os.Remove(storePath)
	os.MkdirAll(filepath.Dir(storePath), 0755)

	container, err := sqlstore.New(context.Background(), "sqlite3", "file:"+storePath+"?_foreign_keys=on", waLog.Noop)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	client := whatsmeow.NewClient(device, waLog.Noop)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	if err := client.Connect(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	defer client.Disconnect()

	item := <-qrChan
	if item.Code == "" {
		fmt.Println("No QR code received")
		return
	}

	fmt.Println("QR received, saving as PNG...")
	png, err := qrcode.Encode(item.Code, qrcode.Medium, 256)
	if err != nil {
		fmt.Printf("ERROR encoding: %v\n", err)
		return
	}
	
	outPath := "/tmp/whatsapp_qr.png"
	if err := os.WriteFile(outPath, png, 0644); err != nil {
		fmt.Printf("ERROR writing: %v\n", err)
		return
	}
	fmt.Printf("QR PNG saved to: %s\n", outPath)
}
