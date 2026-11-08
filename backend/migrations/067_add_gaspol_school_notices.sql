-- +goose Up
SET LOCAL lock_timeout = '5s';
ALTER TABLE gaspol_cohorts DROP CONSTRAINT gaspol_cohorts_cohort_check;
ALTER TABLE gaspol_cohorts ADD CONSTRAINT gaspol_cohorts_cohort_check CHECK
 (cohort IN ('TOP_10','NOT_FAILED','TOP_5','BOTTOM_5','TERGAGAL','ESCAPED_BOTTOM_ZONE','PASSED_NOMINATION'));

-- Backfill only from official, still-locked results. No preview produces a status.
INSERT INTO gaspol_cohorts(year,school_id,cohort,source_lock_id,locked_by,locked_at)
SELECT n.year,n.school_id,'PASSED_NOMINATION',l.id,l.locked_by,l.locked_at
FROM gaspol_cohorts n
JOIN gaspol_result_locks l ON l.year=n.year AND l.lock_type='BOTTOM_5' AND l.status='LOCKED'
 AND l.id=(SELECT latest.id FROM gaspol_result_locks latest
 WHERE latest.year=n.year AND latest.lock_type='BOTTOM_5' AND latest.status='LOCKED'
 ORDER BY latest.locked_at DESC,latest.id DESC LIMIT 1)
WHERE n.cohort='NOT_FAILED' AND n.released_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM gaspol_cohorts b WHERE b.year=n.year AND b.school_id=n.school_id
 AND b.cohort='BOTTOM_5' AND b.source_lock_id=l.id AND b.released_at IS NULL)
ON CONFLICT(year,school_id,cohort) DO NOTHING;

CREATE TABLE gaspol_school_notice_reads (
 school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE RESTRICT,
 year INTEGER NOT NULL,
 cohort_id BIGINT NOT NULL REFERENCES gaspol_cohorts(id) ON DELETE RESTRICT,
 source_lock_id BIGINT NOT NULL REFERENCES gaspol_result_locks(id) ON DELETE RESTRICT,
 read_by INTEGER NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(school_id,year,cohort_id,source_lock_id)
);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 RAISE EXCEPTION 'Migration 067 is forward-only; preserve official statuses and notice acknowledgements';
END $$;
-- +goose StatementEnd
