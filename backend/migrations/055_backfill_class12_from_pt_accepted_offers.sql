-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
  IF to_regclass('public.outcome_pt_offers') IS NOT NULL
     AND to_regclass('public.class12_students') IS NOT NULL
     AND to_regclass('public.student_higher_ed_applications') IS NOT NULL
     AND to_regclass('public.student_cohorts') IS NOT NULL THEN
    WITH offer_students AS (
      SELECT DISTINCT ON (offer.school_id, offer.tahun_lulus, TRIM(COALESCE(offer.nisn, '')))
        offer.school_id,
        offer.tahun_lulus,
        TRIM(COALESCE(offer.nisn, '')) AS nisn,
        COALESCE(NULLIF(TRIM(sc_by_app.nama_siswa), ''), NULLIF(TRIM(sc_by_nisn.nama_siswa), ''), 'Siswa ' || TRIM(COALESCE(offer.nisn, ''))) AS nama_siswa,
        COALESCE(NULLIF(TRIM(sc_by_app.rombel_siswa), ''), NULLIF(TRIM(sc_by_nisn.rombel_siswa), ''), '-') AS rombel
      FROM outcome_pt_offers offer
      LEFT JOIN student_higher_ed_applications app
        ON app.id = offer.application_id
      LEFT JOIN student_cohorts sc_by_app
        ON sc_by_app.id = app.cohort_id
      LEFT JOIN student_cohorts sc_by_nisn
        ON sc_by_nisn.school_id = offer.school_id
       AND sc_by_nisn.tahun_lulus = offer.tahun_lulus
       AND sc_by_nisn.nisn = offer.nisn
      WHERE offer.announcement_status = 'DITERIMA'
        AND offer.tahun_lulus > 0
        AND TRIM(COALESCE(offer.nisn, '')) <> ''
      ORDER BY offer.school_id, offer.tahun_lulus, TRIM(COALESCE(offer.nisn, '')), offer.updated_at DESC NULLS LAST, offer.created_at DESC NULLS LAST
    )
    INSERT INTO class12_students (
      school_id,
      tahun_lulus,
      nisn,
      nama_siswa,
      rombel
    )
    SELECT
      school_id,
      tahun_lulus,
      nisn,
      nama_siswa,
      rombel
    FROM offer_students
    ON CONFLICT (school_id, tahun_lulus, nisn) DO UPDATE
    SET
      nama_siswa = CASE
        WHEN TRIM(COALESCE(class12_students.nama_siswa, '')) = '' THEN EXCLUDED.nama_siswa
        ELSE class12_students.nama_siswa
      END,
      rombel = CASE
        WHEN TRIM(COALESCE(class12_students.rombel, '')) = '' THEN EXCLUDED.rombel
        ELSE class12_students.rombel
      END,
      updated_at = CASE
        WHEN TRIM(COALESCE(class12_students.nama_siswa, '')) = ''
          OR TRIM(COALESCE(class12_students.rombel, '')) = '' THEN NOW()
        ELSE class12_students.updated_at
      END;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Intentionally no-op. This backfill only inserts/soft-fills class12 identity rows
-- needed to reconcile accepted PT announcements at runtime.
