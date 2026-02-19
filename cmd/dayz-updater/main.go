package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/logx"
	"github.com/example/dayz-standalone-mode-updater/internal/service"
)

func main() {
	cfgPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	logger := logx.New()
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		logger.Error("failed to load config", err, nil)
		os.Exit(1)
	}
	logger.Info("starting daemon", map[string]any{
		"poll_interval_seconds": cfg.PollIntervalSeconds,
		"steam_password":        cfg.Steam.Password,
		"rcon_password":         cfg.RCON.Password,
		"sftp_password":         cfg.SFTP.Password,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	svc := service.New(cfg, logger)
	if err := svc.Run(ctx); err != nil {
		logger.Error("service exited with error", err, nil)
		os.Exit(1)
	}
}
