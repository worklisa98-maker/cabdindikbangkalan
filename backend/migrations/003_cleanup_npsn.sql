-- +goose Up
DELETE FROM data_sekolah
WHERE npsn_sekolah IS NULL
   OR TRIM(npsn_sekolah) = ''
   OR LENGTH(TRIM(npsn_sekolah)) < 5;

ALTER TABLE data_sekolah
ALTER COLUMN npsn_sekolah SET NOT NULL;

ALTER TABLE data_sekolah
ADD CONSTRAINT data_sekolah_npsn_min_length
CHECK (LENGTH(TRIM(npsn_sekolah)) >= 5);

-- +goose Down
ALTER TABLE data_sekolah
DROP CONSTRAINT IF EXISTS data_sekolah_npsn_min_length;

ALTER TABLE data_sekolah
ALTER COLUMN npsn_sekolah DROP NOT NULL;
