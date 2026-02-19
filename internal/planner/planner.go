package planner

import (
	"sort"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
	"github.com/example/dayz-standalone-mode-updater/internal/steam"
	"github.com/example/dayz-standalone-mode-updater/internal/util"
)

type Action struct {
	ModID      string
	Title      string
	LocalPath  string
	RemotePath string
	UpdatedAt  time.Time
}

func BuildPlan(modCfg []config.ModConfig, details []steam.ModDetails, st state.State, remoteRoot string) []Action {
	byID := make(map[string]steam.ModDetails, len(details))
	for _, d := range details {
		byID[d.ID] = d
	}

	actions := make([]Action, 0)
	for _, mc := range modCfg {
		d, ok := byID[mc.ID]
		if !ok {
			continue
		}
		ms, ok := st.Mods[mc.ID]
		if ok && !d.UpdatedAt.After(ms.LastSyncedAt) {
			continue
		}
		remoteDir := mc.RemoteDir
		if remoteDir == "" {
			remoteDir = util.Slugify(d.Title)
		}
		actions = append(actions, Action{
			ModID:      mc.ID,
			Title:      d.Title,
			LocalPath:  mc.LocalPath,
			RemotePath: remoteRoot + "/" + remoteDir,
			UpdatedAt:  d.UpdatedAt,
		})
	}

	sort.Slice(actions, func(i, j int) bool {
		return actions[i].UpdatedAt.Before(actions[j].UpdatedAt)
	})
	return actions
}

func CountdownMessages(totalSeconds int) []string {
	if totalSeconds <= 0 {
		return nil
	}
	marks := []int{300, 120, 60, 30, 10, 5, 4, 3, 2, 1}
	out := make([]string, 0)
	for _, sec := range marks {
		if sec <= totalSeconds {
			out = append(out, "Server restart in "+(time.Duration(sec)*time.Second).String())
		}
	}
	return out
}

func NextTick(now time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		interval = time.Minute
	}
	return now.Add(interval)
}
