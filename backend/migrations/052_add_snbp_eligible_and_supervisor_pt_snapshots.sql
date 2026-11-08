-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS student_snbp_eligible_counts (
  id BIGSERIAL PRIMARY KEY,
  school_id INT NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INT NOT NULL,
  eligible_count INT NOT NULL DEFAULT 0,
  note TEXT NULL,
  created_by INT NULL,
  updated_by INT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (school_id, tahun_lulus),
  CHECK (eligible_count >= 0)
);

CREATE INDEX IF NOT EXISTS student_snbp_eligible_counts_year_idx
  ON student_snbp_eligible_counts (tahun_lulus);

CREATE TABLE IF NOT EXISTS supervisor_pt_acceptance_snapshots (
  id BIGSERIAL PRIMARY KEY,
  supervisor_user_id INT NOT NULL,
  school_id INT NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INT NOT NULL,
  snbp_count INT NOT NULL DEFAULT 0,
  snbt_count INT NOT NULL DEFAULT 0,
  mandiri_count INT NOT NULL DEFAULT 0,
  kedinasan_count INT NOT NULL DEFAULT 0,
  lainnya_count INT NOT NULL DEFAULT 0,
  note TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (supervisor_user_id, school_id, tahun_lulus),
  CHECK (snbp_count >= 0),
  CHECK (snbt_count >= 0),
  CHECK (mandiri_count >= 0),
  CHECK (kedinasan_count >= 0),
  CHECK (lainnya_count >= 0)
);

CREATE INDEX IF NOT EXISTS supervisor_pt_acceptance_snapshots_school_year_idx
  ON supervisor_pt_acceptance_snapshots (school_id, tahun_lulus);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS supervisor_pt_acceptance_snapshots;
DROP TABLE IF EXISTS student_snbp_eligible_counts;
-- +goose StatementEnd
