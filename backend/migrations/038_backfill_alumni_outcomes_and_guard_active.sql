-- +goose Up
ALTER TABLE student_outcomes
  ADD COLUMN IF NOT EXISTS source TEXT;

UPDATE student_outcomes
SET source = 'FINAL_SISWA_AKHIR'
WHERE source IS NULL
   OR TRIM(source) = '';

ALTER TABLE student_outcomes
  ALTER COLUMN source SET DEFAULT 'FINAL_SISWA_AKHIR';

ALTER TABLE student_outcomes
  ALTER COLUMN source SET NOT NULL;

ALTER TABLE student_outcomes
  DROP CONSTRAINT IF EXISTS student_outcomes_source_check;

ALTER TABLE student_outcomes
  ADD CONSTRAINT student_outcomes_source_check
  CHECK (source IN ('FINAL_SISWA_AKHIR', 'IMPORT_ALUMNI'));

-- +goose StatementBegin
DO $$
BEGIN
  IF to_regclass('public.student_cohorts') IS NOT NULL
     AND to_regclass('public.student_final_outcomes') IS NOT NULL THEN
    INSERT INTO class12_students (
      school_id,
      tahun_lulus,
      nisn,
      nama_siswa,
      rombel,
      created_at,
      updated_at
    )
    SELECT DISTINCT
      sc.school_id,
      sc.tahun_lulus,
      TRIM(COALESCE(sc.nisn, '')) AS nisn,
      COALESCE(NULLIF(TRIM(sc.nama_siswa), ''), '-') AS nama_siswa,
      COALESCE(NULLIF(TRIM(sc.rombel_siswa), ''), '-') AS rombel,
      COALESCE(sc.created_at, NOW()),
      COALESCE(sc.updated_at, NOW())
    FROM student_cohorts sc
    JOIN student_final_outcomes sfo
      ON sfo.school_id = sc.school_id
     AND sfo.cohort_id = sc.id
    WHERE TRIM(COALESCE(sc.nisn, '')) <> ''
      AND COALESCE(sc.tahun_lulus, 0) > 0
    ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;

    UPDATE student_outcomes o
    SET source = 'IMPORT_ALUMNI'
    FROM student_cohorts sc
    JOIN student_final_outcomes sfo
      ON sfo.school_id = sc.school_id
     AND sfo.cohort_id = sc.id
    WHERE o.school_id = sc.school_id
      AND o.tahun_lulus = sc.tahun_lulus
      AND o.nisn = TRIM(COALESCE(sc.nisn, ''))
      AND TRIM(COALESCE(sc.nisn, '')) <> '';

    INSERT INTO student_outcomes (
      school_id,
      tahun_lulus,
      nisn,
      final_status,
      final_path,
      source,
      finalized_at,
      finalized_by,
      created_at,
      updated_at
    )
    SELECT
      sc.school_id,
      sc.tahun_lulus,
      TRIM(COALESCE(sc.nisn, '')) AS nisn,
      CASE sfo.final_type
        WHEN 'HIGHER_ED' THEN 'PT'
        WHEN 'WORK' THEN 'KERJA'
        WHEN 'ENTREPRENEUR' THEN 'WIRAUSAHA'
        WHEN 'UNDEFINED' THEN 'BELUM'
        ELSE 'BELUM'
      END AS final_status,
      CASE
        WHEN sfo.final_type = 'HIGHER_ED' THEN COALESCE(ofer.jalur_penerimaan, 'LAINNYA'::jalur_penerimaan_type)
        ELSE NULL
      END AS final_path,
      'IMPORT_ALUMNI' AS source,
      COALESCE(sfo.finalized_at, NOW()) AS finalized_at,
      NULL AS finalized_by,
      COALESCE(sfo.created_at, NOW()) AS created_at,
      COALESCE(sfo.updated_at, NOW()) AS updated_at
    FROM student_cohorts sc
    JOIN student_final_outcomes sfo
      ON sfo.school_id = sc.school_id
     AND sfo.cohort_id = sc.id
    LEFT JOIN student_higher_ed_acceptance_offers ofer
      ON ofer.id = sfo.offer_id
    WHERE TRIM(COALESCE(sc.nisn, '')) <> ''
      AND COALESCE(sc.tahun_lulus, 0) > 0
      AND NOT EXISTS (
        SELECT 1
        FROM student_outcomes existing
        WHERE existing.school_id = sc.school_id
          AND existing.nisn = TRIM(COALESCE(sc.nisn, ''))
          AND existing.finalized_at IS NOT NULL
      )
    ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;

    INSERT INTO outcome_pt_details (
      school_id,
      tahun_lulus,
      nisn,
      kampus,
      prodi,
      jalur,
      tahun_diterima,
      created_at,
      updated_at
    )
    SELECT
      sc.school_id,
      sc.tahun_lulus,
      TRIM(COALESCE(sc.nisn, '')) AS nisn,
      COALESCE(NULLIF(TRIM(ofer.nama_perguruan_tinggi), ''), '-') AS kampus,
      COALESCE(NULLIF(TRIM(ofer.jurusan_perguruan_tinggi), ''), '-') AS prodi,
      COALESCE(ofer.jalur_penerimaan, 'LAINNYA'::jalur_penerimaan_type) AS jalur,
      ofer.tahun_diterima,
      COALESCE(ofer.created_at, NOW()),
      COALESCE(ofer.updated_at, NOW())
    FROM student_cohorts sc
    JOIN student_final_outcomes sfo
      ON sfo.school_id = sc.school_id
     AND sfo.cohort_id = sc.id
    JOIN student_higher_ed_acceptance_offers ofer
      ON ofer.id = sfo.offer_id
    JOIN student_outcomes so
      ON so.school_id = sc.school_id
     AND so.tahun_lulus = sc.tahun_lulus
     AND so.nisn = TRIM(COALESCE(sc.nisn, ''))
    WHERE sfo.final_type = 'HIGHER_ED'
      AND COALESCE(so.source, 'FINAL_SISWA_AKHIR') = 'IMPORT_ALUMNI'
    ON CONFLICT (school_id, tahun_lulus, nisn)
    DO UPDATE SET
      kampus = EXCLUDED.kampus,
      prodi = EXCLUDED.prodi,
      jalur = EXCLUDED.jalur,
      tahun_diterima = EXCLUDED.tahun_diterima,
      updated_at = NOW();

    INSERT INTO outcome_work_details (
      school_id,
      tahun_lulus,
      nisn,
      perusahaan,
      jabatan,
      tahun_mulai,
      created_at,
      updated_at
    )
    SELECT
      sc.school_id,
      sc.tahun_lulus,
      TRIM(COALESCE(sc.nisn, '')) AS nisn,
      '-' AS perusahaan,
      '' AS jabatan,
      NULL AS tahun_mulai,
      COALESCE(sfo.created_at, NOW()),
      COALESCE(sfo.updated_at, NOW())
    FROM student_cohorts sc
    JOIN student_final_outcomes sfo
      ON sfo.school_id = sc.school_id
     AND sfo.cohort_id = sc.id
    JOIN student_outcomes so
      ON so.school_id = sc.school_id
     AND so.tahun_lulus = sc.tahun_lulus
     AND so.nisn = TRIM(COALESCE(sc.nisn, ''))
    WHERE sfo.final_type = 'WORK'
      AND COALESCE(so.source, 'FINAL_SISWA_AKHIR') = 'IMPORT_ALUMNI'
    ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;

    INSERT INTO outcome_business_details (
      school_id,
      tahun_lulus,
      nisn,
      bidang_usaha,
      nama_usaha,
      tahun_mulai,
      created_at,
      updated_at
    )
    SELECT
      sc.school_id,
      sc.tahun_lulus,
      TRIM(COALESCE(sc.nisn, '')) AS nisn,
      '-' AS bidang_usaha,
      '' AS nama_usaha,
      NULL AS tahun_mulai,
      COALESCE(sfo.created_at, NOW()),
      COALESCE(sfo.updated_at, NOW())
    FROM student_cohorts sc
    JOIN student_final_outcomes sfo
      ON sfo.school_id = sc.school_id
     AND sfo.cohort_id = sc.id
    JOIN student_outcomes so
      ON so.school_id = sc.school_id
     AND so.tahun_lulus = sc.tahun_lulus
     AND so.nisn = TRIM(COALESCE(sc.nisn, ''))
    WHERE sfo.final_type = 'ENTREPRENEUR'
      AND COALESCE(so.source, 'FINAL_SISWA_AKHIR') = 'IMPORT_ALUMNI'
    ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;
  END IF;

  IF to_regclass('public.siswa_perguruan_tinggi') IS NOT NULL THEN
    INSERT INTO class12_students (
      school_id,
      tahun_lulus,
      nisn,
      nama_siswa,
      rombel,
      created_at,
      updated_at
    )
    SELECT DISTINCT
      spt.school_id,
      COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0)) AS tahun_lulus,
      TRIM(COALESCE(spt.nisn_siswa, '')) AS nisn,
      COALESCE(NULLIF(TRIM(spt.nama_siswa), ''), '-') AS nama_siswa,
      COALESCE(NULLIF(TRIM(spt.rombel_siswa), ''), '-') AS rombel,
      NOW(),
      NOW()
    FROM siswa_perguruan_tinggi spt
    WHERE TRIM(COALESCE(spt.nisn_siswa, '')) <> ''
      AND COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0), 0) > 0
    ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;

    UPDATE student_outcomes o
    SET source = 'IMPORT_ALUMNI'
    FROM siswa_perguruan_tinggi spt
    WHERE o.school_id = spt.school_id
      AND o.nisn = TRIM(COALESCE(spt.nisn_siswa, ''))
      AND o.tahun_lulus = COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0))
      AND TRIM(COALESCE(spt.nisn_siswa, '')) <> '';

    INSERT INTO student_outcomes (
      school_id,
      tahun_lulus,
      nisn,
      final_status,
      final_path,
      source,
      finalized_at,
      finalized_by,
      created_at,
      updated_at
    )
    SELECT
      spt.school_id,
      COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0)) AS tahun_lulus,
      TRIM(COALESCE(spt.nisn_siswa, '')) AS nisn,
      'PT' AS final_status,
      COALESCE(spt.jalur_penerimaan, 'LAINNYA'::jalur_penerimaan_type) AS final_path,
      'IMPORT_ALUMNI' AS source,
      NOW() AS finalized_at,
      NULL AS finalized_by,
      NOW() AS created_at,
      NOW() AS updated_at
    FROM siswa_perguruan_tinggi spt
    WHERE TRIM(COALESCE(spt.nisn_siswa, '')) <> ''
      AND COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0), 0) > 0
      AND TRIM(COALESCE(spt.nama_perguruan_tinggi, '')) <> ''
      AND TRIM(COALESCE(spt.jurusan_perguruan_tinggi, '')) <> ''
      AND NOT EXISTS (
        SELECT 1
        FROM student_outcomes existing
        WHERE existing.school_id = spt.school_id
          AND existing.nisn = TRIM(COALESCE(spt.nisn_siswa, ''))
          AND existing.finalized_at IS NOT NULL
      )
    ON CONFLICT (school_id, tahun_lulus, nisn) DO NOTHING;

    INSERT INTO outcome_pt_details (
      school_id,
      tahun_lulus,
      nisn,
      kampus,
      prodi,
      jalur,
      tahun_diterima,
      created_at,
      updated_at
    )
    SELECT
      spt.school_id,
      COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0)) AS tahun_lulus,
      TRIM(COALESCE(spt.nisn_siswa, '')) AS nisn,
      TRIM(COALESCE(spt.nama_perguruan_tinggi, '')) AS kampus,
      TRIM(COALESCE(spt.jurusan_perguruan_tinggi, '')) AS prodi,
      COALESCE(spt.jalur_penerimaan, 'LAINNYA'::jalur_penerimaan_type) AS jalur,
      spt.tahun_diterima,
      NOW(),
      NOW()
    FROM siswa_perguruan_tinggi spt
    JOIN student_outcomes so
      ON so.school_id = spt.school_id
     AND so.nisn = TRIM(COALESCE(spt.nisn_siswa, ''))
     AND so.tahun_lulus = COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0))
    WHERE TRIM(COALESCE(spt.nisn_siswa, '')) <> ''
      AND COALESCE(so.source, 'FINAL_SISWA_AKHIR') = 'IMPORT_ALUMNI'
      AND so.final_status = 'PT'
    ON CONFLICT (school_id, tahun_lulus, nisn)
    DO UPDATE SET
      kampus = EXCLUDED.kampus,
      prodi = EXCLUDED.prodi,
      jalur = EXCLUDED.jalur,
      tahun_diterima = EXCLUDED.tahun_diterima,
      updated_at = NOW();
  END IF;
END; 
$$;
-- +goose StatementEnd

WITH ranked_active AS (
  SELECT
    id,
    ROW_NUMBER() OVER (
      PARTITION BY school_id, nisn
      ORDER BY
        CASE WHEN COALESCE(source, 'FINAL_SISWA_AKHIR') = 'IMPORT_ALUMNI' THEN 0 ELSE 1 END,
        finalized_at DESC NULLS LAST,
        updated_at DESC,
        id DESC
    ) AS rn
  FROM student_outcomes
  WHERE finalized_at IS NOT NULL
)
UPDATE student_outcomes o
SET finalized_at = NULL,
    updated_at = NOW()
FROM ranked_active ra
WHERE o.id = ra.id
  AND ra.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS student_outcomes_active_school_nisn_unique
  ON student_outcomes (school_id, nisn)
  WHERE finalized_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS student_outcomes_active_school_nisn_unique;
