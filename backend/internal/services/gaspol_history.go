package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
)

const (
	GaspolHistorySubmissions = "submissions"
	GaspolHistoryAssessments = "assessments"
	GaspolHistoryValidations = "validations"
	GaspolHistoryJury        = "jury"
	GaspolHistoryEvents      = "events"
)

func historyEventTimelineScope(hasEntityReference bool) string {
	if hasEntityReference {
		return `gal.year=$1 AND gal.entity_type=$2 AND gal.entity_id=$3`
	}
	return `gal.year=$1 AND gal.entity_id IS NULL AND gal.school_id=$2
		AND ($3='' OR COALESCE(gal.payload->>'month',gal.payload->>'Month','')=$3)
		AND ($4='' OR COALESCE(gal.payload->>'stage',gal.payload->>'Stage','')=$4)`
}

func historyRecordTimelineScope(hasLinkedStageAssessment bool) string {
	if hasLinkedStageAssessment {
		return `gal.year=$1 AND ((gal.entity_type=$2 AND gal.entity_id=$3)
			OR (gal.entity_type='stage_assessment' AND gal.entity_id=$4))`
	}
	return `gal.year=$1 AND gal.entity_type=$2 AND gal.entity_id=$3`
}

func normalizeGaspolHistoryFilter(input GaspolHistoryFilter) (GaspolHistoryFilter, error) {
	input.Tab = strings.ToLower(strings.TrimSpace(input.Tab))
	if input.Tab == "" {
		input.Tab = GaspolHistorySubmissions
	}
	if input.Tab != GaspolHistorySubmissions && input.Tab != GaspolHistoryAssessments && input.Tab != GaspolHistoryValidations && input.Tab != GaspolHistoryJury && input.Tab != GaspolHistoryEvents {
		return input, errors.New("INVALID_HISTORY_TAB")
	}
	if input.Year < 2024 || input.Year > 2100 {
		return input, errors.New("INVALID_YEAR")
	}
	if input.Month < 0 || input.Month > 12 {
		return input, errors.New("INVALID_MONTH")
	}
	for _, value := range []string{input.DateFrom, input.DateTo} {
		if value == "" {
			continue
		}
		if len(value) != 10 || value[4] != '-' || value[7] != '-' {
			return input, errors.New("INVALID_DATE")
		}
	}
	if input.DateFrom != "" && input.DateTo != "" && input.DateFrom > input.DateTo {
		return input, errors.New("INVALID_DATE_RANGE")
	}
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	input.Stage = strings.ToUpper(strings.TrimSpace(input.Stage))
	input.Search = strings.TrimSpace(input.Search)
	if len(input.Search) > 100 {
		return input, errors.New("INVALID_SEARCH")
	}
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}
	return input, nil
}

func (s GaspolService) ensureHistoryAdmin(user auth.User) error {
	if !auth.IsAdmin(user) {
		return auth.ErrForbidden
	}
	return nil
}

type gaspolHistoryQuery struct {
	from      string
	where     string
	args      []any
	eventExpr string
}

