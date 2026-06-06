-- migrate:up
CREATE TABLE deployment_histories (
  id             TEXT PRIMARY KEY,
  application_id TEXT NOT NULL,
  owner_user_id  TEXT NOT NULL,
  repo_owner     TEXT NOT NULL,
  repo_name      TEXT NOT NULL,
  commit_sha     TEXT NOT NULL,
  commit_message TEXT NOT NULL DEFAULT '',
  commit_author  TEXT NOT NULL DEFAULT '',
  commit_at      TIMESTAMPTZ NOT NULL,
  pr_number      INT,
  status         TEXT NOT NULL,
  started_at     TIMESTAMPTZ NOT NULL,
  finished_at    TIMESTAMPTZ,
  CONSTRAINT status_check CHECK (status IN ('QUEUED','IN_PROGRESS','SUCCESS','FAILED')),
  CONSTRAINT finished_after_started CHECK (finished_at IS NULL OR finished_at >= started_at),
  CONSTRAINT pr_number_positive CHECK (pr_number IS NULL OR pr_number > 0)
);

CREATE INDEX ON deployment_histories (application_id, started_at DESC);
CREATE INDEX ON deployment_histories (owner_user_id, started_at DESC);
CREATE INDEX ON deployment_histories (repo_owner, repo_name, commit_sha);

-- migrate:down
DROP TABLE deployment_histories;
