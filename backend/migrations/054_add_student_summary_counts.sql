-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS student_year_summaries (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  total_siswa_akhir INTEGER NOT NULL DEFAULT 0 CHECK (total_siswa_akhir >= 0),
  source TEXT NOT NULL DEFAULT 'MANUAL' CHECK (source IN ('MANUAL', 'AUTO_FROM_LEGACY')),
  created_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  updated_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (school_id, tahun_lulus)
);

CREATE INDEX IF NOT EXISTS student_year_summaries_year_idx
  ON student_year_summaries (tahun_lulus);

CREATE TABLE IF NOT EXISTS student_pt_registration_counts (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  jalur TEXT NOT NULL CHECK (jalur IN ('SNBP', 'SNBT', 'MANDIRI', 'KEDINASAN', 'LAINNYA')),
  registered_count INTEGER NOT NULL DEFAULT 0 CHECK (registered_count >= 0),
  source TEXT NOT NULL DEFAULT 'MANUAL' CHECK (source IN ('MANUAL', 'AUTO_FROM_LEGACY')),
  created_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  updated_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (school_id, tahun_lulus, jalur)
);

CREATE INDEX IF NOT EXISTS student_pt_registration_counts_year_idx
  ON student_pt_registration_counts (tahun_lulus);

CREATE TABLE IF NOT EXISTS student_entrepreneur_plan_counts (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  planned_count INTEGER NOT NULL DEFAULT 0 CHECK (planned_count >= 0),
  source TEXT NOT NULL DEFAULT 'MANUAL' CHECK (source IN ('MANUAL', 'AUTO_FROM_LEGACY')),
  created_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  updated_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (school_id, tahun_lulus)
);

CREATE INDEX IF NOT EXISTS student_entrepreneur_plan_counts_year_idx
  ON student_entrepreneur_plan_counts (tahun_lulus);

INSERT INTO student_year_summaries (
  school_id,
  tahun_lulus,
  total_siswa_akhir,
  source,
  updated_at
)
SELECT
  c.school_id,
  c.tahun_lulus,
  COUNT(*)::INTEGER AS total_siswa_akhir,
  'AUTO_FROM_LEGACY',
  NOW()
FROM class12_students c
WHERE c.tahun_lulus > 0
GROUP BY c.school_id, c.tahun_lulus
ON CONFLICT (school_id, tahun_lulus)
DO UPDATE SET
  total_siswa_akhir = EXCLUDED.total_siswa_akhir,
  updated_at = NOW()
WHERE student_year_summaries.source = 'AUTO_FROM_LEGACY';

INSERT INTO student_pt_registration_counts (
  school_id,
  tahun_lulus,
  jalur,
  registered_count,
  source,
  updated_at
)
SELECT
  sc.school_id,
  sc.tahun_lulus,
  CASE
    WHEN UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%SNBP%' THEN 'SNBP'
    WHEN UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%SNBT%' THEN 'SNBT'
    WHEN UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%MANDIRI%'
      OR UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%SNBM%' THEN 'MANDIRI'
    WHEN UPPER(TRIM(COALESCE(a.jalur_penerimaan::text, ''))) LIKE '%KEDINAS%' THEN 'KEDINASAN'
    ELSE 'LAINNYA'
  END AS jalur,
  COUNT(DISTINCT sc.nisn)::INTEGER AS registered_count,
  'AUTO_FROM_LEGACY',
  NOW()
FROM student_higher_ed_applications a
JOIN student_cohorts sc
  ON sc.id = a.cohort_id
 AND sc.school_id = a.school_id
WHERE sc.tahun_lulus > 0
  AND TRIM(COALESCE(sc.nisn, '')) <> ''
GROUP BY sc.school_id, sc.tahun_lulus, jalur
ON CONFLICT (school_id, tahun_lulus, jalur)
DO UPDATE SET
  registered_count = EXCLUDED.registered_count,
  updated_at = NOW()
WHERE student_pt_registration_counts.source = 'AUTO_FROM_LEGACY';

INSERT INTO student_entrepreneur_plan_counts (
  school_id,
  tahun_lulus,
  planned_count,
  source,
  updated_at
)
SELECT
  sc.school_id,
  sc.tahun_lulus,
  COUNT(DISTINCT sc.nisn)::INTEGER AS planned_count,
  'AUTO_FROM_LEGACY',
  NOW()
FROM student_entrepreneur_plans p
JOIN student_cohorts sc
  ON sc.id = p.cohort_id
 AND sc.school_id = p.school_id
WHERE sc.tahun_lulus > 0
  AND TRIM(COALESCE(sc.nisn, '')) <> ''
GROUP BY sc.school_id, sc.tahun_lulus
ON CONFLICT (school_id, tahun_lulus)
DO UPDATE SET
  planned_count = EXCLUDED.planned_count,
  updated_at = NOW()
WHERE student_entrepreneur_plan_counts.source = 'AUTO_FROM_LEGACY';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS student_entrepreneur_plan_counts;
DROP TABLE IF EXISTS student_pt_registration_counts;
DROP TABLE IF EXISTS student_year_summaries;
-- +goose StatementEnd
