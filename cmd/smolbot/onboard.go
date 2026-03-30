package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newOnboardCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "onboard",
		Short: "Guide the user through initial configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := defaultConfigPath(*opts)
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("config already exists at %s — delete it first or edit it directly", path)
			}
			cfg, err := collectOnboardConfig(context.Background(), *opts)
			if err != nil {
				return err
			}
			if err := writeConfigFile(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote config to %s\n", path)
			return nil
		},
	}
}
