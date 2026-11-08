package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
)

type FinalOutcomeAnalyticsFilters struct {
	Year    int
	Jenjang string
	Status  string
}

type FinalOutcomeAnalyticsKpis struct {
	Total            int      `json:"total"`
	YoYPercent       *float64 `json:"yoyPercent"`
	CurrentYearTotal int      `json:"currentYearTotal"`
	PrevYearTotal    int      `json:"prevYearTotal"`
}

type FinalOutcomeTopItem struct {
	Name         string  `json:"name"`
	Total        int     `json:"total"`
	SharePercent float64 `json:"sharePercent"`
}

type FinalOutcomeAnalyticsSummary struct {
	FiltersEcho      AnalyticsFiltersEcho      `json:"filtersEcho"`
	AvailableYears   []int                     `json:"availableYears"`
	LatestYear       *int                      `json:"latestYear"`
	Kpis             FinalOutcomeAnalyticsKpis `json:"kpis"`
	TrendByYear      []AnalyticsYearPoint      `json:"trendByYear"`
	BreakdownByJalur []AnalyticsKeyTotal       `json:"breakdownByJalur,omitempty"`
	TopKampus        []FinalOutcomeTopItem     `json:"topKampus,omitempty"`
	TopProdi         []FinalOutcomeTopItem     `json:"topProdi,omitempty"`
}

type FinalOutcomeAnalyticsService struct {
	DB *db.DB
}

func normalizeOutcomeJenjang(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	switch value {
	case "SMA", "SMK", "SLB":
		return value
	default:
		return ""
	}
}

func normalizeOutcomeStatus(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	switch value {
	case "PT", "KERJA", "WIRAUSAHA", "BELUM":
		return value
	default:
		return ""
	}
}

func buildOutcomeWhere(normalizedJenjang string, normalizedStatus string, includeYear bool, year int) (string, []interface{}) {
	clauses := []string{"o.tahun_lulus > 0"}
	args := make([]interface{}, 0)
	idx := 1
	addClause := func(condition string, value interface{}) {
		clauses = append(clauses, condition)
		args = append(args, value)
		idx++
	}

	if normalizedStatus != "" {
		addClause(fmt.Sprintf("o.final_status = $%d", idx), normalizedStatus)
	}
	if normalizedJenjang != "" {
		addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx), normalizedJenjang)
	}
	if includeYear && year > 0 {
		addClause(fmt.Sprintf("o.tahun_lulus = $%d", idx), year)
	}

	return " WHERE " + strings.Join(clauses, " AND "), args
}

func buildEmptyFinalOutcomeAnalytics(filters FinalOutcomeAnalyticsFilters, normalizedJenjang string, normalizedStatus string) FinalOutcomeAnalyticsSummary {
	yearLabel := ""
	if filters.Year > 0 {
		yearLabel = strconv.Itoa(filters.Year)
	}
	jenjangLabel := normalizedJenjang
	if jenjangLabel == "" {
		jenjangLabel = "ALL"
	}
	statusLabel := normalizedStatus
	if statusLabel == "" {
		statusLabel = "ALL"
	}
	return FinalOutcomeAnalyticsSummary{
		FiltersEcho: AnalyticsFiltersEcho{
			Year:    yearLabel,
			Jenjang: jenjangLabel,
			Status:  statusLabel,
		},
		AvailableYears:   []int{},
		LatestYear:       nil,
		Kpis:             FinalOutcomeAnalyticsKpis{Total: 0, YoYPercent: nil, CurrentYearTotal: 0, PrevYearTotal: 0},
		TrendByYear:      []AnalyticsYearPoint{},
		BreakdownByJalur: []AnalyticsKeyTotal{},
		TopKampus:        []FinalOutcomeTopItem{},
		TopProdi:         []FinalOutcomeTopItem{},
	}
}

