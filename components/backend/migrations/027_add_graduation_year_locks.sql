-- +goose Up
CREATE TABLE IF NOT EXISTS graduation_year_locks (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  is_closed BOOLEAN NOT NULL DEFAULT FALSE,
  closed_at TIMESTAMP,
  closed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS graduation_year_locks_school_year_unique
  ON graduation_year_locks (school_id, tahun_lulus);

CREATE INDEX IF NOT EXISTS graduation_year_locks_year_idx
  ON graduation_year_locks (tahun_lulus);

CREATE INDEX IF NOT EXISTS graduation_year_locks_closed_idx
  ON graduation_year_locks (is_closed);

-- +goose Down
DROP INDEX IF EXISTS graduation_year_locks_closed_idx;
DROP INDEX IF EXISTS graduation_year_locks_year_idx;
DROP INDEX IF EXISTS graduation_year_locks_school_year_unique;
DROP TABLE IF EXISTS graduation_year_locks;
