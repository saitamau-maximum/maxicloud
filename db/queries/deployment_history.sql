-- name: CreateDeploymentHistory :one
INSERT INTO deployment_histories (
  id, application_id, owner_user_id,
  repo_owner, repo_name,
  commit_sha, commit_message, commit_author, commit_at,
  pr_number, status, started_at
) VALUES (
  $1, $2, $3,
  $4, $5,
  $6, $7, $8, $9,
  $10, $11, $12
) RETURNING id;

-- name: GetDeploymentHistory :one
SELECT * FROM deployment_histories WHERE id = $1;

-- name: UpdateDeploymentHistoryStatus :exec
UPDATE deployment_histories
SET status = $2, finished_at = $3
WHERE id = $1;

-- name: ListDeploymentHistoriesByApplication :many
SELECT * FROM deployment_histories
WHERE application_id = $1
ORDER BY started_at DESC;

-- name: DeleteDeploymentHistory :exec
DELETE FROM deployment_histories WHERE id = $1;
