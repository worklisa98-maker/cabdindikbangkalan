package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
)

type gaspolCandidateCompleteness struct {
	GaspolCompetitionCandidate
	MonthlyCount    int
	HasField1       bool
	HasField2       bool
	HasPresentation bool
	HasNomination   bool
	HasAppeal       bool
}
type GaspolLockInput struct {
	Year              int    `json:"year"`
	BANumber          string `json:"baNumber"`
	BADate            string `json:"baDate"`
	LockType          string `json:"lockType"`
	BADocumentID      *int64 `json:"baDocumentId"`
	ManualTieSchoolID *int   `json:"manualTieSchoolId"`
	Reason            string `json:"reason"`
	Order             []int  `json:"order"`
}
type GaspolLockResult struct {
	ID       int64                        `json:"id"`
	Year     int                          `json:"year"`
	LockType string                       `json:"lockType"`
	Revision int                          `json:"revision"`
	Rows     []GaspolCompetitionCandidate `json:"rows"`
}

type GaspolStageOnePreviewRow struct {
	Rank                *int                          `json:"rank,omitempty"`
	SchoolID            int                           `json:"schoolId"`
	SchoolName          string                        `json:"schoolName"`
	MonthlyScore        *float64                      `json:"monthlyScore,omitempty"`
	MonthlyContribution *float64                      `json:"monthlyContribution,omitempty"`
	FieldStage1Score    *float64                      `json:"fieldStage1Score,omitempty"`
	Field1Contribution  *float64                      `json:"fieldStage1Contribution,omitempty"`
	Total               *float64                      `json:"total,omitempty"`
	KoorStatus          string                        `json:"koorStatus"`
	CabdinStatus        string                        `json:"cabdinStatus"`
	FinalStatus         string                        `json:"finalStatus"`
	WorkflowStatus      string                        `json:"workflowStatus"`
	Assessments         []GaspolStageOneAssessmentRef `json:"assessments"`
}

type GaspolStageOneAssessmentRef struct {
	ID             int64   `json:"id"`
	AssessorName   string  `json:"assessorName"`
	RawScore       float64 `json:"rawScore"`
	WorkflowStatus string  `json:"workflowStatus"`
}

type GaspolRankingCompleteness struct {
	Participants      int `json:"participants"`
	MonthlyFinal      int `json:"monthlyFinal"`
	FieldStage1Filled int `json:"fieldStage1Filled"`
	KoorValidated     int `json:"koorValidated"`
	CabdinValidated   int `json:"cabdinValidated"`
	PendingOrReturned int `json:"pendingOrReturned"`
}

type GaspolRankingLockStatus struct {
	IsLocked bool       `json:"isLocked"`
	LockID   *int64     `json:"lockId,omitempty"`
	Revision *int       `json:"revision,omitempty"`
	LockedAt *time.Time `json:"lockedAt,omitempty"`
	LockedBy string     `json:"lockedBy"`
}

type GaspolStageOnePreview struct {
	Year           int                          `json:"year"`
	Stage          string                       `json:"stage"`
	Rows           []GaspolStageOnePreviewRow   `json:"rows"`
	Completeness   GaspolRankingCompleteness    `json:"completeness"`
	CanLockTopTen  bool                         `json:"canLockTopTen"`
	DisabledReason string                       `json:"disabledReason"`
	HasBoundaryTie bool                         `json:"hasBoundaryTie"`
	Lock           GaspolRankingLockStatus      `json:"lock"`
	OfficialTopTen []GaspolCompetitionCandidate `json:"officialTopTen"`
}

type gaspolStageOnePreviewSource struct {
	gaspolCandidateCompleteness
	Field1Filled  bool
	Workflow      string
	KoorValidated bool
	CabdinFinal   bool
}

func gaspolCompetitionScore(candidate GaspolCompetitionCandidate, path string, completeness gaspolCandidateCompleteness) GaspolScoreBreakdown {
	input := GaspolScoringInput{
		MonthlyComponents: completeMonthlyComponentsForRanking(candidate.MonthlyScore),
		FieldStage1Score:  candidate.FieldStage1Score, FieldStage2Score: candidate.FieldStage2Score,
		PresentationScore: candidate.PresentationScore, NominationScore: candidate.NominationScore, AppealScore: candidate.AppealScore,
		HasFieldStage1: completeness.HasField1, HasFieldStage2: completeness.HasField2,
		HasPresentation: completeness.HasPresentation, HasNomination: completeness.HasNomination, HasAppeal: completeness.HasAppeal,
	}
	stage := GaspolStagePresentation
	if strings.EqualFold(path, "NOT_FAILED") {
		stage = GaspolStageAppeal
	}
	score := ComputeGaspolStageScore(stage, input)
	score.ScoreTotal = candidate.Total
	score.MaximumScore = candidate.Maximum
	score.CurrentMaxScore = GaspolCurrentMaximum(input, path)
	score.Contributions = GaspolScoreContributions(input, path)
	return score
}

func (s GaspolService) loadFinalMonthlyComponents(ctx context.Context, year, schoolID int) ([]GaspolMonthlyComponent, error) {
	now, err := s.Clock.Now(ctx)
	if err != nil {
		return nil, err
	}
	monthly, err := s.Repo.ListMonthlyCompetitionComponents(ctx, year, schoolID, now)
	return monthly[schoolID], err
}

