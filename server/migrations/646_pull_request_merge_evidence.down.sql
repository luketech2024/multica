ALTER TABLE github_pull_request DROP COLUMN IF EXISTS base_branch, DROP COLUMN IF EXISTS merge_commit_sha;
ALTER TABLE vcs_pull_request DROP COLUMN IF EXISTS base_branch, DROP COLUMN IF EXISTS merge_commit_sha;
