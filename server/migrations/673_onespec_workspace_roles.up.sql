CREATE TABLE onespec_workspace_role (
    workspace_id uuid NOT NULL,
    role_key text NOT NULL,
    enabled boolean NOT NULL DEFAULT true,
    configured_by uuid NOT NULL,
    revision bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
