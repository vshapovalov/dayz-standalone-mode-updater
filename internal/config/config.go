package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	PollIntervalSeconds int         `json:"poll_interval_seconds"`
	StatePath           string      `json:"state_path"`
	Steam               SteamConfig `json:"steam"`
	RCON                RCONConfig  `json:"rcon"`
	SFTP                SFTPConfig  `json:"sftp"`
	Mods                []ModConfig `json:"mods"`
}

type SteamConfig struct {
	APIKey   string `json:"api_key"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type RCONConfig struct {
	Address             string `json:"address"`
	Password            string `json:"password"`
	PreRestartCountdown int    `json:"pre_restart_countdown_seconds"`
}

type SFTPConfig struct {
	Address    string `json:"address"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	RemoteRoot string `json:"remote_root"`
}

type ModConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	LocalPath string `json:"local_path"`
	RemoteDir string `json:"remote_dir"`
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
	if c.PollIntervalSeconds <= 0 {
		c.PollIntervalSeconds = 300
	}
	if c.StatePath == "" {
		c.StatePath = "state.json"
	}
}

func (c Config) Validate() error {
	if c.Steam.APIKey == "" {
		return fmt.Errorf("steam.api_key is required")
	}
	if c.SFTP.Address == "" || c.SFTP.Username == "" || c.SFTP.Password == "" {
		return fmt.Errorf("sftp.address, sftp.username and sftp.password are required")
	}
	if len(c.Mods) == 0 {
		return fmt.Errorf("at least one mod is required")
	}
	for i, m := range c.Mods {
		if m.ID == "" || m.LocalPath == "" {
			return fmt.Errorf("mods[%d].id and mods[%d].local_path are required", i, i)
		}
	}
	return nil
}

func (c Config) PollInterval() time.Duration {
	return time.Duration(c.PollIntervalSeconds) * time.Second
}
