# DayZ Standalone Mod Updater

Daemon for polling DayZ server modlists, checking Steam Workshop updates, downloading mods with SteamCMD, and syncing to servers over SFTP.

## Quickstart

```bash
go run ./cmd/dayzmods print-sample-config > config.json
go run ./cmd/dayzmods print-sample-state > state.json
go run ./cmd/dayzmods run --config config.json
```

## Config reference

### Top-level
- `version` (int): config schema version.
- `state_path` (string): path to `state.json`.
- `paths` (object): local and SteamCMD paths.
- `steam` (object): Steam credentials + HTTP/SteamCMD retry knobs.
- `intervals` (object): polling cadence.
- `shutdown` (object): restart announcement policy.
- `concurrency` (object): worker parallelism.
- `servers` ([]object): server definitions.

### `paths`
- `local_mods_root`
- `local_cache_root`
- `steamcmd_path`
- `steamcmd_workshop_content_root`

### `steam`
- `login`
- `password` (secret; masked in logs)
- `workshop_game_id`
- `web_api_key`
- `workshop_http_timeout_seconds`
- `workshop_max_retries`
- `workshop_backoff_millis` (linear backoff multiplier)
- `steamcmd_retries_per_mod`
- `steamcmd_backoff_millis` (linear backoff multiplier)

### `intervals`
- `modlist_poll_seconds`
- `workshop_poll_seconds`
- `rcon_tick_seconds`
- `state_flush_seconds`

### `shutdown`
- `grace_period_seconds`
- `announce_every_seconds`
- `message_template`
- `final_message`

### `concurrency`
- `modlist_poll_parallelism`
- `sftp_sync_parallelism_servers`
- `sftp_sync_parallelism_mods_per_server`
- `workshop_parallelism`
- `workshop_batch_size`

### `servers[]`
- `id`
- `name`
- `sftp.host`
- `sftp.port`
- `sftp.user`
- `sftp.auth.type` (`password` or `private_key`)
- `sftp.auth.password` (secret when using password auth)
- `sftp.auth.private_key_path` (for private key auth)
- `sftp.auth.passphrase` (secret)
- `sftp.remote_modlist_path`
- `sftp.remote_mods_root`
- `sftp.connect_timeout_seconds`
- `sftp.operation_timeout_seconds`
- `sftp.max_retries`
- `sftp.retry_backoff_millis`
- `rcon.host`
- `rcon.port`
- `rcon.password` (secret; masked in logs)

## Production hardening included
- SFTP connect/operation timeouts and retry/backoff.
- Workshop HTTP timeout plus retries with backoff on `429`/`5xx`.
- SteamCMD retries per mod with backoff.
- Structured SFTP sync logs include `server_id`, `mod_id`, `stage`, `duration_ms`, and action counts (`mkdir_count`, `upload_count`, `delete_count`).
- Secret masking for password/passphrase/token/api-key style fields.

## Examples
- `examples/config.json`
- `examples/state.json`
- `examples/dayzmods.service` (systemd)

## Container
Build and run with Docker:
```bash
docker build -t dayzmods-updater .
docker run --rm -v "$PWD/config.json:/app/config.json:ro" -v "$PWD/state.json:/app/state.json" dayzmods-updater
```
