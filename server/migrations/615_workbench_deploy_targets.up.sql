CREATE TABLE IF NOT EXISTS deploy_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    project_id UUID NOT NULL,
    repo_id UUID NOT NULL,
    label TEXT NOT NULL CHECK (char_length(label) BETWEEN 1 AND 120),
    env TEXT NOT NULL CHECK (env IN ('preview', 'staging', 'prod')),
    kind TEXT NOT NULL CHECK (kind IN ('docker-host', 'service-process', 'weapp-ci', 'static-site', 'npm-package')),
    channel TEXT NOT NULL CHECK (channel IN ('runtime', 'ssh', 'platform')),
    runtime_id UUID,
    credential_id UUID,
    location JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_check JSONB,
    revision BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
