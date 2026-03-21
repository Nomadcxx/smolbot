package main

import (
	"github.com/spf13/cobra"
)

type rootOptions struct {
	configPath string
	workspace  string
	verbose    bool
	version    string
}

func NewRootCmd(version string) *cobra.Command {
	opts := &rootOptions{version: version}

	cmd := &cobra.Command{
		Use:   "nanobot",
		Short: "nanobot daemon and CLI",
	}

	cmd.PersistentFlags().StringVar(&opts.configPath, "config", "", "Path to config file")
	cmd.PersistentFlags().StringVar(&opts.workspace, "workspace", "", "Override workspace path")
	cmd.PersistentFlags().BoolVar(&opts.verbose, "verbose", false, "Enable verbose logging")

	cmd.AddCommand(newRunCmd(opts))
	cmd.AddCommand(newChatCmd(opts))
	cmd.AddCommand(newStatusCmd(opts))
	cmd.AddCommand(newOnboardCmd(opts))
	cmd.AddCommand(newChannelsCmd(opts))

	return cmd
}
