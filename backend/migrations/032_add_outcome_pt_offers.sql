-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS outcome_pt_offers (
  id BIGSERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL,
  tahun_lulus INTEGER NOT NULL,
  nisn TEXT NOT NULL,
  jalur TEXT NOT NULL,
  kampus TEXT NOT NULL,
  prodi TEXT NOT NULL,
  tahun_diterima INTEGER NULL,
  source TEXT NOT NULL DEFAULT 'OFFER_IMPORT',
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT outcome_pt_offers_unique_offer UNIQUE (school_id, tahun_lulus, nisn, jalur, kampus, prodi)
);

CREATE INDEX IF NOT EXISTS outcome_pt_offers_school_year_nisn_idx
  ON outcome_pt_offers (school_id, tahun_lulus, nisn);

CREATE INDEX IF NOT EXISTS outcome_pt_offers_school_year_idx
  ON outcome_pt_offers (school_id, tahun_lulus);

DO $$
BEGIN
  IF to_regclass('public.student_higher_ed_acceptance_offers') IS NOT NULL
     AND to_regclass('public.student_cohorts') IS NOT NULL THEN
    INSERT INTO outcome_pt_offers (
      school_id,
      tahun_lulus,
      nisn,
      jalur,
      kampus,
      prodi,
      tahun_diterima,
      source
    )
    SELECT
      o.school_id,
      sc.tahun_lulus,
      sc.nisn,
      CASE
        WHEN TRIM(COALESCE(o.jalur_penerimaan::text, '')) = '' THEN 'LAINNYA'
        ELSE TRIM(o.jalur_penerimaan::text)
      END AS jalur,
      TRIM(COALESCE(o.nama_perguruan_tinggi, '')) AS kampus,
      TRIM(COALESCE(o.jurusan_perguruan_tinggi, '')) AS prodi,
      o.tahun_diterima,
      'MIGRATION'
    FROM student_higher_ed_acceptance_offers o
    JOIN student_cohorts sc
      ON sc.id = o.cohort_id
     AND sc.school_id = o.school_id
    WHERE TRIM(COALESCE(sc.nisn, '')) <> ''
      AND sc.tahun_lulus > 0
      AND TRIM(COALESCE(o.nama_perguruan_tinggi, '')) <> ''
      AND TRIM(COALESCE(o.jurusan_perguruan_tinggi, '')) <> ''
    ON CONFLICT (school_id, tahun_lulus, nisn, jalur, kampus, prodi) DO NOTHING;
  END IF;

  IF to_regclass('public.outcome_pt_details') IS NOT NULL THEN
    INSERT INTO outcome_pt_offers (
      school_id,
      tahun_lulus,
      nisn,
      jalur,
      kampus,
      prodi,
      tahun_diterima,
      source
    )
    SELECT
      pt.school_id,
      pt.tahun_lulus,
      pt.nisn,
      CASE
        WHEN TRIM(COALESCE(pt.jalur::text, '')) = '' THEN 'LAINNYA'
        ELSE TRIM(pt.jalur::text)
      END AS jalur,
      COALESCE(NULLIF(TRIM(pt.kampus), ''), 'Tidak Diisi') AS kampus,
      COALESCE(NULLIF(TRIM(pt.prodi), ''), 'Tidak Diisi') AS prodi,
      pt.tahun_diterima,
      'MIGRATION_FINAL_DETAIL'
    FROM outcome_pt_details pt
    WHERE TRIM(COALESCE(pt.nisn, '')) <> ''
      AND pt.tahun_lulus > 0
    ON CONFLICT (school_id, tahun_lulus, nisn, jalur, kampus, prodi) DO NOTHING;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS outcome_pt_offers;
-- +goose StatementEnd
