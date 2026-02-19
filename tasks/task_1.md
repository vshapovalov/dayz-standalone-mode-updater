MANDATORY TASK LOGGING (do this before any other work):\n1) Ensure there is a directory named `tasks/` at repo root (create if missing).\n2) Determine the next task number by listing existing files `tasks/task_*.md`:\n   - If none exist, start with task number = 1.\n   - Otherwise, set task number = (max existing number + 1).\n3) Create a new markdown file: `tasks/task_{N}.md`.\n4) Save the FULL EXACT PROMPT TEXT (verbatim) into that file, including this mandatory logging block and everything below it.\n5) Only after the file is created, proceed with implementing the task.\n\nYou are a senior Go backend engineer. Build a production-ready long-running CLI service (daemon).
Hard requirements:
- Follow the provided spec exactly. If something is unclear, make ONE reasonable assumption and list it at the top.
- Output format: 1) file tree 2) code grouped by file path 3) commands to run tests and start the service.
- Implement structured JSON logging, config via config.json, persistent state via state.json, graceful shutdown via context cancellation, and robust error handling.
- Avoid leaking secrets in logs (mask steam password, rcon password, sftp password).
- Keep dependencies minimal and documented in README.
- Add unit tests for parsing, slugify, planning logic, countdown calculation, and state persistence (atomic writes).
- Add at least one integration test (docker-compose ok) for SFTP sync planning/execution (using an SFTP container).
- Never output pseudo-code; write real code.
Tech choices:
- Go 1.22+.
- Use github.com/pkg/sftp + golang.org/x/crypto/ssh for SFTP.
- Use github.com/multiplay/go-battleye for BattlEye RCON client.
- Use net/http for Steam Web API calls.