func buildGaspolHistoryQuery(filter GaspolHistoryFilter, isMaster bool) gaspolHistoryQuery {
	q := gaspolHistoryQuery{}
	switch filter.Tab {
	case GaspolHistorySubmissions:
		q.from = `gaspol_school_submissions s
			JOIN data_sekolah ds ON ds.id = s.school_id
			LEFT JOIN users submitter ON submitter.id = s.submitted_by
			LEFT JOIN gaspol_assignments asg ON asg.year=s.year AND asg.month=s.month AND asg.school_id=s.school_id
			LEFT JOIN users sup ON sup.id=asg.supervisor_user_id
			LEFT JOIN gaspol_school_participants p ON p.year=s.year AND p.school_id=s.school_id AND p.is_year_configuration=FALSE
			LEFT JOIN users val ON val.id=p.validator_user_id
			LEFT JOIN LATERAL (SELECT ga.* FROM gaspol_assessments ga WHERE ga.year=s.year AND ga.month=s.month AND ga.school_id=s.school_id AND ga.stage='MONTHLY' AND ga.assessor_role='SUPERVISOR' ORDER BY ga.created_at, ga.id LIMIT 1) a ON TRUE`
		q.eventExpr = "s.submitted_at"
	case GaspolHistoryAssessments, GaspolHistoryValidations:
		q.from = `gaspol_assessments a
			JOIN data_sekolah ds ON ds.id=a.school_id
			LEFT JOIN users sup ON sup.id=a.assessor_user_id
			LEFT JOIN users val ON val.id=a.validated_by
			LEFT JOIN gaspol_school_participants p ON p.year=a.year AND p.school_id=a.school_id AND p.is_year_configuration=FALSE
			LEFT JOIN users assignedval ON assignedval.id=p.validator_user_id
			LEFT JOIN gaspol_school_submissions s ON s.year=a.year AND s.month=a.month AND s.school_id=a.school_id`
		if filter.Tab == GaspolHistoryAssessments {
			q.eventExpr = "a.created_at"
		} else {
			q.eventExpr = "a.validated_at"
		}
	case GaspolHistoryJury:
		q.from = `gaspol_assessments a
			JOIN data_sekolah ds ON ds.id=a.school_id
			LEFT JOIN users sup ON sup.id=a.assessor_user_id
			LEFT JOIN users val ON val.id=a.validated_by
			LEFT JOIN gaspol_school_submissions s ON s.year=a.year AND s.month=a.month AND s.school_id=a.school_id`
		q.eventExpr = "a.created_at"
	}

	conditions := []string{}
	add := func(condition string, value any) {
		q.args = append(q.args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(q.args)))
	}
	yearColumn := "a.year"
	monthColumn := "a.month"
	if filter.Tab == GaspolHistorySubmissions {
		yearColumn, monthColumn = "s.year", "s.month"
	}
	add(yearColumn+" = $%d", filter.Year)
	if filter.Month > 0 {
		add(monthColumn+" = $%d", filter.Month)
	}
	if filter.Tab == GaspolHistoryAssessments || filter.Tab == GaspolHistoryValidations {
		conditions = append(conditions, "a.stage='MONTHLY' AND a.assessor_role='SUPERVISOR'")
	}
	if filter.Tab == GaspolHistoryValidations {
		conditions = append(conditions, "a.is_validated=TRUE AND a.validated_at IS NOT NULL")
	}
	if filter.Tab == GaspolHistoryJury {
		conditions = append(conditions, "a.assessor_role='JURY' AND a.stage IN ('FIELD','PRESENTATION')")
	}
	if !isMaster {
		if len(filter.AllowedLevels) == 0 {
			conditions = append(conditions, "FALSE")
		} else {
			add("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan,''))) = ANY($%d)", filter.AllowedLevels)
		}
	}
	if filter.Search != "" {
		add("(COALESCE(ds.nama_sekolah,'') || ' ' || COALESCE(ds.npsn_sekolah,'')) ILIKE '%%' || $%d || '%%'", filter.Search)
	}
	if filter.SupervisorID > 0 {
		if filter.Tab == GaspolHistorySubmissions {
			add("asg.supervisor_user_id = $%d", filter.SupervisorID)
		} else if filter.Tab != GaspolHistoryJury {
			add("a.assessor_user_id = $%d", filter.SupervisorID)
		}
	}
	if filter.ValidatorID > 0 && filter.Tab != GaspolHistoryJury {
		if filter.Tab == GaspolHistorySubmissions {
			add("p.validator_user_id = $%d", filter.ValidatorID)
		} else if filter.Tab == GaspolHistoryAssessments {
			add("p.validator_user_id = $%d", filter.ValidatorID)
		} else {
			add("a.validated_by = $%d", filter.ValidatorID)
		}
	}
	if filter.Status != "" {
		switch filter.Tab {
		case GaspolHistorySubmissions:
			add("s.status::text = $%d", filter.Status)
		case GaspolHistoryAssessments:
			if filter.Status == "VALIDATED" {
				conditions = append(conditions, "a.is_validated=TRUE")
			} else if filter.Status == "PENDING" {
				conditions = append(conditions, "a.is_validated=FALSE")
			}
		case GaspolHistoryValidations:
			add("COALESCE(a.metadata->>'validationDecision','AGREE') = $%d", filter.Status)
		case GaspolHistoryJury:
			if filter.Status == "VALIDATED" {
				conditions = append(conditions, "a.is_validated=TRUE")
			} else if filter.Status == "PENDING" {
				conditions = append(conditions, "a.is_validated=FALSE")
			}
		}
	}
	if filter.DateFrom != "" {
		add(q.eventExpr+" >= (($%d::date::timestamp AT TIME ZONE 'Asia/Jakarta') AT TIME ZONE 'UTC')", filter.DateFrom)
	}
	if filter.DateTo != "" {
		add(q.eventExpr+" < ((($%d::date + 1)::timestamp AT TIME ZONE 'Asia/Jakarta') AT TIME ZONE 'UTC')", filter.DateTo)
	}
	q.where = strings.Join(conditions, " AND ")
	return q
}

