-- +goose Up
-- +goose StatementBegin
CREATE TYPE gaspol_role_type AS ENUM (
  'SCHOOL_ENTRY',
  'SUPERVISOR',
  'VALIDATOR',
  'JURY',
  'ADMIN_MASTER'
);

CREATE TYPE gaspol_assignment_method_type AS ENUM (
  'MANUAL',
  'RANDOM',
  'RANDOM_FALLBACK',
  'UNASSIGNED'
);

CREATE TYPE gaspol_submission_status_type AS ENUM (
  'DRAFT',
  'SUBMITTED'
);

CREATE TYPE gaspol_assessment_stage_type AS ENUM (
  'MONTHLY',
  'FIELD',
  'PRESENTATION'
);

CREATE TYPE gaspol_ranking_stage_type AS ENUM (
  'MONTHLY',
  'FIELD',
  'PRESENTATION',
  'FINAL'
);

CREATE TABLE IF NOT EXISTS gaspol_school_participants (
  id SERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  school_id INTEGER REFERENCES data_sekolah(id) ON DELETE CASCADE,
  is_year_configuration BOOLEAN NOT NULL DEFAULT FALSE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  validator_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  school_entry_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  notes TEXT,
  created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CHECK (year >= 2024),
  CHECK (
    (is_year_configuration = TRUE AND school_id IS NULL)
    OR
    (is_year_configuration = FALSE AND school_id IS NOT NULL)
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS gaspol_school_participants_year_config_unique
  ON gaspol_school_participants(year)
  WHERE is_year_configuration = TRUE;

CREATE UNIQUE INDEX IF NOT EXISTS gaspol_school_participants_year_school_unique
  ON gaspol_school_participants(year, school_id)
  WHERE is_year_configuration = FALSE;

CREATE INDEX IF NOT EXISTS gaspol_school_participants_year_idx
  ON gaspol_school_participants(year, is_active);

CREATE INDEX IF NOT EXISTS gaspol_school_participants_validator_idx
  ON gaspol_school_participants(validator_user_id, year)
  WHERE is_year_configuration = FALSE;

CREATE TABLE IF NOT EXISTS gaspol_user_roles (
  id SERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role gaspol_role_type NOT NULL,
  school_id INTEGER REFERENCES data_sekolah(id) ON DELETE CASCADE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  assigned_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CHECK (year >= 2024)
);

CREATE UNIQUE INDEX IF NOT EXISTS gaspol_user_roles_year_user_role_school_unique
  ON gaspol_user_roles(year, user_id, role, school_id)
  WHERE school_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS gaspol_user_roles_year_user_role_unique
  ON gaspol_user_roles(year, user_id, role)
  WHERE school_id IS NULL;

CREATE INDEX IF NOT EXISTS gaspol_user_roles_lookup_idx
  ON gaspol_user_roles(year, role, is_active);

CREATE TABLE IF NOT EXISTS gaspol_assignments (
  id SERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  month INTEGER NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  supervisor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  supervisor_name_snapshot TEXT NOT NULL DEFAULT '',
  method gaspol_assignment_method_type NOT NULL DEFAULT 'MANUAL',
  generated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CHECK (year >= 2024),
  CHECK (month BETWEEN 1 AND 12)
);

CREATE UNIQUE INDEX IF NOT EXISTS gaspol_assignments_year_month_school_unique
  ON gaspol_assignments(year, month, school_id);

CREATE INDEX IF NOT EXISTS gaspol_assignments_supervisor_idx
  ON gaspol_assignments(supervisor_user_id, year, month);

CREATE TABLE IF NOT EXISTS gaspol_school_submissions (
  id SERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  month INTEGER NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  submitted_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  report_title TEXT NOT NULL DEFAULT '',
  report_body TEXT NOT NULL DEFAULT '',
  report_link TEXT NOT NULL DEFAULT '',
  attachment_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
  status gaspol_submission_status_type NOT NULL DEFAULT 'DRAFT',
  submitted_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CHECK (year >= 2024),
  CHECK (month BETWEEN 1 AND 12)
);

CREATE UNIQUE INDEX IF NOT EXISTS gaspol_school_submissions_year_month_school_unique
  ON gaspol_school_submissions(year, month, school_id);

CREATE INDEX IF NOT EXISTS gaspol_school_submissions_school_idx
  ON gaspol_school_submissions(school_id, year, month);

CREATE TABLE IF NOT EXISTS gaspol_assessments (
  id SERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  month INTEGER NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  assignment_id INTEGER REFERENCES gaspol_assignments(id) ON DELETE SET NULL,
  assessor_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  assessor_role gaspol_role_type NOT NULL,
  stage gaspol_assessment_stage_type NOT NULL,
  score_monthly NUMERIC(8,2) NOT NULL DEFAULT 0,
  score_field NUMERIC(8,2) NOT NULL DEFAULT 0,
  score_presentation NUMERIC(8,2) NOT NULL DEFAULT 0,
  score_total NUMERIC(8,2) NOT NULL DEFAULT 0,
  notes TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  is_validated BOOLEAN NOT NULL DEFAULT FALSE,
  validated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  validated_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CHECK (year >= 2024),
  CHECK (month BETWEEN 1 AND 12)
);

CREATE UNIQUE INDEX IF NOT EXISTS gaspol_assessments_unique_actor_stage
  ON gaspol_assessments(year, month, school_id, stage, assessor_user_id);

CREATE INDEX IF NOT EXISTS gaspol_assessments_validation_idx
  ON gaspol_assessments(year, stage, is_validated, school_id);

CREATE TABLE IF NOT EXISTS gaspol_ranking_snapshots (
  id SERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  month INTEGER NOT NULL,
  stage gaspol_ranking_stage_type NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  rank_position INTEGER NOT NULL DEFAULT 0,
  score_monthly NUMERIC(8,2) NOT NULL DEFAULT 0,
  score_field NUMERIC(8,2) NOT NULL DEFAULT 0,
  score_presentation NUMERIC(8,2) NOT NULL DEFAULT 0,
  score_total NUMERIC(8,2) NOT NULL DEFAULT 0,
  snapshot_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  published_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  published_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CHECK (year >= 2024),
  CHECK (month BETWEEN 1 AND 12)
);

CREATE UNIQUE INDEX IF NOT EXISTS gaspol_ranking_snapshots_year_month_stage_school_unique
  ON gaspol_ranking_snapshots(year, month, stage, school_id);

CREATE INDEX IF NOT EXISTS gaspol_ranking_snapshots_lookup_idx
  ON gaspol_ranking_snapshots(year, stage, month, rank_position);

CREATE TABLE IF NOT EXISTS gaspol_audit_logs (
  id SERIAL PRIMARY KEY,
  year INTEGER,
  actor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  actor_role TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL,
  entity_type TEXT NOT NULL DEFAULT '',
  entity_id INTEGER,
  school_id INTEGER REFERENCES data_sekolah(id) ON DELETE SET NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS gaspol_audit_logs_year_idx
  ON gaspol_audit_logs(year, action, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS gaspol_audit_logs;
DROP TABLE IF EXISTS gaspol_ranking_snapshots;
DROP TABLE IF EXISTS gaspol_assessments;
DROP TABLE IF EXISTS gaspol_school_submissions;
DROP TABLE IF EXISTS gaspol_assignments;
DROP TABLE IF EXISTS gaspol_user_roles;
DROP TABLE IF EXISTS gaspol_school_participants;

DROP TYPE IF EXISTS gaspol_ranking_stage_type;
DROP TYPE IF EXISTS gaspol_assessment_stage_type;
DROP TYPE IF EXISTS gaspol_submission_status_type;
DROP TYPE IF EXISTS gaspol_assignment_method_type;
DROP TYPE IF EXISTS gaspol_role_type;
-- +goose StatementEnd
