package services

import (
	"context"
	"fmt"
	"time"

	"cabdin-bangkalan-backend/internal/db"
)

// CheckGaspolMasterReturnSchema runs before serving requests, even when automatic
// migrations are disabled. No compatibility fallback may hide an outdated schema.
func CheckGaspolMasterReturnSchema(ctx context.Context, database *db.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, err := database.Pool.Query(ctx, `SELECT koor_notes,cabdin_notes,
		master_return_notes,returned_by_master_user_id,master_returned_at
		FROM gaspol_stage_assessments LIMIT 0`)
	if err != nil {
		return fmt.Errorf("GASPOL schema check failed: verify migration 065 is applied (schema/connection mismatch, not dashboard alias projection): %w", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("GASPOL schema check failed: verify migration 065 is applied (schema/connection mismatch, not dashboard alias projection): %w", err)
	}
	var ready bool
	err = database.Pool.QueryRow(ctx, `SELECT COUNT(*)=2 FROM pg_constraint
		WHERE contype='c' AND convalidated
		AND ((conrelid='gaspol_stage_assessments'::regclass AND conname='gaspol_stage_assessments_workflow_status_check')
		 OR (conrelid='gaspol_assessments'::regclass AND conname='gaspol_assessments_workflow_status_check'))
		AND position('RETURNED_BY_MASTER' in pg_get_constraintdef(oid))>0`).Scan(&ready)
	if err != nil {
		return fmt.Errorf("GASPOL workflow schema check failed; verify migration 065: %w", err)
	}
	if !ready {
		return fmt.Errorf("GASPOL schema mismatch: migration 065 workflow constraints must allow RETURNED_BY_MASTER on both assessment tables")
	}
	return nil
}

func CheckGaspolSchoolNoticeSchema(ctx context.Context, database *db.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, err := database.Pool.Query(ctx, `SELECT school_id,year,cohort_id,source_lock_id,read_by,read_at FROM gaspol_school_notice_reads LIMIT 0`)
	if err != nil {
		return fmt.Errorf("GASPOL school notice schema missing; apply migration 067: %w", err)
	}
	rows.Close()
	var ready bool
	err = database.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='gaspol_cohorts'::regclass AND conname='gaspol_cohorts_cohort_check' AND convalidated AND position('PASSED_NOMINATION' in pg_get_constraintdef(oid))>0)`).Scan(&ready)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("GASPOL cohort schema mismatch; apply migration 067")
	}
	return nil
}
func CheckGaspolPhase234Schema(ctx context.Context, database *db.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, err := database.Pool.Query(ctx, `SELECT r.id,b.rubric_id,i.payload,s.managed,v.present,c.short_code,l.review_status,o.is_active,a.consent_at FROM gaspol_rubric_versions r CROSS JOIN gaspol_rubric_bindings b CROSS JOIN gaspol_innovation_submissions i CROSS JOIN gaspol_voting_sessions s CROSS JOIN gaspol_voting_voters v CROSS JOIN gaspol_voting_candidates c CROSS JOIN gaspol_like_snapshots l CROSS JOIN gaspol_voting_operators o CROSS JOIN gaspol_vote_assistance a LIMIT 0`)
	if err != nil {
		return fmt.Errorf("GASPOL phase 2–4 schema missing; apply migrations 068–071: %w", err)
	}
	rows.Close()
	return rows.Err()
}
