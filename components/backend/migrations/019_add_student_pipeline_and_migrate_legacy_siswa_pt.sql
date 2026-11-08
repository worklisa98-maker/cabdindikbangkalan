-- +goose Up
CREATE TABLE IF NOT EXISTS student_cohorts (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  nisn TEXT NOT NULL,
  nama_siswa TEXT NOT NULL,
  rombel_siswa TEXT NOT NULL,
  tahun_lulus INTEGER NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS student_cohorts_school_nisn_year_unique
  ON student_cohorts (school_id, nisn, tahun_lulus);
CREATE INDEX IF NOT EXISTS student_cohorts_school_year_idx
  ON student_cohorts (school_id, tahun_lulus);

CREATE TABLE IF NOT EXISTS student_higher_ed_applications (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  cohort_id BIGINT NOT NULL REFERENCES student_cohorts(id) ON DELETE CASCADE,
  pilihan_ke SMALLINT NOT NULL CHECK (pilihan_ke BETWEEN 1 AND 3),
  jalur_penerimaan jalur_penerimaan_type NOT NULL DEFAULT 'LAINNYA',
  nama_perguruan_tinggi TEXT NOT NULL,
  jurusan_perguruan_tinggi TEXT NOT NULL,
  source TEXT NOT NULL DEFAULT 'APPLICATION_IMPORT',
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS student_higher_ed_applications_unique_choice
  ON student_higher_ed_applications (school_id, cohort_id, pilihan_ke);
CREATE INDEX IF NOT EXISTS student_higher_ed_applications_cohort_idx
  ON student_higher_ed_applications (cohort_id);

CREATE TABLE IF NOT EXISTS student_higher_ed_acceptance_offers (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  cohort_id BIGINT NOT NULL REFERENCES student_cohorts(id) ON DELETE CASCADE,
  jalur_penerimaan jalur_penerimaan_type NOT NULL DEFAULT 'LAINNYA',
  nama_perguruan_tinggi TEXT NOT NULL,
  jurusan_perguruan_tinggi TEXT NOT NULL,
  tahun_diterima INTEGER,
  source TEXT NOT NULL DEFAULT 'OFFER_IMPORT',
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS student_higher_ed_acceptance_offers_school_cohort_idx
  ON student_higher_ed_acceptance_offers (school_id, cohort_id);
CREATE INDEX IF NOT EXISTS student_higher_ed_acceptance_offers_school_year_idx
  ON student_higher_ed_acceptance_offers (school_id, tahun_diterima);

CREATE TABLE IF NOT EXISTS student_final_outcomes (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  cohort_id BIGINT NOT NULL REFERENCES student_cohorts(id) ON DELETE CASCADE,
  offer_id BIGINT REFERENCES student_higher_ed_acceptance_offers(id) ON DELETE SET NULL,
  final_type TEXT NOT NULL CHECK (final_type IN ('HIGHER_ED')),
  finalized_at TIMESTAMP NOT NULL DEFAULT NOW(),
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS student_final_outcomes_school_cohort_unique
  ON student_final_outcomes (school_id, cohort_id);

CREATE TABLE IF NOT EXISTS student_ukk (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  cohort_id BIGINT NOT NULL REFERENCES student_cohorts(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  jenis_ukk TEXT NOT NULL CHECK (jenis_ukk IN ('UTAMA', 'TAMBAHAN')),
  skema_ukk TEXT,
  hasil TEXT,
  tanggal_ukk DATE,
  source TEXT NOT NULL DEFAULT 'UKK_IMPORT',
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS student_ukk_cohort_idx
  ON student_ukk (cohort_id);
CREATE UNIQUE INDEX IF NOT EXISTS student_ukk_utama_unique
  ON student_ukk (school_id, cohort_id, tahun_lulus)
  WHERE jenis_ukk = 'UTAMA';

CREATE TABLE IF NOT EXISTS student_ukk_assessors (
  id BIGSERIAL PRIMARY KEY,
  student_ukk_id BIGINT NOT NULL REFERENCES student_ukk(id) ON DELETE CASCADE,
  nama_assessor TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS student_ukk_assessors_ukk_idx
  ON student_ukk_assessors (student_ukk_id);

CREATE TABLE IF NOT EXISTS siswa_import_reports (
  id TEXT PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  module TEXT NOT NULL,
  filename TEXT NOT NULL,
  content BYTEA NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS siswa_import_reports_school_created_idx
  ON siswa_import_reports (school_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS siswa_import_reports;
DROP TABLE IF EXISTS student_ukk_assessors;
DROP TABLE IF EXISTS student_ukk;
DROP TABLE IF EXISTS student_final_outcomes;
DROP TABLE IF EXISTS student_higher_ed_acceptance_offers;
DROP TABLE IF EXISTS student_higher_ed_applications;
DROP TABLE IF EXISTS student_cohorts;
