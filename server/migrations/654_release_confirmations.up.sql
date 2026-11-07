ALTER TABLE workbench_release_orders ADD COLUMN IF NOT EXISTS confirmations jsonb NOT NULL DEFAULT '{}';
