-- +goose Up
ALTER TABLE admin_audit_log
  ADD COLUMN IF NOT EXISTS target_user_id INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS count INTEGER NULL;

CREATE INDEX IF NOT EXISTS admin_audit_log_target_user_id_idx
  ON admin_audit_log (target_user_id);

-- +goose Down
DROP INDEX IF EXISTS admin_audit_log_target_user_id_idx;

ALTER TABLE admin_audit_log
  DROP COLUMN IF EXISTS target_user_id,
  DROP COLUMN IF EXISTS count;
