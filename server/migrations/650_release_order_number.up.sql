CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_release_order_number ON workbench_release_orders (workspace_id, number);
