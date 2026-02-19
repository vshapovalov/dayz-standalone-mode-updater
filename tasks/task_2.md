MANDATORY TASK LOGGING (do this before any other work):
1) Ensure there is a directory named `tasks/` at repo root (create if missing).
2) Determine the next task number by listing existing files `tasks/task_*.md`:
   - If none exist, start with task number = 1.
   - Otherwise, set task number = (max existing number + 1).
3) Create a new markdown file: `tasks/task_{N}.md`.
4) Save the FULL EXACT PROMPT TEXT (verbatim) into that file, including this mandatory logging block and everything below it.
5) Only after the file is created, proceed with implementing the task.


Project: DayZ mod updater daemon (CLI service). No HTTP endpoints.

Implement repository scaffold:
- cmd/dayzmods/main.go with Cobra (or urfave/cli) command: `run --config <path>`
- internal packages: config, state, logging, modlist, workshop, steamcmd, localmods, syncer, rcon, orchestrator, util
- Makefile with: fmt, lint (golangci-lint), test, run
- README with quickstart

CLI behavior:
- `run` starts daemon loop(s), reads config.json and state.json from paths in config, and runs until SIGINT/SIGTERM.
- Provide `print-sample-config` command to output a sample config.json to stdout (for user convenience).
- Provide `print-sample-state` command to output a sample empty state.json.

Do not implement full logic yet; only placeholders and interfaces with TODO markers and compileable code.
Return file tree + code + commands.
