-- +goose Up
ALTER TABLE IF EXISTS student_outcomes
  ADD COLUMN IF NOT EXISTS keterangan TEXT;

ALTER TABLE IF EXISTS outcome_work_details
  ADD COLUMN IF NOT EXISTS bidang_perusahaan TEXT;

ALTER TABLE IF EXISTS outcome_business_details
  ADD COLUMN IF NOT EXISTS kategori_usaha TEXT,
  ADD COLUMN IF NOT EXISTS produk_jasa TEXT,
  ADD COLUMN IF NOT EXISTS lokasi_usaha TEXT,
  ADD COLUMN IF NOT EXISTS keterangan TEXT;

-- +goose Down
ALTER TABLE IF EXISTS outcome_business_details
  DROP COLUMN IF EXISTS keterangan,
  DROP COLUMN IF EXISTS lokasi_usaha,
  DROP COLUMN IF EXISTS produk_jasa,
  DROP COLUMN IF EXISTS kategori_usaha;

ALTER TABLE IF EXISTS outcome_work_details
  DROP COLUMN IF EXISTS bidang_perusahaan;

ALTER TABLE IF EXISTS student_outcomes
  DROP COLUMN IF EXISTS keterangan;
