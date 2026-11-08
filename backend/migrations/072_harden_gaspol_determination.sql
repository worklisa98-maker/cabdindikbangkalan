-- +goose Up
-- Additive only: existing locks, awards, scores and documents are preserved.
ALTER TABLE gaspol_ba_documents ADD COLUMN IF NOT EXISTS ba_number TEXT NOT NULL DEFAULT '';
ALTER TABLE gaspol_ba_documents ADD COLUMN IF NOT EXISTS ba_date DATE;
ALTER TABLE gaspol_result_locks ADD COLUMN IF NOT EXISTS source_payload JSONB NOT NULL DEFAULT '[]';
ALTER TABLE gaspol_innovation_nominations ADD COLUMN IF NOT EXISTS source_version TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS gaspol_result_locks_active_unique ON gaspol_result_locks(year,lock_type) WHERE status='LOCKED';
CREATE TABLE IF NOT EXISTS gaspol_result_exports (
 id BIGSERIAL PRIMARY KEY, year INTEGER NOT NULL, data_version TEXT NOT NULL,
 payload JSONB NOT NULL, exported_by INTEGER REFERENCES users(id), exported_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE gaspol_ranking_snapshots_v2 DROP CONSTRAINT IF EXISTS gaspol_ranking_snapshots_v2_snapshot_type_check;
ALTER TABLE gaspol_ranking_snapshots_v2 ADD CONSTRAINT gaspol_ranking_snapshots_v2_snapshot_type_check CHECK(snapshot_type IN ('TOP_10','TOP_5','NOT_FAILED','BOTTOM_5','FINAL','AWARDING','STANDINGS'));
ALTER TABLE gaspol_ba_documents ALTER COLUMN document_type SET DEFAULT 'STAGED';
-- Serialize operational writes with determination, and protect frozen stages.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION gaspol_guard_locked_stage() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE y integer; st text; closing text;
BEGIN
 y:=CASE WHEN TG_OP='DELETE' THEN OLD.year ELSE NEW.year END;
 st:=CASE WHEN TG_OP='DELETE' THEN OLD.stage::text ELSE NEW.stage::text END;
 closing:=CASE st WHEN 'MONTHLY' THEN 'TOP_10' WHEN 'FIELD' THEN 'TOP_10' WHEN 'FIELD_STAGE_1' THEN 'TOP_10' WHEN 'FIELD_STAGE_2' THEN 'TOP_5' WHEN 'PRESENTATION' THEN 'AWARDING' WHEN 'INNOVATION' THEN 'AWARDING' WHEN 'NOT_FAILED_NOMINATION' THEN 'BOTTOM_5' WHEN 'BOTTOM5_APPEAL' THEN 'FINAL' END;
 IF EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=y AND lock_type=closing AND status='LOCKED') THEN
  RAISE EXCEPTION 'RESULT_ALREADY_LOCKED' USING ERRCODE='P0001';
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END;
$$;
-- +goose StatementEnd
DROP TRIGGER IF EXISTS gaspol_stage_frozen ON gaspol_stage_assessments;
CREATE TRIGGER gaspol_stage_frozen BEFORE INSERT OR UPDATE OR DELETE ON gaspol_stage_assessments FOR EACH ROW EXECUTE FUNCTION gaspol_guard_locked_stage();
DROP TRIGGER IF EXISTS gaspol_legacy_stage_frozen ON gaspol_assessments;
CREATE TRIGGER gaspol_legacy_stage_frozen BEFORE INSERT OR UPDATE OR DELETE ON gaspol_assessments FOR EACH ROW EXECUTE FUNCTION gaspol_guard_locked_stage();
DROP TRIGGER IF EXISTS gaspol_assignment_frozen ON gaspol_stage_assignments;
CREATE TRIGGER gaspol_assignment_frozen BEFORE INSERT OR UPDATE OR DELETE ON gaspol_stage_assignments FOR EACH ROW EXECUTE FUNCTION gaspol_guard_locked_stage();
CREATE UNIQUE INDEX IF NOT EXISTS gaspol_standings_position_unique ON gaspol_ranking_snapshots_v2(year,revision,rank_position) WHERE snapshot_type='STANDINGS';
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Forward-only official GASPOL determination; use a reviewed backup for rollback'; END $$;
-- +goose StatementEnd
