package sftpsync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Engine struct {
	now func() time.Time
}

type treeEntry struct {
	Path    string
	IsDir   bool
	Size    int64
	MTime   int64
	ModTime time.Time
}

type syncPlan struct {
	deleteTypeConflicts []treeEntry
	mkdirs              []treeEntry
	uploads             []treeEntry
	deleteExtrasFiles   []treeEntry
	deleteExtrasDirs    []treeEntry
}

func NewEngine() *Engine {
	return &Engine{now: func() time.Time { return time.Now().UTC() }}
}

func (e *Engine) SyncServers(ctx context.Context, cfg config.Config, st *state.State) error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, cfg.Concurrency.SFTPSyncParallelismServers)
	errCh := make(chan error, len(cfg.Servers))
	var mu sync.Mutex

	for _, serverCfg := range cfg.Servers {
		srv := st.Servers[serverCfg.ID]
		if !srv.NeedsModUpdate {
			continue
		}
		serverCfg := serverCfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			mu.Lock()
			srvState := st.Servers[serverCfg.ID]
			mu.Unlock()

			updated, err := e.syncServer(ctx, cfg, serverCfg, st.Mods, srvState)
			mu.Lock()
			st.Servers[serverCfg.ID] = updated
			mu.Unlock()
			if err != nil {
				errCh <- fmt.Errorf("sync server %s: %w", serverCfg.ID, err)
			}
		}()
	}

	wg.Wait()
	close(errCh)
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *Engine) syncServer(ctx context.Context, cfg config.Config, server config.ServerConfig, mods map[string]state.ModState, srv state.ServerState) (state.ServerState, error) {
	if srv.SyncedMods == nil {
		srv.SyncedMods = map[string]time.Time{}
	}
	modsToSync := make([]string, 0, len(srv.LastModIDs))
	for _, id := range srv.LastModIDs {
		mod, ok := mods[id]
		if !ok {
			continue
		}
		if mod.LocalUpdatedAt.IsZero() {
			srv.Stage = state.StageError
			srv.NeedsModUpdate = true
			srv.LastError = fmt.Sprintf("mod %s local_updated_at is null", id)
			srv.LastErrorStage = "compute_mods_to_sync"
			now := e.now()
			srv.LastErrorAt = &now
			return srv, fmt.Errorf("mod %s local_updated_at is zero", id)
		}
		if !mod.LocalUpdatedAt.Equal(srv.SyncedMods[id]) {
			modsToSync = append(modsToSync, id)
		}
	}
	if len(modsToSync) == 0 {
		srv.NeedsModUpdate = false
		srv.NeedsShutdown = true
		srv.Stage = state.StageCountdown
		now := e.now()
		deadline := now.Add(time.Duration(cfg.Shutdown.GracePeriodSeconds) * time.Second)
		srv.ShutdownDeadlineAt = &deadline
		srv.NextAnnounceAt = &now
		return srv, nil
	}

	client, sshClient, err := dialSFTP(server)
	if err != nil {
		srv.Stage = state.StageError
		srv.NeedsModUpdate = true
		recordSyncError(&srv, "connect", "connect sftp", "", err, e.now)
		return srv, err
	}
	defer sshClient.Close()
	defer client.Close()

	sem := make(chan struct{}, cfg.Concurrency.SFTPSyncParallelismModsPerServer)
	var wg sync.WaitGroup
	var mu sync.Mutex
	hadFailure := false
	for _, id := range modsToSync {
		id := id
		mod := mods[id]
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			local := filepath.Join(cfg.Paths.LocalModsRoot, mod.FolderSlug)
			remote := path.Join(server.SFTP.RemoteModsRoot, mod.FolderSlug)
			if err := syncMod(ctx, client, local, remote); err != nil {
				mu.Lock()
				hadFailure = true
				recordSyncError(&srv, "sync_mod", "sync mod", id, err, e.now)
				mu.Unlock()
				return
			}
			mu.Lock()
			srv.SyncedMods[id] = mod.LocalUpdatedAt
			mu.Unlock()
		}()
	}
	wg.Wait()

	if hadFailure {
		srv.NeedsModUpdate = true
		srv.Stage = state.StageError
		return srv, fmt.Errorf("at least one mod failed to sync")
	}
	now := e.now()
	srv.NeedsModUpdate = false
	srv.NeedsShutdown = true
	srv.Stage = state.StageCountdown
	deadline := now.Add(time.Duration(cfg.Shutdown.GracePeriodSeconds) * time.Second)
	srv.ShutdownDeadlineAt = &deadline
	srv.NextAnnounceAt = &now
	srv.LastSuccessSyncAt = &now
	return srv, nil
}

