-- Run this read-only snapshot on a staging restore immediately before and
-- after migration 060. Store both result sets; do not run a final backfill
-- when an existing decision count decreases.
SELECT 'offers_total' AS metric, COUNT(*)::bigint AS value
FROM outcome_pt_offers WHERE deleted_at IS NULL
UNION ALL
SELECT 'offers_without_nisn', COUNT(*)::bigint
FROM outcome_pt_offers
WHERE deleted_at IS NULL AND NULLIF(TRIM(COALESCE(nisn,'')), '') IS NULL
UNION ALL
SELECT 'final_pt_rows', COUNT(*)::bigint
FROM student_outcomes
WHERE final_status='PT' AND finalized_at IS NOT NULL
UNION ALL
SELECT 'final_pt_unique_students',
       COUNT(DISTINCT school_id::text||'|'||tahun_lulus::text||'|'||nisn)::bigint
FROM student_outcomes
WHERE final_status='PT' AND finalized_at IS NOT NULL
UNION ALL
SELECT 'existing_final_decisions', COUNT(*)::bigint
FROM student_outcomes WHERE finalized_at IS NOT NULL
UNION ALL
SELECT 'offer_conflicts', COUNT(*)::bigint
FROM outcome_pt_offers
WHERE deleted_at IS NULL AND COALESCE(conflict_status,'NONE')<>'NONE';

SELECT source_role, COUNT(*) AS total
FROM outcome_pt_offers
WHERE deleted_at IS NULL
GROUP BY source_role
ORDER BY source_role;

SELECT UPPER(TRIM(jalur)) AS jalur, COUNT(*) AS total
FROM outcome_pt_offers
WHERE deleted_at IS NULL
GROUP BY UPPER(TRIM(jalur))
ORDER BY jalur;

SELECT school_id, tahun_lulus, COUNT(*) AS total
FROM outcome_pt_offers
WHERE deleted_at IS NULL
GROUP BY school_id, tahun_lulus
ORDER BY school_id, tahun_lulus;

SELECT conflict_status, COUNT(*) AS total
FROM outcome_pt_offers
WHERE deleted_at IS NULL AND COALESCE(conflict_status,'NONE')<>'NONE'
GROUP BY conflict_status
ORDER BY conflict_status;

-- Run this final query only after migration 060.
SELECT reconciliation_status, reason, COUNT(*) AS total
FROM pt_offer_reconciliations
GROUP BY reconciliation_status, reason
ORDER BY reconciliation_status, reason;
