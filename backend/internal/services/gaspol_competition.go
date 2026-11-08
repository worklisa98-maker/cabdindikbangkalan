package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
)

const (
	GaspolWorkflowWaitingKoor    = "WAITING_KOOR"
	GaspolWorkflowInProgress     = "IN_PROGRESS"
	GaspolWorkflowKoorValidated  = "KOOR_VALIDATED"
	GaspolWorkflowWaitingCabdin  = "WAITING_CABDIN"
	GaspolWorkflowFinal          = "FINAL"
	GaspolWorkflowReturnedKoor   = "RETURNED_BY_KOOR"
	GaspolWorkflowReturnedCabdin = "RETURNED_BY_CABDIN"
	GaspolWorkflowReturnedMaster = "RETURNED_BY_MASTER"
)

var gaspolTwoLevelValidationStages = map[string]struct{}{
	GaspolStageMonthly: {}, GaspolStageField1: {}, GaspolStageNomination: {},
	GaspolStageAppeal: {},
}

// RequiresTwoLevelValidation is the single source of truth for the validation
// workflow. FIELD_STAGE_2 and PRESENTATION deliberately bypass both queues.
func RequiresTwoLevelValidation(stage string) bool {
	_, ok := gaspolTwoLevelValidationStages[normalizeCompetitionStage(stage)]
	return ok
}

func IsDirectFinalStage(stage string) bool {
	stage = normalizeCompetitionStage(stage)
	return stage == GaspolStageField2 || stage == GaspolStagePresentation
}

func gaspolValidationQueueStages() []string {
	return []string{GaspolStageMonthly, GaspolStageField1, GaspolStageNomination, GaspolStageAppeal, "INNOVATION"}
}

func DirectStageReady(requiredAssessors, submittedAssessors int) bool {
	return requiredAssessors > 0 && submittedAssessors == requiredAssessors
}

func NextGaspolReviewStatus(stage, current, level, decision string) (string, error) {
	if !RequiresTwoLevelValidation(stage) {
		return "", errors.New("INVALID_WORKFLOW_TRANSITION")
	}
	level, decision = strings.ToUpper(strings.TrimSpace(level)), strings.ToUpper(strings.TrimSpace(decision))
	switch {
	case level == "KOOR" && current == GaspolWorkflowWaitingKoor && decision == "APPROVE":
		return GaspolWorkflowWaitingCabdin, nil
	case level == "KOOR" && current == GaspolWorkflowWaitingKoor && decision == "RETURN":
		return GaspolWorkflowReturnedKoor, nil
	case level == "CABDIN" && current == GaspolWorkflowWaitingCabdin && decision == "APPROVE":
		return GaspolWorkflowFinal, nil
	case level == "CABDIN" && current == GaspolWorkflowWaitingCabdin && decision == "RETURN":
		return GaspolWorkflowReturnedCabdin, nil
	default:
		return "", errors.New("INVALID_WORKFLOW_TRANSITION")
	}
}

var gaspolStates = map[string]struct{}{
	"MONTHLY_ACTIVE": {}, "AUGUST_DIRECT_ASSESSMENT": {}, "SEPTEMBER_FIELD1_ASSESSMENT": {},
	"WAITING_STAGE1_VALIDATION": {}, "STAGE1_FINAL": {}, "TOP10_LOCKED": {}, "FIELD2_ACTIVE": {},
	"FIELD2_FINAL": {}, "TOP5_WAITING_BA": {}, "TOP5_LOCKED": {}, "PRESENTATION_ACTIVE": {},
	"PRESENTATION_FINAL": {}, "NOT_FAILED_NOMINATION_ACTIVE": {}, "NOT_FAILED_NOMINATION_FINAL": {},
	"BOTTOM5_LOCKED": {}, "APPEAL_ACTIVE": {}, "APPEAL_FINAL": {}, "FINAL_AWARD_READY": {}, "GASPOL_FINALIZED": {},
}

var gaspolStateTransitions = map[string]map[string]bool{
	"MONTHLY_ACTIVE": {"AUGUST_DIRECT_ASSESSMENT": true}, "AUGUST_DIRECT_ASSESSMENT": {"SEPTEMBER_FIELD1_ASSESSMENT": true},
	"SEPTEMBER_FIELD1_ASSESSMENT": {"WAITING_STAGE1_VALIDATION": true}, "WAITING_STAGE1_VALIDATION": {"STAGE1_FINAL": true},
	"STAGE1_FINAL": {"TOP10_LOCKED": true}, "TOP10_LOCKED": {"FIELD2_ACTIVE": true, "NOT_FAILED_NOMINATION_ACTIVE": true},
	"FIELD2_ACTIVE": {"FIELD2_FINAL": true}, "FIELD2_FINAL": {"TOP5_WAITING_BA": true}, "TOP5_WAITING_BA": {"TOP5_LOCKED": true},
	"TOP5_LOCKED": {"PRESENTATION_ACTIVE": true, "NOT_FAILED_NOMINATION_ACTIVE": true}, "PRESENTATION_ACTIVE": {"PRESENTATION_FINAL": true},
	"PRESENTATION_FINAL": {"NOT_FAILED_NOMINATION_ACTIVE": true, "FINAL_AWARD_READY": true}, "NOT_FAILED_NOMINATION_ACTIVE": {"NOT_FAILED_NOMINATION_FINAL": true},
	"NOT_FAILED_NOMINATION_FINAL": {"BOTTOM5_LOCKED": true}, "BOTTOM5_LOCKED": {"APPEAL_ACTIVE": true}, "APPEAL_ACTIVE": {"APPEAL_FINAL": true},
	"APPEAL_FINAL": {"FINAL_AWARD_READY": true}, "FINAL_AWARD_READY": {"GASPOL_FINALIZED": true},
}

func CanTransitionGaspolState(from, to string) bool {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))
	return from == to || gaspolStateTransitions[from][to]
}

type GaspolCompetitionConfig struct {
	Year                     int        `json:"year"`
	State                    string     `json:"state"`
	SeptemberField1Deadline  time.Time  `json:"septemberField1Deadline"`
	LikeThreshold            int        `json:"likeThreshold"`
	InternalVoterDenominator *int       `json:"internalVoterDenominator,omitempty"`
	VoterDenominatorLockedAt *time.Time `json:"voterDenominatorLockedAt,omitempty"`
	LikeSyncEventStart       *time.Time `json:"likeSyncEventStart,omitempty"`
	LikeCutoffAt             *time.Time `json:"likeCutoffAt,omitempty"`
	PublicFullRankingEnabled bool       `json:"publicFullRankingEnabled"`
}

type GaspolCompetitionConfigInput struct {
	Year                     int        `json:"year"`
	State                    string     `json:"state"`
	SeptemberField1Deadline  *time.Time `json:"septemberField1Deadline"`
	LikeThreshold            int        `json:"likeThreshold"`
	InternalVoterDenominator *int       `json:"internalVoterDenominator"`
	LikeSyncEventStart       *time.Time `json:"likeSyncEventStart"`
	LikeCutoffAt             *time.Time `json:"likeCutoffAt"`
	PublicFullRankingEnabled bool       `json:"publicFullRankingEnabled"`
}

type GaspolStageAssignmentInput struct {
	Year           int    `json:"year"`
	Stage          string `json:"stage"`
	SchoolID       *int   `json:"schoolId"`
	AssessorUserID int    `json:"assessorUserId"`
	AssignmentRole string `json:"assignmentRole"`
	IsActive       *bool  `json:"isActive,omitempty"`
}

type GaspolBulkStageAssignmentInput struct {
	Year           int    `json:"year"`
	Stage          string `json:"stage"`
	SchoolIDs      []int  `json:"schoolIds"`
	AssessorUserID int    `json:"assessorUserId"`
	AssignmentRole string `json:"assignmentRole"`
	IsActive       *bool  `json:"isActive,omitempty"`
}