func gaspolHistorySelect(tab string) string {
	if tab == GaspolHistorySubmissions {
		return `s.id,s.year,s.month,'MONTHLY',s.school_id,ds.nama_sekolah,COALESCE(ds.npsn_sekolah,''),COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)),''),s.report_title,s.report_link,s.status::text,
		COALESCE(NULLIF(TRIM(submitter.nama_instansi),''),submitter.username,''),COALESCE(NULLIF(TRIM(sup.nama_instansi),''),sup.username,asg.supervisor_name_snapshot,''),COALESCE(NULLIF(TRIM(val.nama_instansi),''),val.username,''),'',s.submitted_at,
		CASE WHEN a.id IS NULL THEN NULL ELSE COALESCE((a.metadata->>'supervisorScore')::numeric,a.score_monthly) END,
		CASE WHEN a.metadata ? 'validatorScore' THEN (a.metadata->>'validatorScore')::numeric ELSE NULL END,
		CASE WHEN a.id IS NULL THEN NULL ELSE COALESCE((a.metadata->>'finalMonthlyScore')::numeric,a.score_monthly) END,NULL::numeric,'','',CASE WHEN a.is_validated THEN COALESCE(NULLIF(a.metadata->>'validationDecision',''),'AGREE') ELSE '' END`
	}
	validatorName := "COALESCE(NULLIF(TRIM(val.nama_instansi),''),val.username,'')"
	if tab == GaspolHistoryAssessments {
		validatorName = "COALESCE(NULLIF(TRIM(assignedval.nama_instansi),''),assignedval.username,'')"
	}
	return `a.id,a.year,a.month,a.stage::text,a.school_id,ds.nama_sekolah,COALESCE(ds.npsn_sekolah,''),COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)),''),COALESCE(s.report_title,''),COALESCE(s.report_link,''),
	CASE WHEN a.assessor_role='JURY' THEN CASE WHEN a.is_validated THEN 'VALIDATED' ELSE 'SUBMITTED' END WHEN a.is_validated THEN 'VALIDATED' ELSE 'PENDING' END,
	'',COALESCE(NULLIF(TRIM(sup.nama_instansi),''),sup.username,''),` + validatorName + `,COALESCE(NULLIF(TRIM(sup.nama_instansi),''),sup.username,''),` + map[string]string{GaspolHistoryAssessments: "a.created_at", GaspolHistoryValidations: "a.validated_at", GaspolHistoryJury: "a.created_at"}[tab] + `,
	CASE WHEN a.assessor_role='SUPERVISOR' THEN COALESCE((a.metadata->>'supervisorScore')::numeric,a.score_monthly) ELSE NULL END,
	CASE WHEN a.metadata ? 'validatorScore' THEN (a.metadata->>'validatorScore')::numeric ELSE NULL END,
	CASE WHEN a.assessor_role='SUPERVISOR' THEN COALESCE((a.metadata->>'finalMonthlyScore')::numeric,a.score_monthly) ELSE NULL END,
	CASE a.stage::text WHEN 'FIELD' THEN a.score_field WHEN 'PRESENTATION' THEN a.score_presentation ELSE a.score_monthly END,
	a.notes,COALESCE(a.metadata->>'validatorNotes',''),CASE WHEN a.is_validated THEN COALESCE(NULLIF(a.metadata->>'validationDecision',''),'AGREE') ELSE '' END`
}

