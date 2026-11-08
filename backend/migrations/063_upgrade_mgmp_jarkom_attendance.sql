-- +goose Up
ALTER TABLE mgmp_groups
  ADD COLUMN IF NOT EXISTS organization_type TEXT NOT NULL DEFAULT 'MGMP'
  CHECK (organization_type IN ('MGMP','JARKOM'));
ALTER TABLE mgmp_registrations
  ADD COLUMN IF NOT EXISTS organization_type TEXT NOT NULL DEFAULT 'MGMP'
  CHECK (organization_type IN ('MGMP','JARKOM'));

DROP INDEX IF EXISTS mgmp_groups_identity_unique;
CREATE UNIQUE INDEX IF NOT EXISTS mgmp_groups_identity_unique
  ON mgmp_groups (organization_type, LOWER(TRIM(name)), LOWER(TRIM(subject)), LOWER(TRIM(region)));
CREATE INDEX IF NOT EXISTS mgmp_groups_type_status_idx ON mgmp_groups(organization_type,status);
CREATE INDEX IF NOT EXISTS mgmp_registrations_type_status_idx ON mgmp_registrations(organization_type,status,submitted_at DESC);

ALTER TABLE mgmp_members DROP CONSTRAINT IF EXISTS mgmp_members_member_status_check;
ALTER TABLE mgmp_members
  ADD CONSTRAINT mgmp_members_member_status_check
  CHECK (member_status IN ('AKTIF','NONAKTIF','DIKELUARKAN'));
ALTER TABLE mgmp_members
  ADD COLUMN IF NOT EXISTS deactivation_reason TEXT,
  ADD COLUMN IF NOT EXISTS deactivation_note TEXT,
  ADD COLUMN IF NOT EXISTS deactivated_at DATE,
  ADD COLUMN IF NOT EXISTS deactivated_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS mgmp_members_group_status_idx ON mgmp_members(group_id,member_status);

ALTER TABLE mgmp_activities
  ADD COLUMN IF NOT EXISTS active_member_count INTEGER NOT NULL DEFAULT 0 CHECK (active_member_count >= 0);

CREATE TABLE IF NOT EXISTS mgmp_activity_attendance (
  id BIGSERIAL PRIMARY KEY,
  activity_id BIGINT NOT NULL REFERENCES mgmp_activities(id) ON DELETE RESTRICT,
  member_id BIGINT NOT NULL REFERENCES mgmp_members(id) ON DELETE RESTRICT,
  member_name_snapshot TEXT NOT NULL,
  school_name_snapshot TEXT NOT NULL,
  attendance_status TEXT NOT NULL CHECK (attendance_status IN ('PRESENT','ABSENT')),
  absence_type TEXT CHECK (absence_type IN ('EXCUSED','UNEXCUSED')),
  absence_note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(activity_id,member_id),
  CHECK (
    (attendance_status='PRESENT' AND absence_type IS NULL AND absence_note IS NULL)
    OR (attendance_status='ABSENT' AND absence_type='UNEXCUSED' AND absence_note IS NULL)
    OR (attendance_status='ABSENT' AND absence_type='EXCUSED' AND NULLIF(TRIM(absence_note),'') IS NOT NULL)
  )
);
CREATE INDEX IF NOT EXISTS mgmp_attendance_activity_status_idx ON mgmp_activity_attendance(activity_id,attendance_status,absence_type);
CREATE INDEX IF NOT EXISTS mgmp_attendance_member_idx ON mgmp_activity_attendance(member_id,activity_id);

CREATE TABLE IF NOT EXISTS mgmp_member_notifications (
  id BIGSERIAL PRIMARY KEY,
  group_id BIGINT NOT NULL REFERENCES mgmp_groups(id) ON DELETE RESTRICT,
  member_id BIGINT NOT NULL REFERENCES mgmp_members(id) ON DELETE RESTRICT,
  event_type TEXT NOT NULL,
  reason TEXT NOT NULL,
  note TEXT,
  effective_date DATE NOT NULL,
  actor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS mgmp_member_notifications_group_idx ON mgmp_member_notifications(group_id,created_at DESC);

CREATE TABLE IF NOT EXISTS mgmp_member_notification_reads (
  notification_id BIGINT NOT NULL REFERENCES mgmp_member_notifications(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  acknowledged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(notification_id,user_id)
);

CREATE TABLE IF NOT EXISTS mgmp_member_imports (
  id BIGSERIAL PRIMARY KEY,
  group_id BIGINT NOT NULL REFERENCES mgmp_groups(id) ON DELETE RESTRICT,
  actor_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  total_rows INTEGER NOT NULL,
  success_rows INTEGER NOT NULL,
  rejected_rows INTEGER NOT NULL,
  results JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS mgmp_member_imports_group_created_idx ON mgmp_member_imports(group_id,created_at DESC);

-- Existing MGMP records remain MGMP. Legacy activities keep their historical
-- participant count; attendance is required for newly-created activities only.
UPDATE mgmp_groups SET organization_type='MGMP' WHERE organization_type IS NULL;
UPDATE mgmp_registrations SET organization_type='MGMP' WHERE organization_type IS NULL;

-- +goose Down
DROP TABLE IF EXISTS mgmp_member_imports;
DROP TABLE IF EXISTS mgmp_member_notification_reads;
DROP TABLE IF EXISTS mgmp_member_notifications;
DROP TABLE IF EXISTS mgmp_activity_attendance;
ALTER TABLE mgmp_activities DROP COLUMN IF EXISTS active_member_count;
DROP INDEX IF EXISTS mgmp_members_group_status_idx;
ALTER TABLE mgmp_members DROP COLUMN IF EXISTS deactivated_by_user_id;
ALTER TABLE mgmp_members DROP COLUMN IF EXISTS deactivated_at;
ALTER TABLE mgmp_members DROP COLUMN IF EXISTS deactivation_note;
ALTER TABLE mgmp_members DROP COLUMN IF EXISTS deactivation_reason;
ALTER TABLE mgmp_members DROP CONSTRAINT IF EXISTS mgmp_members_member_status_check;
ALTER TABLE mgmp_members ADD CONSTRAINT mgmp_members_member_status_check CHECK (member_status IN ('AKTIF','NONAKTIF'));
DROP INDEX IF EXISTS mgmp_registrations_type_status_idx;
DROP INDEX IF EXISTS mgmp_groups_type_status_idx;
DROP INDEX IF EXISTS mgmp_groups_identity_unique;
CREATE UNIQUE INDEX mgmp_groups_identity_unique ON mgmp_groups (LOWER(TRIM(name)),LOWER(TRIM(subject)),LOWER(TRIM(region)));
ALTER TABLE mgmp_registrations DROP COLUMN IF EXISTS organization_type;
ALTER TABLE mgmp_groups DROP COLUMN IF EXISTS organization_type;
