CREATE TABLE IF NOT EXISTS workbench_release_counters (
    workspace_id UUID NOT NULL,
    next_number BIGINT NOT NULL DEFAULT 1 CHECK (next_number > 0)
);
CREATE TABLE IF NOT EXISTS workbench_release_orders (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    project_id UUID NOT NULL,
    number BIGINT NOT NULL CHECK (number > 0),
    env TEXT NOT NULL CHECK (env IN ('staging','prod')),
    status TEXT NOT NULL DEFAULT 'draft',
    request_id UUID NOT NULL,
    request_hash TEXT NOT NULL,
    manifest JSONB NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS workbench_release_reservations (
    workspace_id UUID NOT NULL,
    release_id UUID NOT NULL,
    repo_id UUID NOT NULL,
    merge_commit TEXT NOT NULL,
    env TEXT NOT NULL CHECK (env IN ('staging','prod'))
);
