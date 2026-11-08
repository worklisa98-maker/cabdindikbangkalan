package services

import (
	"context"
	"fmt"
	"strings"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type SiswaFinalStatusAnalytics struct {
	TahunLulus int `json:"tahun_lulus"`
	FinalCount int `json:"final_count"`
}

func (s SiswaService) AnalyticsFinalByType(
	ctx context.Context,
	user auth.User,
	scopeSchoolID *int,
	finalType string,
	year int,
) ([]SiswaFinalStatusAnalytics, error) {
	normalized := strings.ToUpper(strings.TrimSpace(finalType))
	if normalized == "" {
		return nil, fmt.Errorf("invalid final type")
	}
	if normalized == "BELUM" {
		return s.analyticsFinalBelum(ctx, user, scopeSchoolID, year)
	}
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_outcomes", "data_sekolah")
	if err != nil {
		return nil, err
	}
	if !ok {
		logging.Warnf("siswa_final_analytics", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaFinalStatusAnalytics{}, nil
	}

	query := `
		SELECT c.tahun_lulus,
			   COUNT(*) AS final_count
		FROM student_outcomes f
		JOIN class12_students c
		  ON c.school_id = f.school_id
		 AND c.tahun_lulus = f.tahun_lulus
		 AND c.nisn = f.nisn
		JOIN data_sekolah ds ON ds.id = c.school_id
		WHERE f.final_status = $1
		  AND f.finalized_at IS NOT NULL
	`
	args := []interface{}{normalized}
	idx := 2
	if scopeSchoolID != nil {
		query += " AND c.school_id = $" + fmt.Sprint(idx)
		args = append(args, *scopeSchoolID)
		idx++
	}
	if year > 0 {
		query += " AND c.tahun_lulus = $" + fmt.Sprint(idx)
		args = append(args, year)
		idx++
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			query += " AND 1=0"
		} else {
			query += " AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($" + fmt.Sprint(idx) + ")"
			args = append(args, allowed)
			idx++
		}
	}
	query += " GROUP BY c.tahun_lulus ORDER BY c.tahun_lulus ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SiswaFinalStatusAnalytics, 0)
	for rows.Next() {
		var point SiswaFinalStatusAnalytics
		if scanErr := rows.Scan(&point.TahunLulus, &point.FinalCount); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, point)
	}
	return result, nil
}

func (s SiswaService) analyticsFinalBelum(
	ctx context.Context,
	user auth.User,
	scopeSchoolID *int,
	year int,
) ([]SiswaFinalStatusAnalytics, error) {
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_outcomes", "data_sekolah")
	if err != nil {
		return nil, err
	}
	if !ok {
		logging.Warnf("siswa_final_analytics", "belum", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaFinalStatusAnalytics{}, nil
	}

	lockOK, _, lockErr := tablesExist(ctx, s.DB, "graduation_year_locks")
	if lockErr != nil {
		return nil, lockErr
	}
	yearClosedExpr := "FALSE"
	if lockOK {
		yearClosedExpr = `
			EXISTS (
				SELECT 1
				FROM graduation_year_locks gyl
				WHERE gyl.tahun_lulus = c.tahun_lulus
				  AND gyl.is_closed = TRUE
				  AND (gyl.school_id = c.school_id OR gyl.school_id IS NULL)
			)
		`
	}
	belumCondition := fmt.Sprintf(
		"((%s) AND o.finalized_at IS NULL) OR (NOT (%s) AND o.final_status = 'BELUM' AND o.finalized_at IS NOT NULL)",
		yearClosedExpr,
		yearClosedExpr,
	)

	query := `
		SELECT c.tahun_lulus,
			   COUNT(*) AS final_count
		FROM class12_students c
		JOIN data_sekolah ds ON ds.id = c.school_id
		LEFT JOIN student_outcomes o
		  ON o.school_id = c.school_id
		 AND o.tahun_lulus = c.tahun_lulus
		 AND o.nisn = c.nisn
		WHERE c.tahun_lulus > 0
		  AND (` + belumCondition + `)
	`
	args := []interface{}{}
	idx := 1
	if scopeSchoolID != nil {
		query += " AND c.school_id = $" + fmt.Sprint(idx)
		args = append(args, *scopeSchoolID)
		idx++
	}
	if year > 0 {
		query += " AND c.tahun_lulus = $" + fmt.Sprint(idx)
		args = append(args, year)
		idx++
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			query += " AND 1=0"
		} else {
			query += " AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($" + fmt.Sprint(idx) + ")"
			args = append(args, allowed)
			idx++
		}
	}
	query += " GROUP BY c.tahun_lulus ORDER BY c.tahun_lulus ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SiswaFinalStatusAnalytics, 0)
	for rows.Next() {
		var point SiswaFinalStatusAnalytics
		if scanErr := rows.Scan(&point.TahunLulus, &point.FinalCount); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, point)
	}
	return result, nil
}

func (s SiswaService) LegacyAnalyticsHigherEd(
	ctx context.Context,
	scopeSchoolID *int,
) ([]SiswaFinalStatusAnalytics, error) {
	query := `
		SELECT COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0)) AS tahun_lulus,
			   COUNT(*) AS final_count
		FROM siswa_perguruan_tinggi spt
		WHERE COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0)) IS NOT NULL
	`
	args := make([]interface{}, 0)
	if scopeSchoolID != nil {
		query += " AND spt.school_id = $1"
		args = append(args, *scopeSchoolID)
	}
	query += " GROUP BY tahun_lulus ORDER BY tahun_lulus ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SiswaFinalStatusAnalytics, 0)
	for rows.Next() {
		var point SiswaFinalStatusAnalytics
		if scanErr := rows.Scan(&point.TahunLulus, &point.FinalCount); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, point)
	}
	return result, nil
}
