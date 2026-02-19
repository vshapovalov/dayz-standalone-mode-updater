MANDATORY TASK LOGGING (do this before any other work):\n1) Ensure there is a directory named `tasks/` at repo root (create if missing).\n2) Determine the next task number by listing existing files `tasks/task_*.md`:\n   - If none exist, start with task number = 1.\n   - Otherwise, set task number = (max existing number + 1).\n3) Create a new markdown file: `tasks/task_{N}.md`.\n4) Save the FULL EXACT PROMPT TEXT (verbatim) into that file, including this mandatory logging block and everything below it.\n5) Only after the file is created, proceed with implementing the task.



Implement SFTP sync engine for a server.

Compute mods_to_sync_for_server:
- required = servers[server].last_mod_ids
- mods_to_sync = { id in required | mods[id].local_updated_at != servers[server].synced_mods[id] }
- If mods[id].local_updated_at is null -> server stage="error", keep needs_mod_update=true.

For each mod id:
- local_mod_path = local_mods_root/<folder_slug>
- remote_mod_path = remote_mods_root/<folder_slug>

Tree snapshot:
- Build local tree entries (dir/file, rel path, size, mtime seconds).
- Build remote tree via SFTP readdir/stat.

Comparison policy:
- file equal if size AND mtime (seconds) match.

Plan:
1) delete type conflicts
2) mkdir missing dirs (ascending depth)
3) upload missing/changed files
   - upload to temp name in same dir then rename to final name
   - after upload set remote mtime = local mtime
4) delete extra files/dirs inside this mod folder that are not in local snapshot (files first, dirs descending depth)

Partial progress:
- If mod sync succeeds: servers[server].synced_mods[id] = mods[id].local_updated_at
- If fails mid-way: do not update synced_mods for that id; keep needs_mod_update=true; record last_error with mod id and step.
- Continue other servers in parallel; within server follow config.sftp_sync_parallelism_mods_per_server.

After all mods_to_sync succeed for server:
- needs_mod_update=false
- needs_shutdown=true
- stage="countdown"
- shutdown_deadline_at = now + shutdown.grace_period_seconds
- next_announce_at = now

Add integration test:
- Use docker-compose with an SFTP container (e.g. atmoz/sftp) and a temporary local mod directory.
- Verify plan executes: uploads missing, updates changed, deletes extra, preserves mtime.
Add unit tests:
- planning logic (diff trees)
- deletion ordering
- mtime handling

Return code changes by file path + docker-compose for tests.
