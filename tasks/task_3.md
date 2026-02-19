MANDATORY TASK LOGGING (do this before any other work):\n1) Ensure there is a directory named `tasks/` at repo root (create if missing).\n2) Determine the next task number by listing existing files `tasks/task_*.md`:\n   - If none exist, start with task number = 1.\n   - Otherwise, set task number = (max existing number + 1).\n3) Create a new markdown file: `tasks/task_{N}.md`.\n4) Save the FULL EXACT PROMPT TEXT (verbatim) into that file, including this mandatory logging block and everything below it.\n5) Only after the file is created, proceed with implementing the task.\n\n\nImplement config and state models + validators.

CONFIG (config.json) must include:
- version (int)
- paths: local_mods_root, local_cache_root, steamcmd_path
- steam: login, password, workshop_game_id (default 221100), optional web_api_key (string or empty)
- intervals: modlist_poll_seconds, workshop_poll_seconds, rcon_tick_seconds, state_flush_seconds
- shutdown: grace_period_seconds, announce_every_seconds, message_template (with {minutes}), final_message
- concurrency: modlist_poll_parallelism, sftp_sync_parallelism_servers, sftp_sync_parallelism_mods_per_server, workshop_parallelism, workshop_batch_size
- servers[]: id, name, sftp{host,port,user,auth{type,password|private_key_path|passphrase}, remote_modlist_path, remote_mods_root}, rcon{host,port,password}

STATE (state.json) must include:
- version, updated_at
- mods map: workshop_id -> {display_name, folder_slug, workshop_updated_at, last_workshop_check_at, local_updated_at}
- servers map: server_id -> {
  last_mod_ids[], last_modset_hash,
  needs_mod_update, needs_shutdown,
  stage (enum: idle|planning|local_updating|syncing|countdown|shutting_down|error),
  synced_mods map workshop_id -> local_updated_at,
  shutdown_deadline_at, next_announce_at,
  last_error, last_error_stage, last_error_at,
  last_success_sync_at, shutdown_sent_at
}

Requirements:
- Provide defaults for missing config interval fields, with sensible values.
- Validate required fields and uniqueness of server.id.
- State storage: implement atomic write (write temp file then rename), and file lock or single-writer approach.
- Provide StateStore interface with Load(), Save(), Update(fn) that is concurrency-safe.

Return code changes by file path + a sample config.json and sample state.json in repo under ./examples/.
