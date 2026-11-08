-- +goose Up
ALTER TABLE IF EXISTS class12_students
ADD COLUMN IF NOT EXISTS tingkat_akhir SMALLINT NOT NULL DEFAULT 12;

UPDATE class12_students
SET tingkat_akhir = CASE
    WHEN UPPER(TRIM(rombel)) LIKE 'XIII%' THEN 13
    ELSE 12
END;

-- +goose Down
ALTER TABLE IF EXISTS class12_students
DROP COLUMN IF EXISTS tingkat_akhir;