func (s FinalOutcomeAnalyticsService) Get(ctx context.Context, filters FinalOutcomeAnalyticsFilters) (FinalOutcomeAnalyticsSummary, error) {
	normalizedJenjang := normalizeOutcomeJenjang(filters.Jenjang)
	normalizedStatus := normalizeOutcomeStatus(filters.Status)

	requiredTables := []string{"student_outcomes", "data_sekolah"}
	if normalizedStatus == "BELUM" {
		requiredTables = []string{"class12_students", "student_outcomes", "data_sekolah"}
	}
	ok, missing, err := tablesExist(ctx, s.DB, requiredTables...)
	if err != nil {
		return FinalOutcomeAnalyticsSummary{}, err
	}
	if !ok {
		logging.Warnf("final_outcome_analytics", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return buildEmptyFinalOutcomeAnalytics(filters, normalizedJenjang, normalizedStatus), nil
	}

	if normalizedStatus == "BELUM" {
		return s.getBelumSummary(ctx, filters, normalizedJenjang)
	}

	availableYears, err := s.fetchAvailableYears(ctx, normalizedJenjang, normalizedStatus)
	if err != nil {
		return FinalOutcomeAnalyticsSummary{}, err
	}
	var latestYear *int
	if len(availableYears) > 0 {
		value := availableYears[len(availableYears)-1]
		latestYear = &value
	}

	selectedYear := filters.Year
	if selectedYear <= 0 && latestYear != nil {
		selectedYear = *latestYear
	}

	kpis, err := s.fetchKpis(ctx, selectedYear, normalizedJenjang, normalizedStatus)
	if err != nil {
		return FinalOutcomeAnalyticsSummary{}, err
	}

	trendByYear, err := s.fetchTrendByYear(ctx, selectedYear, normalizedJenjang, normalizedStatus)
	if err != nil {
		return FinalOutcomeAnalyticsSummary{}, err
	}

	breakdown := []AnalyticsKeyTotal{}
	topKampus := []FinalOutcomeTopItem{}
	topProdi := []FinalOutcomeTopItem{}

	if normalizedStatus == "PT" {
		hasPT, missingPT, ptErr := tablesExist(ctx, s.DB, "outcome_pt_details")
		if ptErr != nil {
			return FinalOutcomeAnalyticsSummary{}, ptErr
		}
		if !hasPT {
			logging.Warnf("final_outcome_analytics", "pt", "event=missing_tables tables=%s", strings.Join(missingPT, ","))
		} else {
			breakdown, err = s.fetchBreakdownByJalur(ctx, selectedYear, normalizedJenjang)
			if err != nil {
				return FinalOutcomeAnalyticsSummary{}, err
			}

			topKampus, err = s.fetchTopItems(ctx, selectedYear, normalizedJenjang, "kampus", kpis.CurrentYearTotal)
			if err != nil {
				return FinalOutcomeAnalyticsSummary{}, err
			}
			topProdi, err = s.fetchTopItems(ctx, selectedYear, normalizedJenjang, "prodi", kpis.CurrentYearTotal)
			if err != nil {
				return FinalOutcomeAnalyticsSummary{}, err
			}
		}
	}

	yearLabel := ""
	if selectedYear > 0 {
		yearLabel = strconv.Itoa(selectedYear)
	}
	jenjangLabel := normalizedJenjang
	if jenjangLabel == "" {
		jenjangLabel = "ALL"
	}
	statusLabel := normalizedStatus
	if statusLabel == "" {
		statusLabel = "ALL"
	}

	return FinalOutcomeAnalyticsSummary{
		FiltersEcho: AnalyticsFiltersEcho{
			Year:    yearLabel,
			Jenjang: jenjangLabel,
			Status:  statusLabel,
		},
		AvailableYears:   availableYears,
		LatestYear:       latestYear,
		Kpis:             kpis,
		TrendByYear:      trendByYear,
		BreakdownByJalur: breakdown,
		TopKampus:        topKampus,
		TopProdi:         topProdi,
	}, nil
}

func (s FinalOutcomeAnalyticsService) getBelumSummary(
	ctx context.Context,
	filters FinalOutcomeAnalyticsFilters,
	normalizedJenjang string,
) (FinalOutcomeAnalyticsSummary, error) {
	availableYears, err := s.fetchAvailableYearsBelum(ctx, normalizedJenjang)
	if err != nil {
		return FinalOutcomeAnalyticsSummary{}, err
	}
	var latestYear *int
	if len(availableYears) > 0 {
		value := availableYears[len(availableYears)-1]
		latestYear = &value
	}

	selectedYear := filters.Year
	if selectedYear <= 0 && latestYear != nil {
		selectedYear = *latestYear
	}

	kpis, err := s.fetchKpisBelum(ctx, selectedYear, normalizedJenjang)
	if err != nil {
		return FinalOutcomeAnalyticsSummary{}, err
	}
	trendByYear, err := s.fetchTrendByYearBelum(ctx, selectedYear, normalizedJenjang)
	if err != nil {
		return FinalOutcomeAnalyticsSummary{}, err
	}

	yearLabel := ""
	if selectedYear > 0 {
		yearLabel = strconv.Itoa(selectedYear)
	}
	jenjangLabel := normalizedJenjang
	if jenjangLabel == "" {
		jenjangLabel = "ALL"
	}

	return FinalOutcomeAnalyticsSummary{
		FiltersEcho: AnalyticsFiltersEcho{
			Year:    yearLabel,
			Jenjang: jenjangLabel,
			Status:  "BELUM",
		},
		AvailableYears:   availableYears,
		LatestYear:       latestYear,
		Kpis:             kpis,
		TrendByYear:      trendByYear,
		BreakdownByJalur: []AnalyticsKeyTotal{},
		TopKampus:        []FinalOutcomeTopItem{},
		TopProdi:         []FinalOutcomeTopItem{},
	}, nil
}

func (s FinalOutcomeAnalyticsService) fetchAvailableYears(ctx context.Context, normalizedJenjang string, normalizedStatus string) ([]int, error) {
	where, args := buildOutcomeWhere(normalizedJenjang, normalizedStatus, false, 0)
	query := `
		SELECT DISTINCT o.tahun_lulus
		FROM student_outcomes o
		JOIN data_sekolah ds ON ds.id = o.school_id
	` + where + `
		ORDER BY o.tahun_lulus ASC
	`

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	years := make([]int, 0)
	for rows.Next() {
		var year int
		if scanErr := rows.Scan(&year); scanErr != nil {
			return nil, scanErr
		}
		if year > 0 {
			years = append(years, year)
		}
	}
	sort.Ints(years)
	return years, nil
}

func (s FinalOutcomeAnalyticsService) fetchAvailableYearsBelum(ctx context.Context, normalizedJenjang string) ([]int, error) {
	belumCondition, err := s.buildBelumCondition(ctx)
	if err != nil {
		return nil, err
	}

	clauses := []string{"c.tahun_lulus > 0", "(" + belumCondition + ")"}
	args := make([]interface{}, 0)
	idx := 1
	if normalizedJenjang != "" {
		clauses = append(clauses, fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx))
		args = append(args, normalizedJenjang)
		idx++
	}
	where := " WHERE " + strings.Join(clauses, " AND ")

	query := `
		SELECT DISTINCT c.tahun_lulus
		FROM class12_students c
		JOIN data_sekolah ds ON ds.id = c.school_id
		LEFT JOIN student_outcomes o
		  ON o.school_id = c.school_id
		 AND o.tahun_lulus = c.tahun_lulus
		 AND o.nisn = c.nisn
	` + where + `
		ORDER BY c.tahun_lulus ASC
	`

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	years := make([]int, 0)
	for rows.Next() {
		var year int
		if scanErr := rows.Scan(&year); scanErr != nil {
			return nil, scanErr
		}
		if year > 0 {
			years = append(years, year)
		}
	}
	sort.Ints(years)
	return years, nil
}

