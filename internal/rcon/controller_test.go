package rcon

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
)

func TestRemainingMinutesUsesCeil(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(61 * time.Second)
	if got := RemainingMinutes(deadline, now); got != 2 {
		t.Fatalf("expected 2 remaining minutes, got %d", got)
	}
}

func TestFormatMessage(t *testing.T) {
	got := FormatMessage("Restart in {minutes} minutes", 3)
	if got != "Restart in 3 minutes" {
		t.Fatalf("unexpected formatted message: %q", got)
	}
}

func TestTickStateTransitionsCountdownToShutdownToIdle(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(90 * time.Second)
	stateData := state.State{
		Servers: map[string]state.ServerState{
			"s1": {
				NeedsShutdown:      true,
				Stage:              state.StageCountdown,
				ShutdownDeadlineAt: &deadline,
			},
		},
	}

	fake := &fakeRCONClient{}
	controller := NewController(testConfig()).WithLogger(t.Logf)
	controller.dial = func(address, password string) (commandClient, error) {
		if address != "127.0.0.1:2302" {
			t.Fatalf("unexpected address: %s", address)
		}
		if password != "secret" {
			t.Fatalf("unexpected password: %s", password)
		}
		return fake, nil
	}

	controller.Tick(context.Background(), now, &stateData)
	if len(fake.commands) != 1 || fake.commands[0] != "say -1 Restart in 2 minutes" {
		t.Fatalf("unexpected announce commands: %+v", fake.commands)
	}
	if stateData.Servers["s1"].NextAnnounceAt == nil {
		t.Fatalf("expected next announce time to be set")
	}
	if !stateData.Servers["s1"].NeedsShutdown {
		t.Fatalf("server should still need shutdown before deadline")
	}

	fake.commands = nil
	afterDeadline := now.Add(2 * time.Minute)
	controller.Tick(context.Background(), afterDeadline, &stateData)

	if len(fake.commands) != 2 {
		t.Fatalf("expected final message and shutdown command, got %+v", fake.commands)
	}
	if fake.commands[0] != "say -1 Server shutting down now" || fake.commands[1] != "#shutdown" {
		t.Fatalf("unexpected shutdown command sequence: %+v", fake.commands)
	}
	serverState := stateData.Servers["s1"]
	if serverState.NeedsShutdown {
		t.Fatalf("expected needs_shutdown=false after shutdown")
	}
	if serverState.Stage != state.StageIdle {
		t.Fatalf("expected stage idle, got %s", serverState.Stage)
	}
	if serverState.ShutdownSentAt == nil || !serverState.ShutdownSentAt.Equal(afterDeadline.UTC()) {
		t.Fatalf("expected shutdown_sent_at to be set to tick time")
	}
}

func TestTickKeepsNeedsShutdownOnConnectError(t *testing.T) {
	deadline := time.Now().UTC().Add(time.Minute)
	stateData := state.State{Servers: map[string]state.ServerState{"s1": {
		NeedsShutdown:      true,
		Stage:              state.StageCountdown,
		ShutdownDeadlineAt: &deadline,
	}}}
	controller := NewController(testConfig()).WithLogger(t.Logf)
	controller.dial = func(address, password string) (commandClient, error) {
		return nil, errors.New("dial failed")
	}

	controller.Tick(context.Background(), time.Now().UTC(), &stateData)
	if !stateData.Servers["s1"].NeedsShutdown {
		t.Fatalf("expected needs_shutdown to remain true on connection failure")
	}
}

type fakeRCONClient struct {
	commands []string
}

func (f *fakeRCONClient) Command(command string) (string, error) {
	f.commands = append(f.commands, command)
	return "", nil
}

func (f *fakeRCONClient) Close() error {
	return nil
}

func testConfig() config.Config {
	return config.Config{
		Shutdown: config.ShutdownConfig{
			AnnounceEverySeconds: 30,
			MessageTemplate:      "Restart in {minutes} minutes",
			FinalMessage:         "Server shutting down now",
		},
		Servers: []config.ServerConfig{{
			ID: "s1",
			RCON: config.ServerRCONConfig{
				Host:     "127.0.0.1",
				Port:     2302,
				Password: "secret",
			},
		}},
	}
}

func (f *fakeRCONClient) String() string {
	return fmt.Sprintf("%v", f.commands)
}
