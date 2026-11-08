-- +goose Up
-- GASPOL v2 is intentionally additive. Migration 039 is production history and is not edited.

ALTER TABLE gaspol_assessments
  ADD COLUMN IF NOT EXISTS stage_code TEXT,
  ADD COLUMN IF NOT EXISTS workflow_status TEXT,
  ADD COLUMN IF NOT EXISTS koor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS koor_validated_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS cabdin_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS cabdin_validated_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS score_field_stage_1 NUMERIC(8,2),
  ADD COLUMN IF NOT EXISTS score_field_stage_2 NUMERIC(8,2),
  ADD COLUMN IF NOT EXISTS score_nomination NUMERIC(8,2),
  ADD COLUMN IF NOT EXISTS score_appeal NUMERIC(8,2);

UPDATE gaspol_assessments
SET stage_code = CASE stage::text
      WHEN 'FIELD' THEN 'FIELD_STAGE_1'
      ELSE stage::text
    END,
    workflow_status = CASE
      WHEN is_validated THEN 'FINAL'
      WHEN stage::text = 'PRESENTATION' THEN 'IN_PROGRESS'
      ELSE 'WAITING_KOOR'
    END,
    score_field_stage_1 = CASE WHEN stage::text = 'FIELD' THEN score_field ELSE score_field_stage_1 END,
    cabdin_user_id = CASE WHEN is_validated THEN validated_by ELSE cabdin_user_id END,
    cabdin_validated_at = CASE WHEN is_validated THEN validated_at ELSE cabdin_validated_at END
WHERE stage_code IS NULL OR workflow_status IS NULL;

ALTER TABLE gaspol_assessments
  ALTER COLUMN stage_code SET NOT NULL,
  ALTER COLUMN workflow_status SET NOT NULL,
  ALTER COLUMN workflow_status SET DEFAULT 'ASSESSED';

ALTER TABLE gaspol_assessments DROP CONSTRAINT IF EXISTS gaspol_assessments_workflow_status_check;
ALTER TABLE gaspol_assessments ADD CONSTRAINT gaspol_assessments_workflow_status_check CHECK (
  workflow_status IN ('ASSESSED','IN_PROGRESS','WAITING_KOOR','KOOR_VALIDATED','WAITING_CABDIN','FINAL','RETURNED_BY_KOOR','RETURNED_BY_CABDIN')
);

CREATE TABLE IF NOT EXISTS gaspol_competition_configs (
  year INTEGER PRIMARY KEY CHECK (year >= 2024),
  state TEXT NOT NULL DEFAULT 'MONTHLY_ACTIVE',
  september_field1_deadline TIMESTAMPTZ NOT NULL,
  like_threshold INTEGER NOT NULL DEFAULT 800 CHECK (like_threshold > 0),
  internal_voter_denominator INTEGER CHECK (internal_voter_denominator > 0),
  voter_denominator_locked_at TIMESTAMPTZ,
  like_sync_event_start TIMESTAMPTZ,
  like_cutoff_at TIMESTAMPTZ,
  public_full_ranking_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (state IN (
    'MONTHLY_ACTIVE','AUGUST_DIRECT_ASSESSMENT','SEPTEMBER_FIELD1_ASSESSMENT',
    'WAITING_STAGE1_VALIDATION','STAGE1_FINAL','TOP10_LOCKED','FIELD2_ACTIVE',
    'FIELD2_FINAL','TOP5_WAITING_BA','TOP5_LOCKED','PRESENTATION_ACTIVE',
    'PRESENTATION_FINAL','NOT_FAILED_NOMINATION_ACTIVE','NOT_FAILED_NOMINATION_FINAL',
    'BOTTOM5_LOCKED','APPEAL_ACTIVE','APPEAL_FINAL','FINAL_AWARD_READY','GASPOL_FINALIZED'
  ))
);

INSERT INTO gaspol_competition_configs(year, september_field1_deadline)
SELECT DISTINCT p.year, (make_date(p.year, 10, 15)::timestamp + time '23:59:59') AT TIME ZONE 'Asia/Jakarta'
FROM gaspol_school_participants p
ON CONFLICT (year) DO NOTHING;

