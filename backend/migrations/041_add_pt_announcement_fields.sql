-- +goose Up
-- +goose StatementBegin
ALTER TABLE outcome_pt_offers
  ADD COLUMN IF NOT EXISTS application_id BIGINT NULL REFERENCES student_higher_ed_applications(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS announcement_status TEXT NOT NULL DEFAULT 'DITERIMA',
  ADD COLUMN IF NOT EXISTS pilihan_ke SMALLINT NULL,
  ADD COLUMN IF NOT EXISTS tanggal_pengumuman DATE NULL,
  ADD COLUMN IF NOT EXISTS catatan TEXT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'outcome_pt_offers_announcement_status_check'
  ) THEN
    ALTER TABLE outcome_pt_offers
      ADD CONSTRAINT outcome_pt_offers_announcement_status_check
      CHECK (announcement_status IN ('DITERIMA', 'DITOLAK'));
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'outcome_pt_offers_pilihan_ke_check'
  ) THEN
    ALTER TABLE outcome_pt_offers
      ADD CONSTRAINT outcome_pt_offers_pilihan_ke_check
      CHECK (pilihan_ke IS NULL OR pilihan_ke BETWEEN 1 AND 3);
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS outcome_pt_offers_application_unique
  ON outcome_pt_offers (application_id);

UPDATE outcome_pt_offers
SET announcement_status = 'DITERIMA'
WHERE TRIM(COALESCE(announcement_status, '')) = '';

UPDATE outcome_pt_offers
SET tanggal_pengumuman = COALESCE(tanggal_pengumuman, COALESCE(updated_at, created_at)::date)
WHERE tanggal_pengumuman IS NULL;

WITH matched_apps AS (
  SELECT
    offer.id AS offer_id,
    app.id AS application_id,
    app.pilihan_ke,
    ROW_NUMBER() OVER (
      PARTITION BY offer.id
      ORDER BY app.pilihan_ke ASC, app.id ASC
    ) AS rn
  FROM outcome_pt_offers offer
  JOIN student_cohorts cohort
    ON cohort.school_id = offer.school_id
   AND cohort.nisn = offer.nisn
   AND cohort.tahun_lulus = offer.tahun_lulus
  JOIN student_higher_ed_applications app
    ON app.school_id = cohort.school_id
   AND app.cohort_id = cohort.id
   AND UPPER(TRIM(COALESCE(app.jalur_penerimaan::text, ''))) = UPPER(TRIM(COALESCE(offer.jalur, '')))
   AND TRIM(COALESCE(app.nama_perguruan_tinggi, '')) = TRIM(COALESCE(offer.kampus, ''))
   AND TRIM(COALESCE(app.jurusan_perguruan_tinggi, '')) = TRIM(COALESCE(offer.prodi, ''))
  WHERE offer.application_id IS NULL
)
UPDATE outcome_pt_offers offer
SET
  application_id = matched.application_id,
  pilihan_ke = COALESCE(offer.pilihan_ke, matched.pilihan_ke)
FROM matched_apps matched
WHERE matched.offer_id = offer.id
  AND matched.rn = 1;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS outcome_pt_offers_application_unique;

ALTER TABLE outcome_pt_offers
  DROP CONSTRAINT IF EXISTS outcome_pt_offers_pilihan_ke_check,
  DROP CONSTRAINT IF EXISTS outcome_pt_offers_announcement_status_check,
  DROP COLUMN IF EXISTS catatan,
  DROP COLUMN IF EXISTS tanggal_pengumuman,
  DROP COLUMN IF EXISTS pilihan_ke,
  DROP COLUMN IF EXISTS announcement_status,
  DROP COLUMN IF EXISTS application_id;
-- +goose StatementEnd
