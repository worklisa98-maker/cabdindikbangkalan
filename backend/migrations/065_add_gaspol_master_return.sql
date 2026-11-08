-- +goose Up
ALTER TABLE gaspol_stage_assessments
  DROP CONSTRAINT IF EXISTS gaspol_stage_assessments_workflow_status_check;
ALTER TABLE gaspol_stage_assessments
  ADD CONSTRAINT gaspol_stage_assessments_workflow_status_check CHECK (
    workflow_status IN ('ASSESSED','IN_PROGRESS','WAITING_KOOR','KOOR_VALIDATED','WAITING_CABDIN','FINAL','RETURNED_BY_KOOR','RETURNED_BY_CABDIN','RETURNED_BY_MASTER')
  );

ALTER TABLE gaspol_stage_assessments
  ADD COLUMN IF NOT EXISTS returned_by_master_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS master_return_notes TEXT,
  ADD COLUMN IF NOT EXISTS master_returned_at TIMESTAMPTZ;

ALTER TABLE gaspol_assessments
  DROP CONSTRAINT IF EXISTS gaspol_assessments_workflow_status_check;
ALTER TABLE gaspol_assessments
  ADD CONSTRAINT gaspol_assessments_workflow_status_check CHECK (
    workflow_status IN ('ASSESSED','IN_PROGRESS','WAITING_KOOR','KOOR_VALIDATED','WAITING_CABDIN','FINAL','RETURNED_BY_KOOR','RETURNED_BY_CABDIN','RETURNED_BY_MASTER')
  );

-- +goose Down
UPDATE gaspol_stage_assessments
SET workflow_status='FINAL', final_score=raw_score, locked_at=COALESCE(locked_at,NOW()), updated_at=NOW()
WHERE workflow_status='RETURNED_BY_MASTER';

UPDATE gaspol_assessments legacy
SET workflow_status='FINAL', is_validated=TRUE, validated_at=COALESCE(validated_at,NOW()), updated_at=NOW()
FROM gaspol_stage_assessments stage
WHERE stage.legacy_assessment_id=legacy.id
  AND legacy.workflow_status='RETURNED_BY_MASTER';

ALTER TABLE gaspol_stage_assessments
  DROP CONSTRAINT IF EXISTS gaspol_stage_assessments_workflow_status_check;
ALTER TABLE gaspol_stage_assessments
  ADD CONSTRAINT gaspol_stage_assessments_workflow_status_check CHECK (
    workflow_status IN ('ASSESSED','IN_PROGRESS','WAITING_KOOR','KOOR_VALIDATED','WAITING_CABDIN','FINAL','RETURNED_BY_KOOR','RETURNED_BY_CABDIN')
  );
ALTER TABLE gaspol_stage_assessments
  DROP COLUMN IF EXISTS master_returned_at,
  DROP COLUMN IF EXISTS master_return_notes,
  DROP COLUMN IF EXISTS returned_by_master_user_id;

ALTER TABLE gaspol_assessments
  DROP CONSTRAINT IF EXISTS gaspol_assessments_workflow_status_check;
ALTER TABLE gaspol_assessments
  ADD CONSTRAINT gaspol_assessments_workflow_status_check CHECK (
    workflow_status IN ('ASSESSED','IN_PROGRESS','WAITING_KOOR','KOOR_VALIDATED','WAITING_CABDIN','FINAL','RETURNED_BY_KOOR','RETURNED_BY_CABDIN')
  );
