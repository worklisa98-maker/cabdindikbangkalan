-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS system_time_overrides (
  override_key TEXT PRIMARY KEY,
  is_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  override_base_time TIMESTAMPTZ NULL,
  override_started_real_at TIMESTAMPTZ NULL,
  updated_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (
    (is_enabled = FALSE AND override_base_time IS NULL AND override_started_real_at IS NULL)
    OR
    (is_enabled = TRUE AND override_base_time IS NOT NULL AND override_started_real_at IS NOT NULL)
  )
);

INSERT INTO system_time_overrides (
  override_key,
  is_enabled,
  override_base_time,
  override_started_real_at
)
VALUES ('GLOBAL', FALSE, NULL, NULL)
ON CONFLICT (override_key) DO NOTHING;

CREATE TABLE IF NOT EXISTS system_time_override_audit_logs (
  id SERIAL PRIMARY KEY,
  action TEXT NOT NULL,
  actor_user_id INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  actor_role TEXT NOT NULL DEFAULT '',
  override_base_time TIMESTAMPTZ NULL,
  override_started_real_at TIMESTAMPTZ NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS system_time_override_audit_logs_created_at_idx
  ON system_time_override_audit_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS system_time_override_audit_logs_actor_user_id_idx
  ON system_time_override_audit_logs (actor_user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS system_time_override_audit_logs_actor_user_id_idx;
DROP INDEX IF EXISTS system_time_override_audit_logs_created_at_idx;
DROP TABLE IF EXISTS system_time_override_audit_logs;
DROP TABLE IF EXISTS system_time_overrides;
-- +goose StatementEnd
