package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAtomicAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Unix(1700000000, 0).UTC()
	expected := State{
		Version: 1,
		Mods: map[string]ModState{"1": {
			DisplayName:         "CF",
			FolderSlug:          "cf",
			WorkshopUpdatedAt:   now,
			LastWorkshopCheckAt: now,
			LocalUpdatedAt:      now,
		}},
		Servers: map[string]ServerState{"srv1": {Stage: StageIdle}},
	}
	if err := SaveAtomic(path, expected); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mods["1"].DisplayName != "CF" {
		t.Fatalf("unexpected display name: %s", got.Mods["1"].DisplayName)
	}
	if got.Servers["srv1"].Stage != StageIdle {
		t.Fatalf("unexpected stage: %s", got.Servers["srv1"].Stage)
	}
}

func TestFileStoreUpdate(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "state.json"))
	if err := store.Update(func(s *State) error {
		s.Mods["123"] = ModState{DisplayName: "Test"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if s.Mods["123"].DisplayName != "Test" {
		t.Fatal("expected mod in updated state")
	}
}
