-- +goose Up
CREATE TABLE IF NOT EXISTS competition_poster_winners (
  id SERIAL PRIMARY KEY,
  rank INTEGER NOT NULL,
  entry_id INTEGER NOT NULL REFERENCES competition_poster_entries(id) ON DELETE RESTRICT,
  is_published BOOLEAN NOT NULL DEFAULT false,
  created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT competition_poster_winners_rank_check CHECK (rank IN (1, 2, 3)),
  CONSTRAINT competition_poster_winners_rank_unique UNIQUE (rank),
  CONSTRAINT competition_poster_winners_entry_unique UNIQUE (entry_id)
);

CREATE INDEX IF NOT EXISTS competition_poster_winners_entry_id_idx
  ON competition_poster_winners (entry_id);

-- +goose Down
DROP INDEX IF EXISTS competition_poster_winners_entry_id_idx;
DROP TABLE IF EXISTS competition_poster_winners;
