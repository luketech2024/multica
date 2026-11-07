CREATE TABLE IF NOT EXISTS workbench_business_templates (
    id UUID NOT NULL,
    workspace_id UUID NOT NULL,
    content JSONB NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
