package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newStatusCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon and config status",
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := fetchStatus(context.Background(), *opts)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), formatStatus(report))
			return nil
		},
	}
}
