package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/logging"
	"github.com/example/dayz-standalone-mode-updater/internal/modlist"
	"github.com/example/dayz-standalone-mode-updater/internal/rcon"
	"github.com/example/dayz-standalone-mode-updater/internal/sftpsync"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
	"github.com/example/dayz-standalone-mode-updater/internal/steamcmd"
	"github.com/example/dayz-standalone-mode-updater/internal/workshop"
)

type steamRunner interface {
	UpdateMods(ctx context.Context, modIDs []string, st *state.State) ([]string, error)
}

type syncEngine interface {
	SyncServers(ctx context.Context, cfg config.Config, st *state.State) error
}

type rconTicker interface {
	Tick(ctx context.Context, now time.Time, st *state.State)
}

type modlistPollFn func(ctx context.Context, srv config.ServerConfig, localCacheRoot string, warnf func(string, ...any)) (modlist.PollResult, error)

type Orchestrator struct {
	cfg          config.Config
	store        state.StateStore
	logger       logging.Logger
	workshop     workshop.Client
	steam        steamRunner
	sync         syncEngine
	rcon         rconTicker
	pollModlist  modlistPollFn
	now          func() time.Time
	steamBatchMu sync.Mutex
}

func New(cfg config.Config, logger logging.Logger) *Orchestrator {
	return &Orchestrator{
		cfg:         cfg,
		store:       state.NewFileStore(cfg.StatePath),
		logger:      logger,
		workshop:    workshop.NewWebAPIClient(cfg.Steam.WebAPIKey, time.Duration(cfg.Steam.WorkshopHTTPTimeoutSeconds)*time.Second, cfg.Steam.WorkshopMaxRetries, time.Duration(cfg.Steam.WorkshopBackoffMillis)*time.Millisecond),
		steam:       steamcmd.NewRunner(cfg),
		sync:        sftpsync.NewEngine(),
		rcon:        rcon.NewController(cfg),
		pollModlist: modlist.PollServerModlist,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

func (o *Orchestrator) Run(ctx context.Context) error {
	o.logger.Info("orchestrator started", map[string]any{
		"modlist_poll_seconds":  o.cfg.Intervals.ModlistPollSeconds,
		"workshop_poll_seconds": o.cfg.Intervals.WorkshopPollSeconds,
		"rcon_tick_seconds":     o.cfg.Intervals.RconTickSeconds,
	})

	modlistTicker := time.NewTicker(time.Duration(o.cfg.Intervals.ModlistPollSeconds) * time.Second)
	workshopTicker := time.NewTicker(time.Duration(o.cfg.Intervals.WorkshopPollSeconds) * time.Second)
	rconTicker := time.NewTicker(time.Duration(o.cfg.Intervals.RconTickSeconds) * time.Second)
	flushTicker := time.NewTicker(time.Duration(o.cfg.Intervals.StateFlushSeconds) * time.Second)
	defer modlistTicker.Stop()
	defer workshopTicker.Stop()
	defer rconTicker.Stop()
	defer flushTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			err := o.flushState()
			o.logger.Info("orchestrator stopping", nil)
			return err
		case <-modlistTicker.C:
			o.runModlistPoll(ctx)
		case <-workshopTicker.C:
			o.runWorkshopPoll(ctx)
		case <-rconTicker.C:
			o.runRCONTick(ctx)
		case <-flushTicker.C:
			if err := o.flushState(); err != nil {
				o.logger.Error("state flush failed", err, nil)
			}
		}
	}
}

func (o *Orchestrator) runModlistPoll(ctx context.Context) {
	sem := make(chan struct{}, o.cfg.Concurrency.ModlistPollParallelism)
	var wg sync.WaitGroup

	for _, srv := range o.cfg.Servers {
		srv := srv
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			result, err := o.pollModlist(ctx, srv, o.cfg.Paths.LocalCacheRoot, func(format string, args ...any) {
				o.logger.Info(fmt.Sprintf(format, args...), map[string]any{"server_id": srv.ID})
			})
			if err != nil {
				o.logger.Error("modlist poll failed", err, map[string]any{"server_id": srv.ID})
				return
			}
			if err := o.store.Update(func(st *state.State) error {
				modlist.ApplyPollResult(st, srv.ID, result)
				return nil
			}); err != nil {
				o.logger.Error("failed to persist modlist poll", err, map[string]any{"server_id": srv.ID})
			}
		}()
	}
	wg.Wait()
}

func (o *Orchestrator) runWorkshopPoll(ctx context.Context) {
	modsToUpdate := make([]string, 0)
	err := o.store.Update(func(st *state.State) error {
		var err error
		modsToUpdate, err = workshop.PollMetadata(ctx, o.cfg, st, o.workshop, o.now())
		return err
	})
	if err != nil {
		o.logger.Error("workshop poll failed", err, nil)
		return
	}
	if len(modsToUpdate) == 0 {
		return
	}
	if err := o.runSteamCMDBatch(ctx, modsToUpdate); err != nil {
		o.logger.Error("steamcmd batch failed", err, nil)
		return
	}
	if err := o.runSFTPSyncPhase(ctx); err != nil {
		o.logger.Error("sftp sync phase failed", err, nil)
	}
}

func (o *Orchestrator) runSteamCMDBatch(ctx context.Context, mods []string) error {
	o.steamBatchMu.Lock()
	defer o.steamBatchMu.Unlock()

	for _, modID := range mods {
		if err := o.store.Update(func(st *state.State) error {
			_, err := o.steam.UpdateMods(ctx, []string{modID}, st)
			return err
		}); err != nil {
			return fmt.Errorf("update mod %s: %w", modID, err)
		}
	}
	return nil
}

func (o *Orchestrator) runSFTPSyncPhase(ctx context.Context) error {
	return o.store.Update(func(st *state.State) error {
		return o.sync.SyncServers(ctx, o.cfg, st)
	})
}

func (o *Orchestrator) runRCONTick(ctx context.Context) {
	if err := o.store.Update(func(st *state.State) error {
		o.rcon.Tick(ctx, o.now(), st)
		return nil
	}); err != nil {
		o.logger.Error("rcon tick persist failed", err, nil)
	}
}

func (o *Orchestrator) flushState() error {
	snap, err := o.store.Load()
	if err != nil {
		return err
	}
	if err := o.store.Save(snap); err != nil {
		return err
	}
	return nil
}

func (o *Orchestrator) WithDependencies(store state.StateStore, workshopClient workshop.Client, steamRunner steamRunner, syncer syncEngine, rconController rconTicker, poller modlistPollFn, now func() time.Time) *Orchestrator {
	if store != nil {
		o.store = store
	}
	if workshopClient != nil {
		o.workshop = workshopClient
	}
	if steamRunner != nil {
		o.steam = steamRunner
	}
	if syncer != nil {
		o.sync = syncer
	}
	if rconController != nil {
		o.rcon = rconController
	}
	if poller != nil {
		o.pollModlist = poller
	}
	if now != nil {
		o.now = now
	}
	return o
}
