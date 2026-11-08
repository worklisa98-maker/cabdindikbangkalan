-- +goose Up
ALTER TABLE prestasi_siswa
ADD COLUMN jenis_lomba TEXT DEFAULT 'Siswa';

UPDATE prestasi_siswa
SET jenis_lomba = 'Siswa'
WHERE jenis_lomba IS NULL
   OR TRIM(jenis_lomba) = '';

-- +goose Down
ALTER TABLE prestasi_siswa
DROP COLUMN IF EXISTS jenis_lomba;