func (s FinalOutcomeAnalyticsService) fetchKpis(ctx context.Context, selectedYear int, normalizedJenjang string, normalizedStatus string) (FinalOutcomeAnalyticsKpis, error) {
	if selectedYear <= 0 {
		return FinalOutcomeAnalyticsKpis{Total: 0, YoYPercent: nil, CurrentYearTotal: 0, PrevYearTotal: 0}, nil
	}

	currentTotal, err := s.fetchYearTotal(ctx, selectedYear, normalizedJenjang, normalizedStatus)
	if err != nil {
		return FinalOutcomeAnalyticsKpis{}, err
	}
	prevTotal, err := s.fetchYearTotal(ctx, selectedYear-1, normalizedJenjang, normalizedStatus)
	if err != nil {
		return FinalOutcomeAnalyticsKpis{}, err
	}

	var yoyPercent *float64
	if prevTotal > 0 {
		value := (float64(currentTotal-prevTotal) / float64(prevTotal)) * 100
		yoyPercent = &value
	}

	return FinalOutcomeAnalyticsKpis{
		Total:            currentTotal,
		YoYPercent:       yoyPercent,
		CurrentYearTotal: currentTotal,
		PrevYearTotal:    prevTotal,
	}, nil
}

func (s FinalOutcomeAnalyticsService) fetchKpisBelum(ctx context.Context, selectedYear int, normalizedJenjang string) (FinalOutcomeAnalyticsKpis, error) {
	if selectedYear <= 0 {
		return FinalOutcomeAnalyticsKpis{Total: 0, YoYPercent: nil, CurrentYearTotal: 0, PrevYearTotal: 0}, nil
	}

	currentTotal, err := s.fetchYearTotalBelum(ctx, selectedYear, normalizedJenjang)
	if err != nil {
		return FinalOutcomeAnalyticsKpis{}, err
	}
	prevTotal, err := s.fetchYearTotalBelum(ctx, selectedYear-1, normalizedJenjang)
	if err != nil {
		return FinalOutcomeAnalyticsKpis{}, err
	}

	var yoyPercent *float64
	if prevTotal > 0 {
		value := (float64(currentTotal-prevTotal) / float64(prevTotal)) * 100
		yoyPercent = &value
	}

	return FinalOutcomeAnalyticsKpis{
		Total:            currentTotal,
		YoYPercent:       yoyPercent,
		CurrentYearTotal: currentTotal,
		PrevYearTotal:    prevTotal,
	}, nil
}

