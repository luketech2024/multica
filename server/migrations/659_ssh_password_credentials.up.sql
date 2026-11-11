ALTER TABLE credentials
    DROP CONSTRAINT IF EXISTS credentials_kind_check,
    ADD CONSTRAINT credentials_kind_check CHECK (kind IN ('ssh-key', 'ssh-password', 'token', 'cloud-ak', 'weapp-ci-key', 'registry-token'));
