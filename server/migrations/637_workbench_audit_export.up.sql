CREATE TABLE IF NOT EXISTS workbench_audit_export (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    requester_id UUID NOT NULL,
    filename TEXT NOT NULL,
    content BYTEA NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('configuration', 'security', 'all')),
    range_start TIMESTAMPTZ NOT NULL,
    range_end TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
