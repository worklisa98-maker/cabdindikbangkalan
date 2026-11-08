-- +goose Up
CREATE TABLE IF NOT EXISTS competition_poster_entries (
  id SERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  nisn TEXT NOT NULL,
  nama_siswa TEXT NOT NULL,
  rombel TEXT NOT NULL,
  link_surat_pernyataan_orisinalitas TEXT NOT NULL,
  judul_poster TEXT NOT NULL,
  hasil_karya_path TEXT NOT NULL,
  hasil_karya_thumbnail_path TEXT NOT NULL,
  hasil_karya_mime_type TEXT NOT NULL,
  hasil_karya_size BIGINT NOT NULL,
  created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS competition_poster_entries_school_id_idx
  ON competition_poster_entries (school_id);

CREATE INDEX IF NOT EXISTS competition_poster_entries_created_at_idx
  ON competition_poster_entries (created_at DESC);

CREATE INDEX IF NOT EXISTS competition_poster_entries_deleted_at_idx
  ON competition_poster_entries (deleted_at);

CREATE INDEX IF NOT EXISTS competition_poster_entries_school_created_idx
  ON competition_poster_entries (school_id, created_at DESC)
  WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS competition_poster_entries_school_created_idx;
DROP INDEX IF EXISTS competition_poster_entries_deleted_at_idx;
DROP INDEX IF EXISTS competition_poster_entries_created_at_idx;
DROP INDEX IF EXISTS competition_poster_entries_school_id_idx;
DROP TABLE IF EXISTS competition_poster_entries;
