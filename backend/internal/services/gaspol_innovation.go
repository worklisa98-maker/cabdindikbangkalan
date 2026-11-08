package services

import (
	"cabdin-bangkalan-backend/internal/auth"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strings"
)

type GaspolInnovationSubmission struct {
	Year             int              `json:"year"`
	SchoolID         int              `json:"schoolId"`
	Status           string           `json:"status"`
	Title            string           `json:"title"`
	Background       string           `json:"background"`
	Problem          string           `json:"problem"`
	Description      string           `json:"description"`
	Implementation   string           `json:"implementation"`
	Impact           string           `json:"impact"`
	Sustainability   string           `json:"sustainability"`
	DocumentationURL string           `json:"documentationUrl"`
	EvidenceURLs     []string         `json:"evidenceUrls"`
	History          []map[string]any `json:"history,omitempty"`
}

func (s GaspolService) phaseReadAllowed(ctx context.Context, u auth.User, year, school int, stage string) error {
	if auth.IsAdminMaster(u) {
		return nil
	}
	if u.SchoolID != nil && *u.SchoolID == school {
		return s.ensurePhaseSchool(ctx, u, year)
	}
	var allowed bool
	err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$2 AND is_active AND ((school_id=$3 AND stage=$4 AND assignment_role=$5) OR assignment_role='KOOR_GASPOL')) OR EXISTS(SELECT 1 FROM gaspol_school_participants p JOIN gaspol_user_roles r ON r.year=p.year AND r.user_id=$2 AND r.role='VALIDATOR' AND r.is_active WHERE p.year=$1 AND p.school_id=$3 AND p.validator_user_id=$2 AND p.is_active AND NOT p.is_year_configuration)`, year, u.ID, school, stage, stageAssignmentRole(stage)).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return auth.ErrForbidden
	}
	return nil
}
func (s GaspolService) InnovationSubmission(ctx context.Context, u auth.User, year, school int) (GaspolInnovationSubmission, error) {
	if err := s.phaseReadAllowed(ctx, u, year, school, "INNOVATION"); err != nil {
		return GaspolInnovationSubmission{}, err
	}
	var raw []byte
	var status string
	err := s.DB.Pool.QueryRow(ctx, `SELECT payload,status FROM gaspol_innovation_submissions WHERE year=$1 AND school_id=$2`, year, school).Scan(&raw, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return GaspolInnovationSubmission{Year: year, SchoolID: school, Status: "DRAFT"}, nil
	}
	if err != nil {
		return GaspolInnovationSubmission{}, err
	}
	var out GaspolInnovationSubmission
	err = json.Unmarshal(raw, &out)
	out.Status = status
	if err != nil {
		return out, err
	}
	rows, err := s.DB.Pool.Query(ctx, `SELECT a.action,COALESCE(u.username,''),a.created_at::text FROM gaspol_audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id WHERE a.year=$1 AND a.school_id=$2 AND a.entity_type='innovation_submission' ORDER BY a.id DESC`, year, school)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.History = []map[string]any{}
	for rows.Next() {
		var action, actor, at string
		if err = rows.Scan(&action, &actor, &at); err != nil {
			return out, err
		}
		out.History = append(out.History, map[string]any{"action": action, "actor": actor, "at": at})
	}
	return out, rows.Err()
}
func (s GaspolService) SaveInnovationSubmission(ctx context.Context, u auth.User, in GaspolInnovationSubmission) error {
	if err := s.ensurePhaseSchool(ctx, u, in.Year); err != nil {
		return err
	}
	in.SchoolID = *u.SchoolID
	if in.Status != "DRAFT" && in.Status != "SUBMITTED" {
		return errors.New("INVALID_STATUS")
	}
	if in.DocumentationURL != "" && !validGaspolEvidenceURL(in.DocumentationURL) {
		return errors.New("INVALID_REPORT_LINK")
	}
	for _, link := range in.EvidenceURLs {
		if !validGaspolEvidenceURL(link) {
			return errors.New("INVALID_REPORT_LINK")
		}
	}
	if in.Status == "SUBMITTED" {
		for _, v := range []string{in.Title, in.Background, in.Problem, in.Description, in.Implementation, in.Impact, in.Sustainability, in.DocumentationURL} {
			if strings.TrimSpace(v) == "" {
				return errors.New("INCOMPLETE_SUBMISSION")
			}
		}
		if len(in.EvidenceURLs) == 0 {
			return errors.New("INCOMPLETE_SUBMISSION")
		}
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.year=c.year AND l.status='LOCKED' WHERE c.year=$1 AND c.school_id=$2 AND c.cohort='TOP_5' AND c.released_at IS NULL) AND NOT EXISTS(SELECT 1 FROM gaspol_awards WHERE year=$1 AND award_type='INNOVATIVE')`, in.Year, in.SchoolID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New("SCHOOL_NOT_ELIGIBLE")
	}
	raw, _ := json.Marshal(in)
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO gaspol_innovation_submissions(year,school_id,payload,status,submitted_by,submitted_at) VALUES($1,$2,$3::jsonb,$4,$5,CASE WHEN $4='SUBMITTED' THEN NOW() END) ON CONFLICT(year,school_id) DO UPDATE SET payload=EXCLUDED.payload,status=EXCLUDED.status,submitted_by=EXCLUDED.submitted_by,submitted_at=EXCLUDED.submitted_at,updated_at=NOW() WHERE gaspol_innovation_submissions.status IN ('DRAFT','RETURNED') RETURNING id`, in.Year, in.SchoolID, string(raw), in.Status, u.ID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("SUBMISSION_LOCKED")
	}
	if err != nil {
		return err
	}
	if err = gaspolAuditTx(ctx, tx, u, in.Year, "INNOVATION_SUBMISSION_"+in.Status, "innovation_submission", id, &in.SchoolID, in); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s GaspolService) saveRubricAssessment(ctx context.Context, u auth.User, in GaspolStageAssessmentInput, assignment int64) (GaspolStageAssessmentRow, error) {
	r, err := s.AssessmentRubric(ctx, u, in.Year, in.SchoolID, in.Stage)
	if err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	if in.RubricID != r.ID {
		return GaspolStageAssessmentRow{}, errors.New("RUBRIC_VERSION_MISMATCH")
	}
	score, err := ComputeGaspolRubric(r.Indicators, in.Rubric)
	if err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	if len(in.IndicatorNotes) != len(r.Indicators) {
		return GaspolStageAssessmentRow{}, errors.New("INDICATOR_NOTES_REQUIRED")
	}
	for _, i := range r.Indicators {
		if _, ok := in.IndicatorNotes[i.Key]; !ok {
			return GaspolStageAssessmentRow{}, errors.New("INDICATOR_NOTES_REQUIRED")
		}
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("gaspol-phase:%d", in.Year)); err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	var closed bool
	closing := "BOTTOM_5"
	if in.Stage == "INNOVATION" {
		closing = "AWARDING"
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=$1 AND lock_type=$2 AND status='LOCKED') OR ($3='INNOVATION' AND EXISTS(SELECT 1 FROM gaspol_awards WHERE year=$1 AND award_type='INNOVATIVE'))`, in.Year, closing, in.Stage).Scan(&closed); err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	if closed {
		return GaspolStageAssessmentRow{}, errors.New("ASSESSMENT_ALREADY_SUBMITTED")
	}
	var eligible bool
	cohort := stageAssignmentCohort(in.Stage)
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.year=c.year AND l.status='LOCKED' WHERE c.year=$1 AND c.school_id=$2 AND c.cohort=$3 AND c.released_at IS NULL) AND EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE id=$4 AND is_active)`, in.Year, in.SchoolID, cohort, assignment).Scan(&eligible)
	if err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	if !eligible {
		return GaspolStageAssessmentRow{}, auth.ErrForbidden
	}
	var submitted bool
	if in.Stage == "INNOVATION" {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_innovation_submissions WHERE year=$1 AND school_id=$2 AND status='SUBMITTED')`, in.Year, in.SchoolID).Scan(&submitted)
	} else if in.Stage != GaspolStageNomination {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_nomination_submissions WHERE year=$1 AND school_id=$2 AND submission_type=$3 AND status IN ('SUBMITTED','LOCKED'))`, in.Year, in.SchoolID, in.Stage).Scan(&submitted)
	}
	if err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	if !submitted && in.Stage != GaspolStageNomination {
		return GaspolStageAssessmentRow{}, errors.New("SUBMISSION_NOT_SUBMITTED")
	}
	if score == 0 && strings.TrimSpace(in.Notes) == "" {
		return GaspolStageAssessmentRow{}, errors.New("ZERO_SCORE_REASON_REQUIRED")
	}
	snapshot, _ := json.Marshal(map[string]any{"version": r, "scores": in.Rubric, "indicatorNotes": in.IndicatorNotes})
	var row GaspolStageAssessmentRow
	err = tx.QueryRow(ctx, `INSERT INTO gaspol_stage_assessments(year,stage,month,school_id,assignment_id,assessor_user_id,raw_score,rubric,notes,workflow_status) VALUES($1,$2,0,$3,$4,$5,$6,$7::jsonb,$8,'WAITING_KOOR') ON CONFLICT(year,stage,month,school_id,assessor_user_id) DO UPDATE SET raw_score=EXCLUDED.raw_score,rubric=EXCLUDED.rubric,notes=EXCLUDED.notes,workflow_status='WAITING_KOOR',final_score=NULL,updated_at=NOW() WHERE gaspol_stage_assessments.workflow_status IN ('RETURNED_BY_KOOR','RETURNED_BY_CABDIN','RETURNED_BY_MASTER') AND (gaspol_stage_assessments.rubric->'version'->>'id')::bigint=$9 RETURNING id,year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status,notes,updated_at`, in.Year, in.Stage, in.SchoolID, assignment, u.ID, score, string(snapshot), in.Notes, r.ID).Scan(&row.ID, &row.Year, &row.Stage, &row.SchoolID, &row.AssessorUserID, &row.RawScore, &row.FinalScore, &row.WorkflowStatus, &row.Notes, &row.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errors.New("ASSESSMENT_ALREADY_SUBMITTED")
	}
	if err != nil {
		return row, err
	}
	if in.Stage == "INNOVATION" {
		if _, err = tx.Exec(ctx, `UPDATE gaspol_stage_assessments SET workflow_status='FINAL',final_score=raw_score,locked_at=NOW() WHERE id=$1`, row.ID); err != nil {
			return row, err
		}
		row.WorkflowStatus = "FINAL"
		row.FinalScore = &score
	}
	if err = gaspolAuditTx(ctx, tx, u, in.Year, "RUBRIC_ASSESSMENT_SUBMITTED", "stage_assessment", row.ID, &in.SchoolID, map[string]any{"rubric": r, "scores": in.Rubric, "indicatorNotes": in.IndicatorNotes, "score": score}); err != nil {
		return row, err
	}
	return row, tx.Commit(ctx)
}