func scanGaspolHistoryRows(rows pgx.Rows) ([]GaspolHistoryRow, error) {
	items := make([]GaspolHistoryRow, 0)
	for rows.Next() {
		var item GaspolHistoryRow
		if err := rows.Scan(&item.ID, &item.Year, &item.Month, &item.Stage, &item.SchoolID, &item.SchoolName, &item.NPSN, &item.SchoolLevel, &item.ReportTitle, &item.ReportLink, &item.Status, &item.SubmittedByName, &item.SupervisorName, &item.ValidatorName, &item.ActorName, &item.EventAt, &item.SupervisorScore, &item.ValidatorScore, &item.FinalScore, &item.Score, &item.Notes, &item.ValidatorNotes, &item.ValidationDecision); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const gaspolEventStageExpression = `COALESCE(sa.stage,ns.submission_type,
	CASE
	 WHEN gal.entity_type='innovation_nomination' OR gal.action LIKE 'INNOVATION_%' THEN 'INNOVATION'
	 WHEN gal.action IN ('TOP_10_LOCKED','TOP_10_REOPENED') THEN 'TOP_10_LOCK'
	 WHEN gal.action IN ('TOP_5_LOCKED','TOP_5_REOPENED') THEN 'TOP_5_LOCK'
	 WHEN gal.action IN ('BOTTOM_5_LOCKED','BOTTOM_5_REOPENED') THEN 'BOTTOM_5_LOCK'
	 WHEN gal.action IN ('FINAL_LOCKED','FINAL_REOPENED') THEN 'FINAL_RESULT'
	 WHEN gal.entity_type='award' OR gal.action LIKE '%AWARD_LOCKED' THEN 'AWARDING'
	 WHEN gal.action LIKE 'FAVORITE_VOTE%' THEN 'FAVORITE_VOTE'
	 WHEN gal.action LIKE 'LIKE_%' OR gal.action='FAVORITE_CONTENT_LINK_TESTED' THEN 'LIKE_SYNC'
	 ELSE COALESCE(gal.payload->>'stage',gal.payload->>'Stage','') END)`

const gaspolEventSelect = `gal.id,COALESCE(gal.year,0),COALESCE(sa.month,0),` + gaspolEventStageExpression + `,
	COALESCE(gal.school_id,0),COALESCE(ds.nama_sekolah,''),COALESCE(ds.npsn_sekolah,''),COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)),''),
	'','',COALESCE(NULLIF(gal.payload->>'after',''),gal.action),'',
	CASE WHEN gal.action IN ('ASSESSMENT_SUBMITTED','ASSESSMENT_RESUBMITTED') THEN COALESCE(NULLIF(TRIM(actor.nama_instansi),''),actor.username,'') ELSE '' END,
	CASE WHEN gal.action LIKE 'CABDIN_%' THEN COALESCE(NULLIF(TRIM(actor.nama_instansi),''),actor.username,'') ELSE '' END,
	COALESCE(NULLIF(TRIM(actor.nama_instansi),''),actor.username,''),gal.created_at,
	CASE WHEN gal.action IN ('ASSESSMENT_SUBMITTED','ASSESSMENT_RESUBMITTED') THEN sa.raw_score ELSE NULL END,
	NULL::numeric,sa.final_score,sa.raw_score,
	COALESCE(NULLIF(sa.notes,''),gal.payload->>'notes',''),'',COALESCE(NULLIF(gal.payload->>'after',''),gal.action),
	gal.action,CASE
	 WHEN gal.action LIKE 'KOOR_%' THEN 'KOOR_GASPOL'
	 WHEN gal.action LIKE 'CABDIN_%' THEN 'VALIDATOR_CABDIN'
	 WHEN gal.action IN ('INNOVATION_APPROVED','INNOVATION_RETURNED','INNOVATION_AWARD_LOCKED') THEN 'KASI_SMK'
	 WHEN gal.action IN ('ASSESSMENT_SUBMITTED','ASSESSMENT_RESUBMITTED','DIRECT_STAGE_FINALIZED') AND sa.stage='INNOVATION' THEN 'ASSESSOR_INNOVATION'
	 WHEN gal.action IN ('ASSESSMENT_SUBMITTED','ASSESSMENT_RESUBMITTED','DIRECT_STAGE_FINALIZED') THEN 'ASSESSOR'
	 ELSE gal.actor_role END,
	COALESCE(gal.payload->>'before',''),COALESCE(gal.payload->>'after',''),
	NULLIF(gal.payload->>'aggregate','')::numeric,gal.entity_type || CASE WHEN gal.entity_id IS NULL THEN '' ELSE '#' || gal.entity_id::text END`

func scanGaspolEventRows(rows pgx.Rows) ([]GaspolHistoryRow, error) {
	items := []GaspolHistoryRow{}
	for rows.Next() {
		var item GaspolHistoryRow
		if err := rows.Scan(&item.ID, &item.Year, &item.Month, &item.Stage, &item.SchoolID, &item.SchoolName, &item.NPSN, &item.SchoolLevel, &item.ReportTitle, &item.ReportLink, &item.Status, &item.SubmittedByName, &item.SupervisorName, &item.ValidatorName, &item.ActorName, &item.EventAt, &item.SupervisorScore, &item.ValidatorScore, &item.FinalScore, &item.Score, &item.Notes, &item.ValidatorNotes, &item.ValidationDecision, &item.Action, &item.ActorRole, &item.BeforeStatus, &item.AfterStatus, &item.AggregateScore, &item.Reference); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s GaspolService) historyEvents(ctx context.Context, filter GaspolHistoryFilter, isMaster, export bool, auditID int) (GaspolHistoryResponse, error) {
	from := `gaspol_audit_logs gal
		LEFT JOIN users actor ON actor.id=gal.actor_user_id
		LEFT JOIN data_sekolah ds ON ds.id=gal.school_id
		LEFT JOIN gaspol_stage_assessments sa ON gal.entity_type='stage_assessment' AND sa.id=gal.entity_id
		LEFT JOIN gaspol_nomination_submissions ns ON gal.entity_type='nomination_submission' AND ns.id=gal.entity_id`
	conditions := []string{"gal.year=$1"}
	args := []any{filter.Year}
	add := func(template string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(template, len(args)))
	}
	if auditID > 0 {
		add("gal.id=$%d", auditID)
	}
	if filter.Month > 0 {
		add("COALESCE(sa.month,0)=$%d", filter.Month)
	}
	if filter.Stage != "" {
		add(gaspolEventStageExpression+"=$%d", filter.Stage)
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		index := len(args)
		conditions = append(conditions, fmt.Sprintf("(gal.action=$%d OR COALESCE(gal.payload->>'after','')=$%d)", index, index))
	}
	if filter.Search != "" {
		add("(COALESCE(ds.nama_sekolah,'') || ' ' || COALESCE(ds.npsn_sekolah,'') || ' ' || gal.action) ILIKE '%%' || $%d || '%%'", filter.Search)
	}
	if filter.SupervisorID > 0 {
		add("gal.actor_user_id=$%d", filter.SupervisorID)
	}
	if filter.ValidatorID > 0 {
		add("gal.actor_user_id=$%d", filter.ValidatorID)
	}
	if filter.DateFrom != "" {
		add("gal.created_at >= (($%d::date::timestamp AT TIME ZONE 'Asia/Jakarta') AT TIME ZONE 'UTC')", filter.DateFrom)
	}
	if filter.DateTo != "" {
		add("gal.created_at < ((($%d::date + 1)::timestamp AT TIME ZONE 'Asia/Jakarta') AT TIME ZONE 'UTC')", filter.DateTo)
	}
	if !isMaster {
		if len(filter.AllowedLevels) == 0 {
			conditions = append(conditions, "FALSE")
		} else {
			add("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan,'')))=ANY($%d)", filter.AllowedLevels)
		}
	}
	where := strings.Join(conditions, " AND ")
	var total int
	if err := s.DB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+from+" WHERE "+where, args...).Scan(&total); err != nil {
		return GaspolHistoryResponse{}, err
	}
	queryArgs := append([]any{}, args...)
	query := "SELECT " + gaspolEventSelect + " FROM " + from + " WHERE " + where + " ORDER BY gal.created_at DESC,gal.id DESC"
	if !export {
		queryArgs = append(queryArgs, filter.PageSize, (filter.Page-1)*filter.PageSize)
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(queryArgs)-1, len(queryArgs))
	}
	rows, err := s.DB.Pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return GaspolHistoryResponse{}, err
	}
	defer rows.Close()
	items, err := scanGaspolEventRows(rows)
	if err != nil {
		return GaspolHistoryResponse{}, err
	}
	summary := GaspolHistorySummary{}
	for _, item := range items {
		if item.Action == "ASSESSMENT_SUBMITTED" || item.Action == "ASSESSMENT_RESUBMITTED" {
			summary.Assessed++
		}
		if strings.HasPrefix(item.Action, "KOOR_") || strings.HasPrefix(item.Action, "CABDIN_") {
			summary.Validated++
		}
		if strings.Contains(item.AfterStatus, "WAITING") || strings.Contains(item.AfterStatus, "RETURNED") {
			summary.PendingValidation++
		}
	}
	supervisors, validators, err := s.historyOptions(ctx, filter.Year, filter.AllowedLevels, isMaster)
	if err != nil {
		return GaspolHistoryResponse{}, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(filter.PageSize)))
	}
	return GaspolHistoryResponse{Items: items, Summary: summary, Total: total, Page: filter.Page, PageSize: filter.PageSize, TotalPages: totalPages, Supervisors: supervisors, Validators: validators}, nil
}

