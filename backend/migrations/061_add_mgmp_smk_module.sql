-- +goose Up
ALTER TYPE role_type ADD VALUE IF NOT EXISTS 'USER_MGMP';

CREATE TYPE mgmp_group_status AS ENUM ('PENDING', 'ACTIVE', 'INACTIVE', 'REJECTED');
CREATE TYPE mgmp_registration_status AS ENUM ('PENDING', 'REVISION_REQUIRED', 'APPROVED', 'REJECTED');

CREATE TABLE mgmp_groups (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  subject TEXT NOT NULL,
  level TEXT NOT NULL DEFAULT 'SMK' CHECK (level = 'SMK'),
  region TEXT NOT NULL,
  management_period TEXT,
  status mgmp_group_status NOT NULL DEFAULT 'PENDING',
  delegated_admin_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  activated_at TIMESTAMPTZ,
  deactivated_at TIMESTAMPTZ,
  deactivation_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX mgmp_groups_identity_unique ON mgmp_groups (LOWER(TRIM(name)), LOWER(TRIM(subject)), LOWER(TRIM(region)));
CREATE INDEX mgmp_groups_status_idx ON mgmp_groups(status);
CREATE INDEX mgmp_groups_delegate_idx ON mgmp_groups(delegated_admin_id);

CREATE TABLE mgmp_registrations (
  id BIGSERIAL PRIMARY KEY,
  registration_code TEXT NOT NULL UNIQUE,
  group_id BIGINT REFERENCES mgmp_groups(id) ON DELETE SET NULL,
  name TEXT NOT NULL,
  subject TEXT NOT NULL,
  region TEXT NOT NULL,
  management_period TEXT,
  status mgmp_registration_status NOT NULL DEFAULT 'PENDING',
  delegated_admin_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  revision_note TEXT,
  rejection_reason TEXT,
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX mgmp_registrations_status_idx ON mgmp_registrations(status, submitted_at DESC);

CREATE TABLE mgmp_members (
  id BIGSERIAL PRIMARY KEY,
  registration_id BIGINT REFERENCES mgmp_registrations(id) ON DELETE CASCADE,
  group_id BIGINT REFERENCES mgmp_groups(id) ON DELETE CASCADE,
  full_name TEXT NOT NULL,
  identity_number TEXT,
  school_id INTEGER REFERENCES data_sekolah(id) ON DELETE SET NULL,
  school_name TEXT NOT NULL,
  position TEXT NOT NULL CHECK (position IN ('KETUA','SEKRETARIS_WAKIL','BENDAHARA','ANGGOTA','LAINNYA')),
  email TEXT NOT NULL,
  whatsapp TEXT NOT NULL,
  member_status TEXT NOT NULL DEFAULT 'AKTIF' CHECK (member_status IN ('AKTIF','NONAKTIF')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (registration_id IS NOT NULL OR group_id IS NOT NULL)
);
CREATE UNIQUE INDEX mgmp_members_registration_email_unique ON mgmp_members(registration_id, LOWER(email)) WHERE registration_id IS NOT NULL;
CREATE UNIQUE INDEX mgmp_members_group_email_unique ON mgmp_members(group_id, LOWER(email)) WHERE group_id IS NOT NULL;

CREATE TABLE mgmp_user_links (
  id BIGSERIAL PRIMARY KEY,
  group_id BIGINT NOT NULL REFERENCES mgmp_groups(id) ON DELETE RESTRICT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  member_id BIGINT REFERENCES mgmp_members(id) ON DELETE SET NULL,
  position TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deactivated_at TIMESTAMPTZ,
  UNIQUE(group_id, user_id)
);
CREATE INDEX mgmp_user_links_user_active_idx ON mgmp_user_links(user_id, is_active);

CREATE TABLE mgmp_activities (
  id BIGSERIAL PRIMARY KEY,
  group_id BIGINT NOT NULL REFERENCES mgmp_groups(id) ON DELETE RESTRICT,
  title TEXT NOT NULL,
  activity_date DATE NOT NULL,
  venue TEXT NOT NULL,
  description TEXT NOT NULL,
  participant_count INTEGER NOT NULL CHECK (participant_count >= 0),
  attendance_url TEXT NOT NULL,
  documentation_url TEXT NOT NULL,
  notes TEXT,
  created_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  updated_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX mgmp_activities_group_date_idx ON mgmp_activities(group_id, activity_date DESC);

CREATE TABLE mgmp_monthly_statuses (
  id BIGSERIAL PRIMARY KEY,
  group_id BIGINT NOT NULL REFERENCES mgmp_groups(id) ON DELETE RESTRICT,
  year INTEGER NOT NULL CHECK (year BETWEEN 2000 AND 2200),
  month INTEGER NOT NULL CHECK (month BETWEEN 1 AND 12),
  no_agenda_reason TEXT NOT NULL,
  created_by_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(group_id, year, month)
);

CREATE TABLE mgmp_history (
  id BIGSERIAL PRIMARY KEY,
  group_id BIGINT REFERENCES mgmp_groups(id) ON DELETE RESTRICT,
  registration_id BIGINT REFERENCES mgmp_registrations(id) ON DELETE RESTRICT,
  activity_id BIGINT REFERENCES mgmp_activities(id) ON DELETE SET NULL,
  actor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL,
  old_status TEXT,
  new_status TEXT,
  reason TEXT,
  details JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX mgmp_history_group_created_idx ON mgmp_history(group_id, created_at DESC);
CREATE INDEX mgmp_history_registration_created_idx ON mgmp_history(registration_id, created_at DESC);

CREATE TABLE mgmp_settings (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL,
  updated_by_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO mgmp_settings(key, value) VALUES
  ('active_period', to_jsonb(EXTRACT(YEAR FROM CURRENT_DATE)::INT)),
  ('registration_enabled', 'true'::jsonb),
  ('missing_report_warning_months', '3'::jsonb),
  ('positions', '["KETUA","SEKRETARIS_WAKIL","BENDAHARA","ANGGOTA","LAINNYA"]'::jsonb),
  ('deactivation_reasons', '["Tidak aktif beberapa bulan","Kepengurusan tidak berjalan","Tidak ada laporan kegiatan","Keputusan pimpinan","MGMP dibubarkan/bergabung","Lainnya"]'::jsonb)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS mgmp_settings;
DROP TABLE IF EXISTS mgmp_history;
DROP TABLE IF EXISTS mgmp_monthly_statuses;
DROP TABLE IF EXISTS mgmp_activities;
DROP TABLE IF EXISTS mgmp_user_links;
DROP TABLE IF EXISTS mgmp_members;
DROP TABLE IF EXISTS mgmp_registrations;
DROP TABLE IF EXISTS mgmp_groups;
DROP TYPE IF EXISTS mgmp_registration_status;
DROP TYPE IF EXISTS mgmp_group_status;
