ALTER TABLE issue ADD COLUMN IF NOT EXISTS external_ticket_no TEXT NOT NULL DEFAULT ''
    CHECK (external_ticket_no ~ '^[A-Za-z0-9-]{0,32}$');
