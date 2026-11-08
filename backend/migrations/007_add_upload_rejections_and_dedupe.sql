-- +goose Up
ALTER TABLE prestasi_siswa
ADD COLUMN IF NOT EXISTS dedupe_key TEXT;

ALTER TABLE siswa_perguruan_tinggi
ADD COLUMN IF NOT EXISTS dedupe_key TEXT;

CREATE TABLE IF NOT EXISTS upload_rejections (
  id SERIAL PRIMARY KEY,
  upload_id INTEGER NOT NULL REFERENCES uploads(id) ON DELETE CASCADE,
  row_number INTEGER NOT NULL,
  reason TEXT NOT NULL,
  notes TEXT,
  dedupe_key TEXT,
  search_hint TEXT,
  data JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

UPDATE prestasi_siswa
SET dedupe_key = md5(CONCAT_WS('|',
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(kategori_prestasi, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(tahun_lomba, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_lomba, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(tingkat_lomba, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(peringkat_prestasi, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(jenis_peserta::text, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_peserta, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(mode_pelaksanaan::text, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(penyelenggaraan_lomba, '')), '\s+', ' ', 'g'))
))
WHERE dedupe_key IS NULL OR TRIM(dedupe_key) = '';

UPDATE siswa_perguruan_tinggi
SET dedupe_key = CASE
  WHEN TRIM(COALESCE(nisn_siswa, '')) <> '' THEN md5(CONCAT_WS('|',
    'NISN',
    UPPER(REGEXP_REPLACE(TRIM(COALESCE(nisn_siswa, '')), '\s+', ' ', 'g'))
  ))
  ELSE md5(CONCAT_WS('|',
    UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_siswa, '')), '\s+', ' ', 'g')),
    COALESCE(tahun_diterima::text, ''),
    UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_perguruan_tinggi, '')), '\s+', ' ', 'g')),
    UPPER(REGEXP_REPLACE(TRIM(COALESCE(jurusan_perguruan_tinggi, '')), '\s+', ' ', 'g')),
    UPPER(REGEXP_REPLACE(TRIM(COALESCE(jalur_penerimaan::text, '')), '\s+', ' ', 'g'))
  ))
END
WHERE dedupe_key IS NULL OR TRIM(dedupe_key) = '';

CREATE UNIQUE INDEX IF NOT EXISTS prestasi_siswa_school_dedupe_key_idx
  ON prestasi_siswa (school_id, dedupe_key);

CREATE UNIQUE INDEX IF NOT EXISTS siswa_pt_school_dedupe_key_idx
  ON siswa_perguruan_tinggi (school_id, dedupe_key);

-- +goose Down
DROP INDEX IF EXISTS prestasi_siswa_school_dedupe_key_idx;
DROP INDEX IF EXISTS siswa_pt_school_dedupe_key_idx;
DROP TABLE IF EXISTS upload_rejections;
ALTER TABLE prestasi_siswa DROP COLUMN IF EXISTS dedupe_key;
ALTER TABLE siswa_perguruan_tinggi DROP COLUMN IF EXISTS dedupe_key;