func (s GaspolService) History(ctx context.Context, user auth.User, raw GaspolHistoryFilter, export bool) (GaspolHistoryResponse, error) {
	if err := s.ensureHistoryAdmin(user); err != nil {
		return GaspolHistoryResponse{}, err
	}
	filter, err := normalizeGaspolHistoryFilter(raw)
	if err != nil {
		return GaspolHistoryResponse{}, err
	}
	isMaster := auth.IsAdminMaster(user)
	if !isMaster {
		filter.AllowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
	}
	if filter.Tab == GaspolHistoryEvents {
		return s.historyEvents(ctx, filter, isMaster, export, 0)
	}
	q := buildGaspolHistoryQuery(filter, isMaster)
	var total int
	if err := s.DB.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+q.from+" WHERE "+q.where, q.args...).Scan(&total); err != nil {
		return GaspolHistoryResponse{}, err
	}
	args := append([]any{}, q.args...)
	query := "SELECT " + gaspolHistorySelect(filter.Tab) + " FROM " + q.from + " WHERE " + q.where + " ORDER BY " + q.eventExpr + " DESC NULLS LAST, 1 DESC"
	if !export {
		args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	}
	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return GaspolHistoryResponse{}, err
	}
	defer rows.Close()
	items, err := scanGaspolHistoryRows(rows)
	if err != nil {
		return GaspolHistoryResponse{}, err
	}

	summary, err := s.historySummary(ctx, filter.Tab, q)
	if err != nil {
		return GaspolHistoryResponse{}, err
	}

	supervisors, validators, err := s.historyOptions(ctx, filter.Year, filter.AllowedLevels, isMaster)
	if err != nil {
		return GaspolHistoryResponse{}, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(filter.PageSize)))
	}
	return GaspolHistoryResponse{Items: items, Summary: summary, Total: total, Page: filter.Page, PageSize: filter.PageSize, TotalPages: totalPages, Supervisors: supervisors, Validators: validators}, nil
}