func (s GaspolService) loadCompetitionCandidates(ctx context.Context, year int, cohort string) ([]gaspolCandidateCompleteness, error) {
	now, err := s.Clock.Now(ctx)
	if err != nil {
		return nil, err
	}
	monthlyComponents, err := s.Repo.ListMonthlyCompetitionComponents(ctx, year, 0, now)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.Pool.Query(ctx, `
		WITH monthly_per_month AS (
		 SELECT school_id,month,AVG(final_score) score FROM gaspol_stage_assessments
		 WHERE year=$1 AND stage='MONTHLY' AND workflow_status='FINAL' AND month BETWEEN 4 AND 9 GROUP BY school_id,month
		), monthly AS (
		 SELECT school_id,COUNT(*) month_count,SUM(score*CASE month WHEN 4 THEN .20 WHEN 5 THEN .15 WHEN 6 THEN .20 WHEN 7 THEN .15 WHEN 8 THEN .15 WHEN 9 THEN .15 END) score
		 FROM monthly_per_month GROUP BY school_id
		), stage_scores AS (
		 SELECT school_id,
		 AVG(final_score) FILTER(WHERE stage='FIELD_STAGE_1') field1,
		 AVG(final_score) FILTER(WHERE stage='FIELD_STAGE_2') field2,
		 AVG(final_score) FILTER(WHERE stage='PRESENTATION') presentation,
		 AVG(final_score) FILTER(WHERE stage='NOT_FAILED_NOMINATION') nomination,
		 AVG(final_score) FILTER(WHERE stage='BOTTOM5_APPEAL') appeal
		 FROM gaspol_stage_assessments sa WHERE year=$1 AND workflow_status='FINAL' AND (stage='FIELD_STAGE_1' OR EXISTS(SELECT 1 FROM gaspol_stage_assignments ass WHERE ass.year=sa.year AND ass.stage=sa.stage AND ass.school_id=sa.school_id AND ass.assessor_user_id=sa.assessor_user_id AND ass.is_active AND ass.assignment_role NOT IN ('KOOR_GASPOL','KASI_SMK_APPROVER'))) GROUP BY school_id
		)
		SELECT ds.id,ds.nama_sekolah,COALESCE(m.score,0),COALESCE(m.month_count,0),ss.field1,ss.field2,ss.presentation,ss.nomination,ss.appeal
		FROM gaspol_school_participants p JOIN data_sekolah ds ON ds.id=p.school_id
		LEFT JOIN monthly m ON m.school_id=p.school_id LEFT JOIN stage_scores ss ON ss.school_id=p.school_id
		WHERE p.year=$1 AND p.is_active AND NOT p.is_year_configuration
		AND ($2='' OR EXISTS(SELECT 1 FROM gaspol_cohorts c WHERE c.year=$1 AND c.school_id=p.school_id AND c.cohort=$2 AND c.released_at IS NULL))
		ORDER BY ds.nama_sekolah`, year, cohort)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []gaspolCandidateCompleteness{}
	for rows.Next() {
		var row gaspolCandidateCompleteness
		var f1, f2, p, n, a *float64
		if err = rows.Scan(&row.SchoolID, &row.SchoolName, &row.MonthlyScore, &row.MonthlyCount, &f1, &f2, &p, &n, &a); err != nil {
			return nil, err
		}
		if f1 != nil {
			row.FieldStage1Score = *f1
			row.HasField1 = true
		}
		if f2 != nil {
			row.FieldStage2Score = *f2
			row.HasField2 = true
		}
		if p != nil {
			row.PresentationScore = *p
			row.HasPresentation = true
		}
		if n != nil {
			row.NominationScore = *n
			row.HasNomination = true
		}
		if a != nil {
			row.AppealScore = *a
			row.HasAppeal = true
		}
		row.MonthlyCount = 0
		for _, component := range monthlyComponents[row.SchoolID] {
			if component.IsFinal {
				row.MonthlyCount++
			}
		}
		row.MonthlyScore = calculateGaspolMonthlyWeightedScore(monthlyComponents[row.SchoolID])
		result = append(result, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if cohort != "" {
		kind := "TOP_10"
		if cohort == "TOP_5" {
			kind = "TOP_5"
		}
		if cohort == "BOTTOM_5" {
			kind = "BOTTOM_5"
		}
		official, e := gaspolOfficialCandidates(ctx, s.DB.Pool, year, kind, true)
		if e != nil {
			return nil, e
		}
		byID := map[int]GaspolCompetitionCandidate{}
		for _, r := range official {
			byID[r.SchoolID] = r
		}
		for i := range result {
			if r, ok := byID[result[i].SchoolID]; ok {
				result[i].MonthlyScore = r.MonthlyScore
				result[i].FieldStage1Score = r.FieldStage1Score
				result[i].MonthlyCount = 6
				result[i].HasField1 = true
				if cohort == "TOP_5" {
					result[i].FieldStage2Score = r.FieldStage2Score
					result[i].HasField2 = true
				}
				if cohort == "BOTTOM_5" {
					result[i].NominationScore = r.NominationScore
					result[i].HasNomination = true
				}
			}
		}
	}
	return result, nil
}

func (s GaspolService) PreviewCompetitionRanking(ctx context.Context, user auth.User, year int, stage, cohort string) (any, error) {
	role := auth.NormalizeRole(user.Role)
	if role != auth.RoleAdminMaster && role != auth.RoleAdminCabdin {
		var koor bool
		_ = s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$2 AND assignment_role='KOOR_GASPOL' AND is_active)`, year, user.ID).Scan(&koor)
		if !koor {
			return nil, auth.ErrForbidden
		}
	}
	year = normalizeGaspolYear(year)
	if normalizeCompetitionStage(stage) == GaspolStageField1 && strings.TrimSpace(cohort) == "" {
		return s.stageOneRankingPreview(ctx, year)
	}
	rows, err := s.loadCompetitionCandidates(ctx, year, strings.ToUpper(strings.TrimSpace(cohort)))
	if err != nil {
		return nil, err
	}
	candidates := make([]GaspolCompetitionCandidate, len(rows))
	for i := range rows {
		candidates[i] = rows[i].GaspolCompetitionCandidate
	}
	return RankGaspolCompetition(candidates, normalizeCompetitionStage(stage)), nil
}

func (s GaspolService) loadStageOnePreviewSources(ctx context.Context, year int) ([]gaspolStageOnePreviewSource, error) {
	now, err := s.Clock.Now(ctx)
	if err != nil {
		return nil, err
	}
	monthlyComponents, err := s.Repo.ListMonthlyCompetitionComponents(ctx, year, 0, now)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.Pool.Query(ctx, `
		WITH monthly_per_month AS (
		 SELECT school_id,month,AVG(final_score) score FROM gaspol_stage_assessments
		 WHERE year=$1 AND stage='MONTHLY' AND workflow_status='FINAL' AND month BETWEEN 4 AND 9 GROUP BY school_id,month
		), monthly AS (
		 SELECT school_id,COUNT(*) month_count,SUM(score*CASE month WHEN 4 THEN .20 WHEN 5 THEN .15 WHEN 6 THEN .20 WHEN 7 THEN .15 WHEN 8 THEN .15 WHEN 9 THEN .15 END) score
		 FROM monthly_per_month GROUP BY school_id
		), field_assignments AS (
		 SELECT school_id,COUNT(*) assigned FROM gaspol_stage_assignments
		 WHERE year=$1 AND stage='FIELD_STAGE_1' AND assignment_role='FIELD_STAGE_1_ASSESSOR' AND is_active GROUP BY school_id
		), field AS (
		 SELECT school_id,COUNT(*) submitted,AVG(raw_score) raw_score,
		  AVG(final_score) FILTER (WHERE workflow_status='FINAL') final_score,
		  COUNT(*) FILTER (WHERE workflow_status='FINAL') final_count,
		  COUNT(*) FILTER (WHERE workflow_status IN ('WAITING_CABDIN','FINAL')) koor_count,
		  COUNT(*) FILTER (WHERE workflow_status IN ('WAITING_KOOR','WAITING_CABDIN','RETURNED_BY_KOOR','RETURNED_BY_CABDIN','RETURNED_BY_MASTER','IN_PROGRESS')) unfinished_count,
		  CASE
		   WHEN BOOL_OR(workflow_status='RETURNED_BY_MASTER') THEN 'RETURNED_BY_MASTER'
		   WHEN BOOL_OR(workflow_status='RETURNED_BY_CABDIN') THEN 'RETURNED_BY_CABDIN'
		   WHEN BOOL_OR(workflow_status='RETURNED_BY_KOOR') THEN 'RETURNED_BY_KOOR'
		   WHEN BOOL_OR(workflow_status='WAITING_KOOR') THEN 'WAITING_KOOR'
		   WHEN BOOL_OR(workflow_status='WAITING_CABDIN') THEN 'WAITING_CABDIN'
		   WHEN BOOL_AND(workflow_status='FINAL') THEN 'FINAL'
		   ELSE 'IN_PROGRESS' END workflow
		 FROM gaspol_stage_assessments WHERE year=$1 AND stage='FIELD_STAGE_1' GROUP BY school_id
		)
		SELECT ds.id,ds.nama_sekolah,COALESCE(m.month_count,0),m.score,
		 COALESCE(fa.assigned,0),COALESCE(f.submitted,0),f.raw_score,f.final_score,
		 COALESCE(f.final_count,0),COALESCE(f.koor_count,0),COALESCE(f.unfinished_count,0),COALESCE(f.workflow,'NOT_ASSESSED')
		FROM gaspol_school_participants p JOIN data_sekolah ds ON ds.id=p.school_id
		LEFT JOIN monthly m ON m.school_id=p.school_id
		LEFT JOIN field_assignments fa ON fa.school_id=p.school_id
		LEFT JOIN field f ON f.school_id=p.school_id
		WHERE p.year=$1 AND p.is_active AND NOT p.is_year_configuration
		ORDER BY ds.nama_sekolah`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []gaspolStageOnePreviewSource{}
	for rows.Next() {
		var row gaspolStageOnePreviewSource
		var monthly, rawField, finalField *float64
		var assigned, submitted, finalCount, koorCount, unfinished int
		if err = rows.Scan(&row.SchoolID, &row.SchoolName, &row.MonthlyCount, &monthly, &assigned, &submitted, &rawField, &finalField, &finalCount, &koorCount, &unfinished, &row.Workflow); err != nil {
			return nil, err
		}
		if row.MonthlyCount == 6 && monthly != nil {
			row.MonthlyScore = *monthly
		}
		row.Field1Filled = submitted > 0
		row.KoorValidated = assigned > 0 && submitted == assigned && koorCount == assigned
		row.CabdinFinal = assigned > 0 && submitted == assigned && finalCount == assigned && unfinished == 0
		row.HasField1 = row.CabdinFinal && finalField != nil
		if row.HasField1 {
			row.FieldStage1Score = *finalField
		} else if rawField != nil {
			row.FieldStage1Score = *rawField
		}
		row.MonthlyCount = 0
		for _, component := range monthlyComponents[row.SchoolID] {
			if component.IsFinal {
				row.MonthlyCount++
			}
		}
		row.MonthlyScore = calculateGaspolMonthlyWeightedScore(monthlyComponents[row.SchoolID])
		result = append(result, row)
	}
	return result, rows.Err()
}

func stageOneStatusLabels(source gaspolStageOnePreviewSource) (string, string, string) {
	switch source.Workflow {
	case GaspolWorkflowWaitingKoor:
		return "Menunggu Koor", "Belum masuk", "Menunggu Koor"
	case GaspolWorkflowReturnedKoor:
		return "Dikembalikan Koor", "Belum masuk", "Dikembalikan"
	case GaspolWorkflowWaitingCabdin:
		return "Disetujui", "Menunggu Cabdin", "Menunggu Cabdin"
	case GaspolWorkflowReturnedCabdin:
		return "Disetujui", "Dikembalikan Cabdin", "Dikembalikan"
	case GaspolWorkflowReturnedMaster:
		return "Perlu validasi ulang", "Perlu validasi ulang", "Dikembalikan Master"
	case GaspolWorkflowFinal:
		return "Disetujui", "Disetujui", "Final"
	case GaspolWorkflowInProgress:
		return "Belum lengkap", "Belum masuk", "Penilaian belum lengkap"
	default:
		return "Belum dinilai", "Belum dinilai", "Belum dinilai"
	}
}

func buildStageOnePreview(year int, sources []gaspolStageOnePreviewSource) GaspolStageOnePreview {
	preview := GaspolStageOnePreview{Year: year, Stage: GaspolStageField1, Rows: []GaspolStageOnePreviewRow{}, OfficialTopTen: []GaspolCompetitionCandidate{}}
	preview.Completeness.Participants = len(sources)
	complete := []GaspolCompetitionCandidate{}
	byID := map[int]gaspolStageOnePreviewSource{}
	for _, source := range sources {
		byID[source.SchoolID] = source
		if source.MonthlyCount == 6 {
			preview.Completeness.MonthlyFinal++
		}
		if source.Field1Filled {
			preview.Completeness.FieldStage1Filled++
		}
		if source.KoorValidated {
			preview.Completeness.KoorValidated++
		}
		if source.CabdinFinal {
			preview.Completeness.CabdinValidated++
		}
		if source.Workflow == GaspolWorkflowWaitingKoor || source.Workflow == GaspolWorkflowWaitingCabdin || source.Workflow == GaspolWorkflowReturnedKoor || source.Workflow == GaspolWorkflowReturnedCabdin || source.Workflow == GaspolWorkflowReturnedMaster || source.Workflow == GaspolWorkflowInProgress {
			preview.Completeness.PendingOrReturned++
		}
		if source.MonthlyCount == 6 && source.HasField1 {
			complete = append(complete, source.GaspolCompetitionCandidate)
		}
	}
	ranked := RankGaspolCompetition(complete, GaspolStageField1)
	rankByID := map[int]GaspolCompetitionCandidate{}
	for _, candidate := range ranked {
		rankByID[candidate.SchoolID] = candidate
	}
	ordered := append([]GaspolCompetitionCandidate(nil), ranked...)
	for _, source := range sources {
		if _, ok := rankByID[source.SchoolID]; !ok {
			ordered = append(ordered, source.GaspolCompetitionCandidate)
		}
	}
	for _, candidate := range ordered {
		source := byID[candidate.SchoolID]
		koor, cabdin, finalStatus := stageOneStatusLabels(source)
		if source.MonthlyCount != 6 && finalStatus == "Final" {
			finalStatus = "Nilai Bulanan belum final"
		}
		row := GaspolStageOnePreviewRow{SchoolID: candidate.SchoolID, SchoolName: candidate.SchoolName, KoorStatus: koor, CabdinStatus: cabdin, FinalStatus: finalStatus, WorkflowStatus: source.Workflow}
		if source.MonthlyCount == 6 {
			score, contribution := source.MonthlyScore, roundGaspolScore(source.MonthlyScore*.20)
			row.MonthlyScore, row.MonthlyContribution = &score, &contribution
		}
		if source.Field1Filled {
			score := source.FieldStage1Score
			row.FieldStage1Score = &score
		}
		if source.MonthlyCount == 6 && source.HasField1 {
			rankedRow := rankByID[source.SchoolID]
			rank, contribution, total := rankedRow.Rank, roundGaspolScore(source.FieldStage1Score*.40), rankedRow.Total
			row.Rank, row.Field1Contribution, row.Total = &rank, &contribution, &total
		}
		preview.Rows = append(preview.Rows, row)
	}
	allReady := len(sources) >= 10 && preview.Completeness.MonthlyFinal == len(sources) && preview.Completeness.FieldStage1Filled == len(sources) && preview.Completeness.KoorValidated == len(sources) && preview.Completeness.CabdinValidated == len(sources) && preview.Completeness.PendingOrReturned == 0
	if len(ranked) > 10 {
		preview.HasBoundaryTie = gaspolCandidatesEqualForStage(ranked[9], ranked[10], GaspolStageField1)
	}
	preview.CanLockTopTen = allReady && !preview.HasBoundaryTie
	switch {
	case len(sources) < 10:
		preview.DisabledReason = "Belum dapat ditetapkan: peserta relevan kurang dari 10 sekolah."
	case preview.Completeness.MonthlyFinal < len(sources):
		preview.DisabledReason = fmt.Sprintf("Belum dapat ditetapkan: %d sekolah belum memiliki Nilai Bulanan final.", len(sources)-preview.Completeness.MonthlyFinal)
	case preview.Completeness.FieldStage1Filled < len(sources):
		preview.DisabledReason = fmt.Sprintf("Belum dapat ditetapkan: %d sekolah belum dinilai pada Lapangan Tahap 1.", len(sources)-preview.Completeness.FieldStage1Filled)
	case preview.Completeness.PendingOrReturned > 0:
		preview.DisabledReason = fmt.Sprintf("Top 10 belum dapat ditetapkan karena masih terdapat %d penilaian Lapangan Tahap 1 yang belum selesai direview.", preview.Completeness.PendingOrReturned)
	case preview.Completeness.KoorValidated < len(sources):
		preview.DisabledReason = fmt.Sprintf("Belum dapat ditetapkan: %d sekolah masih menunggu validasi Koor GASPOL.", len(sources)-preview.Completeness.KoorValidated)
	case preview.Completeness.CabdinValidated < len(sources):
		preview.DisabledReason = fmt.Sprintf("Belum dapat ditetapkan: %d sekolah masih menunggu validasi Cabdin.", len(sources)-preview.Completeness.CabdinValidated)
	case preview.HasBoundaryTie:
		preview.DisabledReason = "Belum dapat ditetapkan: peringkat 10 dan 11 masih seri dan memerlukan keputusan eksplisit Admin Master."
	}
	return preview
}

func (s GaspolService) stageOneRankingPreview(ctx context.Context, year int) (GaspolStageOnePreview, error) {
	sources, err := s.loadStageOnePreviewSources(ctx, year)
	if err != nil {
		return GaspolStageOnePreview{}, err
	}
	preview := buildStageOnePreview(year, sources)
	assessmentRows, assessmentErr := s.DB.Pool.Query(ctx, `SELECT a.id,a.school_id,COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username),a.raw_score,a.workflow_status FROM gaspol_stage_assessments a JOIN users u ON u.id=a.assessor_user_id WHERE a.year=$1 AND a.stage='FIELD_STAGE_1' ORDER BY a.school_id,a.id`, year)
	if assessmentErr != nil {
		return GaspolStageOnePreview{}, assessmentErr
	}
	assessmentBySchool := map[int][]GaspolStageOneAssessmentRef{}
	for assessmentRows.Next() {
		var schoolID int
		var item GaspolStageOneAssessmentRef
		if assessmentErr = assessmentRows.Scan(&item.ID, &schoolID, &item.AssessorName, &item.RawScore, &item.WorkflowStatus); assessmentErr != nil {
			assessmentRows.Close()
			return GaspolStageOnePreview{}, assessmentErr
		}
		assessmentBySchool[schoolID] = append(assessmentBySchool[schoolID], item)
	}
	if assessmentErr = assessmentRows.Err(); assessmentErr != nil {
		assessmentRows.Close()
		return GaspolStageOnePreview{}, assessmentErr
	}
	assessmentRows.Close()
	for index := range preview.Rows {
		preview.Rows[index].Assessments = assessmentBySchool[preview.Rows[index].SchoolID]
		if preview.Rows[index].Assessments == nil {
			preview.Rows[index].Assessments = []GaspolStageOneAssessmentRef{}
		}
	}
	var lockID int64
	var revision int
	var lockedAt time.Time
	var lockedBy string
	err = s.DB.Pool.QueryRow(ctx, `SELECT l.id,l.revision,l.locked_at,COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username,'') FROM gaspol_result_locks l LEFT JOIN users u ON u.id=l.locked_by WHERE l.year=$1 AND l.lock_type='TOP_10' AND l.status='LOCKED' ORDER BY l.revision DESC LIMIT 1`, year).Scan(&lockID, &revision, &lockedAt, &lockedBy)
	if err == nil {
		preview.Lock = GaspolRankingLockStatus{IsLocked: true, LockID: &lockID, Revision: &revision, LockedAt: &lockedAt, LockedBy: lockedBy}
		rows, queryErr := s.DB.Pool.Query(ctx, `SELECT payload FROM gaspol_ranking_snapshots_v2 WHERE year=$1 AND snapshot_type='TOP_10' AND revision=$2 ORDER BY rank_position`, year, revision)
		if queryErr != nil {
			return GaspolStageOnePreview{}, queryErr
		}
		for rows.Next() {
			var raw []byte
			var candidate GaspolCompetitionCandidate
			if queryErr = rows.Scan(&raw); queryErr != nil {
				rows.Close()
				return GaspolStageOnePreview{}, queryErr
			}
			if queryErr = json.Unmarshal(raw, &candidate); queryErr != nil {
				rows.Close()
				return GaspolStageOnePreview{}, queryErr
			}
			preview.OfficialTopTen = append(preview.OfficialTopTen, candidate)
		}
		if queryErr = rows.Err(); queryErr != nil {
			rows.Close()
			return GaspolStageOnePreview{}, queryErr
		}
		rows.Close()
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return GaspolStageOnePreview{}, err
	}
	return preview, nil
}

func (s GaspolService) LockCompetitionResult(ctx context.Context, user auth.User, input GaspolLockInput) (GaspolLockResult, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolLockResult{}, err
	}
	year := normalizeGaspolYear(input.Year)
	kind := strings.ToUpper(strings.TrimSpace(input.LockType))
	var cohort, stage string
	var limit int
	switch kind {
	case "TOP_10":
		stage = GaspolStageField1
		limit = 10
	case "TOP_5":
		stage = GaspolStageField2
		cohort = "TOP_10"
		limit = 5
		if input.BADocumentID == nil {
			return GaspolLockResult{}, errors.New("BA_REQUIRED")
		}
	case "BOTTOM_5":
		stage = GaspolStageNomination
		cohort = "NOT_FAILED"
		limit = 5
	case "FINAL":
		if input.BADocumentID == nil {
			return GaspolLockResult{}, errors.New("BA_REQUIRED")
		}
		stage = GaspolStageAppeal
		cohort = "BOTTOM_5"
		limit = 3
	default:
		return GaspolLockResult{}, errors.New("INVALID_LOCK_TYPE")
	}
	version, err := gaspolStageDataVersion(ctx, s.DB.Pool, year)
	if err != nil {
		return GaspolLockResult{}, err
	}
	if kind == "TOP_10" {
		previewSources, previewErr := s.loadStageOnePreviewSources(ctx, year)
		if previewErr != nil {
			return GaspolLockResult{}, previewErr
		}
		preview := buildStageOnePreview(year, previewSources)
		complete := preview.Completeness
		if len(previewSources) < 10 || complete.MonthlyFinal != len(previewSources) || complete.FieldStage1Filled != len(previewSources) || complete.KoorValidated != len(previewSources) || complete.CabdinValidated != len(previewSources) || complete.PendingOrReturned != 0 {
			return GaspolLockResult{}, errors.New("INCOMPLETE_ASSESSMENTS")
		}
	}
	rows, err := s.loadCompetitionCandidates(ctx, year, cohort)
	if err != nil {
		return GaspolLockResult{}, err
	}
	if len(rows) == 0 {
		return GaspolLockResult{}, errors.New("INCOMPLETE_ASSESSMENTS")
	}
	for _, r := range rows {
		if r.MonthlyCount != 6 || !r.HasField1 || (kind == "TOP_5" && !r.HasField2) || (kind == "BOTTOM_5" && !r.HasNomination) || (kind == "FINAL" && !r.HasAppeal) {
			return GaspolLockResult{}, errors.New("INCOMPLETE_ASSESSMENTS")
		}
	}
	if kind != "TOP_10" {
		expected := map[string]int{"TOP_5": 10, "BOTTOM_5": 11, "FINAL": 5}[kind]
		obligations, e := gaspolObligations(ctx, s.DB.Pool, year, cohort, stage)
		if e != nil {
			return GaspolLockResult{}, e
		}
		if len(rows) != expected || !gaspolObligationsReady(obligations, expected) {
			return GaspolLockResult{}, errors.New("INCOMPLETE_ASSESSMENTS")
		}
	}
	candidates := make([]GaspolCompetitionCandidate, len(rows))
	for i := range rows {
		candidates[i] = rows[i].GaspolCompetitionCandidate
	}
	ranked := RankGaspolCompetition(candidates, stage)
	sourcePayload := mustJSON(ranked)
	ranked, err = selectGaspolDetermination(ranked, limit, kind == "BOTTOM_5" || kind == "FINAL", input.ManualTieSchoolID, input.Order, input.Reason)
	if err != nil {
		return GaspolLockResult{}, err
	}
	payload, _ := json.Marshal(ranked)
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return GaspolLockResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `LOCK TABLE gaspol_stage_assessments,gaspol_stage_assignments,gaspol_assessments,gaspol_school_submissions,gaspol_school_participants,gaspol_cohorts,gaspol_result_locks IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return GaspolLockResult{}, err
	}
	latestVersion, e := gaspolStageDataVersion(ctx, tx, year)
	if e != nil {
		return GaspolLockResult{}, e
	}
	if latestVersion != version {
		return GaspolLockResult{}, errors.New("RESULT_PREVIEW_STALE")
	}
	if kind == "TOP_5" || kind == "FINAL" {
		if err = gaspolBindBA(ctx, tx, year, *input.BADocumentID, kind, input.BANumber, input.BADate); err != nil {
			return GaspolLockResult{}, err
		}
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "gaspol-phase:"+strconv.Itoa(year)); err != nil {
		return GaspolLockResult{}, err
	}
	var alreadyLocked bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=$1 AND lock_type=$2 AND status='LOCKED')`, year, kind).Scan(&alreadyLocked); err != nil {
		return GaspolLockResult{}, err
	}
	if alreadyLocked {
		return GaspolLockResult{}, errors.New("RESULT_ALREADY_LOCKED")
	}
	if kind == "TOP_10" {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "gaspol-top10:"+strconv.Itoa(year)); err != nil {
			return GaspolLockResult{}, err
		}
		var unfinished int
		if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_stage_assessments WHERE year=$1 AND stage='FIELD_STAGE_1' AND workflow_status<>'FINAL'`, year).Scan(&unfinished); err != nil {
			return GaspolLockResult{}, err
		}
		if unfinished > 0 {
			return GaspolLockResult{}, errors.New("INCOMPLETE_ASSESSMENTS")
		}
	}
	var revision int
	_ = tx.QueryRow(ctx, `SELECT COALESCE(MAX(revision),0)+1 FROM gaspol_result_locks WHERE year=$1 AND lock_type=$2`, year, kind).Scan(&revision)
	var lockID int64
	err = tx.QueryRow(ctx, `INSERT INTO gaspol_result_locks(year,lock_type,revision,reason,ba_document_id,ranking_payload,tie_resolution,locked_by,source_payload) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8,$9::jsonb) RETURNING id`, year, kind, revision, strings.TrimSpace(input.Reason), input.BADocumentID, string(payload), string(mustJSON(map[string]any{"manualSchoolId": input.ManualTieSchoolID})), user.ID, string(sourcePayload)).Scan(&lockID)
	if err != nil {
		return GaspolLockResult{}, err
	}
	for i, row := range ranked {
		cohortName := kind
		if kind == "BOTTOM_5" {
			cohortName = "BOTTOM_5"
		}
		if kind == "FINAL" {
			cohortName = "TERGAGAL"
		}
		_, err = tx.Exec(ctx, `INSERT INTO gaspol_cohorts(year,school_id,cohort,source_lock_id,locked_by) VALUES($1,$2,$3,$4,$5) ON CONFLICT(year,school_id,cohort) DO UPDATE SET source_lock_id=EXCLUDED.source_lock_id,locked_at=NOW(),locked_by=EXCLUDED.locked_by,released_at=NULL,release_reason=NULL`, year, row.SchoolID, cohortName, lockID, user.ID)
		if err != nil {
			return GaspolLockResult{}, err
		}
		rowPayload, _ := json.Marshal(row)
		_, err = tx.Exec(ctx, `INSERT INTO gaspol_ranking_snapshots_v2(year,snapshot_type,revision,school_id,rank_position,cohort,score_total,maximum_score,payload,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10)`, year, kind, revision, row.SchoolID, i+1, cohortName, row.Total, row.Maximum, string(rowPayload), user.ID)
		if err != nil {
			return GaspolLockResult{}, err
		}
		if kind == "FINAL" {
			_, err = tx.Exec(ctx, `INSERT INTO gaspol_awards(year,award_type,school_id,score,metadata,locked_by) VALUES($1,'FAILED',$2,$3,$4::jsonb,$5) ON CONFLICT(year,award_type,school_id) DO UPDATE SET score=EXCLUDED.score,metadata=EXCLUDED.metadata,locked_by=EXCLUDED.locked_by,locked_at=NOW(),status='LOCKED'`, year, row.SchoolID, row.Total, string(rowPayload), user.ID)
			if err != nil {
				return GaspolLockResult{}, err
			}
		}
	}
	if kind == "TOP_10" {
		selected := map[int]bool{}
		for _, r := range ranked {
			selected[r.SchoolID] = true
		}
		all := rows
		allCandidates := make([]GaspolCompetitionCandidate, len(all))
		for index := range all {
			allCandidates[index] = all[index].GaspolCompetitionCandidate
		}
		remaining := make([]GaspolCompetitionCandidate, 0, len(allCandidates))
		for _, candidate := range RankGaspolCompetition(allCandidates, GaspolStageField1) {
			if !selected[candidate.SchoolID] {
				remaining = append(remaining, candidate)
			}
		}
		// Nominasi Tidak Tergagal is the next eleven positions after the
		// locked Top 10 (cohort ranks 11-21), never every non-Top-10 school.
		if len(remaining) > 11 {
			remaining = remaining[:11]
		}
		for _, candidate := range remaining {
			_, err = tx.Exec(ctx, `INSERT INTO gaspol_cohorts(year,school_id,cohort,source_lock_id,locked_by) VALUES($1,$2,'NOT_FAILED',$3,$4) ON CONFLICT(year,school_id,cohort) DO UPDATE SET source_lock_id=EXCLUDED.source_lock_id,released_at=NULL,release_reason=NULL`, year, candidate.SchoolID, lockID, user.ID)
			if err != nil {
				return GaspolLockResult{}, err
			}
		}
	}
	if kind == "BOTTOM_5" {
		_, err = tx.Exec(ctx, `INSERT INTO gaspol_cohorts(year,school_id,cohort,source_lock_id,locked_by)
  SELECT year,school_id,'PASSED_NOMINATION',$2,$3 FROM gaspol_cohorts n WHERE n.year=$1 AND n.cohort='NOT_FAILED' AND n.released_at IS NULL
  AND NOT EXISTS(SELECT 1 FROM gaspol_cohorts b WHERE b.year=n.year AND b.school_id=n.school_id AND b.cohort='BOTTOM_5' AND b.released_at IS NULL)
  ON CONFLICT(year,school_id,cohort) DO UPDATE SET source_lock_id=EXCLUDED.source_lock_id,locked_at=NOW(),locked_by=EXCLUDED.locked_by,released_at=NULL,release_reason=NULL`, year, lockID, user.ID)
		if err != nil {
			return GaspolLockResult{}, err
		}
	}
	if kind == "BOTTOM_5" || kind == "FINAL" {
		reportKind := GaspolStageNomination
		if kind == "FINAL" {
			reportKind = GaspolStageAppeal
		}
		_, err = tx.Exec(ctx, `UPDATE gaspol_nomination_submissions SET status='LOCKED',updated_at=NOW() WHERE year=$1 AND submission_type=$2`, year, reportKind)
		if err != nil {
			return GaspolLockResult{}, err
		}
	}
	if kind == "FINAL" {
		failed := map[int]bool{}
		for _, r := range ranked {
			failed[r.SchoolID] = true
		}
		all := rows
		for _, r := range all {
			if !failed[r.SchoolID] {
				_, err = tx.Exec(ctx, `INSERT INTO gaspol_cohorts(year,school_id,cohort,source_lock_id,locked_by) VALUES($1,$2,'ESCAPED_BOTTOM_ZONE',$3,$4) ON CONFLICT(year,school_id,cohort) DO UPDATE SET source_lock_id=EXCLUDED.source_lock_id,released_at=NULL,release_reason=NULL`, year, r.SchoolID, lockID, user.ID)
				if err != nil {
					return GaspolLockResult{}, err
				}
			}
		}
	}
	state := map[string]string{"TOP_10": "TOP10_LOCKED", "TOP_5": "TOP5_LOCKED", "BOTTOM_5": "BOTTOM5_LOCKED", "FINAL": "APPEAL_FINAL"}[kind]
	_, err = tx.Exec(ctx, `UPDATE gaspol_competition_configs SET state=$2,updated_by=$3,updated_at=NOW() WHERE year=$1`, year, state, user.ID)
	if err != nil {
		return GaspolLockResult{}, err
	}
	if err = gaspolAuditTx(ctx, tx, user, year, kind+"_LOCKED", "result_lock", lockID, nil, input); err != nil {
		return GaspolLockResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return GaspolLockResult{}, err
	}
	return GaspolLockResult{ID: lockID, Year: year, LockType: kind, Revision: revision, Rows: ranked}, nil
}

