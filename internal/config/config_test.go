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
"paths":{"local_mods_root":"./mods","local_cache_root":"./cache","steamcmd_path":"/usr/bin/steamcmd","steamcmd_workshop_content_root":"/steam/workshop/content"},
"steam":{"login":"u","password":"p"},
"shutdown":{"grace_period_seconds":300,"announce_every_seconds":60,"message_template":"x {minutes}","final_message":"bye"},
"concurrency":{"modlist_poll_parallelism":1,"sftp_sync_parallelism_servers":1,"sftp_sync_parallelism_mods_per_server":1,"workshop_parallelism":1,"workshop_batch_size":10},
"servers":[{"id":"s1","name":"S1","sftp":{"host":"h","port":22,"user":"u","auth":{"type":"password","password":"p"},"remote_modlist_path":"/modlist","remote_mods_root":"/mods"},"rcon":{"host":"r","port":2306,"password":"rp"}}]
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Intervals.ModlistPollSeconds != 60 {
		t.Fatalf("expected default modlist interval, got %d", cfg.Intervals.ModlistPollSeconds)
	}
	if cfg.Steam.WorkshopGameID != defaultWorkshopGameID {
		t.Fatalf("expected default workshop game id, got %d", cfg.Steam.WorkshopGameID)
	}
}

func TestValidateUniqueServerID(t *testing.T) {
	cfg := Sample()
	cfg.Servers = append(cfg.Servers, cfg.Servers[0])
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate server id validation error")
	}
}
