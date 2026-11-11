CREATE TABLE IF NOT EXISTS project_deployment (
    workspace_id UUID NOT NULL,
    project_id UUID NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1,
    config JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS deployment_certificate (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    project_id UUID NOT NULL,
    name TEXT NOT NULL,
    domains TEXT[] NOT NULL,
    not_before TIMESTAMPTZ NOT NULL,
    not_after TIMESTAMPTZ NOT NULL,
    fingerprint TEXT NOT NULL,
    secret_enc BYTEA NOT NULL,
    wrapped_key BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
