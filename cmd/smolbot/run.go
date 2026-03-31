package main

import (
	"context"

	"github.com/spf13/cobra"
)

func newRunCmd(opts *rootOptions) *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the smolbot daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchDaemon(context.Background(), daemonLaunchOptions{
				ConfigPath: defaultConfigPath(*opts),
				Workspace:  opts.workspace,
				Verbose:    opts.verbose,
				Port:       port,
			})
		},
	}
	cmd.Flags().IntVar(&port, "port", 18790, "Gateway port")
	return cmd
}
