package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/channel/qr"
)

var runSignalLogin = runSignalLoginImpl
var newSignalQRRenderer = func(size int) signalQRRenderer {
	return qr.New(size)
}

type signalQRRenderer interface {
	RenderToASCII(string) (string, error)
}

func runSignalLoginImpl(ctx context.Context, opts rootOptions, out io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	configPath := opts.configPath
	if configPath == "" {
		configPath = defaultConfigPath(opts)
	}

	cfg, _, err := loadRuntimeConfig(configPath, opts.workspace, 0)
	if err != nil {
		return err
	}

	var renderer signalQRRenderer
	if out != nil {
		renderer = newSignalQRRenderer(256)
	}

	adapter := newSignalChannel(cfg.Channels.Signal)
	interactive, ok := adapter.(channel.InteractiveLoginHandler)
	if !ok {
		return fmt.Errorf("signal channel does not support interactive login")
	}

	report := func(channel.Status) error { return nil }
	if out != nil {
		report = func(status channel.Status) error {
			return writeSignalLoginStatus(out, renderer, status)
		}
	}

	return interactive.LoginWithUpdates(ctx, report)
}

func writeSignalLoginStatus(out io.Writer, renderer signalQRRenderer, status channel.Status) error {
	state := strings.TrimSpace(status.State)
	if state == "" {
		return nil
	}

	detail := strings.TrimSpace(status.Detail)
	if state == "auth-required" {
		if detail == "" {
			_, err := fmt.Fprintln(out, state)
			return err
		}
		if renderer == nil {
			_, err := fmt.Fprintln(out, state)
			return err
		}
		qrASCII, err := renderer.RenderToASCII(detail)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, state); err != nil {
			return err
		}
		if qrASCII != "" {
			if _, err := fmt.Fprintln(out, qrASCII); err != nil {
				return err
			}
		}
		return nil
	}

	if detail != "" {
		_, err := fmt.Fprintf(out, "%s: %s\n", state, detail)
		return err
	}
	_, err := fmt.Fprintln(out, state)
	return err
}
