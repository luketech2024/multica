CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_release_order_request ON workbench_release_orders (workspace_id, request_id);
