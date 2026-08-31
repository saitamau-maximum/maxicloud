-- name: ListProjectGroupRoles :many
SELECT * FROM project_group_roles WHERE project_id = $1;

-- name: ListProjectGroupRolesByGroups :many
SELECT * FROM project_group_roles WHERE project_id = $1 AND oidc_role = ANY(@oidc_roles::text[]);

-- name: AddProjectGroupRole :one
INSERT INTO project_group_roles (id, project_id, oidc_role, role, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: RemoveProjectGroupRole :exec
DELETE FROM project_group_roles WHERE id = $1;

-- name: UpdateProjectGroupRoleRole :exec
UPDATE project_group_roles SET role = $2 WHERE id = $1;
