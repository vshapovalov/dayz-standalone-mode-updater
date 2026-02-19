package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAtomicAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	expected := State{Mods: map[string]ModState{"1": {LastSyncedAt: time.Unix(1700000000, 0).UTC(), LastTitle: "CF"}}}
	if err := SaveAtomic(path, expected); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mods["1"].LastTitle != "CF" {
		t.Fatalf("unexpected title: %s", got.Mods["1"].LastTitle)
	}
}
