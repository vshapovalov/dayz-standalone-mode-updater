package orchestrator

import (
	"context"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/logging"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
)

type Orchestrator struct {
	cfg    config.Config
	state  state.State
	logger logging.Logger
}

func New(cfg config.Config, st state.State, logger logging.Logger) *Orchestrator {
	return &Orchestrator{cfg: cfg, state: st, logger: logger}
}

func (o *Orchestrator) Run(ctx context.Context) error {
	o.logger.Info("orchestrator started", map[string]any{
		"poll_interval_seconds": o.cfg.PollIntervalSeconds,
		"mods":                  len(o.cfg.Mods),
	})

	ticker := time.NewTicker(o.cfg.PollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.logger.Info("orchestrator stopping", nil)
			return nil
		case <-ticker.C:
			// TODO: implement poll/compare/sync pipeline.
			o.logger.Info("poll tick", map[string]any{"known_mods": len(o.state.Mods)})
		}
	}
}