func (s FinalOutcomeAnalyticsService) fetchYearTotal(ctx context.Context, year int, normalizedJenjang string, normalizedStatus string) (int, error) {
	where, args := buildOutcomeWhere(normalizedJenjang, normalizedStatus, true, year)
	query := `
		SELECT COUNT(*)
		FROM student_outcomes o
		JOIN data_sekolah ds ON ds.id = o.school_id
	` + where

	var total int
	if err := s.DB.Pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s FinalOutcomeAnalyticsService) fetchYearTotalBelum(ctx context.Context, year int, normalizedJenjang string) (int, error) {
	belumCondition, err := s.buildBelumCondition(ctx)
	if err != nil {
		return 0, err
	}

	clauses := []string{"c.tahun_lulus > 0", "c.tahun_lulus = $1", "(" + belumCondition + ")"}
	args := []interface{}{year}
	idx := 2
	if normalizedJenjang != "" {
		clauses = append(clauses, fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx))
		args = append(args, normalizedJenjang)
		idx++
	}
	where := " WHERE " + strings.Join(clauses, " AND ")
	query := `
		SELECT COUNT(*)
		FROM class12_students c
		JOIN data_sekolah ds ON ds.id = c.school_id
		LEFT JOIN student_outcomes o
		  ON o.school_id = c.school_id
		 AND o.tahun_lulus = c.tahun_lulus
		 AND o.nisn = c.nisn
	` + where

	var total int
	if err := s.DB.Pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s FinalOutcomeAnalyticsService) fetchTrendByYear(ctx context.Context, selectedYear int, normalizedJenjang string, normalizedStatus string) ([]AnalyticsYearPoint, error) {
	if selectedYear <= 0 {
		return []AnalyticsYearPoint{}, nil
	}

	startYear := selectedYear - 4
	if startYear < 0 {
		startYear = 0
	}

	where, args := buildOutcomeWhere(normalizedJenjang, normalizedStatus, false, 0)
	idx := len(args) + 1
	where += fmt.Sprintf(" AND o.tahun_lulus BETWEEN $%d AND $%d", idx, idx+1)
	args = append(args, startYear, selectedYear)

	query := `
		SELECT o.tahun_lulus, COUNT(*)
		FROM student_outcomes o
		JOIN data_sekolah ds ON ds.id = o.school_id
	` + where + `
		GROUP BY o.tahun_lulus
		ORDER BY o.tahun_lulus ASC
	`

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]AnalyticsYearPoint, 0)
	for rows.Next() {
		var point AnalyticsYearPoint
		if scanErr := rows.Scan(&point.Year, &point.Total); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, point)
	}
	return result, nil
}

func (s FinalOutcomeAnalyticsService) fetchTrendByYearBelum(ctx context.Context, selectedYear int, normalizedJenjang string) ([]AnalyticsYearPoint, error) {
	if selectedYear <= 0 {
		return []AnalyticsYearPoint{}, nil
	}

	startYear := selectedYear - 4
	if startYear < 0 {
		startYear = 0
	}

	belumCondition, err := s.buildBelumCondition(ctx)
	if err != nil {
		return nil, err
	}

	clauses := []string{"c.tahun_lulus > 0", "(" + belumCondition + ")"}
	args := make([]interface{}, 0)
	idx := 1
	if normalizedJenjang != "" {
		clauses = append(clauses, fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx))
		args = append(args, normalizedJenjang)
		idx++
	}
	clauses = append(clauses, fmt.Sprintf("c.tahun_lulus BETWEEN $%d AND $%d", idx, idx+1))
	args = append(args, startYear, selectedYear)

	query := `
		SELECT c.tahun_lulus, COUNT(*)
		FROM class12_students c
		JOIN data_sekolah ds ON ds.id = c.school_id
		LEFT JOIN student_outcomes o
		  ON o.school_id = c.school_id
		 AND o.tahun_lulus = c.tahun_lulus
		 AND o.nisn = c.nisn
		WHERE ` + strings.Join(clauses, " AND ") + `
		GROUP BY c.tahun_lulus
		ORDER BY c.tahun_lulus ASC
	`

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]AnalyticsYearPoint, 0)
	for rows.Next() {
		var point AnalyticsYearPoint
		if scanErr := rows.Scan(&point.Year, &point.Total); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, point)
	}
	return result, nil
}

