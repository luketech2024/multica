-- name: CountExternalTicketDuplicates :one
SELECT count(*) FROM issue
WHERE workspace_id=$1 AND external_ticket_no=$2
  AND (sqlc.narg('exclude_id')::uuid IS NULL OR id<>sqlc.narg('exclude_id')::uuid);
