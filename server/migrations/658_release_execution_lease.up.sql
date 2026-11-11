ALTER TABLE workbench_release_orders ADD COLUMN IF NOT EXISTS lease_until TIMESTAMPTZ;
