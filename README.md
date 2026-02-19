# DayZ Standalone Mod Updater

CLI daemon scaffold for keeping DayZ workshop mods in sync.

## Quickstart

```bash
# print sample files
go run ./cmd/dayzmods print-sample-config > config.json
go run ./cmd/dayzmods print-sample-state > state.json

# start daemon
go run ./cmd/dayzmods run --config config.json
```

The `run` command loads `config.json`, then loads `state.json` from `config.state_path`, and keeps running polling loops until `SIGINT`/`SIGTERM` is received.

## Commands

- `run --config <path>`: start daemon loop placeholders.
- `print-sample-config`: print sample `config.json` to stdout.
- `print-sample-state`: print sample empty `state.json` to stdout.

## Development

```bash
make fmt
make lint
make test
make run
```

## Notes

This repository currently contains compileable placeholders and interfaces with TODO markers in domain packages:

- `config`, `state`, `logging`, `modlist`, `workshop`, `steamcmd`, `localmods`, `syncer`, `rcon`, `orchestrator`, `util`
