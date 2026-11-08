-- +goose Up
ALTER TABLE student_outcomes
  ADD COLUMN IF NOT EXISTS source TEXT;

UPDATE student_outcomes o
SET source = CASE
  WHEN EXISTS (
    SELECT 1
    FROM student_outcome_audit a
    WHERE a.school_id = o.school_id
      AND a.tahun_lulus = o.tahun_lulus
      AND a.nisn = o.nisn
      AND UPPER(COALESCE(a.reason, '')) LIKE 'IMPORT_HISTORICAL%'
  ) THEN 'IMPORT_ALUMNI'
  ELSE 'FINAL_SISWA_AKHIR'
END
WHERE COALESCE(TRIM(source), '') = '';

ALTER TABLE student_outcomes
  ALTER COLUMN source SET DEFAULT 'FINAL_SISWA_AKHIR';

ALTER TABLE student_outcomes
  ALTER COLUMN source SET NOT NULL;

ALTER TABLE student_outcomes
  DROP CONSTRAINT IF EXISTS student_outcomes_source_check;

ALTER TABLE student_outcomes
  ADD CONSTRAINT student_outcomes_source_check
  CHECK (source IN ('FINAL_SISWA_AKHIR', 'IMPORT_ALUMNI'));

-- +goose Down
ALTER TABLE student_outcomes
  DROP CONSTRAINT IF EXISTS student_outcomes_source_check;

ALTER TABLE student_outcomes
  ALTER COLUMN source DROP DEFAULT;

ALTER TABLE student_outcomes
  DROP COLUMN IF EXISTS source;
