package planner

import (
	"testing"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
	"github.com/example/dayz-standalone-mode-updater/internal/steam"
)

func TestBuildPlan(t *testing.T) {
	now := time.Now().UTC()
	mods := []config.ModConfig{{ID: "1", LocalPath: "/local/1"}, {ID: "2", LocalPath: "/local/2", RemoteDir: "explicit"}}
	details := []steam.ModDetails{{ID: "1", Title: "Cool Mod", UpdatedAt: now}, {ID: "2", Title: "Another", UpdatedAt: now.Add(time.Hour)}}
	st := state.State{Mods: map[string]state.ModState{"1": {LastSyncedAt: now}}}
	plan := BuildPlan(mods, details, st, "/mods")
	if len(plan) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan))
	}
	if plan[0].RemotePath != "/mods/explicit" {
		t.Fatalf("unexpected remote path: %s", plan[0].RemotePath)
	}
}

func TestCountdownMessages(t *testing.T) {
	msgs := CountdownMessages(65)
	if len(msgs) != 8 {
		t.Fatalf("expected 8 messages, got %d", len(msgs))
	}
	if msgs[0] != "Server restart in 1m0s" {
		t.Fatalf("unexpected first message: %s", msgs[0])
	}
}
