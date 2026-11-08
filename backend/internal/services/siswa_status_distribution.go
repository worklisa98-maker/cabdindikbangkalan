package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
)

type StatusDistributionFilters struct {
	Year          int
	Jenjang       string
	StatusSekolah string
	SchoolID      *int
}

type StatusDistributionYear struct {
	Year                int     `json:"year"`
	TotalGraduates      int     `json:"totalGraduates"`
	HigherEducation     int     `json:"higherEducation"`
	Working             int     `json:"working"`
	Entrepreneurship    int     `json:"entrepreneurship"`
	NotContinuing       int     `json:"notContinuing"`
	Unrecorded          int     `json:"unrecorded"`
	PendingValidation   int     `json:"pendingValidation"`
	FinalCount          int     `json:"finalCount"`
	DataCoveragePercent float64 `json:"dataCoveragePercent"`
}

type StatusDistributionPayload struct {
	Selected StatusDistributionYear   `json:"selected"`
	Trend    []StatusDistributionYear `json:"trend"`
}

func applyStatusDistributionGap(item *StatusDistributionYear, explicitNotContinuing, currentYear int) {
	gap := item.TotalGraduates - item.FinalCount - item.PendingValidation
	if gap < 0 {
		gap = 0
	}
	item.NotContinuing = explicitNotContinuing
	item.Unrecorded = 0
	if item.Year < currentYear {
		item.NotContinuing += gap
	} else {
		item.Unrecorded = gap
	}
	if item.TotalGraduates > 0 {
		item.DataCoveragePercent = roundPercent(float64(item.FinalCount) * 100 / float64(item.TotalGraduates))
	}
}

func (s SiswaService) StatusDistribution(ctx context.Context, user auth.User, filters StatusDistributionFilters) (StatusDistributionPayload, error) {
	payload := StatusDistributionPayload{Trend: []StatusDistributionYear{}}
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return payload, err
	}
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return payload, err
		}
	}
	conditions := []string{"summary.tahun_lulus>0"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(clause, len(args)))
	}
	if filters.Year > 0 {
		add("summary.tahun_lulus=$%d", filters.Year)
	}
	if filters.SchoolID != nil {
		add("summary.school_id=$%d", *filters.SchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return payload, nil
		}
		add("summary.school_id=ANY($%d)", scope.SchoolIDs)
	}
	if value := strings.ToUpper(strings.TrimSpace(filters.Jenjang)); value != "" && value != "ALL" {
		add("UPPER(TRIM(school.bentuk_pendidikan))=$%d", value)
	}
	if value := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah)); value != "" && value != "ALL" {
		add("UPPER(TRIM(school.status_sekolah))=$%d", value)
	}
	query := buildFinalPTReconciledCTE() + `,
		scoped AS (
		  SELECT summary.school_id,summary.tahun_lulus,summary.total_siswa_akhir
		  FROM student_year_summaries summary JOIN data_sekolah school ON school.id=summary.school_id
		  WHERE ` + strings.Join(conditions, " AND ") + `
		), non_pt_counts AS (
		  SELECT outcome.school_id,outcome.tahun_lulus,
		    COUNT(*) FILTER(WHERE outcome.finalized_at IS NOT NULL AND outcome.final_status='KERJA')::int kerja,
		    COUNT(*) FILTER(WHERE outcome.finalized_at IS NOT NULL AND outcome.final_status='WIRAUSAHA')::int wirausaha,
		    COUNT(*) FILTER(WHERE outcome.finalized_at IS NOT NULL AND outcome.final_status='BELUM')::int belum
		  FROM student_outcomes outcome JOIN scoped ON scoped.school_id=outcome.school_id AND scoped.tahun_lulus=outcome.tahun_lulus
		  GROUP BY outcome.school_id,outcome.tahun_lulus
		), pt_counts AS (
		  SELECT final.school_id,final.tahun_lulus,COUNT(*)::int pt
		  FROM final_pt final JOIN scoped ON scoped.school_id=final.school_id AND scoped.tahun_lulus=final.tahun_lulus
		  GROUP BY final.school_id,final.tahun_lulus
		), pending AS (
		  SELECT offer.school_id,offer.tahun_lulus,
		    COUNT(DISTINCT COALESCE(NULLIF('NISN:'||TRIM(COALESCE(offer.nisn,'')),'NISN:'),'NAME:'||offer.normalized_student_name))::int pending_count
		  FROM outcome_pt_offers offer
		  JOIN pt_offer_reconciliations reconciliation ON reconciliation.offer_id=offer.id AND reconciliation.reconciliation_status='PENDING'
		  JOIN scoped ON scoped.school_id=offer.school_id AND scoped.tahun_lulus=offer.tahun_lulus
		  WHERE offer.deleted_at IS NULL
		    AND NOT EXISTS(SELECT 1 FROM student_outcomes outcome WHERE outcome.school_id=offer.school_id AND outcome.tahun_lulus=offer.tahun_lulus AND outcome.nisn=offer.nisn AND outcome.finalized_at IS NOT NULL)
		  GROUP BY offer.school_id,offer.tahun_lulus
		)
		SELECT scoped.tahun_lulus,SUM(scoped.total_siswa_akhir)::int,
		  COALESCE(SUM(pt_counts.pt),0)::int,COALESCE(SUM(non_pt_counts.kerja),0)::int,
		  COALESCE(SUM(non_pt_counts.wirausaha),0)::int,COALESCE(SUM(non_pt_counts.belum),0)::int,
		  (COALESCE(SUM(pt_counts.pt),0)+COALESCE(SUM(non_pt_counts.kerja),0)+COALESCE(SUM(non_pt_counts.wirausaha),0)+COALESCE(SUM(non_pt_counts.belum),0))::int,
		  COALESCE(SUM(pending.pending_count),0)::int
		FROM scoped LEFT JOIN non_pt_counts USING(school_id,tahun_lulus) LEFT JOIN pt_counts USING(school_id,tahun_lulus) LEFT JOIN pending USING(school_id,tahun_lulus)
		GROUP BY scoped.tahun_lulus ORDER BY scoped.tahun_lulus`
	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return payload, err
	}
	defer rows.Close()
	currentYear := time.Now().In(JakartaLocation()).Year()
	for rows.Next() {
		var item StatusDistributionYear
		var explicitBelum int
		if err := rows.Scan(&item.Year, &item.TotalGraduates, &item.HigherEducation, &item.Working, &item.Entrepreneurship, &explicitBelum, &item.FinalCount, &item.PendingValidation); err != nil {
			return payload, err
		}
		applyStatusDistributionGap(&item, explicitBelum, currentYear)
		payload.Trend = append(payload.Trend, item)
		if filters.Year > 0 && item.Year == filters.Year {
			payload.Selected = item
		}
	}
	if err := rows.Err(); err != nil {
		return payload, err
	}
	if filters.Year <= 0 && len(payload.Trend) > 0 {
		payload.Selected = payload.Trend[len(payload.Trend)-1]
	}
	return payload, nil
}
