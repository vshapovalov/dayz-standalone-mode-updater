# Technical Context: DayZ Standalone Mod Updater

## 1) Overview

### What this service does

This repository implements a **daemon-style DayZ mod updater**. It continuously:

1. Polls each configured server's remote `modlist.html` over SFTP.
2. Parses workshop mod IDs from that file.
3. Polls Steam Workshop metadata (`time_updated`) for those IDs.
4. Downloads changed mods via SteamCMD to local storage.
5. Materializes local mod folders under a stable slug path.
6. Syncs changed local mod folders to each server over SFTP.
7. Runs an RCON pre-restart countdown and then executes `#shutdown`.

The process is stateful and crash-resilient via `state.json`.

### Non-goals

- No HTTP API, no admin web UI, no gRPC server.
- No inbound control plane; control is config + local process lifecycle.
- Not a generic artifact sync engine; behavior is DayZ modlist + Workshop specific.
- Not a package manager replacement for SteamCMD.

---

## 2) High-level architecture

### Module map and responsibilities

- `internal/config`
  - Loads `config.json`, applies defaults, validates required contract.
- `internal/state`
  - Reads/writes persistent `state.json` atomically.
  - Maintains server stage, sync progress, countdown fields, errors.
- `internal/modlist`
  - Pulls remote `modlist.html` via SFTP and caches it.
  - Parses `DisplayName`, `Link`, `workshop_id`, computes modset hash.
- `internal/workshop`
  - Calls Steam Web API `GetPublishedFileDetails` in parallel batches.
  - Updates `workshop_updated_at` and determines local update set.
- `internal/steamcmd`
  - Runs SteamCMD download commands with retries.
  - Detects success from stdout/stderr log patterns.
  - Mirrors workshop content to local mod folder with atomic swap semantics.
- `internal/sftpsync`
  - Computes local/remote tree snapshots and file-level diff.
  - Executes ordered sync operations with retry-controlled connection.
  - Writes partial progress (`synced_mods`) per mod.
- `internal/rcon`
  - RCON tick loop for countdown announcements + final `#shutdown`.
  - Handles unavailable RCON by leaving countdown state active for retry.
- `internal/orchestrator`
  - Main scheduler driven by interval tickers.
  - Serializes state updates through store operations and phase gates.

### Data flow (text diagram)

`Remote server modlist.html` -> `modlist parser` -> `sorted workshop IDs + modset hash` -> `state update`

`state candidate IDs` -> `workshop metadata poller` -> `mods_to_update_locally`

`mods_to_update_locally` -> `SteamCMD runner` -> `local_mods_root/<folder_slug>` -> `SFTP sync engine` -> `remote_mods_root/<folder_slug>`

`after sync complete` -> `server StageCountdown` -> `RCON minute announcements` -> `final message` -> `#shutdown` -> `StageIdle`

RCON loop runs independently every `intervals.rcon_tick_seconds` and consumes persisted countdown fields.

---

## 3) Configuration contract (`config.json`)

## Full schema (with defaults)

### Top-level

- `version` (int, default: `1`)
- `poll_interval_seconds` (int, optional backward-compat alias for `intervals.modlist_poll_seconds`)
- `paths` (object, required)
- `steam` (object, required)
- `intervals` (object, required with defaults applied)
- `shutdown` (object, required)
- `concurrency` (object, required)
- `servers` (array, required, at least one)
- `state_path` (string, default: `state.json`)
- `mods` (array, legacy backward-compat, optional)
- `rcon` (object, legacy backward-compat, optional)
- `sftp` (object, legacy backward-compat, optional)

### `paths`

- `local_mods_root` (string, required)
- `local_cache_root` (string, required)
- `steamcmd_path` (string, required)
- `steamcmd_workshop_content_root` (string, required)

### `steam`

