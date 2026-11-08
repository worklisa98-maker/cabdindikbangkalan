-- +goose Up
CREATE TABLE IF NOT EXISTS report_jobs (
  id UUID PRIMARY KEY,
  job_type TEXT NOT NULL,
  cache_key TEXT NOT NULL UNIQUE,
  requested_by_user_id BIGINT NOT NULL,
  requested_by_role TEXT NOT NULL,
  requested_school_id BIGINT NULL,
  params JSONB NOT NULL,
  status TEXT NOT NULL,
  progress INT NOT NULL DEFAULT 0,
  result_path TEXT NULL,
  error_message TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_report_jobs_status_created_at
  ON report_jobs(status, created_at);

CREATE INDEX IF NOT EXISTS idx_report_jobs_requested_by_user
  ON report_jobs(requested_by_user_id, status);

CREATE INDEX IF NOT EXISTS idx_report_jobs_cache_key
  ON report_jobs(cache_key);

-- +goose Down
DROP INDEX IF EXISTS idx_report_jobs_cache_key;
DROP INDEX IF EXISTS idx_report_jobs_requested_by_user;
DROP INDEX IF EXISTS idx_report_jobs_status_created_at;
DROP TABLE IF EXISTS report_jobs;