type GaspolBulkStageAssignmentError struct {
	SchoolID int    `json:"schoolId"`
	Error    string `json:"error"`
}

type GaspolBulkStageAssignmentResult struct {
	Created         []int                            `json:"created"`
	SkippedExisting []int                            `json:"skippedExisting"`
	Removed         []int                            `json:"removed"`
	Errors          []GaspolBulkStageAssignmentError `json:"errors"`
}

type GaspolStageAssessmentInput struct {
	Year           int                `json:"year"`
	Stage          string             `json:"stage"`
	Month          int                `json:"month"`
	SchoolID       int                `json:"schoolId"`
	Score          float64            `json:"score"`
	Rubric         map[string]float64 `json:"rubric"`
	RubricID       int64              `json:"rubricId"`
	IndicatorNotes map[string]string  `json:"indicatorNotes"`
	Notes          string             `json:"notes"`
}

type GaspolStageAssessmentRow struct {
	ID             int64     `json:"id"`
	Year           int       `json:"year"`
	Stage          string    `json:"stage"`
	SchoolID       int       `json:"schoolId"`
	SchoolName     string    `json:"schoolName"`
	AssessorUserID int       `json:"assessorUserId"`
	AssessorName   string    `json:"assessorName"`
	RawScore       float64   `json:"rawScore"`
	FinalScore     *float64  `json:"finalScore,omitempty"`
	WorkflowStatus string    `json:"workflowStatus"`
	Notes          string    `json:"notes"`
	ReviewerNotes  string    `json:"reviewerNotes"`
	SubmittedAt    time.Time `json:"submittedAt"`
}