- `api_key` (string, optional alias)
- `login` (string, required)
- `password` (string, required, secret)
- `workshop_game_id` (int, default: `221100`)
- `web_api_key` (string, optional)
- `workshop_http_timeout_seconds` (int, default: `20`)
- `workshop_max_retries` (int, default: `3`)
- `workshop_backoff_millis` (int, default: `500`)
- `steamcmd_retries_per_mod` (int, default: `3`)
- `steamcmd_backoff_millis` (int, default: `1000`)

### `intervals`

- `modlist_poll_seconds` (int, default: `60`)
- `workshop_poll_seconds` (int, default: `300`)
- `rcon_tick_seconds` (int, default: `5`)
- `state_flush_seconds` (int, default: `15`)

### `shutdown`

- `grace_period_seconds` (int, required)
- `announce_every_seconds` (int, required)
- `message_template` (string, required, contains `{minutes}` placeholder)
- `final_message` (string, required)

### `concurrency` (all must be `> 0`)

- `modlist_poll_parallelism`
- `sftp_sync_parallelism_servers`
- `sftp_sync_parallelism_mods_per_server`
- `workshop_parallelism`
- `workshop_batch_size`

### `servers[]`

- `id` (string, unique, required)
- `name` (string, required)
- `sftp` (object, required)
  - `host` (string)
  - `port` (int)
  - `user` (string)
  - `auth` (object)
    - `type`: `password` or `private_key`
    - `password` (required when `type=password`)
    - `private_key_path` (required when `type=private_key`)
    - `passphrase` (optional for encrypted private keys)
  - `remote_modlist_path` (string, default `/modlist.html` if empty)
  - `remote_mods_root` (string)
  - `connect_timeout_seconds` (int, default `10`)
  - `operation_timeout_seconds` (int, default `30`)
  - `max_retries` (int, default `3`)
  - `retry_backoff_millis` (int, default `500`)
- `rcon` (object, required)
  - `host` (string)
  - `port` (int)
  - `password` (string, secret)

### Minimal example (from sample)

```json
{
  "version": 1,
  "state_path": "state.json",
  "paths": {
    "local_mods_root": "./mods",
    "local_cache_root": "./cache",
    "steamcmd_path": "/usr/games/steamcmd",
    "steamcmd_workshop_content_root": "/home/steam/.steam/steam/steamapps/workshop/content"
  },
  "steam": {
    "login": "steam_user",
    "password": "steam_password",
    "workshop_game_id": 221100,
    "web_api_key": "",
    "workshop_http_timeout_seconds": 20,
    "workshop_max_retries": 3,
    "workshop_backoff_millis": 500,
    "steamcmd_retries_per_mod": 3,
    "steamcmd_backoff_millis": 1000
  },
  "intervals": {
    "modlist_poll_seconds": 60,
    "workshop_poll_seconds": 300,
    "rcon_tick_seconds": 5,
    "state_flush_seconds": 15
  }
}
```

### Security notes

- Secrets are stored in plaintext JSON (`steam.password`, SFTP auth, RCON password).
- SteamCMD log output is password-redacted for Steam password only, but config file remains sensitive.
- Restrict file permissions for `config.json` (recommended `0600`) and private key files.
- Avoid committing production credentials; use deployment secret management where possible.

---

## 4) State contract (`state.json`)

### Top-level schema

- `version` (int; normalized to `1` if missing)
- `updated_at` (RFC3339 timestamp; set on every save)
- `mods` (map: `workshop_id -> ModState`)
- `servers` (map: `server_id -> ServerState`)

### `ModState`

- `display_name` (string)
- `folder_slug` (string)
- `workshop_updated_at` (timestamp)
- `last_workshop_check_at` (timestamp)
- `local_updated_at` (timestamp)
- `last_synced_at` (timestamp, currently optional legacy field)
- `last_title` (string, last Workshop title)

### `ServerState`

