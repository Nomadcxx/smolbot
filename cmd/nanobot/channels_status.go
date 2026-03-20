package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newChannelsStatusCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show configured channel states",
		RunE: func(cmd *cobra.Command, args []string) error {
			statuses, err := fetchChannelStatuses(context.Background(), *opts)
			if err != nil {
				return err
			}
			if len(statuses) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no channels configured")
				return nil
			}
			for _, status := range statuses {
				if status.Detail != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (%s)\n", status.Name, status.State, status.Detail)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", status.Name, status.State)
				}
			}
			return nil
		},
	}
}