func (s GaspolService) isKoorGaspol(ctx context.Context, year, userID int) (bool, error) {
	var assigned bool
	err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$2 AND assignment_role='KOOR_GASPOL' AND is_active=TRUE)`, year, userID).Scan(&assigned)
	return assigned, err
}

func (s GaspolService) ListStageAssessmentQueue(ctx context.Context, year int, status string, validatorUserID *int) ([]GaspolStageAssessmentRow, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT a.id,a.year,a.stage,a.school_id,ds.nama_sekolah,a.assessor_user_id,
		       COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username),a.raw_score,a.final_score,
		       a.workflow_status,a.notes,CASE WHEN a.workflow_status='WAITING_CABDIN' THEN a.koor_notes ELSE a.cabdin_notes END,a.updated_at
		FROM gaspol_stage_assessments a
		JOIN data_sekolah ds ON ds.id=a.school_id
		JOIN users u ON u.id=a.assessor_user_id
		LEFT JOIN gaspol_school_participants p ON p.year=a.year AND p.school_id=a.school_id
		  AND p.is_active=TRUE AND p.is_year_configuration=FALSE
		WHERE a.year=$1 AND a.workflow_status=$2 AND a.stage=ANY($3)
		  AND ($4::integer IS NULL OR p.validator_user_id=$4)
		ORDER BY a.updated_at,ds.nama_sekolah,a.stage
	`, year, status, gaspolValidationQueueStages(), validatorUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []GaspolStageAssessmentRow{}
	for rows.Next() {
		var item GaspolStageAssessmentRow
		if err = rows.Scan(&item.ID, &item.Year, &item.Stage, &item.SchoolID, &item.SchoolName, &item.AssessorUserID, &item.AssessorName, &item.RawScore, &item.FinalScore, &item.WorkflowStatus, &item.Notes, &item.ReviewerNotes, &item.SubmittedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s GaspolService) CountUnassignedCabdinAssessments(ctx context.Context, year int) (int, error) {
	var count int
	err := s.DB.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM gaspol_stage_assessments a
		LEFT JOIN gaspol_school_participants p ON p.year=a.year AND p.school_id=a.school_id
		 AND p.is_active=TRUE AND p.is_year_configuration=FALSE
		WHERE a.year=$1 AND a.workflow_status='WAITING_CABDIN' AND a.stage=ANY($2)
		 AND p.validator_user_id IS NULL
	`, year, gaspolValidationQueueStages()).Scan(&count)
	return count, err
}

type GaspolReviewInput struct {
	Decision string `json:"decision"`
	Notes    string `json:"notes"`
}

type GaspolMasterReturnInput struct {
	Notes string `json:"notes"`
}

type GaspolManualLikeInput struct {
	Year        int    `json:"year"`
	SchoolID    int    `json:"schoolId"`
	LikeCount   int    `json:"likeCount"`
	Reason      string `json:"reason"`
	EvidenceURL string `json:"evidenceUrl"`
}

type GaspolBADocument struct {
	ID          int64     `json:"id"`
	Year        int       `json:"year"`
	FileName    string    `json:"fileName"`
	ContentType string    `json:"contentType"`
	SHA256      string    `json:"sha256"`
	IsPublic    bool      `json:"isPublic"`
	UploadedAt  time.Time `json:"uploadedAt"`
}

type GaspolNominationSubmissionInput struct {
	Year               int      `json:"year"`
	SubmissionType     string   `json:"submissionType"`
	ImprovementDetails string   `json:"improvementDetails"`
	ImprovementResult  string   `json:"improvementResult"`
	ResponseText       string   `json:"responseText"`
	DriveURL           string   `json:"driveUrl"`
	EvidenceURLs       []string `json:"evidenceUrls"`
	Status             string   `json:"status"`
}

func (s GaspolService) SaveNominationSubmission(ctx context.Context, user auth.User, input GaspolNominationSubmissionInput) error {
	input.DriveURL = normalizeGaspolEvidenceURL(input.DriveURL)
	links := []string{}
	for _, link := range input.EvidenceURLs {
		if strings.TrimSpace(link) != "" {
			links = append(links, normalizeGaspolEvidenceURL(link))
		}
	}
	input.EvidenceURLs = links

	if err := validateNominationInput(input); err != nil {
		return err
	}
	if err := s.ensurePhaseSchool(ctx, user, input.Year); err != nil {
		return err
	}
	year := input.Year
	kind := strings.ToUpper(strings.TrimSpace(input.SubmissionType))
	cohort, closing := "NOT_FAILED", "BOTTOM_5"
	if kind == GaspolStageAppeal {
		cohort, closing = "BOTTOM_5", "FINAL"
	} else if kind != GaspolStageNomination {
		return errors.New("INVALID_STAGE")
	}
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		status = "DRAFT"
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Shared with official result locks: no report can slip past a closing result.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "gaspol-phase:"+strconv.Itoa(year)); err != nil {
		return err
	}
	var locked, eligible bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_nomination_submissions WHERE year=$1 AND school_id=$2 AND submission_type=$3 AND status='LOCKED')`, year, *user.SchoolID, kind).Scan(&locked); err != nil {
		return err
	}
	if locked {
		return errors.New("SUBMISSION_LOCKED")
	}
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.year=c.year AND l.status='LOCKED' WHERE c.year=$1 AND c.school_id=$2 AND c.cohort=$3 AND c.released_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=$1 AND lock_type=$4 AND status='LOCKED')`, year, *user.SchoolID, cohort, closing).Scan(&eligible); err != nil {
		return err
	}
	if !eligible {
		return errors.New("SCHOOL_NOT_ELIGIBLE")
	}
	evidence, _ := json.Marshal(input.EvidenceURLs)
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO gaspol_nomination_submissions(year,school_id,submission_type,improvement_details,improvement_result,response_text,drive_url,evidence_urls,status,submitted_by,submitted_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,CASE WHEN $9='SUBMITTED' THEN NOW() END)
 ON CONFLICT(year,school_id,submission_type) DO UPDATE SET improvement_details=EXCLUDED.improvement_details,improvement_result=EXCLUDED.improvement_result,response_text=EXCLUDED.response_text,drive_url=EXCLUDED.drive_url,evidence_urls=EXCLUDED.evidence_urls,status=EXCLUDED.status,submitted_by=EXCLUDED.submitted_by,submitted_at=CASE WHEN EXCLUDED.status='SUBMITTED' THEN NOW() ELSE gaspol_nomination_submissions.submitted_at END,updated_at=NOW()
 WHERE gaspol_nomination_submissions.status<>'LOCKED' RETURNING id`, year, *user.SchoolID, kind, strings.TrimSpace(input.ImprovementDetails), strings.TrimSpace(input.ImprovementResult), strings.TrimSpace(input.ResponseText), strings.TrimSpace(input.DriveURL), string(evidence), status, user.ID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("SUBMISSION_LOCKED")
	}
	if err != nil {
		return err
	}
	auditPayload, _ := json.Marshal(input)
	if _, err = tx.Exec(ctx, `INSERT INTO gaspol_audit_logs(year,actor_user_id,actor_role,action,entity_type,entity_id,school_id,payload) VALUES($1,$2,$3,$4,'nomination_submission',$5,$6,$7::jsonb)`, year, user.ID, user.Role, "NOMINATION_SUBMISSION_"+status, id, *user.SchoolID, string(auditPayload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s GaspolService) SchoolCompetitionCohorts(ctx context.Context, year, schoolID int) ([]string, string, error) {
	rows, err := s.DB.Pool.Query(ctx, `SELECT c.cohort FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.year=c.year AND l.status='LOCKED' WHERE c.year=$1 AND c.school_id=$2 AND c.released_at IS NULL ORDER BY l.locked_at DESC,c.id DESC`, year, schoolID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	cohorts := []string{}
	notice := ""
	for rows.Next() {
		var cohort string
		if err = rows.Scan(&cohort); err != nil {
			return nil, "", err
		}
		cohorts = append(cohorts, cohort)
		if notice == "" {
			notice, _ = gaspolNoticeContent(cohort)
		}
	}
	return cohorts, notice, rows.Err()
}

func normalizeCompetitionStage(stage string) string { return strings.ToUpper(strings.TrimSpace(stage)) }

func stageAssignmentRole(stage string) string {
	return map[string]string{
		GaspolStageMonthly:      "SUPERVISOR_MONTHLY",
		GaspolStageField1:       "FIELD_STAGE_1_ASSESSOR",
		GaspolStageField2:       "FIELD_STAGE_2_ASSESSOR",
		GaspolStagePresentation: "PRESENTATION_ASSESSOR",
		GaspolStageNomination:   "NOMINATION_ASSESSOR",
		GaspolStageAppeal:       "APPEAL_PANEL",
		"INNOVATION":            "INNOVATION_ASSESSOR",
	}[normalizeCompetitionStage(stage)]
}

func stageAssignmentCohort(stage string) string {
	return map[string]string{
		GaspolStageField2:       "TOP_10",
		GaspolStagePresentation: "TOP_5",
		GaspolStageNomination:   "NOT_FAILED",
		GaspolStageAppeal:       "BOTTOM_5",
		"INNOVATION":            "TOP_5",
	}[normalizeCompetitionStage(stage)]
}

func bulkAssignmentOperation(active, exists bool) string {
	if active && !exists {
		return "CREATE"
	}
	if !active && exists {
		return "REMOVE"
	}
	return "SKIP"
}

func bulkStageAssignmentAuditAction(active bool) string {
	if active {
		return "STAGE_ASSIGNMENT_BULK_ASSIGNED"
	}
	return "STAGE_ASSIGNMENT_BULK_UNASSIGNED"
}

func (s GaspolService) CompetitionConfig(ctx context.Context, year int) (GaspolCompetitionConfig, error) {
	year = normalizeGaspolYear(year)
	_, err := s.DB.Pool.Exec(ctx, `INSERT INTO gaspol_competition_configs(year, september_field1_deadline) VALUES ($1, (make_date($1,10,15)::timestamp + time '23:59:59') AT TIME ZONE 'Asia/Jakarta') ON CONFLICT DO NOTHING`, year)
	if err != nil {
		return GaspolCompetitionConfig{}, err
	}
	var row GaspolCompetitionConfig
	err = s.DB.Pool.QueryRow(ctx, `SELECT year,state,september_field1_deadline,like_threshold,internal_voter_denominator,voter_denominator_locked_at,like_sync_event_start,like_cutoff_at,public_full_ranking_enabled FROM gaspol_competition_configs WHERE year=$1`, year).Scan(&row.Year, &row.State, &row.SeptemberField1Deadline, &row.LikeThreshold, &row.InternalVoterDenominator, &row.VoterDenominatorLockedAt, &row.LikeSyncEventStart, &row.LikeCutoffAt, &row.PublicFullRankingEnabled)
	return row, err
}

func (s GaspolService) UpdateCompetitionConfig(ctx context.Context, user auth.User, input GaspolCompetitionConfigInput) (GaspolCompetitionConfig, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolCompetitionConfig{}, err
	}
	year := normalizeGaspolYear(input.Year)
	state := strings.ToUpper(strings.TrimSpace(input.State))
	if _, ok := gaspolStates[state]; !ok {
		return GaspolCompetitionConfig{}, errors.New("INVALID_STATE")
	}
	current, err := s.CompetitionConfig(ctx, year)
	if err != nil {
		return GaspolCompetitionConfig{}, err
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return GaspolCompetitionConfig{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT year,state,september_field1_deadline,like_threshold,internal_voter_denominator,voter_denominator_locked_at,like_sync_event_start,like_cutoff_at,public_full_ranking_enabled FROM gaspol_competition_configs WHERE year=$1 FOR UPDATE`, year).Scan(&current.Year, &current.State, &current.SeptemberField1Deadline, &current.LikeThreshold, &current.InternalVoterDenominator, &current.VoterDenominatorLockedAt, &current.LikeSyncEventStart, &current.LikeCutoffAt, &current.PublicFullRankingEnabled)
	if err != nil {
		return GaspolCompetitionConfig{}, err
	}
	if input.PublicFullRankingEnabled && !current.PublicFullRankingEnabled {
		var count int
		if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_ranking_snapshots_v2 WHERE year=$1 AND snapshot_type='STANDINGS'`, year).Scan(&count); err != nil {
			return GaspolCompetitionConfig{}, err
		}
		if count != 21 {
			return GaspolCompetitionConfig{}, errors.New("INCOMPLETE_OFFICIAL_ZONES")
		}
	}
	if !CanTransitionGaspolState(current.State, state) {
		return GaspolCompetitionConfig{}, errors.New("INVALID_STATE_TRANSITION")
	}
	deadline := current.SeptemberField1Deadline
	if input.SeptemberField1Deadline != nil {
		deadline = *input.SeptemberField1Deadline
	}
	threshold := input.LikeThreshold
	if threshold <= 0 {
		threshold = current.LikeThreshold
	}
	if current.VoterDenominatorLockedAt != nil && input.InternalVoterDenominator != nil && (current.InternalVoterDenominator == nil || *input.InternalVoterDenominator != *current.InternalVoterDenominator) {
		return GaspolCompetitionConfig{}, errors.New("VOTER_DENOMINATOR_LOCKED")
	}
	if current.VoterDenominatorLockedAt != nil {
		if threshold != 800 || (input.LikeCutoffAt != nil && (current.LikeCutoffAt == nil || !input.LikeCutoffAt.Equal(*current.LikeCutoffAt))) {
			return GaspolCompetitionConfig{}, errors.New("VOTING_ROSTER_LOCKED")
		}
		input.InternalVoterDenominator = current.InternalVoterDenominator
		input.LikeCutoffAt = current.LikeCutoffAt
	}
	_, err = tx.Exec(ctx, `UPDATE gaspol_competition_configs SET state=$2,september_field1_deadline=$3,like_threshold=$4,internal_voter_denominator=$5,like_sync_event_start=$6,like_cutoff_at=$7,public_full_ranking_enabled=$8,updated_by=$9,updated_at=NOW() WHERE year=$1`, year, state, deadline, threshold, input.InternalVoterDenominator, input.LikeSyncEventStart, input.LikeCutoffAt, input.PublicFullRankingEnabled, user.ID)
	if err != nil {
		return GaspolCompetitionConfig{}, err
	}
	if err = gaspolAuditTx(ctx, tx, user, year, "COMPETITION_CONFIG_UPDATED", "competition_config", 0, nil, input); err != nil {
		return GaspolCompetitionConfig{}, err
	}
	if input.PublicFullRankingEnabled != current.PublicFullRankingEnabled {
		if err = gaspolAuditTx(ctx, tx, user, year, "PUBLICATION_CHANGED", "competition_config", 0, nil, map[string]any{"enabled": input.PublicFullRankingEnabled}); err != nil {
			return GaspolCompetitionConfig{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return GaspolCompetitionConfig{}, err
	}
	return s.CompetitionConfig(ctx, year)
}

func (s GaspolService) AssignCompetitionStage(ctx context.Context, user auth.User, input GaspolStageAssignmentInput) error {
	if err := s.ensureAdminMaster(user); err != nil {
		return err
	}
	stage := normalizeCompetitionStage(input.Stage)
	assignmentRole := strings.ToUpper(strings.TrimSpace(input.AssignmentRole))
	switch stage {
	case GaspolStageMonthly, GaspolStageField1, GaspolStageField2, GaspolStagePresentation, GaspolStageNomination, GaspolStageAppeal, "INNOVATION":
	default:
		return errors.New("INVALID_STAGE")
	}
	if input.AssessorUserID <= 0 || input.Year < 2024 || assignmentRole == "" {
		return errors.New("INVALID_PAYLOAD")
	}
	if assignmentRole == "KASI_SMK_APPROVER" {
		if stage != "INNOVATION" {
			return errors.New("INVALID_STAGE")
		}
		meta, e := s.Repo.GetUserMeta(ctx, input.AssessorUserID)
		if e != nil {
			return e
		}
		if meta.SchoolID != nil {
			return errors.New("USER_NOT_ASSIGNABLE")
		}
		active := input.IsActive == nil || *input.IsActive
		tx, e := s.DB.Pool.Begin(ctx)
		if e != nil {
			return e
		}
		defer tx.Rollback(ctx)
		if _, e = tx.Exec(ctx, `LOCK TABLE gaspol_stage_assignments IN SHARE ROW EXCLUSIVE MODE`); e != nil {
			return e
		}
		if active {
			if _, e = tx.Exec(ctx, `UPDATE gaspol_stage_assignments SET is_active=false,updated_at=NOW() WHERE year=$1 AND assignment_role='KASI_SMK_APPROVER' AND assessor_user_id<>$2 AND is_active`, input.Year, input.AssessorUserID); e != nil {
				return e
			}
		}
		tag, e := tx.Exec(ctx, `UPDATE gaspol_stage_assignments SET is_active=$3,updated_at=NOW(),assigned_by=$4 WHERE year=$1 AND assessor_user_id=$2 AND assignment_role='KASI_SMK_APPROVER'`, input.Year, input.AssessorUserID, active, user.ID)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 && active {
			if _, e = tx.Exec(ctx, `INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role,assigned_by) VALUES($1,'INNOVATION',NULL,$2,'KASI_SMK_APPROVER',$3)`, input.Year, input.AssessorUserID, user.ID); e != nil {
				return e
			}
		}
		if e = gaspolAuditTx(ctx, tx, user, input.Year, "KASI_ASSIGNMENT_UPDATED", "stage_assignment", 0, nil, input); e != nil {
			return e
		}
		return tx.Commit(ctx)
	}
	if assignmentRole == "KOOR_GASPOL" {
		meta, err := s.Repo.GetUserMeta(ctx, input.AssessorUserID)
		if err != nil {
			return err
		}
		if auth.NormalizeRole(meta.BaseRole) != auth.RolePengawas {
			return errors.New("INVALID_KOOR_CANDIDATE")
		}
		var previousID int
		var previousName string
		previousErr := s.DB.Pool.QueryRow(ctx, `SELECT a.assessor_user_id,COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username) FROM gaspol_stage_assignments a JOIN users u ON u.id=a.assessor_user_id WHERE a.year=$1 AND a.assignment_role='KOOR_GASPOL' AND a.is_active=TRUE ORDER BY a.updated_at DESC LIMIT 1`, input.Year).Scan(&previousID, &previousName)
		if previousErr != nil && !errors.Is(previousErr, pgx.ErrNoRows) {
			return previousErr
		}
		active := input.IsActive == nil || *input.IsActive
		tx, err := s.DB.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if active {
			_, err = tx.Exec(ctx, `UPDATE gaspol_stage_assignments SET is_active=FALSE,updated_at=NOW() WHERE year=$1 AND assignment_role='KOOR_GASPOL' AND is_active=TRUE AND assessor_user_id<>$2`, input.Year, input.AssessorUserID)
			if err == nil {
				tag, execErr := tx.Exec(ctx, `UPDATE gaspol_stage_assignments SET stage=$2,is_active=TRUE,assigned_by=$4,updated_at=NOW() WHERE id=(SELECT id FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$3 AND assignment_role='KOOR_GASPOL' ORDER BY updated_at DESC LIMIT 1)`, input.Year, stage, input.AssessorUserID, user.ID)
				err = execErr
				if err == nil && tag.RowsAffected() == 0 {
					_, err = tx.Exec(ctx, `INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role,is_active,assigned_by) VALUES($1,$2,NULL,$3,'KOOR_GASPOL',TRUE,$4)`, input.Year, stage, input.AssessorUserID, user.ID)
				}
			}
		} else {
			_, err = tx.Exec(ctx, `UPDATE gaspol_stage_assignments SET is_active=FALSE,assigned_by=$3,updated_at=NOW() WHERE year=$1 AND assessor_user_id=$2 AND assignment_role='KOOR_GASPOL' AND is_active=TRUE`, input.Year, input.AssessorUserID, user.ID)
		}
		if err != nil {
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		action := "KOOR_GASPOL_UNASSIGNED"
		if active {
			action = "KOOR_GASPOL_ASSIGNED"
			if previousID > 0 && previousID != input.AssessorUserID {
				action = "KOOR_GASPOL_REPLACED"
			}
		}
		return s.Repo.LogAudit(ctx, &input.Year, user.ID, user.Role, action, "stage_assignment", nil, nil, map[string]any{"assessorUserId": input.AssessorUserID, "assessorName": meta.DisplayName, "previousAssessorUserId": previousID, "previousAssessorName": previousName, "stage": stage})
	}
	if stageAssignmentRole(stage) != assignmentRole || input.SchoolID == nil || *input.SchoolID <= 0 {
		return errors.New("INVALID_PAYLOAD")
	}
	active := input.IsActive == nil || *input.IsActive
	if active {
		if err := s.ensureStageSchoolEligible(ctx, input.Year, stage, *input.SchoolID); err != nil {
			return err
		}
		if err := s.ensureStageAssessorEligible(ctx, input.Year, assignmentRole, input.AssessorUserID); err != nil {
			return err
		}
	}
	var err error
	if active {
		_, err = s.DB.Pool.Exec(ctx, `INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role,is_active,assigned_by) VALUES($1,$2,$3,$4,$5,TRUE,$6) ON CONFLICT(year,stage,school_id,assessor_user_id) DO UPDATE SET assignment_role=EXCLUDED.assignment_role,is_active=TRUE,assigned_by=EXCLUDED.assigned_by,updated_at=NOW()`, input.Year, stage, input.SchoolID, input.AssessorUserID, assignmentRole, user.ID)
	} else {
		_, err = s.DB.Pool.Exec(ctx, `UPDATE gaspol_stage_assignments SET is_active=FALSE,assigned_by=$6,updated_at=NOW() WHERE year=$1 AND stage=$2 AND school_id=$3 AND assessor_user_id=$4 AND assignment_role=$5`, input.Year, stage, input.SchoolID, input.AssessorUserID, assignmentRole, user.ID)
	}
	if err == nil {
		action := "STAGE_ASSIGNMENT_UNASSIGNED"
		if active {
			action = "STAGE_ASSIGNMENT_SAVED"
		}
		_ = s.Repo.LogAudit(ctx, &input.Year, user.ID, user.Role, action, "stage_assignment", nil, input.SchoolID, input)
	}
	return err
}

// BulkAssignCompetitionStage reuses the single-assignment eligibility and
// persistence rules while reporting duplicates and invalid schools per item.
func (s GaspolService) BulkAssignCompetitionStage(ctx context.Context, user auth.User, input GaspolBulkStageAssignmentInput) (GaspolBulkStageAssignmentResult, error) {
	result := GaspolBulkStageAssignmentResult{
		Created: []int{}, SkippedExisting: []int{}, Removed: []int{}, Errors: []GaspolBulkStageAssignmentError{},
	}
	if err := s.ensureAdminMaster(user); err != nil {
		return result, err
	}
	stage := normalizeCompetitionStage(input.Stage)
	assignmentRole := strings.ToUpper(strings.TrimSpace(input.AssignmentRole))
	if input.Year < 2024 || input.AssessorUserID <= 0 || len(input.SchoolIDs) == 0 || stageAssignmentRole(stage) != assignmentRole {
		return result, errors.New("INVALID_PAYLOAD")
	}
	active := input.IsActive == nil || *input.IsActive
	if active {
		if err := s.ensureStageAssessorEligible(ctx, input.Year, assignmentRole, input.AssessorUserID); err != nil {
			return result, err
		}
	}

	seen := make(map[int]struct{}, len(input.SchoolIDs))
	eligibleSchoolIDs := make([]int, 0, len(input.SchoolIDs))
	for _, schoolID := range input.SchoolIDs {
		if schoolID <= 0 {
			result.Errors = append(result.Errors, GaspolBulkStageAssignmentError{SchoolID: schoolID, Error: "INVALID_SCHOOL_ID"})
			continue
		}
		if _, exists := seen[schoolID]; exists {
			result.SkippedExisting = append(result.SkippedExisting, schoolID)
			continue
		}
		seen[schoolID] = struct{}{}
		if active {
			if err := s.ensureStageSchoolEligible(ctx, input.Year, stage, schoolID); err != nil {
				result.Errors = append(result.Errors, GaspolBulkStageAssignmentError{SchoolID: schoolID, Error: err.Error()})
				continue
			}
		}
		eligibleSchoolIDs = append(eligibleSchoolIDs, schoolID)
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	changedSchoolIDs := make([]int, 0, len(eligibleSchoolIDs))
	for _, schoolID := range eligibleSchoolIDs {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND stage=$2 AND school_id=$3 AND assessor_user_id=$4 AND assignment_role=$5 AND is_active=TRUE)`, input.Year, stage, schoolID, input.AssessorUserID, assignmentRole).Scan(&exists); err != nil {
			return result, err
		}
		operation := bulkAssignmentOperation(active, exists)
		if operation == "SKIP" {
			result.SkippedExisting = append(result.SkippedExisting, schoolID)
			continue
		}
		if operation == "CREATE" {
			_, err = tx.Exec(ctx, `INSERT INTO gaspol_stage_assignments(year,stage,school_id,assessor_user_id,assignment_role,is_active,assigned_by) VALUES($1,$2,$3,$4,$5,TRUE,$6) ON CONFLICT(year,stage,school_id,assessor_user_id) DO UPDATE SET assignment_role=EXCLUDED.assignment_role,is_active=TRUE,assigned_by=EXCLUDED.assigned_by,updated_at=NOW()`, input.Year, stage, schoolID, input.AssessorUserID, assignmentRole, user.ID)
			if err == nil {
				result.Created = append(result.Created, schoolID)
			}
		} else {
			_, err = tx.Exec(ctx, `UPDATE gaspol_stage_assignments SET is_active=FALSE,assigned_by=$6,updated_at=NOW() WHERE year=$1 AND stage=$2 AND school_id=$3 AND assessor_user_id=$4 AND assignment_role=$5 AND is_active=TRUE`, input.Year, stage, schoolID, input.AssessorUserID, assignmentRole, user.ID)
			if err == nil {
				result.Removed = append(result.Removed, schoolID)
			}
		}
		if err != nil {
			return result, err
		}
		changedSchoolIDs = append(changedSchoolIDs, schoolID)
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	action := bulkStageAssignmentAuditAction(active)
	for _, schoolID := range changedSchoolIDs {
		id := schoolID
		_ = s.Repo.LogAudit(ctx, &input.Year, user.ID, user.Role, action, "stage_assignment", nil, &id, map[string]any{
			"assessorUserId": input.AssessorUserID, "schoolId": schoolID, "stage": stage, "assignmentRole": assignmentRole,
		})
	}
	return result, nil
}

func (s GaspolService) ensureStageSchoolEligible(ctx context.Context, year int, stage string, schoolID int) error {
	var eligible bool
	if stage == GaspolStageField1 || stage == GaspolStageMonthly {
		if err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_school_participants WHERE year=$1 AND school_id=$2 AND is_active=TRUE AND is_year_configuration=FALSE)`, year, schoolID).Scan(&eligible); err != nil {
			return err
		}
	} else {
		cohort := stageAssignmentCohort(stage)
		if err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_cohorts WHERE year=$1 AND school_id=$2 AND cohort=$3 AND released_at IS NULL)`, year, schoolID, cohort).Scan(&eligible); err != nil {
			return err
		}
	}
	if !eligible {
		return errors.New("SCHOOL_NOT_ELIGIBLE")
	}
	return nil
}

func (s GaspolService) ensureStageAssessorEligible(ctx context.Context, year int, assignmentRole string, userID int) error {
	meta, err := s.Repo.GetUserMeta(ctx, userID)
	if err != nil {
		return err
	}
	if meta.SchoolID != nil {
		return errors.New("USER_NOT_ASSIGNABLE")
	}
	requireGaspolRole := func(role string) (bool, error) {
		return s.Repo.UserHasRole(ctx, year, userID, role, nil)
	}
	switch assignmentRole {
	case "FIELD_STAGE_1_ASSESSOR", "NOMINATION_ASSESSOR", "INNOVATION_ASSESSOR", "SUPERVISOR_MONTHLY":
		ok, roleErr := requireGaspolRole(GaspolRoleSupervisor)
		if roleErr != nil {
			return roleErr
		}
		if !ok || auth.NormalizeRole(meta.BaseRole) != auth.RolePengawas {
			return errors.New("USER_NOT_ASSIGNABLE")
		}
	case "FIELD_STAGE_2_ASSESSOR":
		ok, roleErr := requireGaspolRole(GaspolRoleJury)
		if roleErr != nil {
			return roleErr
		}
		if !ok {
			return errors.New("USER_NOT_ASSIGNABLE")
		}
	case "PRESENTATION_ASSESSOR", "APPEAL_PANEL":
		jury, juryErr := requireGaspolRole(GaspolRoleJury)
		if juryErr != nil {
			return juryErr
		}
		supervisor, supervisorErr := requireGaspolRole(GaspolRoleSupervisor)
		if supervisorErr != nil {
			return supervisorErr
		}
		role := auth.NormalizeRole(meta.BaseRole)
		if !jury && !supervisor && role != auth.RoleAdminCabdin && role != auth.RoleAdminMaster && role != auth.RolePengawas {
			return errors.New("USER_NOT_ASSIGNABLE")
		}
	default:
		return errors.New("INVALID_ROLE")
	}
	return nil
}

func (s GaspolService) SaveStageAssessment(ctx context.Context, user auth.User, input GaspolStageAssessmentInput) (GaspolStageAssessmentRow, error) {
	stage := normalizeCompetitionStage(input.Stage)
	switch stage {
	case GaspolStageMonthly, GaspolStageField1, GaspolStageField2, GaspolStagePresentation, GaspolStageNomination, GaspolStageAppeal, "INNOVATION":
	default:
		return GaspolStageAssessmentRow{}, errors.New("INVALID_STAGE")
	}
	if input.Year < 2024 || input.SchoolID <= 0 || input.Score < 0 || input.Score > 100 {
		return GaspolStageAssessmentRow{}, errors.New("INVALID_PAYLOAD")
	}
	switch stage {
	case GaspolStageMonthly:
		if input.Month < 4 || input.Month > 9 {
			return GaspolStageAssessmentRow{}, errors.New("INVALID_MONTH")
		}
	case GaspolStageField2:
		input.Month = 10
	case GaspolStagePresentation:
		input.Month = 11
	default:
		input.Month = 0
	}
	if stage == GaspolStageField1 || (stage == GaspolStageMonthly && input.Month == 9) {
		config, configErr := s.CompetitionConfig(ctx, input.Year)
		if configErr != nil {
			return GaspolStageAssessmentRow{}, configErr
		}
		now, clockErr := s.Clock.Now(ctx)
		if clockErr != nil {
			return GaspolStageAssessmentRow{}, clockErr
		}
		if now.After(config.SeptemberField1Deadline) {
			var returned bool
			if queryErr := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assessments WHERE year=$1 AND stage=$2 AND month=$3 AND school_id=$4 AND assessor_user_id=$5 AND workflow_status IN ('RETURNED_BY_KOOR','RETURNED_BY_CABDIN','RETURNED_BY_MASTER'))`, input.Year, stage, input.Month, input.SchoolID, user.ID).Scan(&returned); queryErr != nil {
				return GaspolStageAssessmentRow{}, queryErr
			}
			if !returned {
				return GaspolStageAssessmentRow{}, errors.New("ASSESSMENT_DEADLINE_PASSED")
			}
		}
	}
	var assignmentID int64
	expectedAssignmentRole := map[string]string{
		GaspolStageMonthly: "SUPERVISOR_MONTHLY", GaspolStageField1: "FIELD_STAGE_1_ASSESSOR",
		GaspolStageField2: "FIELD_STAGE_2_ASSESSOR", GaspolStagePresentation: "PRESENTATION_ASSESSOR",
		GaspolStageNomination: "NOMINATION_ASSESSOR", GaspolStageAppeal: "APPEAL_PANEL", "INNOVATION": "INNOVATION_ASSESSOR",
	}[stage]
	err := s.DB.Pool.QueryRow(ctx, `SELECT id FROM gaspol_stage_assignments WHERE year=$1 AND stage=$2 AND assessor_user_id=$3 AND is_active=TRUE AND school_id=$4 AND assignment_role=$5 ORDER BY updated_at DESC LIMIT 1`, input.Year, stage, user.ID, input.SchoolID, expectedAssignmentRole).Scan(&assignmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GaspolStageAssessmentRow{}, auth.ErrForbidden
		}
		return GaspolStageAssessmentRow{}, err
	}
	if stage == GaspolStageNomination || stage == "INNOVATION" {
		return s.saveRubricAssessment(ctx, user, input, assignmentID)
	}
	rubric, _ := json.Marshal(input.Rubric)
	if RequiresTwoLevelValidation(stage) {
		var previousScore float64
		var previousStatus string
		hadPrevious := false
		previousErr := s.DB.Pool.QueryRow(ctx, `SELECT raw_score,workflow_status FROM gaspol_stage_assessments WHERE year=$1 AND stage=$2 AND month=$3 AND school_id=$4 AND assessor_user_id=$5`, input.Year, stage, input.Month, input.SchoolID, user.ID).Scan(&previousScore, &previousStatus)
		if previousErr == nil {
			hadPrevious = true
		} else if !errors.Is(previousErr, pgx.ErrNoRows) {
			return GaspolStageAssessmentRow{}, previousErr
		}
		var row GaspolStageAssessmentRow
		err = s.DB.Pool.QueryRow(ctx, `
			INSERT INTO gaspol_stage_assessments(year,stage,month,school_id,assignment_id,assessor_user_id,raw_score,rubric,notes,workflow_status)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,'WAITING_KOOR')
			ON CONFLICT(year,stage,month,school_id,assessor_user_id) DO UPDATE SET
			 raw_score=EXCLUDED.raw_score,
			 rubric=CASE WHEN EXCLUDED.rubric='{}'::jsonb THEN gaspol_stage_assessments.rubric ELSE EXCLUDED.rubric END,
			 notes=EXCLUDED.notes,
			 workflow_status='WAITING_KOOR',final_score=NULL,updated_at=NOW()
			WHERE gaspol_stage_assessments.workflow_status IN ('RETURNED_BY_KOOR','RETURNED_BY_CABDIN','RETURNED_BY_MASTER')
			RETURNING id,year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status,notes,updated_at
		`, input.Year, stage, input.Month, input.SchoolID, assignmentID, user.ID, input.Score, string(rubric), strings.TrimSpace(input.Notes)).Scan(&row.ID, &row.Year, &row.Stage, &row.SchoolID, &row.AssessorUserID, &row.RawScore, &row.FinalScore, &row.WorkflowStatus, &row.Notes, &row.SubmittedAt)
		if err == nil {
			action := "ASSESSMENT_SUBMITTED"
			auditPayload := any(input)
			if hadPrevious {
				action = "ASSESSMENT_RESUBMITTED"
				auditPayload = map[string]any{"stage": stage, "beforeScore": previousScore, "afterScore": input.Score, "beforeStatus": previousStatus, "afterStatus": row.WorkflowStatus, "notes": input.Notes}
			}
			_ = s.Repo.LogAudit(ctx, &input.Year, user.ID, user.Role, action, "stage_assessment", int64ToIntPointer(row.ID), &input.SchoolID, auditPayload)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return row, errors.New("ASSESSMENT_ALREADY_SUBMITTED")
		}
		return row, err
	}
	if !IsDirectFinalStage(stage) {
		return GaspolStageAssessmentRow{}, errors.New("INVALID_STAGE")
	}

	// Serialize submissions per stage/school so two assessors submitting at the
	// same moment cannot leave a fully-complete stage stuck in IN_PROGRESS.
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	defer tx.Rollback(ctx)
	lockKey := strconv.Itoa(input.Year) + ":" + stage + ":" + strconv.Itoa(input.SchoolID)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, lockKey); err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	var row GaspolStageAssessmentRow
	err = tx.QueryRow(ctx, `
		INSERT INTO gaspol_stage_assessments(year,stage,month,school_id,assignment_id,assessor_user_id,raw_score,rubric,notes,workflow_status)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,'IN_PROGRESS')
		ON CONFLICT(year,stage,month,school_id,assessor_user_id) DO UPDATE SET
		 raw_score=EXCLUDED.raw_score,rubric=EXCLUDED.rubric,notes=EXCLUDED.notes,workflow_status='IN_PROGRESS',final_score=NULL,updated_at=NOW()
		WHERE gaspol_stage_assessments.workflow_status='RETURNED_BY_MASTER'
		RETURNING id,year,stage,school_id,assessor_user_id,raw_score,final_score,workflow_status,notes,updated_at
	`, input.Year, stage, input.Month, input.SchoolID, assignmentID, user.ID, input.Score, string(rubric), strings.TrimSpace(input.Notes)).Scan(&row.ID, &row.Year, &row.Stage, &row.SchoolID, &row.AssessorUserID, &row.RawScore, &row.FinalScore, &row.WorkflowStatus, &row.Notes, &row.SubmittedAt)
	if err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	var required, submitted int
	var aggregate *float64
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*),COUNT(sa.id),AVG(sa.raw_score)
		FROM gaspol_stage_assignments a
		LEFT JOIN gaspol_stage_assessments sa ON sa.year=a.year AND sa.stage=a.stage
		 AND sa.school_id=a.school_id AND sa.assessor_user_id=a.assessor_user_id AND sa.month=$4 AND sa.workflow_status IN ('IN_PROGRESS','FINAL')
		WHERE a.year=$1 AND a.stage=$2 AND a.school_id=$3 AND a.is_active=TRUE
	`, input.Year, stage, input.SchoolID, input.Month).Scan(&required, &submitted, &aggregate)
	if err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	finalized := DirectStageReady(required, submitted) && aggregate != nil
	var rawScoresJSON string
	if finalized {
		_, err = tx.Exec(ctx, `
			UPDATE gaspol_stage_assessments sa SET workflow_status='FINAL',final_score=$5,locked_at=NOW(),updated_at=NOW()
			WHERE sa.year=$1 AND sa.stage=$2 AND sa.school_id=$3 AND sa.month=$4
			 AND EXISTS(SELECT 1 FROM gaspol_stage_assignments a WHERE a.year=sa.year AND a.stage=sa.stage
			   AND a.school_id=sa.school_id AND a.assessor_user_id=sa.assessor_user_id AND a.is_active=TRUE)
		`, input.Year, stage, input.SchoolID, input.Month, *aggregate)
		if err != nil {
			return GaspolStageAssessmentRow{}, err
		}
		row.WorkflowStatus, row.FinalScore = GaspolWorkflowFinal, aggregate
		if err = tx.QueryRow(ctx, `SELECT COALESCE(jsonb_object_agg(assessor_user_id::text,raw_score)::text,'{}') FROM gaspol_stage_assessments WHERE year=$1 AND stage=$2 AND school_id=$3 AND month=$4`, input.Year, stage, input.SchoolID, input.Month).Scan(&rawScoresJSON); err != nil {
			return GaspolStageAssessmentRow{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return GaspolStageAssessmentRow{}, err
	}
	_ = s.Repo.LogAudit(ctx, &input.Year, user.ID, user.Role, "ASSESSMENT_SUBMITTED", "stage_assessment", int64ToIntPointer(row.ID), &input.SchoolID, input)
	if finalized {
		var rawScores map[string]float64
		_ = json.Unmarshal([]byte(rawScoresJSON), &rawScores)
		_ = s.Repo.LogAudit(ctx, &input.Year, user.ID, user.Role, "DIRECT_STAGE_FINALIZED", "stage_assessment", int64ToIntPointer(row.ID), &input.SchoolID, map[string]any{"stage": stage, "aggregate": *aggregate, "rawScores": rawScores, "assessorCount": required})
	}
	return row, nil
}

func CanMasterReturnStageAssessment(stage, status string, topTenLocked bool, notes string) error {
	switch normalizeCompetitionStage(stage) {
	case GaspolStageField1, GaspolStageField2, GaspolStagePresentation, GaspolStageNomination, GaspolStageAppeal, "INNOVATION":
	default:
		return errors.New("MASTER_RETURN_STAGE_NOT_ALLOWED")
	}
	if status != GaspolWorkflowFinal && !(IsDirectFinalStage(stage) && status == GaspolWorkflowInProgress) {
		return errors.New("INVALID_WORKFLOW_TRANSITION")
	}
	if topTenLocked && normalizeCompetitionStage(stage) == GaspolStageField1 {
		return errors.New("TOP_10_ALREADY_LOCKED")
	}
	if strings.TrimSpace(notes) == "" {
		return errors.New("MASTER_RETURN_NOTES_REQUIRED")
	}
	return nil
}

func (s GaspolService) MasterReturnStageAssessment(ctx context.Context, user auth.User, assessmentID int64, input GaspolMasterReturnInput) error {
	if !auth.IsAdminMaster(user) {
		return auth.ErrForbidden
	}
	input.Notes = strings.TrimSpace(input.Notes)
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var year, schoolID int
	var stage, status string
	var legacyAssessmentID *int64
	if err = tx.QueryRow(ctx, `SELECT year,school_id,stage,workflow_status,legacy_assessment_id FROM gaspol_stage_assessments WHERE id=$1 FOR UPDATE`, assessmentID).Scan(&year, &schoolID, &stage, &status, &legacyAssessmentID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "gaspol-top10:"+strconv.Itoa(year)); err != nil {
		return err
	}
	closing := map[string]string{GaspolStageField1: "TOP_10", GaspolStageField2: "TOP_5", GaspolStagePresentation: "AWARDING", "INNOVATION": "AWARDING", GaspolStageNomination: "BOTTOM_5", GaspolStageAppeal: "FINAL"}[stage]
	var closed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=$1 AND lock_type=$2 AND status='LOCKED')`, year, closing).Scan(&closed); err != nil {
		return err
	}
	if closed {
		return errors.New("RESULT_ALREADY_LOCKED")
	}
	var old []byte
	if err = tx.QueryRow(ctx, `SELECT to_jsonb(a) FROM gaspol_stage_assessments a WHERE id=$1`, assessmentID).Scan(&old); err != nil {
		return err
	}
	if stage == "INNOVATION" {
		if _, err = tx.Exec(ctx, `UPDATE gaspol_innovation_nominations SET status='RETURNED',kasi_notes='Penilaian dikembalikan Master; wajib persetujuan ulang' WHERE year=$1 AND status IN ('APPROVED','SUBMITTED')`, year); err != nil {
			return err
		}
	}
	var topTenLocked bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=$1 AND lock_type='TOP_10' AND status='LOCKED')`, year).Scan(&topTenLocked); err != nil {
		return err
	}
	if err = CanMasterReturnStageAssessment(stage, status, topTenLocked, input.Notes); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `UPDATE gaspol_stage_assessments SET workflow_status='RETURNED_BY_MASTER',final_score=NULL,locked_at=NULL,koor_user_id=NULL,koor_validated_at=NULL,cabdin_user_id=NULL,cabdin_validated_at=NULL,returned_by_master_user_id=$2,master_return_notes=$3,master_returned_at=NOW(),updated_at=NOW() WHERE id=$1 AND workflow_status IN ('FINAL','IN_PROGRESS')`, assessmentID, user.ID, input.Notes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("INVALID_WORKFLOW_TRANSITION")
	}
	if legacyAssessmentID != nil {
		if _, err = tx.Exec(ctx, `UPDATE gaspol_assessments SET workflow_status='RETURNED_BY_MASTER',is_validated=FALSE,validated_by=NULL,validated_at=NULL,updated_at=NOW() WHERE id=$1`, *legacyAssessmentID); err != nil {
			return err
		}
	}
	payload, err := json.Marshal(map[string]any{"previousAssessment": json.RawMessage(old), "before": status, "after": GaspolWorkflowReturnedMaster, "notes": input.Notes, "stage": stage})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gaspol_audit_logs(year,actor_user_id,actor_role,action,entity_type,entity_id,school_id,payload) VALUES($1,$2,$3,'MASTER_RETURN','stage_assessment',$4,$5,$6::jsonb)`, year, user.ID, user.Role, assessmentID, schoolID, string(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func int64ToIntPointer(value int64) *int { converted := int(value); return &converted }

func (s GaspolService) ReviewStageAssessment(ctx context.Context, user auth.User, assessmentID int64, level string, input GaspolReviewInput) error {
	decision := strings.ToUpper(strings.TrimSpace(input.Decision))
	if decision != "APPROVE" && decision != "RETURN" {
		return errors.New("INVALID_PAYLOAD")
	}
	level = strings.ToUpper(strings.TrimSpace(level))
	tx, beginErr := s.DB.Pool.Begin(ctx)
	if beginErr != nil {
		return beginErr
	}
	defer tx.Rollback(ctx)
	var year, schoolID int
	var stage string
	var current string
	if err := tx.QueryRow(ctx, `SELECT year,school_id,stage,workflow_status FROM gaspol_stage_assessments WHERE id=$1 FOR UPDATE`, assessmentID).Scan(&year, &schoolID, &stage, &current); err != nil {
		return err
	}
	if !RequiresTwoLevelValidation(stage) {
		return errors.New("INVALID_WORKFLOW_TRANSITION")
	}
	var next string
	var rowsAffected int64
	var err error
	if level == "KOOR" {
		var assigned bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$2 AND assignment_role='KOOR_GASPOL' AND is_active=TRUE)`, year, user.ID).Scan(&assigned)
		if err != nil || !assigned {
			return auth.ErrForbidden
		}
		next, err = NextGaspolReviewStatus(stage, current, level, decision)
		if err != nil {
			return err
		}
		result, execErr := tx.Exec(ctx, `UPDATE gaspol_stage_assessments SET workflow_status=$2,koor_user_id=$3,koor_notes=$4,koor_validated_at=CASE WHEN $2='WAITING_CABDIN' THEN NOW() END,updated_at=NOW() WHERE id=$1 AND workflow_status='WAITING_KOOR'`, assessmentID, next, user.ID, strings.TrimSpace(input.Notes))
		err = execErr
		rowsAffected = result.RowsAffected()
	} else if level == "CABDIN" {
		normalizedRole := auth.NormalizeRole(user.Role)
		if normalizedRole != auth.RoleAdminCabdin && normalizedRole != auth.RoleAdminMaster {
			return auth.ErrForbidden
		}
		if normalizedRole != auth.RoleAdminMaster {
			var assigned bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_school_participants WHERE year=$1 AND school_id=$2 AND validator_user_id=$3 AND is_active=TRUE AND is_year_configuration=FALSE)`, year, schoolID, user.ID).Scan(&assigned); err != nil {
				return err
			}
			if !assigned {
				return auth.ErrForbidden
			}
		}
		next, err = NextGaspolReviewStatus(stage, current, level, decision)
		if err != nil {
			return errors.New("KOOR_VALIDATION_REQUIRED")
		}
		result, execErr := tx.Exec(ctx, `UPDATE gaspol_stage_assessments SET workflow_status=$2,cabdin_user_id=$3,cabdin_notes=$4,cabdin_validated_at=CASE WHEN $2='FINAL' THEN NOW() END,final_score=CASE WHEN $2='FINAL' THEN raw_score END,locked_at=CASE WHEN $2='FINAL' THEN NOW() END,updated_at=NOW() WHERE id=$1 AND workflow_status='WAITING_CABDIN'`, assessmentID, next, user.ID, strings.TrimSpace(input.Notes))
		err = execErr
		rowsAffected = result.RowsAffected()
	} else {
		return errors.New("INVALID_ROLE")
	}
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return errors.New("INVALID_WORKFLOW_TRANSITION")
	}
	// Keep compatibility readers aligned with v2 without using the legacy row
	// to drive transitions. This runs only after the conditional update above
	// changed exactly one stage assessment.
	_, err = tx.Exec(ctx, `
		UPDATE gaspol_assessments legacy SET
		 workflow_status=$2,
		 is_validated=CASE WHEN $2='FINAL' THEN TRUE ELSE FALSE END,
		 validated_by=CASE WHEN $2='FINAL' THEN $3::integer ELSE NULL END,
		 validated_at=CASE WHEN $2='FINAL' THEN NOW() ELSE NULL END,
		 koor_user_id=CASE WHEN $4='KOOR' THEN $3::integer ELSE legacy.koor_user_id END,
		 koor_validated_at=CASE WHEN $2='WAITING_CABDIN' THEN NOW() ELSE legacy.koor_validated_at END,
		 cabdin_user_id=CASE WHEN $4='CABDIN' THEN $3::integer ELSE legacy.cabdin_user_id END,
		 cabdin_validated_at=CASE WHEN $2='FINAL' THEN NOW() ELSE legacy.cabdin_validated_at END,
		 updated_at=NOW()
		FROM gaspol_stage_assessments v2
		WHERE v2.id=$1 AND legacy.id=v2.legacy_assessment_id
	`, assessmentID, next, user.ID, level)
	if err != nil {
		return err
	}
	if err = gaspolAuditTx(ctx, tx, user, year, level+"_"+decision, "stage_assessment", assessmentID, &schoolID, map[string]any{"before": current, "after": next, "notes": input.Notes}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s GaspolService) RecordManualLike(ctx context.Context, user auth.User, input GaspolManualLikeInput) error {
	if err := s.ensureAdminMaster(user); err != nil {
		return err
	}
	return errors.New("LIKE_VERIFICATION_REQUIRED")
}

