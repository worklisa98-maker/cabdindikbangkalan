-- +goose Up
ALTER TABLE student_ukk
  ADD COLUMN IF NOT EXISTS nilai_akhir DOUBLE PRECISION,
  ADD COLUMN IF NOT EXISTS predikat TEXT,
  ADD COLUMN IF NOT EXISTS nomor_sertifikat TEXT;

ALTER TABLE student_ukk_assessors
  ADD COLUMN IF NOT EXISTS instansi TEXT;

-- +goose Down
ALTER TABLE student_ukk_assessors
  DROP COLUMN IF EXISTS instansi;

ALTER TABLE student_ukk
  DROP COLUMN IF EXISTS nomor_sertifikat,
  DROP COLUMN IF EXISTS predikat,
  DROP COLUMN IF EXISTS nilai_akhir;
