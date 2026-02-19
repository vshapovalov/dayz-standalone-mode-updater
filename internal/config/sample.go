package config

func Sample() Config {
	return Config{
		Version: 1,
		Paths: PathsConfig{
			LocalModsRoot:               "./mods",
			LocalCacheRoot:              "./cache",
			SteamcmdPath:                "/usr/games/steamcmd",
			SteamcmdWorkshopContentRoot: "/home/steam/.steam/steam/steamapps/workshop/content",
		},
		Steam: SteamConfig{
			Login:          "steam_user",
			Password:       "steam_password",
			WorkshopGameID: defaultWorkshopGameID,
			WebAPIKey:      "",
		},
		Intervals: IntervalsConfig{
			ModlistPollSeconds:  60,
			WorkshopPollSeconds: 300,
			RconTickSeconds:     5,
			StateFlushSeconds:   15,
		},
		Shutdown: ShutdownConfig{
			GracePeriodSeconds:   300,
			AnnounceEverySeconds: 60,
			MessageTemplate:      "Server restart in {minutes} minute(s)",
			FinalMessage:         "Server restarting now",
		},
		Concurrency: ConcurrencyConfig{
			ModlistPollParallelism:           4,
			SFTPSyncParallelismServers:       2,
			SFTPSyncParallelismModsPerServer: 2,
			WorkshopParallelism:              4,
			WorkshopBatchSize:                50,
		},
		Servers: []ServerConfig{{
			ID:   "server-1",
			Name: "Primary",
			SFTP: ServerSFTPConfig{
				Host: "127.0.0.1",
				Port: 22,
				User: "sftp_user",
				Auth: SFTPAuthConfig{
					Type:     "password",
					Password: "sftp_password",
				},
				RemoteModlistPath: "/dayz/modlist.txt",
				RemoteModsRoot:    "/dayz/mods",
			},
			RCON: ServerRCONConfig{
				Host:     "127.0.0.1",
				Port:     2306,
				Password: "rcon_password",
			},
		}},
		StatePath: "state.json",
	}
}
