-- +goose Up
ALTER TABLE prestasi_siswa
ADD COLUMN IF NOT EXISTS import_batch_id INTEGER;

ALTER TABLE siswa_perguruan_tinggi
ADD COLUMN IF NOT EXISTS import_batch_id INTEGER;

CREATE INDEX IF NOT EXISTS idx_prestasi_siswa_import_batch_id
  ON prestasi_siswa (import_batch_id);

CREATE INDEX IF NOT EXISTS idx_siswa_pt_import_batch_id
  ON siswa_perguruan_tinggi (import_batch_id);

-- +goose Down
DROP INDEX IF EXISTS idx_prestasi_siswa_import_batch_id;
DROP INDEX IF EXISTS idx_siswa_pt_import_batch_id;
ALTER TABLE prestasi_siswa DROP COLUMN IF EXISTS import_batch_id;
ALTER TABLE siswa_perguruan_tinggi DROP COLUMN IF EXISTS import_batch_id;
