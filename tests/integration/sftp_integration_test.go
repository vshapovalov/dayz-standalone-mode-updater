//go:build integration

package integration

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/sftpsync"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func TestSFTPSyncDirectoryPlanExecution(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION") != "1" {
		t.Skip("set RUN_INTEGRATION=1 and run docker compose up -d")
	}
	local := t.TempDir()
	remoteRoot := "/upload/mods/test"

	mustMkdirAll(t, filepath.Join(local, "a", "b"))
	writeFileWithMtime(t, filepath.Join(local, "a", "b", "mod.txt"), []byte("changed"), time.Unix(1700000010, 0).UTC())
	writeFileWithMtime(t, filepath.Join(local, "new.txt"), []byte("new"), time.Unix(1700000020, 0).UTC())

	client, sshConn := connectSFTP(t)
	defer sshConn.Close()
	defer client.Close()

	mustMkdirRemoteAll(t, client, path.Join(remoteRoot, "a", "b"))
	writeRemoteFileWithMtime(t, client, path.Join(remoteRoot, "a", "b", "mod.txt"), []byte("old"), time.Unix(1700000000, 0).UTC())
	writeRemoteFileWithMtime(t, client, path.Join(remoteRoot, "extra.txt"), []byte("extra"), time.Unix(1700000005, 0).UTC())

	syncer := sftpsync.New("127.0.0.1:2222", "foo", "pass")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := syncer.SyncDirectory(ctx, local, remoteRoot); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	assertRemoteFileEquals(t, client, path.Join(remoteRoot, "a", "b", "mod.txt"), []byte("changed"))
	assertRemoteFileEquals(t, client, path.Join(remoteRoot, "new.txt"), []byte("new"))
	if _, err := client.Stat(path.Join(remoteRoot, "extra.txt")); err == nil {
		t.Fatalf("expected extra file to be deleted")
	}
	info, err := client.Stat(path.Join(remoteRoot, "a", "b", "mod.txt"))
	if err != nil {
		t.Fatalf("stat changed file: %v", err)
	}
	if got, want := info.ModTime().UTC().Truncate(time.Second), time.Unix(1700000010, 0).UTC(); !got.Equal(want) {
		t.Fatalf("mtime mismatch got=%s want=%s", got, want)
	}
}

func connectSFTP(t *testing.T) (*sftp.Client, *ssh.Client) {
	t.Helper()
	conn, err := ssh.Dial("tcp", "127.0.0.1:2222", &ssh.ClientConfig{
		User:            "foo",
		Auth:            []ssh.AuthMethod{ssh.Password("pass")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatalf("dial sftp: %v", err)
	}
	client, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		t.Fatalf("new sftp client: %v", err)
	}
	return client, conn
}

func mustMkdirAll(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFileWithMtime(t *testing.T, p string, content []byte, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func mustMkdirRemoteAll(t *testing.T, client *sftp.Client, p string) {
	t.Helper()
	if err := client.MkdirAll(p); err != nil {
		t.Fatalf("mkdir remote: %v", err)
	}
}

func writeRemoteFileWithMtime(t *testing.T, client *sftp.Client, p string, content []byte, mtime time.Time) {
	t.Helper()
	f, err := client.Create(p)
	if err != nil {
		t.Fatalf("create remote file: %v", err)
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		t.Fatalf("write remote file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close remote file: %v", err)
	}
	if err := client.Chtimes(p, mtime, mtime); err != nil {
		t.Fatalf("set remote mtime: %v", err)
	}
}

func assertRemoteFileEquals(t *testing.T, client *sftp.Client, p string, want []byte) {
	t.Helper()
	f, err := client.Open(p)
	if err != nil {
		t.Fatalf("open remote file: %v", err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read remote file: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("content mismatch got=%q want=%q", got, want)
	}
}