func (s GaspolService) historySummary(ctx context.Context, tab string, q gaspolHistoryQuery) (GaspolHistorySummary, error) {
	aliases := `(id,year,month,stage,school_id,school_name,npsn,school_level,report_title,report_link,status,submitted_by_name,supervisor_name,validator_name,actor_name,event_at,supervisor_score,validator_score,final_score,score,notes,validator_notes,validation_decision)`
	juryExpression := "0"
	if tab == GaspolHistoryJury {
		juryExpression = "COUNT(*)"
	}
	query := `SELECT
		COUNT(*) FILTER (WHERE report_link<>'' AND status<>'DRAFT'),
		COUNT(supervisor_score),
		COUNT(*) FILTER (WHERE validation_decision<>'' OR status='VALIDATED'),
		COUNT(*) FILTER (WHERE supervisor_score IS NOT NULL AND validation_decision=''),
		` + juryExpression + `
		FROM (SELECT ` + gaspolHistorySelect(tab) + ` FROM ` + q.from + ` WHERE ` + q.where + `) AS history` + aliases
	var result GaspolHistorySummary
	err := s.DB.Pool.QueryRow(ctx, query, q.args...).Scan(&result.Submitted, &result.Assessed, &result.Validated, &result.PendingValidation, &result.JuryAssessed)
	return result, err
}

