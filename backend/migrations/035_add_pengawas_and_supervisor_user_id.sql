-- +goose Up
-- +goose StatementBegin
ALTER TYPE role_type ADD VALUE IF NOT EXISTS 'PENGAWAS';

ALTER TABLE data_sekolah
ADD COLUMN IF NOT EXISTS supervisor_user_id INTEGER NULL REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_data_sekolah_supervisor_user_id
ON data_sekolah(supervisor_user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_data_sekolah_supervisor_user_id;

ALTER TABLE data_sekolah
DROP COLUMN IF EXISTS supervisor_user_id;

-- NOTE:
-- PostgreSQL enum value cannot be removed safely in a reversible migration.
-- 'PENGAWAS' is intentionally kept in role_type.
-- +goose StatementEnd
