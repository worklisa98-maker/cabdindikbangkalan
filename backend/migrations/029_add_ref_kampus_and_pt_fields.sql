-- +goose Up
CREATE TABLE IF NOT EXISTS ref_kampus (
  id SERIAL PRIMARY KEY,
  nama TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE student_higher_ed_applications
  ADD COLUMN IF NOT EXISTS kampus_id INT NULL REFERENCES ref_kampus(id),
  ADD COLUMN IF NOT EXISTS kampus_nama_snapshot TEXT NULL,
  ADD COLUMN IF NOT EXISTS jenjang_prodi TEXT NULL;

ALTER TABLE student_higher_ed_applications
  DROP CONSTRAINT IF EXISTS student_higher_ed_applications_jenjang_prodi_check;

ALTER TABLE student_higher_ed_applications
  ADD CONSTRAINT student_higher_ed_applications_jenjang_prodi_check
  CHECK (
    jenjang_prodi IS NULL OR
    jenjang_prodi IN ('D1', 'D2', 'D3', 'D4', 'S1', 'S2', 'S3', 'UNKNOWN')
  );

CREATE INDEX IF NOT EXISTS student_higher_ed_applications_kampus_id_idx
  ON student_higher_ed_applications (kampus_id);

-- +goose Down
DROP INDEX IF EXISTS student_higher_ed_applications_kampus_id_idx;

ALTER TABLE student_higher_ed_applications
  DROP CONSTRAINT IF EXISTS student_higher_ed_applications_jenjang_prodi_check;

ALTER TABLE student_higher_ed_applications
  DROP COLUMN IF EXISTS jenjang_prodi,
  DROP COLUMN IF EXISTS kampus_nama_snapshot,
  DROP COLUMN IF EXISTS kampus_id;

DROP TABLE IF EXISTS ref_kampus;
