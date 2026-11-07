CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_deploy_targets_runtime_check ON deploy_targets(runtime_id) WHERE channel='runtime' AND last_check->>'status' IN ('queued','running');
