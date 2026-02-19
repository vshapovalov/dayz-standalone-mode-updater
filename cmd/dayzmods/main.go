package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/logging"
	"github.com/example/dayz-standalone-mode-updater/internal/orchestrator"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "dayzmods",
		Short: "DayZ mod updater daemon",
	}

	root.AddCommand(newRunCmd())
	root.AddCommand(newPrintSampleConfigCmd())
	root.AddCommand(newPrintSampleStateCmd())

	return root
}

func newRunCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "run --config <path>",
		Short: "Start the updater daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.New()

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			st, err := state.Load(cfg.StatePath)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			orch := orchestrator.New(cfg, st, logger)
			return orch.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "config.json", "path to config.json")
	return cmd
}

func newPrintSampleConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "print-sample-config",
		Short: "Print a sample config.json to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(config.Sample())
		},
	}
}

func newPrintSampleStateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "print-sample-state",
		Short: "Print a sample empty state.json to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(state.Sample())
		},
	}
}
