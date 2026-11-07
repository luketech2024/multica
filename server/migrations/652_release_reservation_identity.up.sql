CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_release_reservation_identity ON workbench_release_reservations (workspace_id, repo_id, merge_commit, env);
