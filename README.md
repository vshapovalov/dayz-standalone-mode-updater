# DayZ Standalone Mod Updater

Assumption: local mod folders are already downloaded on disk (for example by SteamCMD), and this daemon only checks Steam metadata for update timestamps and then synchronizes changed folders to an SFTP target.

## Features
- Long-running CLI daemon with graceful shutdown (`SIGINT`, `SIGTERM`).
- Structured JSON logs with password masking.
- `config.json` driven runtime configuration.
- Persistent `state.json` tracking last synchronized workshop update per mod.
- Steam Workshop metadata polling over `net/http`.
- BattlEye RCON restart countdown broadcasting.
- SFTP sync using `github.com/pkg/sftp` and `golang.org/x/crypto/ssh`.

## Dependencies
- Go 1.22+
- `github.com/pkg/sftp`
- `golang.org/x/crypto/ssh`
- `github.com/multiplay/go-battleye`

## Example `config.json`
```json
{
  "poll_interval_seconds": 300,
  "state_path": "state.json",
  "steam": {
    "api_key": "STEAM_API_KEY",
    "username": "steam_user",
    "password": "steam_password"
  },
  "rcon": {
    "address": "127.0.0.1:2306",
    "password": "rcon_password",
    "pre_restart_countdown_seconds": 120
  },
  "sftp": {
    "address": "127.0.0.1:2222",
    "username": "foo",
    "password": "pass",
    "remote_root": "/upload/mods"
  },
  "mods": [
    {
      "id": "1559212036",
      "name": "CF",
      "local_path": "./mods/1559212036",
      "remote_dir": "cf"
    }
  ]
}
```

## Run
```bash
go run ./cmd/dayz-updater -config config.json
```

## Test
```bash
go test ./...
go test -tags=integration ./tests/integration -v
```
