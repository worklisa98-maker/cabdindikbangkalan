package services

import (
	"context"
	"fmt"
	"math"
	"strings"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type SiswaOutcomeDashboardFilters struct {
	SchoolID *int
	Status   string
	Year     int
	Jenjang  string
}

type SiswaOutcomeDashboardKpis struct {
	Total            int      `json:"total"`
	CurrentYear      int      `json:"currentYear"`
	PrevYear         int      `json:"prevYear"`
	CurrentYearTotal int      `json:"currentYearTotal"`
	PrevYearTotal    int      `json:"prevYearTotal"`
	YoYPercent       *float64 `json:"yoyPercent"`
}

type SiswaOutcomeDashboardPoint struct {
	Year  int `json:"year"`
	Total int `json:"total"`
}

type SiswaOutcomeDashboardItem struct {
	Label        string  `json:"label"`
	Total        int     `json:"total"`
	SharePercent float64 `json:"sharePercent"`
}

type SiswaOutcomeDashboardPayload struct {
	Kpis           SiswaOutcomeDashboardKpis    `json:"kpis"`
	TrendByYear    []SiswaOutcomeDashboardPoint `json:"trendByYear"`
	BreakdownJalur []SiswaOutcomeDashboardItem  `json:"breakdownJalur"`
	TopKampus      []SiswaOutcomeDashboardItem  `json:"topKampus"`
	TopProdi       []SiswaOutcomeDashboardItem  `json:"topProdi"`
}

func emptyOutcomeDashboard() SiswaOutcomeDashboardPayload {
	return SiswaOutcomeDashboardPayload{
		Kpis: SiswaOutcomeDashboardKpis{
			Total:            0,
			CurrentYear:      0,
			PrevYear:         0,
			CurrentYearTotal: 0,
			PrevYearTotal:    0,
			YoYPercent:       nil,
		},
		TrendByYear:    []SiswaOutcomeDashboardPoint{},
		BreakdownJalur: []SiswaOutcomeDashboardItem{},
		TopKampus:      []SiswaOutcomeDashboardItem{},
		TopProdi:       []SiswaOutcomeDashboardItem{},
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func roundPercent(value float64) float64 {
	return math.Round(value*10) / 10
}

func (s SiswaService) OutcomeDashboard(
	ctx context.Context,
	user auth.User,
	filters SiswaOutcomeDashboardFilters,
) (SiswaOutcomeDashboardPayload, error) {
	payload := emptyOutcomeDashboard()

	statusFilter := strings.ToUpper(strings.TrimSpace(filters.Status))
	if statusFilter == "" {
		statusFilter = "ALL"
	}
	if statusFilter != "ALL" {
		normalized, err := normalizeFinalStatus(statusFilter)
		if err != nil {
			return payload, ErrInvalidOutcome
		}
		statusFilter = normalized
	}

	jenjangFilter := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
	if jenjangFilter == "ALL" {
		jenjangFilter = ""
	}

	allowed := []string{}
	if user.Role == "ADMIN" {
		allowed = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return payload, nil
		}
		if jenjangFilter != "" && !containsString(allowed, jenjangFilter) {
			return payload, nil
		}
	}

	ok, missing, err := tablesExist(ctx, s.DB, "student_outcomes", "data_sekolah")
	if err != nil {
		return payload, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "dashboard", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return payload, nil
	}
	if statusFilter == "BELUM" {
		return s.outcomeDashboardBelum(ctx, user, filters, jenjangFilter, allowed)
	}

	baseConditions := []string{"o.finalized_at IS NOT NULL"}
	baseArgs := make([]interface{}, 0)
	addClause := func(conditions *[]string, args *[]interface{}, clause string, value interface{}) {
		*conditions = append(*conditions, fmt.Sprintf(clause, len(*args)+1))
		*args = append(*args, value)
	}

	if statusFilter != "ALL" {
		addClause(&baseConditions, &baseArgs, "o.final_status = $%d", statusFilter)
	}
	if filters.SchoolID != nil {
		addClause(&baseConditions, &baseArgs, "o.school_id = $%d", *filters.SchoolID)
	}
	if jenjangFilter != "" {
		addClause(&baseConditions, &baseArgs, "UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", jenjangFilter)
	} else if user.Role == "ADMIN" {
		addClause(&baseConditions, &baseArgs, "UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", allowed)
	}

	targetYear := filters.Year
	if targetYear <= 0 {
		maxQuery := `
            SELECT COALESCE(MAX(o.tahun_lulus), 0)
            FROM student_outcomes o
            JOIN data_sekolah ds ON ds.id = o.school_id
        `
		where := ""
		if len(baseConditions) > 0 {
			where = " WHERE " + strings.Join(baseConditions, " AND ")
		}
		if err := s.DB.Pool.QueryRow(ctx, maxQuery+where, baseArgs...).Scan(&targetYear); err != nil {
			return payload, err
		}
	}
	if targetYear <= 0 {
		return payload, nil
	}

	prevYear := targetYear - 1
	countForYear := func(year int) (int, error) {
		if year <= 0 {
			return 0, nil
		}
		conds := append([]string{}, baseConditions...)
		args := append([]interface{}{}, baseArgs...)
		addClause(&conds, &args, "o.tahun_lulus = $%d", year)
		query := `
            SELECT COUNT(*)
            FROM student_outcomes o
            JOIN data_sekolah ds ON ds.id = o.school_id
        `
		where := ""
		if len(conds) > 0 {
			where = " WHERE " + strings.Join(conds, " AND ")
		}
		var total int
		if err := s.DB.Pool.QueryRow(ctx, query+where, args...).Scan(&total); err != nil {
			return 0, err
		}
		return total, nil
	}

	currentTotal, err := countForYear(targetYear)
	if err != nil {
		return payload, err
	}
	prevTotal, err := countForYear(prevYear)
	if err != nil {
		return payload, err
	}

	var yoy *float64
	if prevTotal > 0 {
		value := (float64(currentTotal-prevTotal) / float64(prevTotal)) * 100
		value = roundPercent(value)
		yoy = &value
	}

	payload.Kpis = SiswaOutcomeDashboardKpis{
		Total:            currentTotal,
		CurrentYear:      targetYear,
		PrevYear:         prevYear,
		CurrentYearTotal: currentTotal,
		PrevYearTotal:    prevTotal,
		YoYPercent:       yoy,
	}

	trendConditions := append([]string{}, baseConditions...)
	trendArgs := append([]interface{}{}, baseArgs...)
	addClause(&trendConditions, &trendArgs, "o.tahun_lulus <= $%d", targetYear)
	trendQuery := `
        SELECT o.tahun_lulus,
               COUNT(*) AS total
        FROM student_outcomes o
        JOIN data_sekolah ds ON ds.id = o.school_id
    `
	trendWhere := " WHERE " + strings.Join(trendConditions, " AND ")
	trendQuery += trendWhere + " GROUP BY o.tahun_lulus ORDER BY o.tahun_lulus DESC LIMIT 5"

	trendRows, err := s.DB.Pool.Query(ctx, trendQuery, trendArgs...)
	if err != nil {
		return payload, err
	}
	defer trendRows.Close()

	trend := make([]SiswaOutcomeDashboardPoint, 0)
	for trendRows.Next() {
		var point SiswaOutcomeDashboardPoint
		if scanErr := trendRows.Scan(&point.Year, &point.Total); scanErr != nil {
			return payload, scanErr
		}
		trend = append(trend, point)
	}
	for i, j := 0, len(trend)-1; i < j; i, j = i+1, j-1 {
		trend[i], trend[j] = trend[j], trend[i]
	}
	payload.TrendByYear = trend

	includePT := statusFilter == "ALL" || statusFilter == "PT"
	if !includePT {
		return payload, nil
	}

	ok, missing, err = tablesExist(ctx, s.DB, "outcome_pt_details")
	if err != nil {
		return payload, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "dashboard", "event=missing_pt_details tables=%s", strings.Join(missing, ","))
		return payload, nil
	}

	ptConditions := append([]string{}, baseConditions...)
	ptArgs := append([]interface{}{}, baseArgs...)
	if statusFilter == "ALL" {
		addClause(&ptConditions, &ptArgs, "o.final_status = $%d", "PT")
	}
	addClause(&ptConditions, &ptArgs, "o.tahun_lulus = $%d", targetYear)
	ptWhere := " WHERE " + strings.Join(ptConditions, " AND ")

	var ptTotal int
	totalQuery := `
        SELECT COUNT(*)
        FROM student_outcomes o
        JOIN data_sekolah ds ON ds.id = o.school_id
    ` + ptWhere
	if err := s.DB.Pool.QueryRow(ctx, totalQuery, ptArgs...).Scan(&ptTotal); err != nil {
		return payload, err
	}
	if ptTotal == 0 {
		return payload, nil
	}

	jalurCase := `
        CASE
          WHEN LOWER(COALESCE(pt.jalur::text, '')) LIKE '%snbp%' THEN 'SNBP'
          WHEN LOWER(COALESCE(pt.jalur::text, '')) LIKE '%snbt%' THEN 'SNBT'
          WHEN LOWER(COALESCE(pt.jalur::text, '')) LIKE '%mandiri%' OR LOWER(COALESCE(pt.jalur::text, '')) LIKE '%snbm%' THEN 'SNBM (Mandiri)'
          WHEN LOWER(COALESCE(pt.jalur::text, '')) LIKE '%kedinas%' THEN 'Kedinasan'
          ELSE 'Lainnya'
        END
    `

	breakdownQuery := `
        SELECT ` + jalurCase + ` AS label,
               COUNT(*) AS total
        FROM student_outcomes o
        JOIN data_sekolah ds ON ds.id = o.school_id
        LEFT JOIN outcome_pt_details pt
          ON pt.school_id = o.school_id
         AND pt.tahun_lulus = o.tahun_lulus
         AND pt.nisn = o.nisn
    ` + ptWhere + `
        GROUP BY label
        ORDER BY total DESC, label ASC
    `

	breakdownRows, err := s.DB.Pool.Query(ctx, breakdownQuery, ptArgs...)
	if err != nil {
		return payload, err
	}
	defer breakdownRows.Close()

	breakdown := make([]SiswaOutcomeDashboardItem, 0)
	for breakdownRows.Next() {
		var item SiswaOutcomeDashboardItem
		if scanErr := breakdownRows.Scan(&item.Label, &item.Total); scanErr != nil {
			return payload, scanErr
		}
		item.SharePercent = roundPercent((float64(item.Total) / float64(ptTotal)) * 100)
		breakdown = append(breakdown, item)
	}
	payload.BreakdownJalur = breakdown

	topQuery := `
        SELECT COALESCE(NULLIF(TRIM(pt.%s), ''), '') AS label,
               COUNT(*) AS total
        FROM student_outcomes o
        JOIN data_sekolah ds ON ds.id = o.school_id
        JOIN outcome_pt_details pt
          ON pt.school_id = o.school_id
         AND pt.tahun_lulus = o.tahun_lulus
         AND pt.nisn = o.nisn
    ` + ptWhere + `
        AND COALESCE(NULLIF(TRIM(pt.%s), ''), '') <> ''
        GROUP BY label
        ORDER BY total DESC, label ASC
        LIMIT 10
    `

	fetchTop := func(column string) ([]SiswaOutcomeDashboardItem, error) {
		query := fmt.Sprintf(topQuery, column, column)
		rows, err := s.DB.Pool.Query(ctx, query, ptArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := make([]SiswaOutcomeDashboardItem, 0)
		for rows.Next() {
			var item SiswaOutcomeDashboardItem
			if scanErr := rows.Scan(&item.Label, &item.Total); scanErr != nil {
				return nil, scanErr
			}
			item.SharePercent = roundPercent((float64(item.Total) / float64(ptTotal)) * 100)
			items = append(items, item)
		}
		return items, nil
	}

	if items, err := fetchTop("kampus"); err != nil {
		return payload, err
	} else {
		payload.TopKampus = items
	}
	if items, err := fetchTop("prodi"); err != nil {
		return payload, err
	} else {
		payload.TopProdi = items
	}

	return payload, nil
}

func (s SiswaService) outcomeDashboardBelum(
	ctx context.Context,
	user auth.User,
	filters SiswaOutcomeDashboardFilters,
	jenjangFilter string,
	allowed []string,
) (SiswaOutcomeDashboardPayload, error) {
	payload := emptyOutcomeDashboard()

	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_outcomes", "data_sekolah")
	if err != nil {
		return payload, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "dashboard_belum", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return payload, nil
	}

	lockOK, _, lockErr := tablesExist(ctx, s.DB, "graduation_year_locks")
	if lockErr != nil {
		return payload, lockErr
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

	baseConditions := []string{"c.tahun_lulus > 0"}
	baseArgs := make([]interface{}, 0)
	addClause := func(conditions *[]string, args *[]interface{}, clause string, value interface{}) {
		*conditions = append(*conditions, fmt.Sprintf(clause, len(*args)+1))
		*args = append(*args, value)
	}

	if filters.SchoolID != nil {
		addClause(&baseConditions, &baseArgs, "c.school_id = $%d", *filters.SchoolID)
	}
	if jenjangFilter != "" {
		addClause(&baseConditions, &baseArgs, "UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", jenjangFilter)
	} else if user.Role == "ADMIN" {
		addClause(&baseConditions, &baseArgs, "UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", allowed)
	}

	targetYear := filters.Year
	if targetYear <= 0 {
		maxQuery := `
            SELECT COALESCE(MAX(c.tahun_lulus), 0)
            FROM class12_students c
            JOIN data_sekolah ds ON ds.id = c.school_id
        `
		where := ""
		if len(baseConditions) > 0 {
			where = " WHERE " + strings.Join(baseConditions, " AND ")
		}
		if err := s.DB.Pool.QueryRow(ctx, maxQuery+where, baseArgs...).Scan(&targetYear); err != nil {
			return payload, err
		}
	}
	if targetYear <= 0 {
		return payload, nil
	}

	prevYear := targetYear - 1
	countForYear := func(year int) (int, error) {
		if year <= 0 {
			return 0, nil
		}
		conds := append([]string{}, baseConditions...)
		args := append([]interface{}{}, baseArgs...)
		addClause(&conds, &args, "c.tahun_lulus = $%d", year)
		query := `
            SELECT COUNT(*)
            FROM class12_students c
            JOIN data_sekolah ds ON ds.id = c.school_id
            LEFT JOIN student_outcomes o
              ON o.school_id = c.school_id
             AND o.tahun_lulus = c.tahun_lulus
             AND o.nisn = c.nisn
        `
		where := ""
		if len(conds) > 0 {
			where = " WHERE " + strings.Join(conds, " AND ")
		}
		where += " AND (" + belumCondition + ")"
		var total int
		if err := s.DB.Pool.QueryRow(ctx, query+where, args...).Scan(&total); err != nil {
			return 0, err
		}
		return total, nil
	}

	currentTotal, err := countForYear(targetYear)
	if err != nil {
		return payload, err
	}
	prevTotal, err := countForYear(prevYear)
	if err != nil {
		return payload, err
	}

	var yoy *float64
	if prevTotal > 0 {
		value := (float64(currentTotal-prevTotal) / float64(prevTotal)) * 100
		value = roundPercent(value)
		yoy = &value
	}

	payload.Kpis = SiswaOutcomeDashboardKpis{
		Total:            currentTotal,
		CurrentYear:      targetYear,
		PrevYear:         prevYear,
		CurrentYearTotal: currentTotal,
		PrevYearTotal:    prevTotal,
		YoYPercent:       yoy,
	}

	trendConditions := append([]string{}, baseConditions...)
	trendArgs := append([]interface{}{}, baseArgs...)
	addClause(&trendConditions, &trendArgs, "c.tahun_lulus <= $%d", targetYear)
	trendQuery := `
        SELECT c.tahun_lulus,
               COUNT(*) AS total
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
    `
	trendWhere := ""
	if len(trendConditions) > 0 {
		trendWhere = " WHERE " + strings.Join(trendConditions, " AND ")
	}
	trendWhere += " AND (" + belumCondition + ")"
	trendQuery += trendWhere + " GROUP BY c.tahun_lulus ORDER BY c.tahun_lulus DESC LIMIT 5"

	trendRows, err := s.DB.Pool.Query(ctx, trendQuery, trendArgs...)
	if err != nil {
		return payload, err
	}
	defer trendRows.Close()

	trend := make([]SiswaOutcomeDashboardPoint, 0)
	for trendRows.Next() {
		var point SiswaOutcomeDashboardPoint
		if scanErr := trendRows.Scan(&point.Year, &point.Total); scanErr != nil {
			return payload, scanErr
		}
		trend = append(trend, point)
	}
	for i, j := 0, len(trend)-1; i < j; i, j = i+1, j-1 {
		trend[i], trend[j] = trend[j], trend[i]
	}
	payload.TrendByYear = trend

	return payload, nil
}
