package steamcmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
)

var (
	successPattern = regexp.MustCompile(`(?mi)Success\.\s+Downloaded\s+item\s+([0-9]+)`)
)

type Runner interface {
	UpdateMods(ctx context.Context, modIDs []string, st *state.State) ([]string, error)
}

type CommandRunner struct {
	cfg config.Config
}

func NewRunner(cfg config.Config) *CommandRunner {
	return &CommandRunner{cfg: cfg}
}

func (r *CommandRunner) UpdateMods(ctx context.Context, modIDs []string, st *state.State) ([]string, error) {
	if len(modIDs) == 0 {
		return nil, nil
	}
	output, err := r.runSteamCMD(ctx, modIDs)
	if err != nil {
		return nil, err
	}

	results := ParseSuccessByModID(output)
	succeeded := make([]string, 0, len(modIDs))
	for _, id := range modIDs {
		if !results[id] || !r.hasDownloadedContent(id) {
			continue
		}
		modState := st.Mods[id]
		if err := MirrorWorkshopContent(
			r.cfg.Paths.SteamcmdWorkshopContentRoot,
			fmt.Sprintf("%d", r.cfg.Steam.WorkshopGameID),
			id,
			r.cfg.Paths.LocalModsRoot,
			modState.FolderSlug,
			r.cfg.Paths.LocalCacheRoot,
		); err != nil {
			return succeeded, fmt.Errorf("mirror workshop mod %s: %w", id, err)
		}

		if modState.WorkshopUpdatedAt.IsZero() {
			modState.LocalUpdatedAt = time.Now().UTC()
		} else {
			modState.LocalUpdatedAt = modState.WorkshopUpdatedAt
		}
		st.Mods[id] = modState
		MarkServersUsingModForPlanning(st, id)
		succeeded = append(succeeded, id)
	}
	return succeeded, nil
}

func (r *CommandRunner) runSteamCMD(ctx context.Context, modIDs []string) (string, error) {
	args := []string{"+login", r.cfg.Steam.Login, r.cfg.Steam.Password}
	for _, id := range modIDs {
		args = append(args, "+workshop_download_item", fmt.Sprintf("%d", r.cfg.Steam.WorkshopGameID), id, "validate")
	}
	args = append(args, "+quit")

	cmd := exec.CommandContext(ctx, r.cfg.Paths.SteamcmdPath, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	sanitized := RedactPassword(output.String(), r.cfg.Steam.Password)
	if writeErr := writeSteamCMDLog(r.cfg.Paths.LocalCacheRoot, sanitized); writeErr != nil {
		return sanitized, writeErr
	}
	if err != nil {
		return sanitized, fmt.Errorf("run steamcmd: %w", err)
	}
	return sanitized, nil
}

func ParseSuccessByModID(logOutput string) map[string]bool {
	matches := successPattern.FindAllStringSubmatch(logOutput, -1)
	res := make(map[string]bool, len(matches))
	for _, m := range matches {
		res[m[1]] = true
	}
	return res
}

func RedactPassword(logOutput, password string) string {
	if password == "" {
		return logOutput
	}
	return strings.ReplaceAll(logOutput, password, "[REDACTED]")
}

func writeSteamCMDLog(localCacheRoot, content string) error {
	path := filepath.Join(localCacheRoot, "logs", "steamcmd.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure steamcmd log dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write steamcmd log: %w", err)
	}
	return nil
}

func (r *CommandRunner) hasDownloadedContent(modID string) bool {
	path := filepath.Join(
		r.cfg.Paths.SteamcmdWorkshopContentRoot,
		fmt.Sprintf("%d", r.cfg.Steam.WorkshopGameID),
		modID,
	)
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func MirrorWorkshopContent(steamcmdContentRoot, appID, workshopID, localModsRoot, folderSlug, localCacheRoot string) error {
	if strings.TrimSpace(folderSlug) == "" {
		folderSlug = "mod-" + workshopID
	}
	source := filepath.Join(steamcmdContentRoot, appID, workshopID)
	target := filepath.Join(localModsRoot, folderSlug)
	stagingRoot := filepath.Join(localCacheRoot, "staging")
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return fmt.Errorf("ensure staging root: %w", err)
	}
	stagingDir, err := os.MkdirTemp(stagingRoot, folderSlug+"-")
	if err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	if err := copyDir(source, stagingDir); err != nil {
		return fmt.Errorf("copy workshop dir: %w", err)
	}
	if err := os.MkdirAll(localModsRoot, 0o755); err != nil {
		return fmt.Errorf("ensure local mods root: %w", err)
	}

	backup := filepath.Join(stagingRoot, folderSlug+"-backup-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("backup existing mod dir: %w", err)
		}
	}
	if err := os.Rename(stagingDir, target); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("atomic swap mod dir: %w", err)
	}
	_ = os.RemoveAll(backup)
	return nil
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not directory: %s", src)
	}
	return filepath.Walk(src, func(path string, d os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, d.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target, d.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func MarkServersUsingModForPlanning(st *state.State, modID string) {
	for serverID, srv := range st.Servers {
		if contains(srv.LastModIDs, modID) {
			srv.NeedsModUpdate = true
			srv.Stage = state.StagePlanning
			st.Servers[serverID] = srv
		}
	}
}

func contains(ids []string, modID string) bool {
	for _, id := range ids {
		if id == modID {
			return true
		}
	}
	return false
}
