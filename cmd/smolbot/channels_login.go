package main

import (
	"github.com/spf13/cobra"
)

func newChannelsLoginCmd(opts *rootOptions) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "login <channel>",
		Short: "Run channel-specific login or auth flow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			channelName := args[0]
			if channelName == "signal" {
				return runSignalLogin(cmd.Context(), *opts, cmd.OutOrStdout())
			}
			if channelName == "whatsapp" {
				if jsonOutput {
					return runWhatsAppLoginJSON(cmd.Context(), *opts)
				}
				return runWhatsAppLogin(cmd.Context(), *opts)
			}
			if err := runChannelLogin(cmd.Context(), *opts, channelName, cmd.OutOrStdout()); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output login events as JSON (for installer integration)")

	return cmd
}
