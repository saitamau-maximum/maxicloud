-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpsertUser :one
INSERT INTO users (id, display_id, display_name, roles)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE
  SET display_id   = EXCLUDED.display_id,
      display_name = EXCLUDED.display_name,
      roles        = EXCLUDED.roles
RETURNING *;