func (s GaspolService) historyOptions(ctx context.Context, year int, levels []string, isMaster bool) ([]GaspolHistoryOption, []GaspolHistoryOption, error) {
	condition := ""
	args := []any{year}
	if !isMaster {
		if len(levels) == 0 {
			condition = " AND FALSE"
		} else {
			args = append(args, levels)
			condition = " AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan,'')))=ANY($2)"
		}
	}
	rows, err := s.DB.Pool.Query(ctx, `SELECT DISTINCT u.id,COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username) FROM gaspol_assignments a JOIN data_sekolah ds ON ds.id=a.school_id JOIN users u ON u.id=a.supervisor_user_id WHERE a.year=$1`+condition+` ORDER BY 2`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	supervisors := []GaspolHistoryOption{}
	for rows.Next() {
		var x GaspolHistoryOption
		if err := rows.Scan(&x.ID, &x.Name); err != nil {
			return nil, nil, err
		}
		supervisors = append(supervisors, x)
	}
	rows2, err := s.DB.Pool.Query(ctx, `SELECT DISTINCT u.id,COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username) FROM gaspol_school_participants p JOIN data_sekolah ds ON ds.id=p.school_id JOIN users u ON u.id=p.validator_user_id WHERE p.year=$1`+condition+` ORDER BY 2`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows2.Close()
	validators := []GaspolHistoryOption{}
	for rows2.Next() {
		var x GaspolHistoryOption
		if err := rows2.Scan(&x.ID, &x.Name); err != nil {
			return nil, nil, err
		}
		validators = append(validators, x)
	}
	return supervisors, validators, rows2.Err()
}

func (s GaspolService) HistoryDetail(ctx context.Context, user auth.User, tab string, id int) (GaspolHistoryDetail, error) {
	if err := s.ensureHistoryAdmin(user); err != nil {
		return GaspolHistoryDetail{}, err
	}
	if id <= 0 {
		return GaspolHistoryDetail{}, errors.New("INVALID_ID")
	}
	filter := GaspolHistoryFilter{Tab: tab, Year: 2024, Page: 1, PageSize: 1}
	filter, err := normalizeGaspolHistoryFilter(filter)
	if err != nil {
		return GaspolHistoryDetail{}, err
	}
	isMaster := auth.IsAdminMaster(user)
	if !isMaster {
		filter.AllowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
	}
	if filter.Tab == GaspolHistoryEvents {
		if err = s.DB.Pool.QueryRow(ctx, `SELECT COALESCE(year,0) FROM gaspol_audit_logs WHERE id=$1`, id).Scan(&filter.Year); err != nil {
			return GaspolHistoryDetail{}, err
		}
		payload, eventErr := s.historyEvents(ctx, filter, isMaster, false, id)
		if eventErr != nil {
			return GaspolHistoryDetail{}, eventErr
		}
		if len(payload.Items) == 0 {
			return GaspolHistoryDetail{}, pgx.ErrNoRows
		}
		logs, timelineErr := s.historyEventTimeline(ctx, id)
		if timelineErr != nil {
			return GaspolHistoryDetail{}, timelineErr
		}
		return GaspolHistoryDetail{Row: payload.Items[0], Timeline: logs}, nil
	}
	q := buildGaspolHistoryQuery(filter, isMaster)
	// Replace the synthetic year predicate with an id predicate; scope clauses remain intact.
	q.where = strings.Replace(q.where, map[bool]string{true: "s.year = $1", false: "a.year = $1"}[tab == GaspolHistorySubmissions], map[bool]string{true: "s.id = $1", false: "a.id = $1"}[tab == GaspolHistorySubmissions], 1)
	q.args[0] = id
	rows, err := s.DB.Pool.Query(ctx, "SELECT "+gaspolHistorySelect(tab)+" FROM "+q.from+" WHERE "+q.where, q.args...)
	if err != nil {
		return GaspolHistoryDetail{}, err
	}
	defer rows.Close()
	items, err := scanGaspolHistoryRows(rows)
	if err != nil {
		return GaspolHistoryDetail{}, err
	}
	if len(items) == 0 {
		return GaspolHistoryDetail{}, pgx.ErrNoRows
	}
	logs, err := s.historyTimeline(ctx, items[0], tab)
	if err != nil {
		return GaspolHistoryDetail{}, err
	}
	return GaspolHistoryDetail{Row: items[0], Timeline: logs}, nil
}

func (s GaspolService) historyEventTimeline(ctx context.Context, auditID int) ([]GaspolAuditLogRow, error) {
	var year int
	var entityType string
	var entityID, schoolID *int
	var month, stage string
	if err := s.DB.Pool.QueryRow(ctx, `
		SELECT COALESCE(year,0),COALESCE(entity_type,''),entity_id,school_id,
		       COALESCE(payload->>'month',payload->>'Month',''),
		       COALESCE(payload->>'stage',payload->>'Stage','')
		FROM gaspol_audit_logs WHERE id=$1
	`, auditID).Scan(&year, &entityType, &entityID, &schoolID, &month, &stage); err != nil {
		return nil, err
	}
	baseQuery := `SELECT gal.id,gal.year,gal.actor_user_id,COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username,''),gal.actor_role,gal.action,gal.entity_type,gal.entity_id,gal.school_id,COALESCE(ds.nama_sekolah,''),gal.payload::text,gal.created_at
		FROM gaspol_audit_logs gal LEFT JOIN users u ON u.id=gal.actor_user_id LEFT JOIN data_sekolah ds ON ds.id=gal.school_id
		WHERE `
	var rows pgx.Rows
	var err error
	if strings.TrimSpace(entityType) != "" && entityID != nil {
		// A valid entity reference is authoritative. Never broaden this query to
		// the school, because one school has many independent assessments.
		rows, err = s.DB.Pool.Query(ctx, baseQuery+historyEventTimelineScope(true)+` ORDER BY gal.created_at,gal.id`, year, entityType, *entityID)
	} else {
		// Legacy fallback is deliberately limited to other unreferenced audit
		// rows and, when present, the same month/stage payload.
		rows, err = s.DB.Pool.Query(ctx, baseQuery+historyEventTimelineScope(false)+` ORDER BY gal.created_at,gal.id`, year, schoolID, month, stage)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []GaspolAuditLogRow{}
	for rows.Next() {
		var item GaspolAuditLogRow
		if err = rows.Scan(&item.ID, &item.Year, &item.ActorUserID, &item.ActorName, &item.ActorRole, &item.Action, &item.EntityType, &item.EntityID, &item.SchoolID, &item.SchoolName, &item.Payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s GaspolService) historyTimeline(ctx context.Context, row GaspolHistoryRow, tab string) ([]GaspolAuditLogRow, error) {
	entityType := "gaspol_assessments"
	if tab == GaspolHistorySubmissions {
		entityType = "gaspol_school_submissions"
	}
	baseQuery := `SELECT gal.id,gal.year,gal.actor_user_id,COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username,''),gal.actor_role,gal.action,gal.entity_type,gal.entity_id,gal.school_id,COALESCE(ds.nama_sekolah,''),gal.payload::text,gal.created_at FROM gaspol_audit_logs gal LEFT JOIN users u ON u.id=gal.actor_user_id LEFT JOIN data_sekolah ds ON ds.id=gal.school_id WHERE `
	var rows pgx.Rows
	var err error
	if entityType == "gaspol_assessments" {
		var stageAssessmentID int64
		linkErr := s.DB.Pool.QueryRow(ctx, `SELECT id FROM gaspol_stage_assessments WHERE legacy_assessment_id=$1`, row.ID).Scan(&stageAssessmentID)
		if linkErr == nil {
			rows, err = s.DB.Pool.Query(ctx, baseQuery+historyRecordTimelineScope(true)+` ORDER BY gal.created_at,gal.id`, row.Year, entityType, row.ID, stageAssessmentID)
		} else if errors.Is(linkErr, pgx.ErrNoRows) {
			rows, err = s.DB.Pool.Query(ctx, baseQuery+historyRecordTimelineScope(false)+` ORDER BY gal.created_at,gal.id`, row.Year, entityType, row.ID)
		} else {
			return nil, linkErr
		}
	} else {
		rows, err = s.DB.Pool.Query(ctx, baseQuery+historyRecordTimelineScope(false)+` ORDER BY gal.created_at,gal.id`, row.Year, entityType, row.ID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []GaspolAuditLogRow{}
	for rows.Next() {
		var x GaspolAuditLogRow
		if err := rows.Scan(&x.ID, &x.Year, &x.ActorUserID, &x.ActorName, &x.ActorRole, &x.Action, &x.EntityType, &x.EntityID, &x.SchoolID, &x.SchoolName, &x.Payload, &x.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}