func mustJSON(value any) []byte { raw, _ := json.Marshal(value); return raw }

func (s GaspolService) ReopenCompetitionResult(ctx context.Context, user auth.User, lockID int64, reason string) error {
	if err := s.ensureAdminMaster(user); err != nil {
		return err
	}
	if strings.TrimSpace(reason) == "" {
		return errors.New("REOPEN_REASON_REQUIRED")
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var year int
	var kind string
	if err = tx.QueryRow(ctx, `SELECT year,lock_type FROM gaspol_result_locks WHERE id=$1 AND status='LOCKED'`, lockID).Scan(&year, &kind); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "gaspol-phase:"+strconv.Itoa(year)); err != nil {
		return err
	}
	var dependent bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=$1 AND status='LOCKED' AND (($2='TOP_10' AND lock_type IN ('TOP_5','BOTTOM_5','FINAL')) OR ($2='BOTTOM_5' AND lock_type='FINAL') OR ($2='TOP_5' AND lock_type='AWARDING')))`, year, kind).Scan(&dependent); err != nil {
		return err
	}
	if kind == "TOP_10" || kind == "AWARDING" {
		return errors.New("OFFICIAL_REVISION_REQUIRED")
	}
	if dependent {
		return errors.New("RESULT_HAS_LOCKED_DEPENDENTS")
	}
	var standingsExist bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_ranking_snapshots_v2 WHERE year=$1 AND snapshot_type='STANDINGS')`, year).Scan(&standingsExist); err != nil {
		return err
	}
	if standingsExist {
		return errors.New("RESULT_HAS_LOCKED_DEPENDENTS")
	}
	err = tx.QueryRow(ctx, `UPDATE gaspol_result_locks SET status='REOPENED',reopened_by=$2,reopened_at=NOW(),reopen_reason=$3 WHERE id=$1 AND status='LOCKED' RETURNING year,lock_type`, lockID, user.ID, strings.TrimSpace(reason)).Scan(&year, &kind)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE gaspol_cohorts SET released_at=NOW(),release_reason=$2 WHERE source_lock_id=$1 AND released_at IS NULL`, lockID, strings.TrimSpace(reason))
	if err != nil {
		return err
	}
	if kind == "FINAL" {
		if _, err = tx.Exec(ctx, `UPDATE gaspol_awards SET status='REOPENED' WHERE year=$1 AND award_type='FAILED'`, year); err != nil {
			return err
		}
	}
	if err = gaspolAuditTx(ctx, tx, user, year, kind+"_REOPENED", "result_lock", lockID, nil, map[string]any{"reason": reason}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s GaspolService) publicLockedRanking(ctx context.Context, year int, state string) ([]GaspolRankingRow, string, error) {
	if state == "NOT_FAILED_NOMINATION_FINAL" || state == "BOTTOM5_LOCKED" || state == "APPEAL_ACTIVE" || state == "APPEAL_FINAL" || state == "FINAL_AWARD_READY" || state == "GASPOL_FINALIZED" {
		return s.publicFullCompetitionRanking(ctx, year, state)
	}
	typeName := ""
	stage := GaspolStageMonthly
	switch state {
	case "TOP10_LOCKED", "FIELD2_ACTIVE", "FIELD2_FINAL", "TOP5_WAITING_BA":
		typeName = "TOP_10"
		stage = GaspolStageField1
	case "TOP5_LOCKED", "PRESENTATION_ACTIVE", "PRESENTATION_FINAL", "NOT_FAILED_NOMINATION_ACTIVE", "NOT_FAILED_NOMINATION_FINAL", "BOTTOM5_LOCKED", "APPEAL_ACTIVE", "APPEAL_FINAL", "FINAL_AWARD_READY", "GASPOL_FINALIZED":
		typeName = "TOP_5"
		stage = GaspolStageField2
	}
	if typeName == "" {
		return nil, stage, nil
	}
	now, err := s.Clock.Now(ctx)
	if err != nil {
		return nil, stage, err
	}
	monthly, err := s.Repo.ListMonthlyCompetitionComponents(ctx, year, 0, now)
	if err != nil {
		return nil, stage, err
	}
	rows, err := s.DB.Pool.Query(ctx, `SELECT s.payload FROM gaspol_ranking_snapshots_v2 s JOIN gaspol_result_locks l ON l.year=s.year AND l.lock_type=s.snapshot_type AND l.revision=s.revision AND l.status='LOCKED' WHERE s.year=$1 AND s.snapshot_type=$2 ORDER BY s.rank_position`, year, typeName)
	if err != nil {
		return nil, stage, err
	}
	defer rows.Close()
	result := []GaspolRankingRow{}
	for rows.Next() {
		var raw []byte
		var candidate GaspolCompetitionCandidate
		if err = rows.Scan(&raw); err != nil {
			return nil, stage, err
		}
		if err = json.Unmarshal(raw, &candidate); err != nil {
			return nil, stage, err
		}
		rank := candidate.Rank
		if typeName == "TOP_5" {
			rank = 0
		}
		completeness := gaspolCandidateCompleteness{GaspolCompetitionCandidate: candidate, HasField1: true, HasField2: typeName == "TOP_5"}
		components := monthly[candidate.SchoolID]
		result = append(result, GaspolRankingRow{Rank: rank, Year: year, Stage: stage, SchoolID: candidate.SchoolID, SchoolName: candidate.SchoolName, Path: "MAIN", MonthlyComponents: components, MonthlyFinal: gaspolMonthlySeriesIsFinal(components), Scores: gaspolCompetitionScore(candidate, "MAIN", completeness)})
	}
	if typeName == "TOP_5" {
		sort.Slice(result, func(i, j int) bool {
			return strings.ToLower(result[i].SchoolName) < strings.ToLower(result[j].SchoolName)
		})
	}
	return result, stage, rows.Err()
}

func (s GaspolService) publicFullCompetitionRanking(ctx context.Context, year int, state string) ([]GaspolRankingRow, string, error) {
	rows, err := s.officialStandings(ctx, year)
	return rows, GaspolStageFinal, err
}