- `last_mod_ids` ([]string): latest parsed mod ID set for this server.
- `last_modset_hash` (string): SHA-256 hash of sorted `last_mod_ids`.
- `needs_mod_update` (bool): server has pending sync work.
- `needs_shutdown` (bool): server should run restart sequence.
- `stage` (enum string): lifecycle marker.
- `synced_mods` (map `mod_id -> timestamp`): per-mod remote sync watermark.
- `shutdown_deadline_at` (timestamp pointer): countdown end.
- `next_announce_at` (timestamp pointer): next RCON announce timestamp.
- `last_error`, `last_error_stage`, `last_error_at`: troubleshooting context.
- `last_success_sync_at`: last successful sync completion time.
- `shutdown_sent_at`: timestamp when `#shutdown` succeeded.

### Crash recovery behavior

- State writes are atomic: encode to temp file then `rename`.
- On process restart, state is loaded from disk, normalized (missing maps and empty stage repaired).
- Because countdown fields are persisted, RCON countdown resumes naturally from saved deadline.
- Partial sync progress survives restart via `synced_mods` map; already-synced mods are skipped unless local timestamp changed.

### Stage enum and transitions

- `idle`
  - steady state, no pending actions.
- `planning`
  - set when modlist hash changed or when SteamCMD updates a mod used by server.
- `local_updating`
  - declared enum value (currently not actively set by orchestrator path).
- `syncing`
  - entered while SFTP sync executes.
- `countdown`
  - entered when sync finished (or nothing left to sync) and shutdown countdown started.
- `shutting_down`
  - declared enum value (currently not actively set before idle reset).
- `error`
  - entered on sync/connect/mod validation failures; retry needed.

Typical transition path: `idle/planning -> syncing -> countdown -> idle`, with `-> error` on failures.

---

## 5) Parsing specs

### `modlist.html` parsing

The parser uses regex-based extraction from HTML rows:

- Row selector: `<tr ... data-type="ModContainer" ...>...</tr>`
- Display name selector: `<td ... data-type="DisplayName" ...>...</td>`
- Link selector: `<a ... data-type="Link" href="...">`

For each row:

1. Extract `DisplayName` text (strip nested tags, trim spaces).
2. Extract `Link` href.
3. Parse workshop ID from query param `id=...`.
4. Keep only numeric IDs (`^[0-9]+$`); invalid rows are skipped with warning.

### `folder_slug` rules

`SlugifyFolder(displayName, workshopID)`:

1. Lowercase + trim.
2. Collapse whitespace into `-`.
3. Remove chars not in `[a-z0-9-]`.
4. Collapse repeated dashes.
5. Trim edge dashes.
6. If empty, fallback: `mod-<workshopID>`.

### Modset hashing

`modset_hash = SHA256(strings.Join(sorted(workshop_ids), ","))` encoded as lowercase hex.

---

## 6) Workshop polling

### Metadata source and request model

- Endpoint: `ISteamRemoteStorage/GetPublishedFileDetails/v1/`.
- Request method: POST form-urlencoded.
- Payload:
  - optional `key`
  - `itemcount`
  - `publishedfileids[0..n]`
- Response fields used:
  - `publishedfileid`
  - `title`
  - `time_updated`

### Batching and concurrency

- IDs are batched by `concurrency.workshop_batch_size`.
- Batches run in parallel up to `concurrency.workshop_parallelism`.
- Candidate IDs come from union of all servers' `last_mod_ids`.

### Poll cadence and rate limiting behavior

- A mod is skipped if `now - last_workshop_check_at < intervals.workshop_poll_seconds`.
- HTTP retries happen on network errors, `429`, and `>=500` responses.
- Retry backoff is linear: `workshop_backoff_millis * attempt`.

### `mods_to_update_locally` decision

A mod is marked for local update if:

- `local_updated_at` is zero, **or**
- `workshop_updated_at > local_updated_at`.

Result list is sorted ascending by mod ID.

---

## 7) SteamCMD local update

### Command sequence

For each mod ID (sequentially):

