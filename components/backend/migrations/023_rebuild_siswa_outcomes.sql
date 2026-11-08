-- +goose Up
ALTER TABLE IF EXISTS student_plans RENAME TO student_plans_legacy;

CREATE TABLE IF NOT EXISTS class12_students (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  nisn TEXT NOT NULL,
  nama_siswa TEXT NOT NULL,
  rombel TEXT NOT NULL,
  import_batch_id INTEGER,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS class12_students_school_year_nisn_unique
  ON class12_students (school_id, tahun_lulus, nisn);
CREATE INDEX IF NOT EXISTS class12_students_school_year_idx
  ON class12_students (school_id, tahun_lulus);
CREATE INDEX IF NOT EXISTS class12_students_import_batch_idx
  ON class12_students (import_batch_id);

CREATE TABLE IF NOT EXISTS student_plans (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  nisn TEXT NOT NULL,
  plan_jalur jalur_penerimaan_type[] NOT NULL DEFAULT '{}',
  plan_choices JSONB,
  plan_source TEXT NOT NULL CHECK (plan_source IN ('MANUAL', 'AUTO_FROM_FINAL')),
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  FOREIGN KEY (school_id, tahun_lulus, nisn)
    REFERENCES class12_students (school_id, tahun_lulus, nisn)
    ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS student_plans_school_year_nisn_unique
  ON student_plans (school_id, tahun_lulus, nisn);
CREATE INDEX IF NOT EXISTS student_plans_school_year_idx
  ON student_plans (school_id, tahun_lulus);

CREATE TABLE IF NOT EXISTS student_outcomes (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  nisn TEXT NOT NULL,
  final_status TEXT NOT NULL CHECK (final_status IN ('PT', 'KERJA', 'WIRAUSAHA', 'BELUM')),
  final_path jalur_penerimaan_type,
  finalized_at TIMESTAMP,
  finalized_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  FOREIGN KEY (school_id, tahun_lulus, nisn)
    REFERENCES class12_students (school_id, tahun_lulus, nisn)
    ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS student_outcomes_school_year_nisn_unique
  ON student_outcomes (school_id, tahun_lulus, nisn);
CREATE INDEX IF NOT EXISTS student_outcomes_school_year_idx
  ON student_outcomes (school_id, tahun_lulus);

CREATE TABLE IF NOT EXISTS outcome_pt_details (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL,
  tahun_lulus INTEGER NOT NULL,
  nisn TEXT NOT NULL,
  kampus TEXT,
  prodi TEXT,
  jalur jalur_penerimaan_type,
  tahun_diterima INTEGER,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (school_id, tahun_lulus, nisn),
  FOREIGN KEY (school_id, tahun_lulus, nisn)
    REFERENCES student_outcomes (school_id, tahun_lulus, nisn)
    ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS outcome_work_details (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL,
  tahun_lulus INTEGER NOT NULL,
  nisn TEXT NOT NULL,
  perusahaan TEXT,
  jabatan TEXT,
  tahun_mulai INTEGER,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (school_id, tahun_lulus, nisn),
  FOREIGN KEY (school_id, tahun_lulus, nisn)
    REFERENCES student_outcomes (school_id, tahun_lulus, nisn)
    ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS outcome_business_details (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL,
  tahun_lulus INTEGER NOT NULL,
  nisn TEXT NOT NULL,
  bidang_usaha TEXT,
  nama_usaha TEXT,
  tahun_mulai INTEGER,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (school_id, tahun_lulus, nisn),
  FOREIGN KEY (school_id, tahun_lulus, nisn)
    REFERENCES student_outcomes (school_id, tahun_lulus, nisn)
    ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS student_outcome_audit (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL,
  tahun_lulus INTEGER NOT NULL,
  nisn TEXT NOT NULL,
  changed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  changed_at TIMESTAMP NOT NULL DEFAULT NOW(),
  before_json JSONB,
  after_json JSONB,
  reason TEXT
);

CREATE INDEX IF NOT EXISTS student_outcome_audit_school_year_idx
  ON student_outcome_audit (school_id, tahun_lulus);

INSERT INTO class12_students (
  school_id,
  tahun_lulus,
  nisn,
  nama_siswa,
  rombel,
  import_batch_id,
  created_at,
  updated_at
)
SELECT
  c.school_id,
  c.tahun_lulus,
  c.nisn,
  c.nama_siswa,
  c.rombel_siswa,
  c.import_batch_id,
  c.created_at,
  c.updated_at
FROM student_cohorts c
ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;

INSERT INTO student_outcomes (
  school_id,
  tahun_lulus,
  nisn,
  final_status,
  final_path,
  finalized_at
)
SELECT
  c.school_id,
  c.tahun_lulus,
  c.nisn,
  CASE f.final_type
    WHEN 'HIGHER_ED' THEN 'PT'
    WHEN 'WORK' THEN 'KERJA'
    WHEN 'ENTREPRENEUR' THEN 'WIRAUSAHA'
    WHEN 'UNDEFINED' THEN 'BELUM'
    ELSE 'BELUM'
  END AS final_status,
  CASE
    WHEN f.final_type = 'HIGHER_ED' THEN COALESCE(o.jalur_penerimaan, 'LAINNYA')
    ELSE NULL
  END AS final_path,
  f.finalized_at
FROM student_final_outcomes f
JOIN student_cohorts c
  ON c.school_id = f.school_id
 AND c.id = f.cohort_id
LEFT JOIN student_higher_ed_acceptance_offers o
  ON o.id = f.offer_id
ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;

INSERT INTO outcome_pt_details (
  school_id,
  tahun_lulus,
  nisn,
  kampus,
  prodi,
  jalur,
  tahun_diterima
)
SELECT
  c.school_id,
  c.tahun_lulus,
  c.nisn,
  o.nama_perguruan_tinggi,
  o.jurusan_perguruan_tinggi,
  o.jalur_penerimaan,
  o.tahun_diterima
FROM student_final_outcomes f
JOIN student_cohorts c
  ON c.school_id = f.school_id
 AND c.id = f.cohort_id
JOIN student_higher_ed_acceptance_offers o
  ON o.id = f.offer_id
WHERE f.final_type = 'HIGHER_ED'
ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;

INSERT INTO student_plans (
  school_id,
  tahun_lulus,
  nisn,
  plan_jalur,
  plan_choices,
  plan_source
)
SELECT
  c.school_id,
  c.tahun_lulus,
  c.nisn,
  ARRAY[COALESCE(o.jalur_penerimaan, 'LAINNYA')]::jalur_penerimaan_type[],
  CASE
    WHEN COALESCE(TRIM(o.nama_perguruan_tinggi), '') = ''
      OR COALESCE(TRIM(o.jurusan_perguruan_tinggi), '') = ''
    THEN NULL
    ELSE jsonb_build_array(
      jsonb_build_object(
        'kampus', o.nama_perguruan_tinggi,
        'prodi', o.jurusan_perguruan_tinggi
      )
    )
  END,
  'AUTO_FROM_FINAL'
FROM student_final_outcomes f
JOIN student_cohorts c
  ON c.school_id = f.school_id
 AND c.id = f.cohort_id
LEFT JOIN student_higher_ed_acceptance_offers o
  ON o.id = f.offer_id
WHERE f.final_type = 'HIGHER_ED'
ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS student_outcome_audit;
DROP TABLE IF EXISTS outcome_business_details;
DROP TABLE IF EXISTS outcome_work_details;
DROP TABLE IF EXISTS outcome_pt_details;
DROP TABLE IF EXISTS student_outcomes;
DROP TABLE IF EXISTS student_plans;
DROP TABLE IF EXISTS class12_students;

ALTER TABLE IF EXISTS student_plans_legacy RENAME TO student_plans;
