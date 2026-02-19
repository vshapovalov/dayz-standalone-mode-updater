package steamcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/example/dayz-standalone-mode-updater/internal/state"
)

func TestParseSuccessByModID(t *testing.T) {
	successLog, err := os.ReadFile(filepath.Join("testdata", "steamcmd_success.log"))
	if err != nil {
		t.Fatal(err)
	}
	failureLog, err := os.ReadFile(filepath.Join("testdata", "steamcmd_failure.log"))
	if err != nil {
		t.Fatal(err)
	}

	success := ParseSuccessByModID(string(successLog))
	if !success["1559212036"] || !success["2222222222"] {
		t.Fatalf("expected success markers for both mods, got %#v", success)
	}

	failed := ParseSuccessByModID(string(failureLog))
	if len(failed) != 0 {
		t.Fatalf("expected no success markers in failure log, got %#v", failed)
	}
}

func TestMirrorWorkshopContentAtomicSwap(t *testing.T) {
	root := t.TempDir()
	steamRoot := filepath.Join(root, "steam", "content")
	localMods := filepath.Join(root, "mods")
	cacheRoot := filepath.Join(root, "cache")
	appID := "221100"
	workshopID := "1559212036"
	slug := "cf-tools"

	src := filepath.Join(steamRoot, appID, workshopID)
	if err := os.MkdirAll(filepath.Join(src, "addons"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "meta.cpp"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "addons", "a.pbo"), []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(localMods, slug)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MirrorWorkshopContent(steamRoot, appID, workshopID, localMods, slug, cacheRoot); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(target, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected old file to be removed, stat err=%v", err)
	}
	b, err := os.ReadFile(filepath.Join(target, "meta.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new" {
		t.Fatalf("unexpected mirrored content: %q", string(b))
	}
	if _, err := os.Stat(filepath.Join(target, "addons", "a.pbo")); err != nil {
		t.Fatalf("expected nested mirrored file, got err=%v", err)
	}
}

func TestMarkServersUsingModForPlanning(t *testing.T) {
	st := state.State{
		Version: 1,
		Mods:    map[string]state.ModState{},
		Servers: map[string]state.ServerState{
			"s1": {LastModIDs: []string{"1", "2"}, Stage: state.StageIdle},
			"s2": {LastModIDs: []string{"3"}, Stage: state.StageSyncing},
		},
	}

	MarkServersUsingModForPlanning(&st, "2")

	if !st.Servers["s1"].NeedsModUpdate || st.Servers["s1"].Stage != state.StagePlanning {
		t.Fatalf("expected server s1 to be marked for planning, got %#v", st.Servers["s1"])
	}
	if st.Servers["s2"].NeedsModUpdate || st.Servers["s2"].Stage != state.StageSyncing {
		t.Fatalf("expected server s2 unchanged, got %#v", st.Servers["s2"])
	}
}
