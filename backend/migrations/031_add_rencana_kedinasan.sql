-- +goose Up
CREATE TABLE IF NOT EXISTS rencana_kedinasan (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  cohort_id BIGINT NOT NULL REFERENCES student_cohorts(id) ON DELETE CASCADE,
  jenis TEXT NOT NULL CHECK (jenis IN ('SEKOLAH_KEDINASAN', 'TNI', 'POLRI')),
  matra TEXT CHECK (matra IS NULL OR matra IN ('AD', 'AL', 'AU')),
  jenis_pendidikan TEXT CHECK (jenis_pendidikan IS NULL OR jenis_pendidikan IN ('AKADEMI', 'BINTARA', 'TAMTAMA')),
  source TEXT NOT NULL DEFAULT 'MANUAL',
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT rencana_kedinasan_unique_cohort UNIQUE (school_id, cohort_id)
);

CREATE INDEX IF NOT EXISTS rencana_kedinasan_school_cohort_idx
  ON rencana_kedinasan (school_id, cohort_id);

-- +goose Down
DROP TABLE IF EXISTS rencana_kedinasan;
