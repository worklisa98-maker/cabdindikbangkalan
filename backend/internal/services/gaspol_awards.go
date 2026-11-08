package services

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"

	"cabdin-bangkalan-backend/internal/auth"
)

type GaspolInnovationDecisionInput struct {
	Decision string `json:"decision"`
	Notes    string `json:"notes"`
}
type GaspolFinalizeFavoriteInput struct {
	Year              int  `json:"year"`
	ManualTieSchoolID *int `json:"manualTieSchoolId"`
}

func pendingGaspolPublicAward(kind string) GaspolPublicAward {
	return GaspolPublicAward{AwardType: kind, SchoolName: "Belum ditetapkan", IsFinal: false}
}

func (s GaspolService) PublicAwards(ctx context.Context, year int, provisionalBest *GaspolRankingRow) (GaspolPublicAwards, error) {
	result := GaspolPublicAwards{
		Best:       pendingGaspolPublicAward("BEST"),
		Innovative: pendingGaspolPublicAward("INNOVATIVE"),
		Favorite:   pendingGaspolPublicAward("FAVORITE"),
		Failed: []GaspolPublicAward{
			pendingGaspolPublicAward("FAILED"),
			pendingGaspolPublicAward("FAILED"),
			pendingGaspolPublicAward("FAILED"),
		},
	}
	var publish bool
	if err := s.DB.Pool.QueryRow(ctx, `SELECT public_full_ranking_enabled FROM gaspol_competition_configs WHERE year=$1`, year).Scan(&publish); err != nil {
		return result, err
	}
	if !publish {
		return result, nil
	}
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT a.award_type,a.school_id,ds.nama_sekolah,a.score
		FROM gaspol_awards a
		JOIN data_sekolah ds ON ds.id=a.school_id
		WHERE a.year=$1 AND a.status='LOCKED'
		ORDER BY CASE a.award_type WHEN 'BEST' THEN 1 WHEN 'INNOVATIVE' THEN 2 WHEN 'FAVORITE' THEN 3 ELSE 4 END,a.locked_at,a.id
	`, year)
	if err != nil {
		return GaspolPublicAwards{}, err
	}
	defer rows.Close()
	failed := []GaspolPublicAward{}
	for rows.Next() {
		var kind, schoolName string
		var schoolID int
		var score *float64
		if err = rows.Scan(&kind, &schoolID, &schoolName, &score); err != nil {
			return GaspolPublicAwards{}, err
		}
		award := GaspolPublicAward{AwardType: kind, SchoolID: &schoolID, SchoolName: schoolName, Score: score, IsFinal: true}
		switch kind {
		case "BEST":
			result.Best = award
		case "INNOVATIVE":
			result.Innovative = award
		case "FAVORITE":
			result.Favorite = award
		case "FAILED":
			award.Score = nil
			if len(failed) < 3 {
				failed = append(failed, award)
			}
		}
	}
	if err = rows.Err(); err != nil {
		return GaspolPublicAwards{}, err
	}
	if len(failed) == 0 {
		fallbackRows, fallbackErr := s.DB.Pool.Query(ctx, `SELECT c.school_id,ds.nama_sekolah FROM gaspol_cohorts c JOIN data_sekolah ds ON ds.id=c.school_id WHERE c.year=$1 AND c.cohort='TERGAGAL' AND c.released_at IS NULL ORDER BY c.locked_at,c.id LIMIT 3`, year)
		if fallbackErr != nil {
			return GaspolPublicAwards{}, fallbackErr
		}
		for fallbackRows.Next() {
			var schoolID int
			var schoolName string
			if fallbackErr = fallbackRows.Scan(&schoolID, &schoolName); fallbackErr != nil {
				fallbackRows.Close()
				return GaspolPublicAwards{}, fallbackErr
			}
			failed = append(failed, GaspolPublicAward{AwardType: "FAILED", SchoolID: &schoolID, SchoolName: schoolName, IsFinal: true})
		}
		fallbackRows.Close()
	}
	for len(failed) < 3 {
		failed = append(failed, pendingGaspolPublicAward("FAILED"))
	}
	result.Failed = failed
	return result, nil
}

func (s GaspolService) ProposeInnovationAward(ctx context.Context, user auth.User, year int) (int64, error) {
	year = normalizeGaspolYear(year)
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `LOCK TABLE gaspol_innovation_nominations,gaspol_stage_assessments,gaspol_stage_assignments,gaspol_result_locks,gaspol_cohorts IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return 0, err
	}
	var assigned, closed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$2 AND assignment_role='INNOVATION_ASSESSOR' AND is_active),EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=$1 AND lock_type='AWARDING' AND status='LOCKED')`, year, user.ID).Scan(&assigned, &closed); err != nil {
		return 0, err
	}
	if !assigned {
		return 0, auth.ErrForbidden
	}
	if closed {
		return 0, errors.New("RESULT_ALREADY_LOCKED")
	}
	obligations, err := gaspolObligations(ctx, tx, year, "TOP_5", "INNOVATION")
	if err != nil {
		return 0, err
	}
	if !gaspolObligationsReady(obligations, 5) {
		return 0, errors.New("INCOMPLETE_ASSESSMENTS")
	}
	version := gaspolInnovationVersion(obligations)
	var existing int64
	if err = tx.QueryRow(ctx, `SELECT id FROM gaspol_innovation_nominations WHERE year=$1 AND source_version=$2 AND status IN ('SUBMITTED','APPROVED') ORDER BY revision DESC LIMIT 1`, year, version).Scan(&existing); err == nil {
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	scores := map[int]float64{}
	counts := map[int]int{}
	for _, r := range obligations {
		scores[r.SchoolID] += *r.FinalScore
		counts[r.SchoolID]++
	}
	var schoolID int
	var score float64
	for _, r := range obligations {
		value := scores[r.SchoolID] / float64(counts[r.SchoolID])
		if schoolID == 0 || value > score {
			schoolID = r.SchoolID
			score = value
		}
	}
	var id int64
	if err = tx.QueryRow(ctx, `INSERT INTO gaspol_innovation_nominations(year,school_id,score,status,proposed_by,revision,source_version) SELECT $1,$2,$3,'SUBMITTED',$4,COALESCE(MAX(revision),0)+1,$5 FROM gaspol_innovation_nominations WHERE year=$1 RETURNING id`, year, schoolID, score, user.ID, version).Scan(&id); err != nil {
		return 0, err
	}
	if err = gaspolAuditTx(ctx, tx, user, year, "INNOVATION_PROPOSED", "innovation_nomination", id, nil, map[string]any{"sourceVersion": version}); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (s GaspolService) DecideInnovationAward(ctx context.Context, user auth.User, id int64, input GaspolInnovationDecisionInput) error {
	decision := strings.ToUpper(strings.TrimSpace(input.Decision))
	if decision != "APPROVE" && decision != "RETURN" {
		return errors.New("INVALID_PAYLOAD")
	}
	var assigned bool
	var year, schoolID int
	var score float64
	if err := s.DB.Pool.QueryRow(ctx, `SELECT year,school_id,score FROM gaspol_innovation_nominations WHERE id=$1 AND status='SUBMITTED'`, id).Scan(&year, &schoolID, &score); err != nil {
		return err
	}
	if err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$2 AND assignment_role='KASI_SMK_APPROVER' AND is_active)`, year, user.ID).Scan(&assigned); err != nil {
		return err
	}
	if !assigned {
		return auth.ErrForbidden
	}
	status := "RETURNED"
	if decision == "APPROVE" {
		status = "APPROVED"
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `LOCK TABLE gaspol_innovation_nominations,gaspol_stage_assessments,gaspol_stage_assignments,gaspol_result_locks,gaspol_cohorts IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return err
	}
	obligations, err := gaspolObligations(ctx, tx, year, "TOP_5", "INNOVATION")
	if err != nil {
		return err
	}
	var sourceVersion string
	if err = tx.QueryRow(ctx, `SELECT source_version FROM gaspol_innovation_nominations WHERE id=$1 AND status='SUBMITTED'`, id).Scan(&sourceVersion); err != nil {
		return err
	}
	if !gaspolObligationsReady(obligations, 5) || sourceVersion != gaspolInnovationVersion(obligations) {
		return errors.New("RESULT_PREVIEW_STALE")
	}
	tag, err := tx.Exec(ctx, `UPDATE gaspol_innovation_nominations SET status=$2,kasi_user_id=$3,kasi_notes=$4,decided_at=NOW() WHERE id=$1 AND status='SUBMITTED'`, id, status, user.ID, strings.TrimSpace(input.Notes))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("INVALID_WORKFLOW_TRANSITION")
	}
	if err = gaspolAuditTx(ctx, tx, user, year, "INNOVATION_"+status, "innovation_nomination", id, &schoolID, map[string]any{"notes": input.Notes}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s GaspolService) FinalizeBestAward(ctx context.Context, user auth.User, year int) (int, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return 0, err
	}
	return 0, errors.New("JOINT_FINALIZATION_REQUIRED")
}

func (s GaspolService) FinalizeFavoriteAward(ctx context.Context, user auth.User, input GaspolFinalizeFavoriteInput) (int, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return 0, err
	}
	return 0, errors.New("JOINT_FINALIZATION_REQUIRED")
}
