//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/sftpsync"
)

func TestSFTPSyncDirectory(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 and run docker compose up -d")
	}
	local := t.TempDir()
	nested := filepath.Join(local, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "mod.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	syncer := sftpsync.New("127.0.0.1:2222", "foo", "pass")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := syncer.SyncDirectory(ctx, local, "/upload/mods/test"); err != nil {
		t.Fatalf("sync failed: %v", err)
	}
}
