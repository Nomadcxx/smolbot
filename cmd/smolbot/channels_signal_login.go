package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Nomadcxx/smolbot/pkg/channel"
)

var runSignalLogin = runSignalLoginImpl

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

	adapter := newSignalChannel(cfg.Channels.Signal)
	interactive, ok := adapter.(channel.InteractiveLoginHandler)
	if !ok {
		return fmt.Errorf("signal channel does not support interactive login")
	}

	var report func(channel.Status) error
	if out != nil {
		report = func(status channel.Status) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if strings.TrimSpace(status.State) == "" {
				return nil
			}
			if strings.TrimSpace(status.Detail) != "" {
				_, err := fmt.Fprintf(out, "%s: %s\n", status.State, status.Detail)
				return err
			}
			_, err := fmt.Fprintf(out, "%s\n", status.State)
			return err
		}
	}

	return interactive.LoginWithUpdates(ctx, report)
}
