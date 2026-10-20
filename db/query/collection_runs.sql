-- name: GetPublishedRun :one
SELECT r.*
FROM publication_state p
JOIN collection_runs r ON r.id = p.published_run_id
WHERE p.singleton = true;

-- name: GetRunByID :one
SELECT *
FROM collection_runs
WHERE id = $1;

-- name: LockPublicationState :one
SELECT *
FROM publication_state
WHERE singleton = true
FOR UPDATE;

-- name: PublishRun :exec
UPDATE publication_state
SET published_run_id = $1, updated_at = now()
WHERE singleton = true;
