-- +goose Up
ALTER TABLE ref_kampus
  ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'seed_csv',
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE ref_kampus
SET is_active = COALESCE(is_active, TRUE),
    source = COALESCE(NULLIF(TRIM(source), ''), 'seed_csv'),
    updated_at = COALESCE(updated_at, created_at, NOW());

CREATE INDEX IF NOT EXISTS ref_kampus_active_nama_idx
  ON ref_kampus (is_active, nama);

-- +goose Down
DROP INDEX IF EXISTS ref_kampus_active_nama_idx;

ALTER TABLE ref_kampus
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS source,
  DROP COLUMN IF EXISTS is_active;