1. Build command:
   - `+login <steam.login> <steam.password>`
   - `+workshop_download_item <workshop_game_id> <mod_id> validate`
   - `+quit`
2. Run SteamCMD binary at `paths.steamcmd_path`.
3. Capture combined stdout/stderr.
4. Redact password from captured output and write to `cache/logs/steamcmd.log`.
5. Parse success marker regex:
   - `Success. Downloaded item <id>`
6. Verify downloaded directory exists at:
   - `<steamcmd_workshop_content_root>/<workshop_game_id>/<mod_id>`

Retries per mod follow `steam.steamcmd_retries_per_mod` with linear backoff `steamcmd_backoff_millis * attempt`.

### Success detection and failure modes

Success requires both:

- Success marker present for the specific mod ID.
- Downloaded content directory exists.

Failure modes include:

- SteamCMD process error/exit failure.
- Missing success marker.
- Missing workshop directory.
- Local mirror copy/swap failure.

### Local materialization + atomic swap

Source:
- `steamcmd_workshop_content_root/<app_id>/<workshop_id>`

Target:
- `local_mods_root/<folder_slug>`

Algorithm:

1. Copy source tree into temporary staging directory under `local_cache_root/staging/`.
2. If target exists, rename target to backup path.
3. Rename staging dir to target (atomic swap on same filesystem).
4. Remove backup on success; attempt rollback if swap rename fails.

After success:
- `mod.local_updated_at = workshop_updated_at` (or current time if workshop time unknown).
- Any server using that mod is marked `needs_mod_update=true`, `stage=planning`.

---

## 8) SFTP sync stage

### Tree snapshot model

Per mod sync computes:

- Local tree via filesystem walk.
- Remote tree via SFTP walker.

Each entry tracks: path, `is_dir`, file size, truncated UTC mtime (seconds).

### Diff rules

- Upload file if missing remotely.
- Upload file if same path exists but `size` or `mtime` differs.
- Create directory if missing remotely.
- Handle type conflicts (file vs dir) by deleting conflicting remote entry first.
- Delete remote entries absent in local tree (inside that mod folder only).

### Operation ordering

1. Delete type conflicts (deepest-first).
2. `mkdir -p` directories (shallow-first).
3. Upload changed files using temp path + rename + setmtime.
4. Delete extra remote files.
5. Delete extra remote directories (deepest-first).

### Atomic upload detail

For each file:
- upload to `<remote>.tmp-<nanos>`
- rename temp to final
- set atime/mtime on remote file to local mtime (seconds precision)

### Partial progress rules

- `server.synced_mods[mod_id] = mod.local_updated_at` only after that mod sync completes successfully.
- If one mod fails and others succeeded, successes remain recorded in `synced_mods`.
- On next run, only unsynced/stale mods are retried.

### Deletion scope safety

- The engine syncs **only** `remote_mods_root/<folder_slug>` for mods in current `last_mod_ids` selected for sync.
- It does **not** sweep/delete arbitrary sibling folders in `remote_mods_root` for mods no longer listed.
- Therefore mods not currently in the modlist are not globally pruned.

---

## 9) RCON countdown + shutdown

### Behavior contract

When a server reaches countdown stage:

1. Set `shutdown_deadline_at = now + grace_period_seconds`.
2. Set `next_announce_at = now`.
3. On each RCON tick while `now < deadline`:
   - if `now >= next_announce_at`, send `say -1 <message_template with {minutes}>`
   - advance `next_announce_at += announce_every_seconds`
4. Once `now >= deadline`:
   - send `say -1 <final_message>`
   - send `#shutdown`
   - on successful shutdown command: clear `needs_shutdown`, set `stage=idle`, set `shutdown_sent_at`.

### Time rounding rule

Remaining minutes are computed as:
- `ceil((deadline - now) / 60 seconds)`

### RCON unavailable behavior

- If RCON connect fails, countdown state is preserved and retried on next tick.
- If announce/final/shutdown command fails, errors are logged but state remains, so retries continue.
- Since countdown timestamps are persisted in `state.json`, behavior resumes after process restart.