func GenerateGaspolVotingToken() (plain, hash string, err error) {
	buffer := make([]byte, 32)
	if _, err = rand.Read(buffer); err != nil {
		return "", "", err
	}
	plain = hex.EncodeToString(buffer)
	sum := sha256.Sum256([]byte(plain))
	hash = hex.EncodeToString(sum[:])
	return plain, hash, nil
}

func (s GaspolService) UploadBADocument(ctx context.Context, user auth.User, year int, fileName, contentType string, content []byte, isPublic bool) (GaspolBADocument, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolBADocument{}, err
	}
	if year < 2024 || strings.TrimSpace(fileName) == "" || len(content) < 5 || len(content) > 10<<20 || string(content[:5]) != "%PDF-" || !strings.EqualFold(filepath.Ext(fileName), ".pdf") {
		return GaspolBADocument{}, errors.New("INVALID_PAYLOAD")
	}
	contentType = "application/pdf"
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	var row GaspolBADocument
	err := s.DB.Pool.QueryRow(ctx, `INSERT INTO gaspol_ba_documents(year,file_name,content_type,content,sha256,is_public,uploaded_by) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id,year,file_name,content_type,sha256,is_public,uploaded_at`, year, strings.TrimSpace(fileName), strings.TrimSpace(contentType), content, digest, isPublic, user.ID).Scan(&row.ID, &row.Year, &row.FileName, &row.ContentType, &row.SHA256, &row.IsPublic, &row.UploadedAt)
	if err == nil {
		_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "BA_UPLOADED", "ba_document", int64ToIntPointer(row.ID), nil, map[string]any{"fileName": row.FileName, "sha256": row.SHA256, "isPublic": isPublic})
	}
	return row, err
}

