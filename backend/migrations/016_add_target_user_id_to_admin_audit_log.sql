-- +goose Up
ALTER TABLE admin_audit_log
ADD COLUMN IF NOT EXISTS target_user_id BIGINT;

-- +goose Down
ALTER TABLE admin_audit_log
DROP COLUMN IF EXISTS target_user_id;