func recordSyncError(srv *state.ServerState, stage, step, modID string, err error, nowFn func() time.Time) {
	now := nowFn()
	if modID != "" {
		srv.LastError = fmt.Sprintf("mod %s step %s: %v", modID, step, err)
	} else {
		srv.LastError = fmt.Sprintf("step %s: %v", step, err)
	}
	srv.LastErrorStage = stage
	srv.LastErrorAt = &now
}

func syncMod(ctx context.Context, client *sftp.Client, localModPath, remoteModPath string) error {
	localTree, err := buildLocalTree(localModPath)
	if err != nil {
		return fmt.Errorf("build local tree: %w", err)
	}
	remoteTree, err := buildRemoteTree(client, remoteModPath)
	if err != nil {
		return fmt.Errorf("build remote tree: %w", err)
	}
	plan := buildPlan(localTree, remoteTree)

	for _, entry := range plan.deleteTypeConflicts {
		if err := deleteRemoteEntry(client, path.Join(remoteModPath, entry.Path), entry.IsDir); err != nil {
			return fmt.Errorf("delete type conflict %s: %w", entry.Path, err)
		}
	}
	for _, dir := range plan.mkdirs {
		if err := client.MkdirAll(path.Join(remoteModPath, dir.Path)); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir.Path, err)
		}
	}
	for _, file := range plan.uploads {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := uploadAtomically(client, filepath.Join(localModPath, filepath.FromSlash(file.Path)), path.Join(remoteModPath, file.Path), file.ModTime); err != nil {
			return fmt.Errorf("upload %s: %w", file.Path, err)
		}
	}
	for _, file := range plan.deleteExtrasFiles {
		if err := client.Remove(path.Join(remoteModPath, file.Path)); err != nil {
			return fmt.Errorf("delete extra file %s: %w", file.Path, err)
		}
	}
	for _, dir := range plan.deleteExtrasDirs {
		if err := client.RemoveDirectory(path.Join(remoteModPath, dir.Path)); err != nil {
			return fmt.Errorf("delete extra dir %s: %w", dir.Path, err)
		}
	}
	return nil
}

func deleteRemoteEntry(client *sftp.Client, fullPath string, isDir bool) error {
	if isDir {
		return client.RemoveDirectory(fullPath)
	}
	return client.Remove(fullPath)
}

func uploadAtomically(client *sftp.Client, localPath, remotePath string, mtime time.Time) error {
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	tmpPath := remotePath + fmt.Sprintf(".tmp-%d", time.Now().UnixNano())
	dst, err := client.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		_ = client.Remove(tmpPath)
		return err
	}
	if err := dst.Close(); err != nil {
		_ = client.Remove(tmpPath)
		return err
	}
	if err := client.Rename(tmpPath, remotePath); err != nil {
		_ = client.Remove(tmpPath)
		return err
	}
	sec := mtime.UTC().Truncate(time.Second)
	if err := client.Chtimes(remotePath, sec, sec); err != nil {
		return err
	}
	return nil
}

func buildLocalTree(root string) (map[string]treeEntry, error) {
	tree := map[string]treeEntry{}
	if err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		mt := info.ModTime().UTC().Truncate(time.Second)
		tree[rel] = treeEntry{Path: rel, IsDir: info.IsDir(), Size: info.Size(), MTime: mt.Unix(), ModTime: mt}
		return nil
	}); err != nil {
		return nil, err
	}
	return tree, nil
}

