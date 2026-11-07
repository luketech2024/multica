-- name: ListWorkbenchReleasePullRequests :many
WITH family AS (
    SELECT i.id FROM issue i WHERE i.workspace_id = @workspace_id
        AND (i.id = @issue_id OR i.parent_issue_id = @issue_id)
)
SELECT ipr.issue_id, pr.id AS pr_id, 'github'::text AS provider,
    ('https://github.com/' || pr.repo_owner || '/' || pr.repo_name)::text AS repo_url,
    pr.html_url, pr.state, pr.base_branch, pr.head_sha, pr.merge_commit_sha
FROM github_pull_request pr
JOIN issue_pull_request ipr ON ipr.pull_request_id = pr.id
JOIN family ON family.id = ipr.issue_id
WHERE pr.workspace_id = @workspace_id
UNION ALL
SELECT ipr.issue_id, pr.id AS pr_id, pr.provider,
    (rtrim(c.instance_url, '/') || '/' || pr.repo_owner || '/' || pr.repo_name)::text AS repo_url,
    pr.html_url, pr.state, pr.base_branch, pr.head_sha, pr.merge_commit_sha
FROM vcs_pull_request pr
JOIN vcs_connection c ON c.id = pr.connection_id AND c.workspace_id = pr.workspace_id
JOIN issue_vcs_pull_request ipr ON ipr.pull_request_id = pr.id
JOIN family ON family.id = ipr.issue_id
WHERE pr.workspace_id = @workspace_id
ORDER BY pr_id, issue_id;