func (s GaspolService) BADocumentContent(ctx context.Context, user *auth.User, id int64) (GaspolBADocument, []byte, error) {
	var row GaspolBADocument
	var content []byte
	err := s.DB.Pool.QueryRow(ctx, `SELECT id,year,file_name,content_type,sha256,is_public,uploaded_at,content FROM gaspol_ba_documents WHERE id=$1`, id).Scan(&row.ID, &row.Year, &row.FileName, &row.ContentType, &row.SHA256, &row.IsPublic, &row.UploadedAt, &content)
	if err != nil {
		return GaspolBADocument{}, nil, err
	}
	if row.IsPublic {
		var published bool
		if err = s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_result_locks l JOIN gaspol_competition_configs cfg ON cfg.year=l.year WHERE l.ba_document_id=$1 AND l.status='LOCKED' AND cfg.public_full_ranking_enabled)`, id).Scan(&published); err != nil {
			return GaspolBADocument{}, nil, err
		}
		row.IsPublic = published
	}
	if !row.IsPublic && (user == nil || (auth.NormalizeRole(user.Role) != auth.RoleAdminMaster && auth.NormalizeRole(user.Role) != auth.RoleAdminCabdin)) {
		return GaspolBADocument{}, nil, auth.ErrForbidden
	}
	return row, content, nil
}
