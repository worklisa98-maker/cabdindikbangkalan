-- +goose Up
UPDATE prestasi_siswa
SET dedupe_key = md5(CONCAT_WS('|',
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(tingkat_lomba, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(tahun_lomba, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_lomba, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(peringkat_prestasi, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(mode_pelaksanaan::text, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(penyelenggaraan_lomba, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(jenis_peserta::text, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(jenis_lomba, '')), '\s+', ' ', 'g'))
));

UPDATE siswa_perguruan_tinggi
SET dedupe_key = md5(CONCAT_WS('|',
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(nisn_siswa, '')), '\s+', ' ', 'g')),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_siswa, '')), '\s+', ' ', 'g')),
  COALESCE(COALESCE(tahun_lulus, tahun_diterima)::text, '')
))
WHERE TRIM(COALESCE(nisn_siswa, '')) <> ''
  AND TRIM(COALESCE(nama_siswa, '')) <> ''
  AND COALESCE(tahun_lulus, tahun_diterima, 0) <> 0;

-- +goose Down
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
));

UPDATE siswa_perguruan_tinggi
SET dedupe_key = md5(CONCAT_WS('|',
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(nisn_siswa, '')), '\s+', ' ', 'g')),
  COALESCE(tahun_diterima::text, ''),
  UPPER(REGEXP_REPLACE(TRIM(COALESCE(nama_perguruan_tinggi, '')), '\s+', ' ', 'g'))
))
WHERE TRIM(COALESCE(nisn_siswa, '')) <> ''
  AND COALESCE(tahun_diterima, 0) <> 0
  AND TRIM(COALESCE(nama_perguruan_tinggi, '')) <> '';
