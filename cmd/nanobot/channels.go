package main

import "github.com/spf13/cobra"

func newChannelsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "Inspect and manage configured channels",
	}
	cmd.AddCommand(newChannelsStatusCmd(opts))
	cmd.AddCommand(newChannelsLoginCmd(opts))
	return cmd
}
