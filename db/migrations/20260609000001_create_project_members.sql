-- migrate:up
CREATE TABLE project_members (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('editor', 'admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  TEXT NOT NULL,
    UNIQUE (project_id, user_id)
);
CREATE INDEX idx_project_members_project_id ON project_members(project_id);

CREATE TABLE project_group_roles (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    oidc_role   TEXT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('editor', 'admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by  TEXT NOT NULL,
    UNIQUE (project_id, oidc_role)
);
CREATE INDEX idx_project_group_roles_project_id ON project_group_roles(project_id);
