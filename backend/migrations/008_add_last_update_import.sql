-- +goose Up
ALTER TABLE data_sekolah
ADD COLUMN IF NOT EXISTS last_update_prestasi TIMESTAMP NULL,
ADD COLUMN IF NOT EXISTS last_update_siswa_pt TIMESTAMP NULL;

-- +goose Down
ALTER TABLE data_sekolah
DROP COLUMN IF EXISTS last_update_prestasi,
DROP COLUMN IF EXISTS last_update_siswa_pt;
