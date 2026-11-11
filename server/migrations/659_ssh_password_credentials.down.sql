-- Refuse rollback while password credentials exist rather than changing their meaning.
ALTER TABLE credentials
    DROP CONSTRAINT IF EXISTS credentials_kind_check,
    ADD CONSTRAINT credentials_kind_check CHECK (kind IN ('ssh-key', 'token', 'cloud-ak', 'weapp-ci-key', 'registry-token'));
