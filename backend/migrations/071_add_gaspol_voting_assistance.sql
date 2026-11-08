-- +goose Up
CREATE TABLE gaspol_voting_operators (
 session_id BIGINT NOT NULL REFERENCES gaspol_voting_sessions(id), user_id INTEGER NOT NULL REFERENCES users(id),
 assigned_by INTEGER REFERENCES users(id), is_active BOOLEAN NOT NULL DEFAULT TRUE, PRIMARY KEY(session_id,user_id)
);
CREATE TABLE gaspol_vote_assistance (
 id BIGSERIAL PRIMARY KEY, session_id BIGINT NOT NULL REFERENCES gaspol_voting_sessions(id),
 voter_user_id INTEGER NOT NULL REFERENCES users(id), school_id INTEGER NOT NULL REFERENCES data_sekolah(id),
 reason TEXT NOT NULL, consent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 operator_user_id INTEGER REFERENCES users(id), executed_at TIMESTAMPTZ,
 UNIQUE(session_id,voter_user_id)
);
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Forward-only; preserve operator consent and vote audit history'; END $$;
-- +goose StatementEnd
