-- +goose Up
ALTER TABLE gaspol_voting_sessions ADD COLUMN managed BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE gaspol_voting_sessions DROP CONSTRAINT gaspol_voting_sessions_status_check;
ALTER TABLE gaspol_voting_sessions ADD CONSTRAINT gaspol_voting_sessions_status_check CHECK(status IN ('DRAFT','OPEN','CLOSED','CANCELLED','ESTABLISHED'));
CREATE UNIQUE INDEX gaspol_one_managed_session_year ON gaspol_voting_sessions(year) WHERE managed AND status<>'CANCELLED';
CREATE TABLE gaspol_voting_candidates (
 session_id BIGINT NOT NULL REFERENCES gaspol_voting_sessions(id), school_id INTEGER NOT NULL REFERENCES data_sekolah(id),
 short_code TEXT NOT NULL, PRIMARY KEY(session_id,school_id), UNIQUE(session_id,short_code)
);
CREATE TABLE gaspol_voting_voters (
 session_id BIGINT NOT NULL REFERENCES gaspol_voting_sessions(id), user_id INTEGER NOT NULL REFERENCES users(id),
 present BOOLEAN NOT NULL DEFAULT FALSE, is_active BOOLEAN NOT NULL DEFAULT TRUE,
 assigned_by INTEGER REFERENCES users(id), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY(session_id,user_id)
);
CREATE TABLE gaspol_like_verifiers (
 year INTEGER NOT NULL CHECK(year>=2024), school_id INTEGER NOT NULL REFERENCES data_sekolah(id),
 user_id INTEGER NOT NULL REFERENCES users(id), is_active BOOLEAN NOT NULL DEFAULT TRUE,
 assigned_by INTEGER REFERENCES users(id), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY(year,school_id,user_id)
);
ALTER TABLE gaspol_like_snapshots ADD COLUMN review_status TEXT CHECK(review_status IN ('DRAFT','SUBMITTED','RETURNED','VERIFIED')),
 ADD COLUMN reviewed_by INTEGER REFERENCES users(id), ADD COLUMN reviewed_at TIMESTAMPTZ,
 ADD COLUMN review_notes TEXT NOT NULL DEFAULT '', ADD COLUMN content_url TEXT NOT NULL DEFAULT '',
 ADD COLUMN observation_at TIMESTAMPTZ;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Forward-only GASPOL voting administration; preserve ballots and audit history'; END $$;
-- +goose StatementEnd
