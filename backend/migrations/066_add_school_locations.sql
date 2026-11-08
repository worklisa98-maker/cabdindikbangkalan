-- +goose Up
-- Fail quickly if production traffic holds the school table; retry later.
SET LOCAL lock_timeout = '5s';
ALTER TABLE data_sekolah
  ADD COLUMN latitude DOUBLE PRECISION,
  ADD COLUMN longitude DOUBLE PRECISION,
  ADD COLUMN location_source TEXT NOT NULL DEFAULT '',
  ADD COLUMN location_verified BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN location_updated_at TIMESTAMPTZ,
  ADD COLUMN location_updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  ADD CONSTRAINT school_location_pair CHECK ((latitude IS NULL) = (longitude IS NULL)),
  ADD CONSTRAINT school_latitude_valid CHECK (latitude BETWEEN -90 AND 90),
  ADD CONSTRAINT school_longitude_valid CHECK (longitude BETWEEN -180 AND 180),
  ADD CONSTRAINT school_verified_location CHECK (NOT location_verified OR latitude IS NOT NULL);

-- +goose Down
-- Application rollback is supported: old versions ignore these additive columns.
-- Refuse a schema downgrade so Goose cannot mark retained columns as unapplied.
-- +goose StatementBegin
DO $$ BEGIN
  RAISE EXCEPTION 'Migration 066 preserves school locations; roll back the application only';
END $$;
-- +goose StatementEnd
