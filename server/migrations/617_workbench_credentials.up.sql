CREATE TABLE IF NOT EXISTS credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    project_id UUID,
    label TEXT NOT NULL CHECK (char_length(label) BETWEEN 1 AND 120),
    kind TEXT NOT NULL CHECK (kind IN ('ssh-key', 'token', 'cloud-ak', 'weapp-ci-key', 'registry-token')),
    usage TEXT[] NOT NULL,
    secret_enc BYTEA NOT NULL,
    wrapped_key BYTEA NOT NULL,
    last_four TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    rotated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
