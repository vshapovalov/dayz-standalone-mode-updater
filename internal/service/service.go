package service

import (
	"context"
	"fmt"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/logx"
	"github.com/example/dayz-standalone-mode-updater/internal/planner"
	"github.com/example/dayz-standalone-mode-updater/internal/rcon"
	"github.com/example/dayz-standalone-mode-updater/internal/sftpsync"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
	"github.com/example/dayz-standalone-mode-updater/internal/steam"
)

type Service struct {
	cfg    config.Config
	log    *logx.Logger
	steam  *steam.Client
	syncer *sftpsync.Syncer
	rcon   *rcon.Client
}

func New(cfg config.Config, log *logx.Logger) *Service {
	return &Service{
		cfg:    cfg,
		log:    log,
		steam:  steam.NewClient(cfg.Steam.APIKey),
		syncer: sftpsync.New(cfg.SFTP.Address, cfg.SFTP.Username, cfg.SFTP.Password),
		rcon:   rcon.New(cfg.RCON.Address, cfg.RCON.Password),
	}
}

func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.cfg.PollInterval())
	defer ticker.Stop()

	for {
		if err := s.runOnce(ctx); err != nil {
			s.log.Error("sync cycle failed", err, nil)
		}
		select {
		case <-ctx.Done():
			s.log.Info("shutdown complete", nil)
			return nil
		case <-ticker.C:
		}
	}
}

func (s *Service) runOnce(ctx context.Context) error {
	st, err := state.Load(s.cfg.StatePath)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(s.cfg.Mods))
	for _, m := range s.cfg.Mods {
		ids = append(ids, m.ID)
	}
	details, err := s.steam.FetchModDetails(ctx, ids)
	if err != nil {
		return err
	}
	actions := planner.BuildPlan(s.cfg.Mods, details, st, s.cfg.SFTP.RemoteRoot)
	if len(actions) == 0 {
		s.log.Info("no updates", map[string]any{"mods": len(ids)})
		return nil
	}
	for _, msg := range planner.CountdownMessages(s.cfg.RCON.PreRestartCountdown) {
		if err := s.rcon.Say(msg); err != nil {
			s.log.Error("rcon broadcast failed", err, nil)
		}
	}
	for _, a := range actions {
		s.log.Info("syncing mod", map[string]any{"mod_id": a.ModID, "title": a.Title, "remote_path": a.RemotePath})
		if err := s.syncer.SyncDirectory(ctx, a.LocalPath, a.RemotePath); err != nil {
			return fmt.Errorf("sync mod %s: %w", a.ModID, err)
		}
		st.Mods[a.ModID] = state.ModState{LastSyncedAt: a.UpdatedAt, LastTitle: a.Title}
	}
	if err := state.SaveAtomic(s.cfg.StatePath, st); err != nil {
		return err
	}
	return nil
}
