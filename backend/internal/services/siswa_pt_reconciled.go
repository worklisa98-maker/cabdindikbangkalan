package services

// buildFinalPTReconciledCTE is the single source for final PT reporting.
// Existing finalized student_outcomes are authoritative. AUTO_FINAL offers are
// added only when the reconciliation service explicitly approved them and no
// manual final decision exists for the same scoped student identity.
func buildFinalPTReconciledCTE() string {
	return `
		WITH manual_pt AS (
			SELECT c.id::bigint AS id,
			       c.school_id,
			       COALESCE(ds.npsn_sekolah, '') AS npsn,
			       COALESCE(ds.nama_sekolah, '') AS school_name,
			       COALESCE(ds.bentuk_pendidikan, '') AS school_level,
			       COALESCE(ds.status_sekolah, '') AS school_status,
			       COALESCE(c.nisn, '') AS nisn,
			       COALESCE(c.nama_siswa, '') AS nama_siswa,
			       COALESCE(c.rombel, '') AS rombel,
			       c.tahun_lulus,
			       'PT'::text AS final_status,
			       COALESCE(o.final_path::text, '') AS final_path,
			       COALESCE(o.source, '` + OutcomeSourceFinalSiswaAkhir + `') AS source,
			       o.finalized_at,
			       COALESCE(pt.kampus, '') AS pt_kampus,
			       COALESCE(pt.prodi, '') AS pt_prodi,
			       COALESCE(pt.jalur::text, o.final_path::text, '') AS pt_jalur,
			       pt.tahun_diterima AS pt_tahun_diterima,
			       (SELECT COUNT(*)::int FROM outcome_pt_offers offer
			        WHERE offer.school_id=c.school_id AND offer.tahun_lulus=c.tahun_lulus
			          AND offer.nisn=c.nisn AND offer.announcement_status='DITERIMA'
			          AND offer.deleted_at IS NULL) AS pt_offer_count,
			       '` + OutcomeProgressStateFinal + `'::text AS progress_state
			FROM class12_students c
			JOIN data_sekolah ds ON ds.id=c.school_id
			JOIN student_outcomes o
			  ON o.school_id=c.school_id AND o.tahun_lulus=c.tahun_lulus AND o.nisn=c.nisn
			 AND o.final_status='PT' AND o.finalized_at IS NOT NULL
			LEFT JOIN outcome_pt_details pt
			  ON pt.school_id=o.school_id AND pt.tahun_lulus=o.tahun_lulus AND pt.nisn=o.nisn
		),
		auto_pt_roster AS (
			SELECT c.id::bigint AS id,
			       c.school_id,
			       COALESCE(ds.npsn_sekolah, '') AS npsn,
			       COALESCE(ds.nama_sekolah, '') AS school_name,
			       COALESCE(ds.bentuk_pendidikan, '') AS school_level,
			       COALESCE(ds.status_sekolah, '') AS school_status,
			       COALESCE(c.nisn, '') AS nisn,
			       COALESCE(c.nama_siswa, offer.student_name, '') AS nama_siswa,
			       COALESCE(c.rombel, '') AS rombel,
			       offer.tahun_lulus,
			       'PT'::text AS final_status,
			       COALESCE(offer.jalur, '') AS final_path,
			       '` + OutcomeSourcePTAcceptedPending + `'::text AS source,
			       reconciliation.reconciled_at AS finalized_at,
			       COALESCE(offer.kampus, '') AS pt_kampus,
			       COALESCE(offer.prodi, '') AS pt_prodi,
			       COALESCE(offer.jalur, '') AS pt_jalur,
			       offer.tahun_diterima AS pt_tahun_diterima,
			       1::int AS pt_offer_count,
			       '` + OutcomeProgressStateFinal + `'::text AS progress_state
			FROM pt_offer_reconciliations reconciliation
			JOIN outcome_pt_offers offer ON offer.id=reconciliation.offer_id
			JOIN class12_students c
			  ON c.school_id=offer.school_id AND c.tahun_lulus=offer.tahun_lulus
			 AND (
			   (NULLIF(TRIM(COALESCE(offer.nisn,'')), '') IS NOT NULL AND c.nisn=offer.nisn)
			   OR
			   (NULLIF(TRIM(COALESCE(offer.nisn,'')), '') IS NULL AND
			    LOWER(REGEXP_REPLACE(TRIM(COALESCE(c.nama_siswa,'')), '\s+', ' ', 'g'))=offer.normalized_student_name)
			 )
			 AND ` + activeClass12StudentCondition("c") + `
			JOIN data_sekolah ds ON ds.id=offer.school_id
			WHERE reconciliation.reconciliation_status = 'AUTO_FINAL'
			  AND offer.announcement_status='DITERIMA'
			  AND offer.deleted_at IS NULL
			  AND COALESCE(offer.conflict_status, 'NONE') = 'NONE'
			  AND NOT EXISTS (
			    SELECT 1 FROM student_outcomes o
			    WHERE o.school_id=c.school_id AND o.tahun_lulus=c.tahun_lulus
			      AND o.nisn=c.nisn AND o.finalized_at IS NOT NULL
			  )
		),
		orphan_auto_pt AS (
			SELECT (-offer.id)::bigint AS id,
			       offer.school_id,
			       COALESCE(ds.npsn_sekolah, '') AS npsn,
			       COALESCE(ds.nama_sekolah, '') AS school_name,
			       COALESCE(ds.bentuk_pendidikan, '') AS school_level,
			       COALESCE(ds.status_sekolah, '') AS school_status,
			       COALESCE(offer.nisn, '') AS nisn,
			       COALESCE(offer.student_name, '') AS nama_siswa,
			       ''::text AS rombel,
			       offer.tahun_lulus,
			       'PT'::text AS final_status,
			       COALESCE(offer.jalur, '') AS final_path,
			       '` + OutcomeSourcePTAcceptedPending + `'::text AS source,
			       reconciliation.reconciled_at AS finalized_at,
			       COALESCE(offer.kampus, '') AS pt_kampus,
			       COALESCE(offer.prodi, '') AS pt_prodi,
			       COALESCE(offer.jalur, '') AS pt_jalur,
			       offer.tahun_diterima AS pt_tahun_diterima,
			       1::int AS pt_offer_count,
			       '` + OutcomeProgressStateFinal + `'::text AS progress_state
			FROM pt_offer_reconciliations reconciliation
			JOIN outcome_pt_offers offer ON offer.id=reconciliation.offer_id
			JOIN data_sekolah ds ON ds.id=offer.school_id
			WHERE reconciliation.reconciliation_status = 'AUTO_FINAL'
			  AND offer.announcement_status='DITERIMA'
			  AND offer.deleted_at IS NULL
			  AND COALESCE(offer.conflict_status, 'NONE') = 'NONE'
			  AND NOT EXISTS (
			    SELECT 1 FROM class12_students c
			    WHERE c.school_id=offer.school_id AND c.tahun_lulus=offer.tahun_lulus
			      AND ` + activeClass12StudentCondition("c") + `
			      AND (
			        (NULLIF(TRIM(COALESCE(offer.nisn,'')), '') IS NOT NULL AND c.nisn=offer.nisn)
			        OR
			        (NULLIF(TRIM(COALESCE(offer.nisn,'')), '') IS NULL AND
			         LOWER(REGEXP_REPLACE(TRIM(COALESCE(c.nama_siswa,'')), '\s+', ' ', 'g'))=offer.normalized_student_name)
			      )
			  )
			  AND NOT EXISTS (
			    SELECT 1 FROM student_outcomes o
			    WHERE o.school_id=offer.school_id AND o.tahun_lulus=offer.tahun_lulus
			      AND o.finalized_at IS NOT NULL
			      AND NULLIF(TRIM(COALESCE(offer.nisn,'')), '') IS NOT NULL
			      AND o.nisn=offer.nisn
			  )
		),
		final_pt AS (
			SELECT * FROM manual_pt
			UNION ALL SELECT * FROM auto_pt_roster
			UNION ALL SELECT * FROM orphan_auto_pt
		)
	`
}
