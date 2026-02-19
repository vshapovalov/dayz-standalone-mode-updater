package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Stage string

const (
	StageIdle          Stage = "idle"
	StagePlanning      Stage = "planning"
	StageLocalUpdating Stage = "local_updating"
	StageSyncing       Stage = "syncing"
	StageCountdown     Stage = "countdown"
	StageShuttingDown  Stage = "shutting_down"
	StageError         Stage = "error"
)

type State struct {
	Version   int                    `json:"version"`
	UpdatedAt time.Time              `json:"updated_at"`
	Mods      map[string]ModState    `json:"mods"`
	Servers   map[string]ServerState `json:"servers"`
}

type ModState struct {
	DisplayName         string    `json:"display_name"`
	FolderSlug          string    `json:"folder_slug"`
	WorkshopUpdatedAt   time.Time `json:"workshop_updated_at"`
	LastWorkshopCheckAt time.Time `json:"last_workshop_check_at"`
	LocalUpdatedAt      time.Time `json:"local_updated_at"`
	LastSyncedAt        time.Time `json:"last_synced_at,omitempty"`
	LastTitle           string    `json:"last_title,omitempty"`
}

type ServerState struct {
	LastModIDs         []string             `json:"last_mod_ids"`
	LastModsetHash     string               `json:"last_modset_hash"`
	NeedsModUpdate     bool                 `json:"needs_mod_update"`
	NeedsShutdown      bool                 `json:"needs_shutdown"`
	Stage              Stage                `json:"stage"`
	SyncedMods         map[string]time.Time `json:"synced_mods"`
	ShutdownDeadlineAt *time.Time           `json:"shutdown_deadline_at,omitempty"`
	NextAnnounceAt     *time.Time           `json:"next_announce_at,omitempty"`
	LastError          string               `json:"last_error,omitempty"`
	LastErrorStage     string               `json:"last_error_stage,omitempty"`
	LastErrorAt        *time.Time           `json:"last_error_at,omitempty"`
	LastSuccessSyncAt  *time.Time           `json:"last_success_sync_at,omitempty"`
	ShutdownSentAt     *time.Time           `json:"shutdown_sent_at,omitempty"`
}

type StateStore interface {
	Load() (State, error)
	Save(State) error
	Update(func(*State) error) error
}

type FileStore struct {
	path string
	mu   sync.Mutex
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func Load(path string) (State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultState(), nil
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	normalize(&s)
	return s, nil
}

func (fs *FileStore) Load() (State, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return Load(fs.path)
}

func (fs *FileStore) Save(s State) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return SaveAtomic(fs.path, s)
}

func (fs *FileStore) Update(fn func(*State) error) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	s, err := Load(fs.path)
	if err != nil {
		return err
	}
	if err := fn(&s); err != nil {
		return err
	}
	return SaveAtomic(fs.path, s)
}

func SaveAtomic(path string, s State) error {
	normalize(&s)
	s.UpdatedAt = time.Now().UTC()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ensure state dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		tmp.Close()
		return fmt.Errorf("encode state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic replace state: %w", err)
	}
	return nil
}

func defaultState() State {
	return State{Version: 1, Mods: map[string]ModState{}, Servers: map[string]ServerState{}}
}

func normalize(s *State) {
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Mods == nil {
		s.Mods = map[string]ModState{}
	}
	if s.Servers == nil {
		s.Servers = map[string]ServerState{}
	}
	for id, server := range s.Servers {
		if server.Stage == "" {
			server.Stage = StageIdle
		}
		if server.SyncedMods == nil {
			server.SyncedMods = map[string]time.Time{}
		}
		s.Servers[id] = server
	}
}