---

## 10) Orchestrator scheduling

### Tickers and cadence

On startup, orchestrator creates 4 periodic loops:

- modlist poll ticker (`intervals.modlist_poll_seconds`)
- workshop poll ticker (`intervals.workshop_poll_seconds`)
- RCON ticker (`intervals.rcon_tick_seconds`)
- state flush ticker (`intervals.state_flush_seconds`)

### Concurrency limits

- Modlist polling parallelism: `concurrency.modlist_poll_parallelism`.
- Workshop metadata parallelism: `concurrency.workshop_parallelism` (with batch size).
- SFTP sync parallelism:
  - servers: `concurrency.sftp_sync_parallelism_servers`
  - mods per server: `concurrency.sftp_sync_parallelism_mods_per_server`
- SteamCMD updates are serialized by a mutex and executed one mod at a time.

### Phase ordering constraints

Normal update path:

1. Modlist poll updates server modset and possibly marks planning.
2. Workshop poll computes mods needing local update.
3. SteamCMD local updates run.
4. SFTP sync phase runs.
5. RCON tick loop handles countdown/shutdown independently.

State mutations use `state.Store.Update(...)`, preserving atomic read-modify-write semantics.

---

## 11) Operational notes

### Logging behavior

- Main app logs to stdout with simple `INFO/ERROR` prefix and `fields=...` map payload.
- SFTP engine additionally emits structured slog records for connect and per-mod sync metrics.
- SteamCMD writes sanitized command output to:
  - `<local_cache_root>/logs/steamcmd.log`

Common useful fields:
- `server_id`, `mod_id`, `stage`, `duration_ms`, `mkdir_count`, `upload_count`, `delete_count`.

### Troubleshooting checklist

1. Validate config loads:
   - run `go run ./cmd/dayzmods run --config config.json` and check immediate validation errors.
2. Verify SFTP connectivity/auth to each server.
3. Confirm remote `modlist.html` path and content format (`ModContainer`, `DisplayName`, `Link`).
4. Check Workshop API key, timeout, retry behavior for 429/5xx/network.
5. Inspect `cache/logs/steamcmd.log` for download failures.
6. Confirm Steam workshop content root path exists and matches app ID directory.
7. Confirm local mod folder slugs match expected remote folder names.
8. Review `state.json` per server:
   - `needs_mod_update`, `stage`, `synced_mods`, `last_error*`, countdown fields.
9. If stuck in `error`, fix root cause and allow next poll/sync cycle to retry.

### Local run

```bash
go run ./cmd/dayzmods print-sample-config > config.json
go run ./cmd/dayzmods print-sample-state > state.json
go run ./cmd/dayzmods run --config config.json
```

### Tests and integration

Unit tests:

```bash
go test ./...
```

Integration tests exist under `tests/integration` and include a docker-compose fixture (`atmoz/sftp` at port `2222`). Typical workflow:

```bash
docker compose -f tests/integration/docker-compose.yml up -d
go test ./tests/integration/...
docker compose -f tests/integration/docker-compose.yml down -v
```

Prereqs:
- Go toolchain
- SteamCMD binary available at configured path
- Network access to Steam Web API and target SFTP/RCON endpoints
- Docker + docker compose for integration harness

---

## 12) Roadmap / known limitations

- HTML parsing uses regex (fragile to substantial modlist markup changes).
- Remote diff identity is size+mtime only (no content hash).
- SFTP engine currently supports password auth only; config supports key auth but sync dial path does not yet.
- Stage enum includes values not fully exercised (`local_updating`, `shutting_down`).
- SteamCMD log path is single rolling file (no rotation/history).
- Backpressure and global job queueing are basic; large fleets may need smarter scheduling.
- No built-in metrics endpoint / tracing.
- Host key verification for SSH is disabled (`InsecureIgnoreHostKey`) and should be hardened for production.
