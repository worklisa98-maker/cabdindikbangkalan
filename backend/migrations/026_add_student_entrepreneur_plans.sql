-- +goose Up
CREATE TABLE IF NOT EXISTS student_entrepreneur_plans (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  cohort_id BIGINT NOT NULL REFERENCES student_cohorts(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  rencana_ke SMALLINT NOT NULL CHECK (rencana_ke BETWEEN 1 AND 2),
  nama_usaha TEXT NOT NULL,
  kategori_usaha TEXT NOT NULL,
  status_kesiapan TEXT NOT NULL,
  rencana_mulai TEXT NOT NULL,
  modal_awal TEXT NOT NULL,
  sumber_modal TEXT NOT NULL,
  lokasi TEXT,
  platform TEXT,
  catatan TEXT,
  source TEXT NOT NULL DEFAULT 'ENTREPRENEUR_IMPORT',
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS student_entrepreneur_plans_unique
  ON student_entrepreneur_plans (school_id, cohort_id, rencana_ke);

CREATE INDEX IF NOT EXISTS student_entrepreneur_plans_cohort_idx
  ON student_entrepreneur_plans (cohort_id);

-- +goose Down
DROP INDEX IF EXISTS student_entrepreneur_plans_cohort_idx;
DROP INDEX IF EXISTS student_entrepreneur_plans_unique;
DROP TABLE IF EXISTS student_entrepreneur_plans;
