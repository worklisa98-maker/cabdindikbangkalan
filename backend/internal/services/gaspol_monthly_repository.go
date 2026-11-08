package services

import (
	"context"
	"time"
)

// Stage workflow overrides legacy assessments, including returned/in-progress rows.
// Synthetic zeroes are computed only in memory using the effective application clock.
// Monthly resubmission updates rows in place; no current/version flag exists.
// Latest workflow activity wins per school/year/month, with creation/id tie breakers.
func (r GaspolRepository) ListMonthlyCompetitionComponents(ctx context.Context, year, schoolID int, now time.Time) (map[int][]GaspolMonthlyComponent, error) {
	rows, err := r.DB.Pool.Query(ctx, `
 WITH schools AS (
  SELECT school_id FROM gaspol_school_participants
  WHERE year=$1 AND is_active AND NOT is_year_configuration AND $2::integer=0
  UNION SELECT $2::integer WHERE $2::integer<>0
 )
 SELECT p.school_id,m.month,CASE WHEN a.present THEN COALESCE(a.score,0) ELSE COALESCE(legacy.score_monthly,0) END,
        CASE WHEN a.present THEN a.final ELSE COALESCE(legacy.is_validated,FALSE) END,
        (a.present IS TRUE OR legacy.id IS NOT NULL),
        COALESCE(s.status='SUBMITTED' OR s.submitted_at IS NOT NULL,FALSE)
 FROM schools p CROSS JOIN generate_series(4,9) m(month)
 LEFT JOIN LATERAL (
  SELECT TRUE present,(workflow_status='FINAL' AND final_score IS NOT NULL) final,final_score score
  FROM gaspol_stage_assessments WHERE year=$1 AND school_id=p.school_id AND stage='MONTHLY' AND month=m.month
  ORDER BY updated_at DESC NULLS LAST,created_at DESC NULLS LAST,id DESC LIMIT 1
 ) a ON TRUE
 LEFT JOIN LATERAL (
  SELECT id,score_monthly,is_validated FROM gaspol_assessments
  WHERE year=$1 AND school_id=p.school_id AND stage='MONTHLY' AND month=m.month
  ORDER BY updated_at DESC NULLS LAST,created_at DESC NULLS LAST,id DESC LIMIT 1
 ) legacy ON TRUE
 LEFT JOIN gaspol_school_submissions s ON s.year=$1 AND s.school_id=p.school_id AND s.month=m.month
 ORDER BY p.school_id,m.month`, year, schoolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	submissions := map[int]map[int]GaspolSchoolSubmission{}
	assessments := map[int]map[int]GaspolAssessment{}
	for rows.Next() {
		var id, month int
		var score float64
		var final, hasAssessment, submitted bool
		if err := rows.Scan(&id, &month, &score, &final, &hasAssessment, &submitted); err != nil {
			return nil, err
		}
		if submissions[id] == nil {
			submissions[id] = map[int]GaspolSchoolSubmission{}
			assessments[id] = map[int]GaspolAssessment{}
		}
		if submitted {
			submissions[id][month] = GaspolSchoolSubmission{Month: month, Status: GaspolSubmissionSubmitted}
		}
		if hasAssessment {
			assessments[id][month] = GaspolAssessment{Month: month, Stage: GaspolStageMonthly, ScoreMonthly: score, IsValidated: final}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := map[int][]GaspolMonthlyComponent{}
	for id, reports := range submissions {
		result[id] = buildGaspolMonthlyComponents(year, reports, assessments[id], now)
	}
	return result, nil
}
