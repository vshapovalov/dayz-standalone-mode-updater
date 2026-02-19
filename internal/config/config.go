package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const defaultWorkshopGameID = 221100

type Config struct {
	Version             int               `json:"version"`
	PollIntervalSeconds int               `json:"poll_interval_seconds,omitempty"` // backward-compatible optional field.
	Paths               PathsConfig       `json:"paths"`
	Steam               SteamConfig       `json:"steam"`
	Intervals           IntervalsConfig   `json:"intervals"`
	Shutdown            ShutdownConfig    `json:"shutdown"`
	Concurrency         ConcurrencyConfig `json:"concurrency"`
	Servers             []ServerConfig    `json:"servers"`
	StatePath           string            `json:"state_path,omitempty"` // backward-compatible optional field.
	Mods                []ModConfig       `json:"mods,omitempty"`       // backward-compatible optional field.
	RCON                LegacyRCONConfig  `json:"rcon,omitempty"`
	SFTP                LegacySFTPConfig  `json:"sftp,omitempty"`
}

type PathsConfig struct {
	LocalModsRoot               string `json:"local_mods_root"`
	LocalCacheRoot              string `json:"local_cache_root"`
	SteamcmdPath                string `json:"steamcmd_path"`
	SteamcmdWorkshopContentRoot string `json:"steamcmd_workshop_content_root"`
}

type SteamConfig struct {
	APIKey         string `json:"api_key,omitempty"`
	Login          string `json:"login"`
	Password       string `json:"password"`
	WorkshopGameID int    `json:"workshop_game_id"`
	WebAPIKey      string `json:"web_api_key,omitempty"`
}

type IntervalsConfig struct {
	ModlistPollSeconds  int `json:"modlist_poll_seconds"`
	WorkshopPollSeconds int `json:"workshop_poll_seconds"`
	RconTickSeconds     int `json:"rcon_tick_seconds"`
	StateFlushSeconds   int `json:"state_flush_seconds"`
}

type ShutdownConfig struct {
	GracePeriodSeconds   int    `json:"grace_period_seconds"`
	AnnounceEverySeconds int    `json:"announce_every_seconds"`
	MessageTemplate      string `json:"message_template"`
	FinalMessage         string `json:"final_message"`
}

type ConcurrencyConfig struct {
	ModlistPollParallelism           int `json:"modlist_poll_parallelism"`
	SFTPSyncParallelismServers       int `json:"sftp_sync_parallelism_servers"`
	SFTPSyncParallelismModsPerServer int `json:"sftp_sync_parallelism_mods_per_server"`
	WorkshopParallelism              int `json:"workshop_parallelism"`
	WorkshopBatchSize                int `json:"workshop_batch_size"`
}

type ServerConfig struct {
	ID   string           `json:"id"`
	Name string           `json:"name"`
	SFTP ServerSFTPConfig `json:"sftp"`
	RCON ServerRCONConfig `json:"rcon"`
}

type ServerSFTPConfig struct {
	Host              string         `json:"host"`
	Port              int            `json:"port"`
	User              string         `json:"user"`
	Auth              SFTPAuthConfig `json:"auth"`
	RemoteModlistPath string         `json:"remote_modlist_path"`
	RemoteModsRoot    string         `json:"remote_mods_root"`
}

type SFTPAuthConfig struct {
	Type           string `json:"type"`
	Password       string `json:"password,omitempty"`
	PrivateKeyPath string `json:"private_key_path,omitempty"`
	Passphrase     string `json:"passphrase,omitempty"`
}

type ServerRCONConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
}

type ModConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	LocalPath string `json:"local_path"`
	RemoteDir string `json:"remote_dir"`
}

type LegacyRCONConfig struct {
	Address             string `json:"address"`
	Password            string `json:"password"`
	PreRestartCountdown int    `json:"pre_restart_countdown_seconds"`
}

