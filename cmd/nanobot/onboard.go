package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newOnboardCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "onboard",
		Short: "Guide the user through initial configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := collectOnboardConfig(context.Background(), *opts)
			if err != nil {
				return err
			}
			path := defaultConfigPath(*opts)
			if err := writeConfigFile(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote config to %s\n", path)
			return nil
		},
	}
}
