package main

import (
	"github.com/spf13/cobra"
)

func newChannelsLoginCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "login <channel>",
		Short: "Run channel-specific login or auth flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			channelName := args[0]
			if channelName == "whatsapp" {
				return runWhatsAppLogin(cmd.Context(), *opts)
			}
			if err := runChannelLogin(cmd.Context(), *opts, channelName, cmd.OutOrStdout()); err != nil {
				return err
			}
			return nil
		},
	}
}