CREATE TABLE IF NOT EXISTS gaspol_stage_assignments (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL CHECK (year >= 2024),
  stage TEXT NOT NULL,
  school_id INTEGER REFERENCES data_sekolah(id) ON DELETE CASCADE,
  assessor_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  assignment_role TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  assigned_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(year, stage, school_id, assessor_user_id),
  CHECK (stage IN ('MONTHLY','FIELD_STAGE_1','FIELD_STAGE_2','PRESENTATION','NOT_FAILED_NOMINATION','BOTTOM5_APPEAL','INNOVATION')),
  CHECK (assignment_role IN ('SUPERVISOR_MONTHLY','FIELD_STAGE_1_ASSESSOR','FIELD_STAGE_2_ASSESSOR','PRESENTATION_ASSESSOR','NOMINATION_ASSESSOR','KOOR_GASPOL','APPEAL_PANEL','INNOVATION_ASSESSOR','KASI_SMK_APPROVER'))
);
CREATE INDEX IF NOT EXISTS gaspol_stage_assignments_actor_idx ON gaspol_stage_assignments(year, assessor_user_id, stage) WHERE is_active;

CREATE TABLE IF NOT EXISTS gaspol_stage_assessments (
  id BIGSERIAL PRIMARY KEY,
  legacy_assessment_id INTEGER UNIQUE REFERENCES gaspol_assessments(id) ON DELETE SET NULL,
  year INTEGER NOT NULL CHECK (year >= 2024),
  stage TEXT NOT NULL,
  month INTEGER NOT NULL DEFAULT 0 CHECK (month BETWEEN 0 AND 12),
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  assignment_id BIGINT REFERENCES gaspol_stage_assignments(id) ON DELETE SET NULL,
  assessor_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  raw_score NUMERIC(8,2) NOT NULL CHECK (raw_score BETWEEN 0 AND 100),
  rubric JSONB NOT NULL DEFAULT '{}'::jsonb,
  notes TEXT NOT NULL DEFAULT '',
  workflow_status TEXT NOT NULL DEFAULT 'ASSESSED',
  koor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  koor_notes TEXT NOT NULL DEFAULT '',
  koor_validated_at TIMESTAMPTZ,
  cabdin_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  cabdin_notes TEXT NOT NULL DEFAULT '',
  cabdin_validated_at TIMESTAMPTZ,
  final_score NUMERIC(8,2) CHECK (final_score BETWEEN 0 AND 100),
  locked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(year, stage, month, school_id, assessor_user_id),
  CHECK (stage IN ('MONTHLY','FIELD_STAGE_1','FIELD_STAGE_2','PRESENTATION','NOT_FAILED_NOMINATION','BOTTOM5_APPEAL','INNOVATION')),
  CHECK (workflow_status IN ('ASSESSED','IN_PROGRESS','WAITING_KOOR','KOOR_VALIDATED','WAITING_CABDIN','FINAL','RETURNED_BY_KOOR','RETURNED_BY_CABDIN'))
);
CREATE INDEX IF NOT EXISTS gaspol_stage_assessments_queue_idx ON gaspol_stage_assessments(year, workflow_status, stage);

INSERT INTO gaspol_stage_assessments(
  legacy_assessment_id, year, stage, month, school_id, assessor_user_id, raw_score,
  notes, workflow_status, cabdin_user_id, cabdin_validated_at, final_score,
  locked_at, created_at, updated_at
)
SELECT ga.id, ga.year,
  CASE ga.stage::text WHEN 'FIELD' THEN 'FIELD_STAGE_1' ELSE ga.stage::text END,
  ga.month, ga.school_id, ga.assessor_user_id,
  CASE ga.stage::text WHEN 'MONTHLY' THEN ga.score_monthly WHEN 'FIELD' THEN ga.score_field ELSE ga.score_presentation END,
  ga.notes,
  CASE
    WHEN ga.is_validated THEN 'FINAL'
    WHEN ga.stage::text = 'PRESENTATION' THEN 'IN_PROGRESS'
    ELSE 'WAITING_KOOR'
  END,
  CASE WHEN ga.is_validated THEN ga.validated_by END,
  CASE WHEN ga.is_validated THEN ga.validated_at END,
  CASE WHEN ga.is_validated THEN CASE ga.stage::text WHEN 'MONTHLY' THEN ga.score_monthly WHEN 'FIELD' THEN ga.score_field ELSE ga.score_presentation END END,
  CASE WHEN ga.is_validated THEN ga.validated_at END,
  ga.created_at, ga.updated_at
