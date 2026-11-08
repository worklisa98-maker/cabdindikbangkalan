-- +goose Up
CREATE TABLE gaspol_rubric_versions (
 id BIGSERIAL PRIMARY KEY, year INTEGER NOT NULL CHECK(year>=2024),
 stage TEXT NOT NULL CHECK(stage IN ('NOT_FAILED_NOMINATION','INNOVATION')),
 version INTEGER NOT NULL, title TEXT NOT NULL, guidance TEXT NOT NULL DEFAULT '',
 provisional BOOLEAN NOT NULL DEFAULT TRUE, indicators JSONB NOT NULL,
 status TEXT NOT NULL DEFAULT 'DRAFT' CHECK(status IN ('DRAFT','ACTIVE','ARCHIVED')),
 created_by INTEGER REFERENCES users(id), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 activated_at TIMESTAMPTZ, UNIQUE(year,stage,version)
);
CREATE UNIQUE INDEX gaspol_rubric_active ON gaspol_rubric_versions(year,stage) WHERE status='ACTIVE';
CREATE TABLE gaspol_rubric_bindings (
 year INTEGER NOT NULL, stage TEXT NOT NULL, school_id INTEGER NOT NULL REFERENCES data_sekolah(id),
 rubric_id BIGINT NOT NULL REFERENCES gaspol_rubric_versions(id), bound_by INTEGER REFERENCES users(id),
 bound_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY(year,stage,school_id)
);
CREATE TABLE gaspol_innovation_submissions (
 id BIGSERIAL PRIMARY KEY, year INTEGER NOT NULL CHECK(year>=2024), school_id INTEGER NOT NULL REFERENCES data_sekolah(id),
 payload JSONB NOT NULL, status TEXT NOT NULL CHECK(status IN ('DRAFT','SUBMITTED','RETURNED')),
 submitted_by INTEGER REFERENCES users(id), submitted_at TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(year,school_id)
);
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Forward-only GASPOL rubric history; restore a reviewed backup to downgrade'; END $$;
-- +goose StatementEnd
