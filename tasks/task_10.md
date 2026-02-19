MANDATORY TASK LOGGING (do this before any other work):\n1) Ensure there is a directory named `tasks/` at repo root (create if missing).\n2) Determine the next task number by listing existing files `tasks/task_*.md`:\n   - If none exist, start with task number = 1.\n   - Otherwise, set task number = (max existing number + 1).\n3) Create a new markdown file: `tasks/task_{N}.md`.\n4) Save the FULL EXACT PROMPT TEXT (verbatim) into that file, including this mandatory logging block and everything below it.\n5) Only after the file is created, proceed with implementing the task.\n


Harden the service for production use.

Add:
- SFTP connect/op timeouts + retries (configurable)
- Workshop HTTP timeouts + retries + backoff on 429/5xx
- SteamCMD retries_per_mod and backoff
- Structured logs: include server_id, mod_id, stage, durations, counts (mkdir/upload/delete)
- Ensure secrets are masked (steam password, rcon password)
- Document all config fields in README + provide examples config/state under ./examples/
- Add systemd unit example (optional) and Dockerfile (optional) for running the daemon.

Return code changes by file path.
