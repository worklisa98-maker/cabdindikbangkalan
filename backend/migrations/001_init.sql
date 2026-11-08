-- +goose Up
CREATE TYPE role_type AS ENUM ('ADMIN', 'SCHOOL_ADMIN');
CREATE TYPE lomba_type AS ENUM ('LKSN', 'O2SN', 'FLS3N', 'OSN', 'PERGURUAN_TINGGI', 'LAINNYA');
CREATE TYPE mode_pelaksanaan_type AS ENUM ('DARING', 'LURING');
CREATE TYPE jenis_peserta_type AS ENUM ('PERSEORANGAN', 'TIM');
CREATE TYPE jalur_penerimaan_type AS ENUM ('SNBP', 'SNBT', 'MANDIRI', 'KEDINASAN', 'LAINNYA');
CREATE TYPE upload_type AS ENUM ('ACHIEVEMENT', 'ACCEPTANCE');
CREATE TYPE upload_status AS ENUM ('PENDING', 'PROCESSING', 'SUCCESS', 'FAILED');

CREATE TABLE data_sekolah (
  id SERIAL PRIMARY KEY,
  npsn_sekolah TEXT UNIQUE,
  nama_sekolah TEXT NOT NULL,
  bentuk_pendidikan TEXT,
  status_sekolah TEXT,
  tmt_akreditasi TEXT,
  akreditasi TEXT,
  jumlah_total_guru INTEGER NOT NULL DEFAULT 0,
  jumlah_guru_asn INTEGER NOT NULL DEFAULT 0,
  jumlah_guru_honorer INTEGER NOT NULL DEFAULT 0,
  jumlah_total_tendik INTEGER NOT NULL DEFAULT 0,
  jumlah_tendik_asn INTEGER NOT NULL DEFAULT 0,
  jumlah_tendik_honorer INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE prestasi_siswa (
  id_prestasi TEXT NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  kategori_prestasi TEXT,
  jenjang_pendidikan TEXT,
  type_lomba lomba_type NOT NULL DEFAULT 'LAINNYA',
  nama_lomba TEXT,
  peringkat_prestasi TEXT,
  tingkat_lomba TEXT,
  tahun_lomba TEXT,
  mode_pelaksanaan mode_pelaksanaan_type,
  penyelenggaraan_lomba TEXT,
  jenis_peserta jenis_peserta_type,
  nama_peserta TEXT,
  link_gdrive TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id_prestasi, school_id)
);

CREATE INDEX prestasi_siswa_school_year_idx ON prestasi_siswa (school_id, tahun_lomba);

CREATE TABLE siswa_perguruan_tinggi (
  id TEXT NOT NULL,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  nisn_siswa TEXT,
  nama_siswa TEXT NOT NULL,
  jenjang_pendidikan TEXT,
  rombel_siswa TEXT,
  tahun_lulus INTEGER,
  nama_perguruan_tinggi TEXT NOT NULL,
  jurusan_perguruan_tinggi TEXT,
  tahun_diterima INTEGER,
  jalur_penerimaan jalur_penerimaan_type NOT NULL DEFAULT 'LAINNYA',
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id, school_id, jalur_penerimaan)
);

CREATE INDEX siswa_pt_school_year_idx ON siswa_perguruan_tinggi (school_id, tahun_diterima);
CREATE UNIQUE INDEX siswa_pt_school_nisn_unique ON siswa_perguruan_tinggi (school_id, nisn_siswa) WHERE nisn_siswa IS NOT NULL;

CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role role_type NOT NULL,
  school_id INTEGER REFERENCES data_sekolah(id) ON DELETE SET NULL,
  npsn_instansi TEXT,
  nama_instansi TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE uploads (
  id SERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type upload_type NOT NULL,
  year INTEGER NOT NULL,
  storage_key TEXT NOT NULL,
  original_filename TEXT NOT NULL,
  status upload_status NOT NULL DEFAULT 'PENDING',
  error_log TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS uploads;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS siswa_perguruan_tinggi;
DROP TABLE IF EXISTS prestasi_siswa;
DROP TABLE IF EXISTS data_sekolah;
DROP TYPE IF EXISTS upload_status;
DROP TYPE IF EXISTS upload_type;
DROP TYPE IF EXISTS jalur_penerimaan_type;
DROP TYPE IF EXISTS jenis_peserta_type;
DROP TYPE IF EXISTS mode_pelaksanaan_type;
DROP TYPE IF EXISTS lomba_type;
DROP TYPE IF EXISTS role_type;