func buildRemoteTree(client *sftp.Client, root string) (map[string]treeEntry, error) {
	tree := map[string]treeEntry{}
	if _, err := client.Stat(root); err != nil {
		if os.IsNotExist(err) || strings.Contains(strings.ToLower(err.Error()), "not exist") {
			return tree, nil
		}
		return nil, err
	}
	walker := client.Walk(root)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return nil, err
		}
		info := walker.Stat()
		if info == nil {
			continue
		}
		rel := strings.TrimPrefix(walker.Path(), root)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || rel == "." {
			continue
		}
		mt := info.ModTime().UTC().Truncate(time.Second)
		tree[rel] = treeEntry{Path: rel, IsDir: info.IsDir(), Size: info.Size(), MTime: mt.Unix(), ModTime: mt}
	}
	return tree, nil
}

func buildPlan(localTree, remoteTree map[string]treeEntry) syncPlan {
	plan := syncPlan{}
	for rel, local := range localTree {
		remote, exists := remoteTree[rel]
		if !exists {
			if local.IsDir {
				plan.mkdirs = append(plan.mkdirs, local)
			} else {
				plan.uploads = append(plan.uploads, local)
			}
			continue
		}
		if local.IsDir != remote.IsDir {
			plan.deleteTypeConflicts = append(plan.deleteTypeConflicts, remote)
			if local.IsDir {
				plan.mkdirs = append(plan.mkdirs, local)
			} else {
				plan.uploads = append(plan.uploads, local)
			}
			continue
		}
		if !local.IsDir && (local.Size != remote.Size || local.MTime != remote.MTime) {
			plan.uploads = append(plan.uploads, local)
		}
	}
	for rel, remote := range remoteTree {
		if _, exists := localTree[rel]; exists {
			continue
		}
		if remote.IsDir {
			plan.deleteExtrasDirs = append(plan.deleteExtrasDirs, remote)
		} else {
			plan.deleteExtrasFiles = append(plan.deleteExtrasFiles, remote)
		}
	}

	sort.Slice(plan.deleteTypeConflicts, func(i, j int) bool {
		return pathDepth(plan.deleteTypeConflicts[i].Path) > pathDepth(plan.deleteTypeConflicts[j].Path)
	})
	sort.Slice(plan.mkdirs, func(i, j int) bool {
		di := pathDepth(plan.mkdirs[i].Path)
		dj := pathDepth(plan.mkdirs[j].Path)
		if di == dj {
			return plan.mkdirs[i].Path < plan.mkdirs[j].Path
		}
		return di < dj
	})
	sort.Slice(plan.uploads, func(i, j int) bool { return plan.uploads[i].Path < plan.uploads[j].Path })
	sort.Slice(plan.deleteExtrasFiles, func(i, j int) bool { return plan.deleteExtrasFiles[i].Path < plan.deleteExtrasFiles[j].Path })
	sort.Slice(plan.deleteExtrasDirs, func(i, j int) bool {
		di := pathDepth(plan.deleteExtrasDirs[i].Path)
		dj := pathDepth(plan.deleteExtrasDirs[j].Path)
		if di == dj {
			return plan.deleteExtrasDirs[i].Path > plan.deleteExtrasDirs[j].Path
		}
		return di > dj
	})
	return plan
}

func pathDepth(p string) int {
	if p == "" {
		return 0
	}
	return strings.Count(p, "/") + 1
}

func dialSFTP(server config.ServerConfig) (*sftp.Client, *ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{User: server.SFTP.User, HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	switch server.SFTP.Auth.Type {
	case "password":
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(server.SFTP.Auth.Password)}
	default:
		return nil, nil, fmt.Errorf("unsupported sftp auth type %q", server.SFTP.Auth.Type)
	}
	addr := fmt.Sprintf("%s:%d", server.SFTP.Host, server.SFTP.Port)
	sshConn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, nil, err
	}
	client, err := sftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return nil, nil, err
	}
	return client, sshConn, nil
}