type LegacySFTPConfig struct {
	Address    string `json:"address"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	RemoteRoot string `json:"remote_root"`
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.StatePath == "" {
		c.StatePath = "state.json"
	}
	if c.Version == 0 {
		c.Version = 1
	}
	if c.Steam.WorkshopGameID == 0 {
		c.Steam.WorkshopGameID = defaultWorkshopGameID
	}
	if c.Intervals.ModlistPollSeconds <= 0 {
		if c.PollIntervalSeconds > 0 {
			c.Intervals.ModlistPollSeconds = c.PollIntervalSeconds
		} else {
			c.Intervals.ModlistPollSeconds = 60
		}
	}
	if c.Intervals.WorkshopPollSeconds <= 0 {
		c.Intervals.WorkshopPollSeconds = 300
	}
	if c.Intervals.RconTickSeconds <= 0 {
		c.Intervals.RconTickSeconds = 5
	}
	if c.Intervals.StateFlushSeconds <= 0 {
		c.Intervals.StateFlushSeconds = 15
	}
	for i := range c.Servers {
		if c.Servers[i].SFTP.RemoteModlistPath == "" {
			c.Servers[i].SFTP.RemoteModlistPath = "/modlist.html"
		}
	}
}

func (c Config) Validate() error {
	if c.Paths.LocalModsRoot == "" || c.Paths.LocalCacheRoot == "" || c.Paths.SteamcmdPath == "" || c.Paths.SteamcmdWorkshopContentRoot == "" {
		return fmt.Errorf("paths.local_mods_root, paths.local_cache_root, paths.steamcmd_path, and paths.steamcmd_workshop_content_root are required")
	}
	if c.Steam.Login == "" || c.Steam.Password == "" {
		return fmt.Errorf("steam.login and steam.password are required")
	}
	if c.Shutdown.GracePeriodSeconds <= 0 || c.Shutdown.AnnounceEverySeconds <= 0 || c.Shutdown.MessageTemplate == "" || c.Shutdown.FinalMessage == "" {
		return fmt.Errorf("shutdown.grace_period_seconds, shutdown.announce_every_seconds, shutdown.message_template, and shutdown.final_message are required")
	}
	if c.Concurrency.ModlistPollParallelism <= 0 || c.Concurrency.SFTPSyncParallelismServers <= 0 || c.Concurrency.SFTPSyncParallelismModsPerServer <= 0 || c.Concurrency.WorkshopParallelism <= 0 || c.Concurrency.WorkshopBatchSize <= 0 {
		return fmt.Errorf("all concurrency fields must be greater than zero")
	}
	if len(c.Servers) == 0 {
		return fmt.Errorf("at least one server is required")
	}
	seen := make(map[string]struct{}, len(c.Servers))
	for i := range c.Servers {
		srv := c.Servers[i]
		if srv.ID == "" || srv.Name == "" {
			return fmt.Errorf("servers[%d].id and servers[%d].name are required", i, i)
		}
		if _, ok := seen[srv.ID]; ok {
			return fmt.Errorf("servers[%d].id %q is duplicated", i, srv.ID)
		}
		seen[srv.ID] = struct{}{}
		if srv.SFTP.Host == "" || srv.SFTP.Port <= 0 || srv.SFTP.User == "" || srv.SFTP.RemoteModlistPath == "" || srv.SFTP.RemoteModsRoot == "" {
			return fmt.Errorf("servers[%d].sftp host/port/user/remote_modlist_path/remote_mods_root are required", i)
		}
		if err := validateSFTPAuth(i, srv.SFTP.Auth); err != nil {
			return err
		}
		if srv.RCON.Host == "" || srv.RCON.Port <= 0 || srv.RCON.Password == "" {
			return fmt.Errorf("servers[%d].rcon host/port/password are required", i)
		}
	}
	return nil
}

func validateSFTPAuth(i int, auth SFTPAuthConfig) error {
	switch auth.Type {
	case "password":
		if auth.Password == "" {
			return fmt.Errorf("servers[%d].sftp.auth.password is required when auth.type=password", i)
		}
	case "private_key":
		if auth.PrivateKeyPath == "" {
			return fmt.Errorf("servers[%d].sftp.auth.private_key_path is required when auth.type=private_key", i)
		}
	default:
		return fmt.Errorf("servers[%d].sftp.auth.type must be one of: password, private_key", i)
	}
	return nil
}

func (c Config) PollInterval() time.Duration {
	return time.Duration(c.Intervals.ModlistPollSeconds) * time.Second
}
