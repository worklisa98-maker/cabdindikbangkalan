-- +goose Up
-- Self-heal legacy monthly assessments without changing any existing v2
-- workflow state. Linking an existing tuple is safe and makes the relationship
-- explicit; only genuinely missing tuples are inserted.

UPDATE gaspol_stage_assessments sa
SET legacy_assessment_id = ga.id
FROM gaspol_assessments ga
WHERE sa.legacy_assessment_id IS NULL
  AND ga.stage::text = 'MONTHLY'
  AND ga.assessor_role::text = 'SUPERVISOR'
  AND sa.year = ga.year
  AND sa.stage = 'MONTHLY'
  AND sa.month = ga.month
  AND sa.school_id = ga.school_id
  AND sa.assessor_user_id = ga.assessor_user_id
  AND NOT EXISTS (
    SELECT 1
    FROM gaspol_stage_assessments linked
    WHERE linked.legacy_assessment_id = ga.id
  );

INSERT INTO gaspol_stage_assessments(
  legacy_assessment_id, year, stage, month, school_id, assessor_user_id,
  raw_score, notes, workflow_status, cabdin_user_id, cabdin_validated_at,
  final_score, locked_at, created_at, updated_at
)
SELECT ga.id, ga.year, 'MONTHLY', ga.month, ga.school_id, ga.assessor_user_id,
       ga.score_monthly, ga.notes,
       CASE WHEN ga.is_validated THEN 'FINAL' ELSE 'WAITING_KOOR' END,
       CASE WHEN ga.is_validated THEN ga.validated_by END,
       CASE WHEN ga.is_validated THEN ga.validated_at END,
       CASE WHEN ga.is_validated THEN ga.score_monthly END,
       CASE WHEN ga.is_validated THEN ga.validated_at END,
       ga.created_at, ga.updated_at
FROM gaspol_assessments ga
WHERE ga.stage::text = 'MONTHLY'
  AND ga.assessor_role::text = 'SUPERVISOR'
  AND NOT EXISTS (
    SELECT 1
    FROM gaspol_stage_assessments sa
    WHERE sa.legacy_assessment_id = ga.id
       OR (sa.year = ga.year
           AND sa.stage = 'MONTHLY'
           AND sa.month = ga.month
           AND sa.school_id = ga.school_id
           AND sa.assessor_user_id = ga.assessor_user_id)
  )
ON CONFLICT DO NOTHING;

-- +goose Down
-- Intentionally non-destructive: rows may have advanced in the v2 workflow
-- after this migration and must never be deleted or reset during rollback.
SELECT 1;
