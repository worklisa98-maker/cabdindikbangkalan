-- +goose Up
-- +goose StatementBegin
ALTER TABLE outcome_pt_offers
  ALTER COLUMN nisn DROP NOT NULL,
  ADD COLUMN IF NOT EXISTS student_name TEXT,
  ADD COLUMN IF NOT EXISTS normalized_student_name TEXT,
  ADD COLUMN IF NOT EXISTS ref_kampus_id INTEGER REFERENCES ref_kampus(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS source_role TEXT NOT NULL DEFAULT 'SCHOOL_ADMIN',
  ADD COLUMN IF NOT EXISTS source_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS import_batch_id INTEGER REFERENCES uploads(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS school_confirmed_at TIMESTAMP,
  ADD COLUMN IF NOT EXISTS school_confirmed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS conflict_status TEXT NOT NULL DEFAULT 'NONE',
  ADD COLUMN IF NOT EXISTS conflict_detail JSONB,
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP;

ALTER TABLE outcome_pt_offers
  DROP CONSTRAINT IF EXISTS outcome_pt_offers_source_role_check,
  DROP CONSTRAINT IF EXISTS outcome_pt_offers_conflict_status_check;

ALTER TABLE outcome_pt_offers
  ADD CONSTRAINT outcome_pt_offers_source_role_check
    CHECK (source_role IN ('SCHOOL_ADMIN', 'ADMIN', 'ADMIN_MASTER', 'MIGRATION')),
  ADD CONSTRAINT outcome_pt_offers_conflict_status_check
    CHECK (conflict_status IN ('NONE', 'MULTI_OFFER', 'SAME_PATH_DIFFERENT_DESTINATION', 'NISN_CONFLICT', 'CROSS_OUTCOME'));

UPDATE outcome_pt_offers offer
SET
  student_name = COALESCE(
    (SELECT NULLIF(TRIM(c.nama_siswa), '') FROM class12_students c
      WHERE c.school_id = offer.school_id AND c.tahun_lulus = offer.tahun_lulus AND c.nisn = offer.nisn LIMIT 1),
    (SELECT NULLIF(TRIM(c.nama_siswa), '') FROM student_cohorts c
      WHERE c.school_id = offer.school_id AND c.tahun_lulus = offer.tahun_lulus AND c.nisn = offer.nisn LIMIT 1),
    'Tanpa Nama'
  ),
  normalized_student_name = LOWER(REGEXP_REPLACE(TRIM(COALESCE(
    (SELECT NULLIF(TRIM(c.nama_siswa), '') FROM class12_students c
      WHERE c.school_id = offer.school_id AND c.tahun_lulus = offer.tahun_lulus AND c.nisn = offer.nisn LIMIT 1),
    (SELECT NULLIF(TRIM(c.nama_siswa), '') FROM student_cohorts c
      WHERE c.school_id = offer.school_id AND c.tahun_lulus = offer.tahun_lulus AND c.nisn = offer.nisn LIMIT 1),
    'Tanpa Nama'
  )), '\s+', ' ', 'g')),
  source_role = CASE
    WHEN UPPER(COALESCE(offer.source, '')) LIKE '%MIGRATION%' THEN 'MIGRATION'
    ELSE 'SCHOOL_ADMIN'
  END
WHERE offer.student_name IS NULL OR offer.normalized_student_name IS NULL;

UPDATE outcome_pt_offers
SET
  student_name = COALESCE(NULLIF(TRIM(student_name), ''), 'Tanpa Nama'),
  normalized_student_name = LOWER(REGEXP_REPLACE(TRIM(COALESCE(NULLIF(student_name, ''), 'Tanpa Nama')), '\s+', ' ', 'g'))
WHERE student_name IS NULL OR normalized_student_name IS NULL;

ALTER TABLE outcome_pt_offers
  ALTER COLUMN student_name SET NOT NULL,
  ALTER COLUMN normalized_student_name SET NOT NULL;

ALTER TABLE outcome_pt_offers
  DROP CONSTRAINT IF EXISTS outcome_pt_offers_unique_offer;

DROP INDEX IF EXISTS outcome_pt_offers_unique_offer;

CREATE UNIQUE INDEX IF NOT EXISTS outcome_pt_offers_identity_offer_unique
  ON outcome_pt_offers (
    school_id,
    tahun_lulus,
    normalized_student_name,
    UPPER(TRIM(jalur)),
    UPPER(TRIM(kampus)),
    UPPER(TRIM(prodi))
  )
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS outcome_pt_offers_identity_idx
  ON outcome_pt_offers (school_id, tahun_lulus, normalized_student_name)
  WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS outcome_pt_offers_nisn_idx
  ON outcome_pt_offers (school_id, tahun_lulus, nisn)
  WHERE nisn IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS outcome_pt_offers_import_batch_idx
  ON outcome_pt_offers (import_batch_id)
  WHERE import_batch_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS outcome_pt_offers_conflict_idx
  ON outcome_pt_offers (conflict_status, school_id, tahun_lulus)
  WHERE conflict_status <> 'NONE' AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS pt_acceptance_offer_audit (
  id BIGSERIAL PRIMARY KEY,
  offer_id BIGINT NOT NULL REFERENCES outcome_pt_offers(id) ON DELETE CASCADE,
  action TEXT NOT NULL,
  actor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  before_json JSONB,
  after_json JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS pt_acceptance_offer_audit_offer_idx
  ON pt_acceptance_offer_audit (offer_id, created_at DESC);

CREATE TABLE IF NOT EXISTS pt_official_acceptance_counts (
  id BIGSERIAL PRIMARY KEY,
  tahun_lulus INTEGER NOT NULL,
  jalur TEXT NOT NULL,
  official_count INTEGER NOT NULL CHECK (official_count >= 0),
  source_note TEXT,
  source_updated_at DATE NOT NULL DEFAULT CURRENT_DATE,
  created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (tahun_lulus, jalur)
);
CREATE INDEX IF NOT EXISTS pt_official_acceptance_counts_year_idx
  ON pt_official_acceptance_counts (tahun_lulus, jalur);

CREATE TABLE IF NOT EXISTS gaspol_assignment_previews (
  id UUID PRIMARY KEY,
  year INTEGER NOT NULL,
  month INTEGER NOT NULL,
  created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  payload JSONB NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMP NOT NULL,
  committed_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS gaspol_assignment_previews_expiry_idx
  ON gaspol_assignment_previews (expires_at)
  WHERE committed_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pt_official_acceptance_counts;
DROP TABLE IF EXISTS pt_acceptance_offer_audit;
DROP TABLE IF EXISTS gaspol_assignment_previews;
DROP INDEX IF EXISTS outcome_pt_offers_conflict_idx;
DROP INDEX IF EXISTS outcome_pt_offers_import_batch_idx;
DROP INDEX IF EXISTS outcome_pt_offers_nisn_idx;
DROP INDEX IF EXISTS outcome_pt_offers_identity_idx;
DROP INDEX IF EXISTS outcome_pt_offers_identity_offer_unique;
CREATE UNIQUE INDEX IF NOT EXISTS outcome_pt_offers_unique_offer
  ON outcome_pt_offers (school_id, tahun_lulus, nisn, jalur, kampus, prodi);
ALTER TABLE outcome_pt_offers
  DROP CONSTRAINT IF EXISTS outcome_pt_offers_conflict_status_check,
  DROP CONSTRAINT IF EXISTS outcome_pt_offers_source_role_check,
  DROP COLUMN IF EXISTS deleted_at,
  DROP COLUMN IF EXISTS conflict_detail,
  DROP COLUMN IF EXISTS conflict_status,
  DROP COLUMN IF EXISTS school_confirmed_by,
  DROP COLUMN IF EXISTS school_confirmed_at,
  DROP COLUMN IF EXISTS import_batch_id,
  DROP COLUMN IF EXISTS source_user_id,
  DROP COLUMN IF EXISTS source_role,
  DROP COLUMN IF EXISTS ref_kampus_id,
  DROP COLUMN IF EXISTS normalized_student_name,
  DROP COLUMN IF EXISTS student_name;
ALTER TABLE outcome_pt_offers ALTER COLUMN nisn SET NOT NULL;
-- +goose StatementEnd
