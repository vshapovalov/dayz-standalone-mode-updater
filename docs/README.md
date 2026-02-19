# Documentation Index

This folder contains project documentation intended for onboarding, operations, and development work.

## Files

- `TECH_CONTEXT.md`
  - Comprehensive technical context for the DayZ updater daemon.
  - Read this first when starting feature work, debugging behavior, or planning changes.

## How to use these docs

1. Start with `TECH_CONTEXT.md` for architecture, contracts, and lifecycle behavior.
2. Cross-reference with repository examples:
   - `examples/config.json`
   - `examples/state.json`
3. Validate assumptions with tests (`go test ./...`) before shipping changes.