func (s FinalOutcomeAnalyticsService) fetchBreakdownByJalur(ctx context.Context, selectedYear int, normalizedJenjang string) ([]AnalyticsKeyTotal, error) {
	if selectedYear <= 0 {
		return []AnalyticsKeyTotal{}, nil
	}

	where, args := buildOutcomeWhere(normalizedJenjang, "PT", true, selectedYear)
	query := `
		SELECT
			CASE
				WHEN LOWER(COALESCE(pt.jalur::text, o.final_path::text, '')) LIKE '%snbp%' THEN 'SNBP'
				WHEN LOWER(COALESCE(pt.jalur::text, o.final_path::text, '')) LIKE '%snbt%' THEN 'SNBT'
				WHEN LOWER(COALESCE(pt.jalur::text, o.final_path::text, '')) LIKE '%mandiri%'
					OR LOWER(COALESCE(pt.jalur::text, o.final_path::text, '')) LIKE '%snbm%' THEN 'SNBM (Mandiri)'
				WHEN LOWER(COALESCE(pt.jalur::text, o.final_path::text, '')) LIKE '%kedinas%' THEN 'Kedinasan'
				ELSE 'Lainnya'
			END AS key,
			COUNT(*) AS total
		FROM student_outcomes o
		JOIN data_sekolah ds ON ds.id = o.school_id
		LEFT JOIN outcome_pt_details pt
		  ON pt.school_id = o.school_id
		 AND pt.tahun_lulus = o.tahun_lulus
		 AND pt.nisn = o.nisn
	` + where + `
		GROUP BY key
		ORDER BY key ASC
	`

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]AnalyticsKeyTotal, 0)
	for rows.Next() {
		var item AnalyticsKeyTotal
		if scanErr := rows.Scan(&item.Key, &item.Total); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, nil
}

func (s FinalOutcomeAnalyticsService) fetchTopItems(
	ctx context.Context,
	selectedYear int,
	normalizedJenjang string,
	column string,
	totalBase int,
) ([]FinalOutcomeTopItem, error) {
	if selectedYear <= 0 {
		return []FinalOutcomeTopItem{}, nil
	}

	columnName := ""
	switch column {
	case "kampus":
		columnName = "pt.kampus"
	case "prodi":
		columnName = "pt.prodi"
	default:
		return []FinalOutcomeTopItem{}, nil
	}

	where, args := buildOutcomeWhere(normalizedJenjang, "PT", true, selectedYear)
	query := `
		SELECT TRIM(COALESCE(` + columnName + `, '')) AS name,
		       COUNT(*) AS total
		FROM student_outcomes o
		JOIN data_sekolah ds ON ds.id = o.school_id
		JOIN outcome_pt_details pt
		  ON pt.school_id = o.school_id
		 AND pt.tahun_lulus = o.tahun_lulus
		 AND pt.nisn = o.nisn
	` + where + `
		AND TRIM(COALESCE(` + columnName + `, '')) <> ''
		GROUP BY name
		ORDER BY total DESC, name ASC
		LIMIT 10
	`

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]FinalOutcomeTopItem, 0)
	for rows.Next() {
		var item FinalOutcomeTopItem
		if scanErr := rows.Scan(&item.Name, &item.Total); scanErr != nil {
			return nil, scanErr
		}
		if totalBase > 0 {
			item.SharePercent = (float64(item.Total) / float64(totalBase)) * 100
		} else {
			item.SharePercent = 0
		}
		result = append(result, item)
	}
	return result, nil
}

func (s FinalOutcomeAnalyticsService) buildBelumCondition(ctx context.Context) (string, error) {
	lockOK, _, lockErr := tablesExist(ctx, s.DB, "graduation_year_locks")
	if lockErr != nil {
		return "", lockErr
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
	return fmt.Sprintf(
		"((%s) AND o.finalized_at IS NULL) OR (NOT (%s) AND o.final_status = 'BELUM' AND o.finalized_at IS NOT NULL)",
		yearClosedExpr,
		yearClosedExpr,
	), nil
}
