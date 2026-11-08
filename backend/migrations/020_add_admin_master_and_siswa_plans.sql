-- +goose Up
ALTER TYPE role_type ADD VALUE IF NOT EXISTS 'ADMIN_MASTER';

ALTER TYPE upload_type ADD VALUE IF NOT EXISTS 'COHORTS';
ALTER TYPE upload_type ADD VALUE IF NOT EXISTS 'APPLICATIONS';

ALTER TABLE users
ADD COLUMN IF NOT EXISTS allowed_levels TEXT[];

UPDATE users
SET allowed_levels = ARRAY['SMA', 'SMK', 'SLB']
WHERE role = 'ADMIN'
  AND allowed_levels IS NULL;

ALTER TABLE uploads
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

ALTER TABLE uploads
ADD COLUMN IF NOT EXISTS deleted_by INTEGER REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE student_cohorts
ADD COLUMN IF NOT EXISTS import_batch_id INTEGER;

CREATE INDEX IF NOT EXISTS idx_student_cohorts_import_batch_id
  ON student_cohorts (import_batch_id);

ALTER TABLE student_higher_ed_applications
ADD COLUMN IF NOT EXISTS import_batch_id INTEGER;

CREATE INDEX IF NOT EXISTS idx_student_higher_ed_apps_import_batch_id
  ON student_higher_ed_applications (import_batch_id);

CREATE TABLE IF NOT EXISTS student_plans (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  cohort_id BIGINT NOT NULL REFERENCES student_cohorts(id) ON DELETE CASCADE,
  plan_type TEXT NOT NULL CHECK (plan_type IN ('PT', 'KERJA', 'WIRAUSAHA')),
  jalur_penerimaan jalur_penerimaan_type,
  pt_pilihan1_pt TEXT,
  pt_pilihan1_jurusan TEXT,
  pt_pilihan2_pt TEXT,
  pt_pilihan2_jurusan TEXT,
  pt_pilihan3_pt TEXT,
  pt_pilihan3_jurusan TEXT,
  kerja_status TEXT,
  kerja_perusahaan TEXT,
  kerja_posisi TEXT,
  kerja_bidang TEXT,
  wirausaha_bidang TEXT,
  wirausaha_nama TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS student_plans_school_cohort_type_unique
  ON student_plans (school_id, cohort_id, plan_type);

CREATE INDEX IF NOT EXISTS student_plans_cohort_idx
  ON student_plans (cohort_id);

-- +goose Down
DROP INDEX IF EXISTS student_plans_cohort_idx;
DROP INDEX IF EXISTS student_plans_school_cohort_type_unique;
DROP TABLE IF EXISTS student_plans;

DROP INDEX IF EXISTS idx_student_higher_ed_apps_import_batch_id;
ALTER TABLE student_higher_ed_applications DROP COLUMN IF EXISTS import_batch_id;

DROP INDEX IF EXISTS idx_student_cohorts_import_batch_id;
ALTER TABLE student_cohorts DROP COLUMN IF EXISTS import_batch_id;

ALTER TABLE uploads DROP COLUMN IF EXISTS deleted_by;
ALTER TABLE uploads DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE users DROP COLUMN IF EXISTS allowed_levels;

-- Note: enum values (role_type, upload_type) are not removed in down migration.
