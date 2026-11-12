CREATE TABLE onespec_workspace_config (
    workspace_id uuid NOT NULL,
    default_runtime_id uuid,
    revision bigint NOT NULL DEFAULT 1
);

CREATE TABLE onespec_project_config (
    workspace_id uuid NOT NULL,
    project_id uuid NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    runtime_id uuid,
    overview_repository_id uuid,
    revision bigint NOT NULL DEFAULT 1
);

CREATE TABLE onespec_flow (
    workspace_id uuid NOT NULL,
    issue_id uuid NOT NULL,
    state jsonb NOT NULL,
    revision bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now()
);
