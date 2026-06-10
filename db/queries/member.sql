-- name: GetProjectMember :one
SELECT * FROM project_members WHERE project_id = $1 AND user_id = $2 LIMIT 1;

-- name: ListProjectMembers :many
SELECT * FROM project_members WHERE project_id = $1;

-- name: AddProjectMember :one
INSERT INTO project_members (id, project_id, user_id, role, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: RemoveProjectMember :exec
DELETE FROM project_members WHERE id = $1;

-- name: UpdateProjectMemberRole :exec
UPDATE project_members SET role = $2 WHERE id = $1;
