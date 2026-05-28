-- migrate:up
CREATE TABLE users (
  id           TEXT PRIMARY KEY,
  display_id   TEXT NOT NULL,
  display_name TEXT NOT NULL,
  roles        TEXT[] NOT NULL DEFAULT '{}',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- migrate:down
DROP TABLE users;