FROM gaspol_assessments ga
ON CONFLICT (legacy_assessment_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS gaspol_cohorts (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  cohort TEXT NOT NULL,
  source_lock_id BIGINT,
  locked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  locked_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  released_at TIMESTAMPTZ,
  release_reason TEXT,
  UNIQUE(year, school_id, cohort),
  CHECK (cohort IN ('TOP_10','NOT_FAILED','TOP_5','BOTTOM_5','TERGAGAL','ESCAPED_BOTTOM_ZONE'))
);

CREATE TABLE IF NOT EXISTS gaspol_result_locks (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  lock_type TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'LOCKED',
  reason TEXT NOT NULL DEFAULT '',
  ba_document_id BIGINT,
  ranking_payload JSONB NOT NULL,
  tie_resolution JSONB NOT NULL DEFAULT '{}'::jsonb,
  locked_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  locked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  reopened_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  reopened_at TIMESTAMPTZ,
  reopen_reason TEXT,
  UNIQUE(year, lock_type, revision),
  CHECK (lock_type IN ('TOP_10','TOP_5','BOTTOM_5','FINAL','AWARDING')),
  CHECK (status IN ('LOCKED','REOPENED','SUPERSEDED'))
);

CREATE TABLE IF NOT EXISTS gaspol_ba_documents (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  document_type TEXT NOT NULL DEFAULT 'TOP5_DETERMINATION',
  file_name TEXT NOT NULL,
  content_type TEXT NOT NULL,
  content BYTEA NOT NULL,
  sha256 TEXT NOT NULL,
  is_public BOOLEAN NOT NULL DEFAULT FALSE,
  uploaded_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (octet_length(content) > 0)
);
ALTER TABLE gaspol_result_locks DROP CONSTRAINT IF EXISTS gaspol_result_locks_ba_document_id_fkey;
ALTER TABLE gaspol_result_locks ADD CONSTRAINT gaspol_result_locks_ba_document_id_fkey FOREIGN KEY (ba_document_id) REFERENCES gaspol_ba_documents(id) ON DELETE RESTRICT;

CREATE TABLE IF NOT EXISTS gaspol_nomination_submissions (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  submission_type TEXT NOT NULL,
  improvement_details TEXT NOT NULL DEFAULT '',
  improvement_result TEXT NOT NULL DEFAULT '',
  response_text TEXT NOT NULL DEFAULT '',
  drive_url TEXT NOT NULL DEFAULT '',
  evidence_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
  status TEXT NOT NULL DEFAULT 'DRAFT',
  submitted_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  submitted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(year, school_id, submission_type),
  CHECK (submission_type IN ('NOT_FAILED_NOMINATION','BOTTOM5_APPEAL')),
  CHECK (status IN ('DRAFT','SUBMITTED','LOCKED'))
);

CREATE TABLE IF NOT EXISTS gaspol_voting_sessions (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'DRAFT',
  voter_denominator INTEGER NOT NULL CHECK (voter_denominator > 0),
  opened_at TIMESTAMPTZ,
  closes_at TIMESTAMPTZ NOT NULL,
  closed_at TIMESTAMPTZ,
  created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (status IN ('DRAFT','OPEN','CLOSED','CANCELLED'))
);

CREATE TABLE IF NOT EXISTS gaspol_internal_votes (
  id BIGSERIAL PRIMARY KEY,
  session_id BIGINT NOT NULL REFERENCES gaspol_voting_sessions(id) ON DELETE RESTRICT,
  voter_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE RESTRICT,
  voted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(session_id, voter_user_id)
);

CREATE TABLE IF NOT EXISTS gaspol_favorite_content (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  content_url TEXT NOT NULL DEFAULT '',
  platform TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'NOT_SUBMITTED',
  last_auto_like_count INTEGER,
  effective_like_count INTEGER,
  last_error TEXT NOT NULL DEFAULT '',
  link_locked_at TIMESTAMPTZ,
  frozen_at TIMESTAMPTZ,
  updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(year, school_id),
  CHECK (status IN ('NOT_SUBMITTED','VALIDATING','LINK_VALID','SYNC_ACTIVE','SYNC_ERROR','FROZEN'))
);

CREATE TABLE IF NOT EXISTS gaspol_like_snapshots (
  id BIGSERIAL PRIMARY KEY,
  favorite_content_id BIGINT NOT NULL REFERENCES gaspol_favorite_content(id) ON DELETE CASCADE,
  like_count INTEGER NOT NULL CHECK (like_count >= 0),
  source TEXT NOT NULL,
  fetch_status TEXT NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  evidence_url TEXT NOT NULL DEFAULT '',
  actor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (source IN ('AUTO','MANUAL')),
  CHECK (fetch_status IN ('SUCCESS','ERROR','OVERRIDE','FINAL'))
);

CREATE TABLE IF NOT EXISTS gaspol_awards (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  award_type TEXT NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE RESTRICT,
  status TEXT NOT NULL DEFAULT 'LOCKED',
  score NUMERIC(8,2),
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  locked_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  locked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(year, award_type, school_id),
  CHECK (award_type IN ('BEST','INNOVATIVE','FAVORITE','FAILED'))
);
CREATE UNIQUE INDEX IF NOT EXISTS gaspol_awards_single_winner_unique ON gaspol_awards(year, award_type) WHERE award_type IN ('BEST','INNOVATIVE','FAVORITE');

CREATE TABLE IF NOT EXISTS gaspol_innovation_nominations (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE RESTRICT,
  score NUMERIC(8,2) NOT NULL CHECK (score BETWEEN 0 AND 100),
  status TEXT NOT NULL DEFAULT 'SUBMITTED',
  proposed_by INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  proposed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  kasi_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  kasi_notes TEXT NOT NULL DEFAULT '',
  decided_at TIMESTAMPTZ,
  revision INTEGER NOT NULL DEFAULT 1,
  UNIQUE(year, revision),
  CHECK (status IN ('SUBMITTED','APPROVED','RETURNED'))
);

CREATE TABLE IF NOT EXISTS gaspol_ranking_snapshots_v2 (
  id BIGSERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  snapshot_type TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 1,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE RESTRICT,
  rank_position INTEGER,
  cohort TEXT,
  score_total NUMERIC(8,2),
  maximum_score NUMERIC(8,2),
  payload JSONB NOT NULL,
  created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(year, snapshot_type, revision, school_id),
  CHECK (snapshot_type IN ('TOP_10','TOP_5','NOT_FAILED','BOTTOM_5','FINAL','AWARDING'))
);

-- +goose Down
DROP TABLE IF EXISTS gaspol_ranking_snapshots_v2;
DROP TABLE IF EXISTS gaspol_innovation_nominations;
DROP TABLE IF EXISTS gaspol_awards;
DROP TABLE IF EXISTS gaspol_like_snapshots;
DROP TABLE IF EXISTS gaspol_favorite_content;
DROP TABLE IF EXISTS gaspol_internal_votes;
DROP TABLE IF EXISTS gaspol_voting_sessions;
DROP TABLE IF EXISTS gaspol_nomination_submissions;
ALTER TABLE gaspol_result_locks DROP CONSTRAINT IF EXISTS gaspol_result_locks_ba_document_id_fkey;
DROP TABLE IF EXISTS gaspol_ba_documents;
DROP TABLE IF EXISTS gaspol_result_locks;
DROP TABLE IF EXISTS gaspol_cohorts;
DROP TABLE IF EXISTS gaspol_stage_assessments;
DROP TABLE IF EXISTS gaspol_stage_assignments;
DROP TABLE IF EXISTS gaspol_competition_configs;
ALTER TABLE gaspol_assessments
  DROP COLUMN IF EXISTS score_appeal,
  DROP COLUMN IF EXISTS score_nomination,
  DROP COLUMN IF EXISTS score_field_stage_2,
  DROP COLUMN IF EXISTS score_field_stage_1,
  DROP COLUMN IF EXISTS cabdin_validated_at,
  DROP COLUMN IF EXISTS cabdin_user_id,
  DROP COLUMN IF EXISTS koor_validated_at,
  DROP COLUMN IF EXISTS koor_user_id,
  DROP COLUMN IF EXISTS workflow_status,
  DROP COLUMN IF EXISTS stage_code;
