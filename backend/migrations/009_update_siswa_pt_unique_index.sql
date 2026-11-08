-- +goose Up
DROP INDEX IF EXISTS siswa_pt_school_nisn_unique;

CREATE UNIQUE INDEX IF NOT EXISTS siswa_pt_school_nisn_year_pt_unique
  ON siswa_perguruan_tinggi (school_id, nisn_siswa, tahun_diterima, nama_perguruan_tinggi)
  WHERE nisn_siswa IS NOT NULL AND TRIM(nisn_siswa) <> '';

UPDATE siswa_perguruan_tinggi
SET dedupe_key = md5(CONCAT_WS('|',
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(nisn_siswa, '')), '\s+', ' ', 'g')),
  COALESCE(tahun_diterima::text, ''),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_perguruan_tinggi, '')), '\s+', ' ', 'g'))
))
WHERE TRIM(COALESCE(nisn_siswa, '')) <> ''
  AND COALESCE(tahun_diterima, 0) <> 0
  AND TRIM(COALESCE(nama_perguruan_tinggi, '')) <> '';

-- +goose Down
DROP INDEX IF EXISTS siswa_pt_school_nisn_year_pt_unique;

CREATE UNIQUE INDEX IF NOT EXISTS siswa_pt_school_nisn_unique
  ON siswa_perguruan_tinggi (school_id, nisn_siswa)
  WHERE nisn_siswa IS NOT NULL;

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
