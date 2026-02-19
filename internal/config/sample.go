package config

func Sample() Config {
	return Config{
		PollIntervalSeconds: 300,
		StatePath:           "state.json",
		Steam: SteamConfig{
			APIKey:   "YOUR_STEAM_API_KEY",
			Username: "steam_user",
			Password: "steam_password",
		},
		RCON: RCONConfig{
			Address:             "127.0.0.1:2306",
			Password:            "rcon_password",
			PreRestartCountdown: 120,
		},
		SFTP: SFTPConfig{
			Address:    "127.0.0.1:2222",
			Username:   "sftp_user",
			Password:   "sftp_password",
			RemoteRoot: "/upload/mods",
		},
		Mods: []ModConfig{{
			ID:        "1559212036",
			Name:      "CF",
			LocalPath: "./mods/1559212036",
			RemoteDir: "cf",
		}},
	}
}
