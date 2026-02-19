package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{
"steam":{"api_key":"k"},
"sftp":{"address":"a","username":"u","password":"p"},
"mods":[{"id":"1","local_path":"./m"}]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PollIntervalSeconds != 300 {
		t.Fatalf("expected default poll interval, got %d", cfg.PollIntervalSeconds)
	}
	if cfg.StatePath != "state.json" {
		t.Fatalf("expected default state path, got %q", cfg.StatePath)
	}
}
