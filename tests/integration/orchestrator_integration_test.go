//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/logging"
	"github.com/example/dayz-standalone-mode-updater/internal/modlist"
	"github.com/example/dayz-standalone-mode-updater/internal/orchestrator"
	"github.com/example/dayz-standalone-mode-updater/internal/sftpsync"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
	"github.com/example/dayz-standalone-mode-updater/internal/steamcmd"
	"github.com/example/dayz-standalone-mode-updater/internal/workshop"
)

func TestOrchestratorWorkshopSteamCMDSFTPSyncFlow(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 and run docker compose up -d")
	}

	workshopUpdated := time.Now().UTC().Truncate(time.Second)
	workshopServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		resp := map[string]any{"response": map[string]any{"publishedfiledetails": []map[string]any{{
			"publishedfileid": "123",
			"title":           "Test Mod",
			"time_updated":    workshopUpdated.Unix(),
		}}}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer workshopServer.Close()

	localMods := t.TempDir()
	localCache := t.TempDir()
	statePath := filepath.Join(t.TempDir(), "state.json")
	store := state.NewFileStore(statePath)
	initial := state.State{
		Version: 1,
		Mods: map[string]state.ModState{
			"123": {DisplayName: "Test Mod", FolderSlug: "test-mod"},
		},
		Servers: map[string]state.ServerState{
			"srv1": {
				LastModIDs:     []string{"123"},
				LastModsetHash: "seed",
				SyncedMods:     map[string]time.Time{},
				Stage:          state.StageIdle,
			},
		},
	}
	if err := store.Save(initial); err != nil {
		t.Fatalf("save initial state: %v", err)
	}

	cfg := config.Config{
		StatePath: statePath,
		Paths: config.PathsConfig{
			LocalModsRoot:               localMods,
			LocalCacheRoot:              localCache,
			SteamcmdPath:                "steamcmd",
			SteamcmdWorkshopContentRoot: t.TempDir(),
		},
		Steam: config.SteamConfig{WorkshopGameID: 221100},
		Intervals: config.IntervalsConfig{
			ModlistPollSeconds:  3600,
			WorkshopPollSeconds: 1,
			RconTickSeconds:     3600,
			StateFlushSeconds:   1,
		},
		Shutdown: config.ShutdownConfig{
			GracePeriodSeconds:   120,
			AnnounceEverySeconds: 30,
			MessageTemplate:      "Restart in {minutes}",
			FinalMessage:         "Restart now",
		},
		Concurrency: config.ConcurrencyConfig{
			ModlistPollParallelism:           1,
			SFTPSyncParallelismServers:       1,
			SFTPSyncParallelismModsPerServer: 1,
			WorkshopParallelism:              1,
			WorkshopBatchSize:                50,
		},
		Servers: []config.ServerConfig{{
			ID:   "srv1",
			Name: "Server 1",
			SFTP: config.ServerSFTPConfig{
				Host:           "127.0.0.1",
				Port:           2222,
				User:           "foo",
				Auth:           config.SFTPAuthConfig{Type: "password", Password: "pass"},
				RemoteModsRoot: "/upload/mods/testsrv",
			},
			RCON: config.ServerRCONConfig{Host: "127.0.0.1", Port: 2302, Password: "ignored"},
		}},
	}

	client, sshConn := connectSFTP(t)
	_ = client.RemoveDirectory(path.Join(cfg.Servers[0].SFTP.RemoteModsRoot, "test-mod"))
	_ = client.Remove(path.Join(cfg.Servers[0].SFTP.RemoteModsRoot, "test-mod", "mod.txt"))
	sshConn.Close()
	client.Close()

	orch := orchestrator.New(cfg, logging.New()).WithDependencies(
		store,
		&httpWorkshopClient{url: workshopServer.URL},
		&mockSteamRunner{localModsRoot: localMods},
		sftpsync.NewEngine(),
		noopRCONTicker{},
		func(ctx context.Context, srv config.ServerConfig, localCacheRoot string, warnf func(string, ...any)) (modlist.PollResult, error) {
			return modlist.PollResult{}, nil
		},
		func() time.Time { return time.Now().UTC() },
	)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = orch.Run(ctx) }()

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		snap, err := store.Load()
		if err != nil {
			t.Fatalf("load state: %v", err)
		}
		server := snap.Servers["srv1"]
		if server.NeedsShutdown && server.Stage == state.StageCountdown && server.ShutdownDeadlineAt != nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	cancel()

	snap, err := store.Load()
	if err != nil {
		t.Fatalf("load final state: %v", err)
	}
	server := snap.Servers["srv1"]
	if !server.NeedsShutdown {
		t.Fatalf("expected needs_shutdown=true")
	}
	if server.Stage != state.StageCountdown {
		t.Fatalf("expected stage countdown, got %s", server.Stage)
	}
	if server.ShutdownDeadlineAt == nil {
		t.Fatalf("expected shutdown deadline")
	}

	client, sshConn = connectSFTP(t)
	defer sshConn.Close()
	defer client.Close()
	assertRemoteFileEquals(t, client, "/upload/mods/testsrv/test-mod/mod.txt", []byte("downloaded"))
}

type httpWorkshopClient struct {
	url string
}

func (h *httpWorkshopClient) FetchMetadata(ctx context.Context, modIDs []string) (map[string]workshop.ModMetadata, error) {
	_ = modIDs
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload struct {
		Response struct {
			PublishedFileDetails []struct {
				PublishedFileID string `json:"publishedfileid"`
				Title           string `json:"title"`
				TimeUpdated     int64  `json:"time_updated"`
			} `json:"publishedfiledetails"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	out := map[string]workshop.ModMetadata{}
	for _, d := range payload.Response.PublishedFileDetails {
		out[d.PublishedFileID] = workshop.ModMetadata{ID: d.PublishedFileID, Title: d.Title, UpdatedAt: time.Unix(d.TimeUpdated, 0).UTC()}
	}
	return out, nil
}

type mockSteamRunner struct {
	localModsRoot string
}

func (m *mockSteamRunner) UpdateMods(ctx context.Context, modIDs []string, st *state.State) ([]string, error) {
	_ = ctx
	succeeded := make([]string, 0, len(modIDs))
	for _, id := range modIDs {
		mod := st.Mods[id]
		folder := filepath.Join(m.localModsRoot, mod.FolderSlug)
		if err := os.MkdirAll(folder, 0o755); err != nil {
			return succeeded, err
		}
		if err := os.WriteFile(filepath.Join(folder, "mod.txt"), []byte("downloaded"), 0o644); err != nil {
			return succeeded, err
		}
		if mod.WorkshopUpdatedAt.IsZero() {
			mod.LocalUpdatedAt = time.Now().UTC()
		} else {
			mod.LocalUpdatedAt = mod.WorkshopUpdatedAt
		}
		st.Mods[id] = mod
		steamcmd.MarkServersUsingModForPlanning(st, id)
		succeeded = append(succeeded, id)
	}
	return succeeded, nil
}

type noopRCONTicker struct{}

func (noopRCONTicker) Tick(ctx context.Context, now time.Time, st *state.State) {
	_, _, _ = ctx, now, st
}
