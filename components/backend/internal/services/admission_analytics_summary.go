package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

type AdmissionAnalyticsSummaryFilters struct {
	Role      string
	SchoolID  *int
	Year      string
	Jenjang   string
	Status    string
	SkipFinal bool
	AllowedLevels []string
}

type AdmissionAnalyticsKpis struct {
	TotalDiterima    int      `json:"totalDiterima"`
	YoYPercent       *float64 `json:"yoyPercent"`
	CurrentYearTotal int      `json:"currentYearTotal"`
	PrevYearTotal    int      `json:"prevYearTotal"`
}

type AdmissionAnalyticsDistribution struct {
	ByJalur []AnalyticsKeyTotal `json:"byJalur,omitempty"`
}

type AdmissionTopSchool struct {
	SchoolID   int      `json:"school_id"`
	SchoolName string   `json:"school_name"`
	Total      int      `json:"total"`
	YoYPercent *float64 `json:"yoyPercent"`
}

type AdmissionTrendByJalurPoint struct {
	Year      int `json:"year"`
	SNBP      int `json:"SNBP"`
	SNBT      int `json:"SNBT"`
	MANDIRI   int `json:"MANDIRI"`
	KEDINASAN int `json:"KEDINASAN"`
	LAINNYA   int `json:"LAINNYA"`
}

type AdmissionTopPerguruanTinggi struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
}

type AdmissionAnalyticsSummary struct {
	FiltersEcho        AnalyticsFiltersEcho           `json:"filtersEcho"`
	AvailableYears     []int                          `json:"availableYears"`
	LatestYear         *int                           `json:"latestYear"`
	Kpis               AdmissionAnalyticsKpis         `json:"kpis"`
	TrendByYear        []AnalyticsYearPoint           `json:"trendByYear"`
	Distribution       AdmissionAnalyticsDistribution `json:"distribution"`
	TopSchools         []AdmissionTopSchool           `json:"topSchools,omitempty"`
	TrendByJalur       []AdmissionTrendByJalurPoint   `json:"trendByJalur,omitempty"`
	TopPerguruanTinggi []AdmissionTopPerguruanTinggi  `json:"topPerguruanTinggi,omitempty"`
}

type AdmissionAnalyticsSummaryService struct {
	DB *db.DB
}

func (s AdmissionAnalyticsSummaryService) Get(
	ctx context.Context,
	filters AdmissionAnalyticsSummaryFilters,
) (AdmissionAnalyticsSummary, error) {
	role := strings.ToUpper(strings.TrimSpace(filters.Role))
	if role == "" {
		role = "PUBLIC"
	}
	isAdminMaster := role == "ADMIN_MASTER"
	if isAdminMaster {
		role = "ADMIN"
	}

	var scopeSchoolID *int
	switch role {
	case "SCHOOL_ADMIN":
		if filters.SchoolID == nil {
			return AdmissionAnalyticsSummary{}, auth.ErrForbidden
		}
		scopeSchoolID = filters.SchoolID
	case "ADMIN":
		scopeSchoolID = filters.SchoolID
	case "PUBLIC":
		scopeSchoolID = nil
	default:
		return AdmissionAnalyticsSummary{}, auth.ErrForbidden
	}

	normalizedJenjang := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
	if normalizedJenjang == "ALL" {
		normalizedJenjang = ""
	}
	normalizedStatus := strings.ToUpper(strings.TrimSpace(filters.Status))
	if normalizedStatus == "ALL" {
		normalizedStatus = ""
	}

	parseYear := func(value string) (*int, error) {
		raw := strings.TrimSpace(value)
		if raw == "" || strings.EqualFold(raw, "ALL") {
			return nil, nil
		}
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			return nil, fmt.Errorf("invalid year")
		}
		return &parsed, nil
	}

	yearValue, err := parseYear(filters.Year)
	if err != nil {
		return AdmissionAnalyticsSummary{}, err
	}

	base := `
        WITH normalized AS (
            SELECT spt.school_id,
                   ds.nama_sekolah,
                   CASE
                       WHEN COALESCE(spt.tahun_diterima, 0) > 0 THEN spt.tahun_diterima
                       ELSE NULL
                   END AS year,
                   CASE
                       WHEN UPPER(TRIM(COALESCE(spt.jalur_penerimaan::text, ''))) IN ('SNBP', 'SNBT', 'MANDIRI', 'KEDINASAN') THEN UPPER(TRIM(COALESCE(spt.jalur_penerimaan::text, '')))
                       ELSE 'LAINNYA'
                   END AS jalur,
                   UPPER(TRIM(COALESCE(spt.jenjang_pendidikan, ds.bentuk_pendidikan, ''))) AS jenjang,
                   UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) AS status
            FROM siswa_perguruan_tinggi spt
            JOIN data_sekolah ds ON ds.id = spt.school_id
        )
    `

	buildWhere := func(includeYear bool, yearList []int) (string, []interface{}) {
		clauses := []string{"year IS NOT NULL"}
		var args []interface{}
		idx := 1
		addClause := func(condition string, value interface{}) {
			clauses = append(clauses, condition)
			args = append(args, value)
			idx++
		}

		if scopeSchoolID != nil {
			addClause(fmt.Sprintf("school_id = $%d", idx), *scopeSchoolID)
		}
		if role == "ADMIN" && !isAdminMaster {
			allowed := auth.NormalizeAllowedLevels(filters.AllowedLevels)
			if len(allowed) == 0 {
				clauses = append(clauses, "1=0")
			} else {
				addClause(fmt.Sprintf("jenjang = ANY($%d)", idx), allowed)
			}
		}
		if role != "SCHOOL_ADMIN" && normalizedJenjang != "" {
			addClause(fmt.Sprintf("jenjang = $%d", idx), normalizedJenjang)
		}
		if role != "SCHOOL_ADMIN" && normalizedStatus != "" {
			addClause(fmt.Sprintf("status = $%d", idx), normalizedStatus)
		}
		if includeYear && yearValue != nil {
			addClause(fmt.Sprintf("year = $%d", idx), *yearValue)
		}
		if len(yearList) > 0 {
			placeholders := make([]string, 0, len(yearList))
			for _, year := range yearList {
				placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
				args = append(args, year)
				idx++
			}
			clauses = append(clauses, fmt.Sprintf("year IN (%s)", strings.Join(placeholders, ", ")))
		}

		return " WHERE " + strings.Join(clauses, " AND "), args
	}

	buildFinalWhere := func(includeYear bool) (string, []interface{}) {
		clauses := []string{
			"fo.final_type = 'HIGHER_ED'",
			"fo.offer_id IS NOT NULL",
		}
		var args []interface{}
		idx := 1
		addClause := func(condition string, value interface{}) {
			clauses = append(clauses, condition)
			args = append(args, value)
			idx++
		}

		if scopeSchoolID != nil {
			addClause(fmt.Sprintf("c.school_id = $%d", idx), *scopeSchoolID)
		}
		if role != "SCHOOL_ADMIN" && normalizedJenjang != "" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx), normalizedJenjang)
		}
		if role != "SCHOOL_ADMIN" && normalizedStatus != "" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $%d", idx), normalizedStatus)
		}
		if includeYear && yearValue != nil {
			addClause(fmt.Sprintf("c.tahun_lulus = $%d", idx), *yearValue)
		}

		return " WHERE " + strings.Join(clauses, " AND "), args
	}

	legacyBase := `
        WITH normalized AS (
            SELECT spt.school_id,
                   COALESCE(NULLIF(spt.tahun_lulus, 0), NULLIF(spt.tahun_diterima, 0)) AS year_ref,
                   CASE
                       WHEN UPPER(TRIM(COALESCE(spt.jalur_penerimaan::text, ''))) IN ('SNBP', 'SNBT', 'MANDIRI', 'KEDINASAN') THEN UPPER(TRIM(COALESCE(spt.jalur_penerimaan::text, '')))
                       ELSE 'LAINNYA'
                   END AS jalur,
                   UPPER(TRIM(COALESCE(spt.jenjang_pendidikan, ds.bentuk_pendidikan, ''))) AS jenjang,
                   UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) AS status,
                   TRIM(COALESCE(spt.nama_perguruan_tinggi, '')) AS nama_perguruan_tinggi
            FROM siswa_perguruan_tinggi spt
            JOIN data_sekolah ds ON ds.id = spt.school_id
        )
    `

	buildLegacyWhere := func(requireYearRef bool, includeYear bool, requireName bool) (string, []interface{}) {
		var clauses []string
		var args []interface{}
		idx := 1
		addClause := func(condition string, value interface{}) {
			clauses = append(clauses, condition)
			args = append(args, value)
			idx++
		}

		if requireYearRef {
			clauses = append(clauses, "year_ref IS NOT NULL")
		}
		if requireName {
			clauses = append(clauses, "nama_perguruan_tinggi <> ''")
		}
		if scopeSchoolID != nil {
			addClause(fmt.Sprintf("school_id = $%d", idx), *scopeSchoolID)
		}
		if role != "SCHOOL_ADMIN" && normalizedJenjang != "" {
			addClause(fmt.Sprintf("jenjang = $%d", idx), normalizedJenjang)
		}
		if role != "SCHOOL_ADMIN" && normalizedStatus != "" {
			addClause(fmt.Sprintf("status = $%d", idx), normalizedStatus)
		}
		if includeYear && yearValue != nil {
			addClause(fmt.Sprintf("year_ref = $%d", idx), *yearValue)
		}

		if len(clauses) == 0 {
			return "", args
		}
		return " WHERE " + strings.Join(clauses, " AND "), args
	}

	availableWhere, availableArgs := buildWhere(false, nil)
	availableQuery := base + `
        SELECT DISTINCT year
        FROM normalized
    ` + availableWhere + `
        ORDER BY year DESC
    `
	yearRows, err := s.DB.Pool.Query(ctx, availableQuery, availableArgs...)
	if err != nil {
		return AdmissionAnalyticsSummary{}, err
	}
	defer yearRows.Close()

	var availableYears []int
	for yearRows.Next() {
		var year int
		if scanErr := yearRows.Scan(&year); scanErr == nil {
			availableYears = append(availableYears, year)
		}
	}

	var latestYear *int
	if len(availableYears) > 0 {
		latest := availableYears[0]
		latestYear = &latest
	}

	totalWhere, totalArgs := buildWhere(true, nil)
	totalQuery := base + `
        SELECT COUNT(*)
        FROM normalized
    ` + totalWhere

	var total int
	if err := s.DB.Pool.QueryRow(ctx, totalQuery, totalArgs...).Scan(&total); err != nil {
		return AdmissionAnalyticsSummary{}, err
	}

	currentYear := 0
	if yearValue != nil {
		currentYear = *yearValue
	} else if latestYear != nil {
		currentYear = *latestYear
	}
	prevYear := 0
	if currentYear > 0 {
		prevYear = currentYear - 1
	}

	currentTotal := 0
	prevTotal := 0
	if currentYear > 0 {
		yearsFilter := []int{currentYear}
		if prevYear > 0 {
			yearsFilter = append(yearsFilter, prevYear)
		}
		compareWhere, compareArgs := buildWhere(false, yearsFilter)
		compareQuery := base + `
        SELECT year, COUNT(*)
        FROM normalized
    ` + compareWhere + `
        GROUP BY year
    `
		compareRows, err := s.DB.Pool.Query(ctx, compareQuery, compareArgs...)
		if err != nil {
			return AdmissionAnalyticsSummary{}, err
		}
		defer compareRows.Close()
		for compareRows.Next() {
			var year int
			var count int
			if scanErr := compareRows.Scan(&year, &count); scanErr == nil {
				if year == currentYear {
					currentTotal = count
				} else if year == prevYear {
					prevTotal = count
				}
			}
		}
	}

	var yoyPercent *float64
	if prevTotal > 0 {
		value := (float64(currentTotal-prevTotal) / float64(prevTotal)) * 100
		yoyPercent = &value
	}

	trendYears := []int{}
	if yearValue != nil && currentYear > 0 {
		trendYears = append(trendYears, currentYear)
		if prevYear > 0 {
			trendYears = append(trendYears, prevYear)
		}
	}
	trendWhere, trendArgs := buildWhere(false, trendYears)
	trendQuery := base + `
        SELECT year, COUNT(*)
        FROM normalized
    ` + trendWhere + `
        GROUP BY year
        ORDER BY year ASC
    `
	trendRows, err := s.DB.Pool.Query(ctx, trendQuery, trendArgs...)
	if err != nil {
		return AdmissionAnalyticsSummary{}, err
	}
	defer trendRows.Close()

	var trend []AnalyticsYearPoint
	for trendRows.Next() {
		var year int
		var count int
		if scanErr := trendRows.Scan(&year, &count); scanErr == nil {
			trend = append(trend, AnalyticsYearPoint{Year: year, Total: count})
		}
	}

	distributionWhere, distributionArgs := buildWhere(true, nil)
	jalurQuery := base + `
        SELECT jalur, COUNT(*)
        FROM normalized
    ` + distributionWhere + `
        GROUP BY jalur
    `
	jalurRows, err := s.DB.Pool.Query(ctx, jalurQuery, distributionArgs...)
	if err != nil {
		return AdmissionAnalyticsSummary{}, err
	}
	defer jalurRows.Close()

	jalurOrder := []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}
	jalurLabel := map[string]string{
		"SNBP":      "SNBP",
		"SNBT":      "SNBT",
		"MANDIRI":   "Mandiri",
		"KEDINASAN": "Kedinasan",
		"LAINNYA":   "Lainnya",
	}
	jalurMap := map[string]int{}
	for jalurRows.Next() {
		var jalur string
		var count int
		if scanErr := jalurRows.Scan(&jalur, &count); scanErr == nil {
			jalurMap[jalur] = count
		}
	}
	var byJalur []AnalyticsKeyTotal
	for _, key := range jalurOrder {
		label := jalurLabel[key]
		if label == "" {
			label = key
		}
		byJalur = append(byJalur, AnalyticsKeyTotal{Key: label, Total: jalurMap[key]})
	}

	var topSchools []AdmissionTopSchool
	if role == "ADMIN" && currentYear > 0 {
		yearFilter := []int{currentYear}
		if prevYear > 0 {
			yearFilter = append(yearFilter, prevYear)
		}
		topWhere, topArgs := buildWhere(false, yearFilter)
		topQuery := base + fmt.Sprintf(`
        SELECT school_id,
               nama_sekolah,
               SUM(CASE WHEN year = %d THEN 1 ELSE 0 END) AS current_total,
               SUM(CASE WHEN year = %d THEN 1 ELSE 0 END) AS prev_total
        FROM normalized
    `, currentYear, prevYear) + topWhere + `
        GROUP BY school_id, nama_sekolah
        ORDER BY current_total DESC, nama_sekolah ASC
        LIMIT 10
    `
		topRows, err := s.DB.Pool.Query(ctx, topQuery, topArgs...)
		if err != nil {
			return AdmissionAnalyticsSummary{}, err
		}
		defer topRows.Close()

		for topRows.Next() {
			var schoolID int
			var schoolName string
			var currentCount int
			var prevCount int
			if scanErr := topRows.Scan(&schoolID, &schoolName, &currentCount, &prevCount); scanErr == nil {
				var schoolYoY *float64
				if prevCount > 0 {
					value := (float64(currentCount-prevCount) / float64(prevCount)) * 100
					schoolYoY = &value
				}
				topSchools = append(topSchools, AdmissionTopSchool{
					SchoolID:   schoolID,
					SchoolName: schoolName,
					Total:      currentCount,
					YoYPercent: schoolYoY,
				})
			}
		}
	}

	if yearValue == nil {
		sort.Slice(trend, func(i, j int) bool {
			return trend[i].Year < trend[j].Year
		})
	}

	fetchFinalTrendByJalur := func() ([]AdmissionTrendByJalurPoint, error) {
		finalTrendWhere, finalTrendArgs := buildFinalWhere(true)
		trendByJalurQuery := `
        SELECT c.tahun_lulus,
               SUM(CASE WHEN UPPER(TRIM(COALESCE(ao.jalur_penerimaan::text, ''))) = 'SNBP' THEN 1 ELSE 0 END) AS snbp,
               SUM(CASE WHEN UPPER(TRIM(COALESCE(ao.jalur_penerimaan::text, ''))) = 'SNBT' THEN 1 ELSE 0 END) AS snbt,
               SUM(CASE WHEN UPPER(TRIM(COALESCE(ao.jalur_penerimaan::text, ''))) IN ('MANDIRI', 'SNBM') THEN 1 ELSE 0 END) AS mandiri,
               SUM(CASE WHEN UPPER(TRIM(COALESCE(ao.jalur_penerimaan::text, ''))) = 'KEDINASAN' THEN 1 ELSE 0 END) AS kedinasan,
               SUM(CASE WHEN UPPER(TRIM(COALESCE(ao.jalur_penerimaan::text, ''))) NOT IN ('SNBP', 'SNBT', 'MANDIRI', 'SNBM', 'KEDINASAN') THEN 1 ELSE 0 END) AS lainnya
        FROM student_final_outcomes fo
        JOIN student_cohorts c
          ON c.school_id = fo.school_id
         AND c.id = fo.cohort_id
        JOIN student_higher_ed_acceptance_offers ao
          ON ao.id = fo.offer_id
        JOIN data_sekolah ds ON ds.id = c.school_id
    ` + finalTrendWhere + `
        GROUP BY c.tahun_lulus
        ORDER BY c.tahun_lulus ASC
    `
		rows, err := s.DB.Pool.Query(ctx, trendByJalurQuery, finalTrendArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var trendByJalur []AdmissionTrendByJalurPoint
		for rows.Next() {
			var point AdmissionTrendByJalurPoint
			if scanErr := rows.Scan(&point.Year, &point.SNBP, &point.SNBT, &point.MANDIRI, &point.KEDINASAN, &point.LAINNYA); scanErr != nil {
				return nil, scanErr
			}
			trendByJalur = append(trendByJalur, point)
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return trendByJalur, nil
	}

	fetchLegacyTrendByJalur := func() ([]AdmissionTrendByJalurPoint, error) {
		legacyTrendWhere, legacyTrendArgs := buildLegacyWhere(true, true, false)
		legacyTrendQuery := legacyBase + `
        SELECT year_ref,
               SUM(CASE WHEN jalur = 'SNBP' THEN 1 ELSE 0 END) AS snbp,
               SUM(CASE WHEN jalur = 'SNBT' THEN 1 ELSE 0 END) AS snbt,
               SUM(CASE WHEN jalur = 'MANDIRI' THEN 1 ELSE 0 END) AS mandiri,
               SUM(CASE WHEN jalur = 'KEDINASAN' THEN 1 ELSE 0 END) AS kedinasan,
               SUM(CASE WHEN jalur = 'LAINNYA' THEN 1 ELSE 0 END) AS lainnya
        FROM normalized
    ` + legacyTrendWhere + `
        GROUP BY year_ref
        ORDER BY year_ref ASC
    `
		rows, err := s.DB.Pool.Query(ctx, legacyTrendQuery, legacyTrendArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var trendByJalur []AdmissionTrendByJalurPoint
		for rows.Next() {
			var point AdmissionTrendByJalurPoint
			if scanErr := rows.Scan(&point.Year, &point.SNBP, &point.SNBT, &point.MANDIRI, &point.KEDINASAN, &point.LAINNYA); scanErr != nil {
				return nil, scanErr
			}
			trendByJalur = append(trendByJalur, point)
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return trendByJalur, nil
	}

	fetchFinalTopPerguruanTinggi := func() ([]AdmissionTopPerguruanTinggi, error) {
		finalTopWhere, finalTopArgs := buildFinalWhere(true)
		finalTopWhere += " AND ao.nama_perguruan_tinggi IS NOT NULL AND TRIM(ao.nama_perguruan_tinggi) <> ''"
		topPerguruanTinggiQuery := `
        SELECT MIN(TRIM(ao.nama_perguruan_tinggi)) AS name,
               COUNT(*) AS total
        FROM student_final_outcomes fo
        JOIN student_cohorts c
          ON c.school_id = fo.school_id
         AND c.id = fo.cohort_id
        JOIN student_higher_ed_acceptance_offers ao
          ON ao.id = fo.offer_id
        JOIN data_sekolah ds ON ds.id = c.school_id
    ` + finalTopWhere + `
        GROUP BY UPPER(TRIM(ao.nama_perguruan_tinggi))
        ORDER BY total DESC, name ASC
        LIMIT 10
    `
		rows, err := s.DB.Pool.Query(ctx, topPerguruanTinggiQuery, finalTopArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var topPerguruanTinggi []AdmissionTopPerguruanTinggi
		for rows.Next() {
			var row AdmissionTopPerguruanTinggi
			if scanErr := rows.Scan(&row.Name, &row.Total); scanErr != nil {
				return nil, scanErr
			}
			topPerguruanTinggi = append(topPerguruanTinggi, row)
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return topPerguruanTinggi, nil
	}

	fetchLegacyTopPerguruanTinggi := func() ([]AdmissionTopPerguruanTinggi, error) {
		legacyTopWhere, legacyTopArgs := buildLegacyWhere(false, true, true)
		legacyTopQuery := legacyBase + `
        SELECT MIN(nama_perguruan_tinggi) AS name,
               COUNT(*) AS total
        FROM normalized
    ` + legacyTopWhere + `
        GROUP BY UPPER(nama_perguruan_tinggi)
        ORDER BY total DESC, name ASC
        LIMIT 10
    `
		rows, err := s.DB.Pool.Query(ctx, legacyTopQuery, legacyTopArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var topPerguruanTinggi []AdmissionTopPerguruanTinggi
		for rows.Next() {
			var row AdmissionTopPerguruanTinggi
			if scanErr := rows.Scan(&row.Name, &row.Total); scanErr != nil {
				return nil, scanErr
			}
			topPerguruanTinggi = append(topPerguruanTinggi, row)
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return topPerguruanTinggi, nil
	}

	useLegacyFinal := filters.SkipFinal
	var trendByJalur []AdmissionTrendByJalurPoint
	if useLegacyFinal {
		trendByJalur, err = fetchLegacyTrendByJalur()
	} else {
		trendByJalur, err = fetchFinalTrendByJalur()
	}
	if err != nil {
		return AdmissionAnalyticsSummary{}, err
	}

	topPerguruanTinggi, err := func() ([]AdmissionTopPerguruanTinggi, error) {
		if useLegacyFinal {
			return fetchLegacyTopPerguruanTinggi()
		}
		return fetchFinalTopPerguruanTinggi()
	}()
	if err != nil {
		return AdmissionAnalyticsSummary{}, err
	}

	return AdmissionAnalyticsSummary{
		FiltersEcho: AnalyticsFiltersEcho{
			Year:     filters.Year,
			Jenjang:  filters.Jenjang,
			Status:   filters.Status,
			SchoolID: scopeSchoolID,
		},
		AvailableYears: availableYears,
		LatestYear:     latestYear,
		Kpis: AdmissionAnalyticsKpis{
			TotalDiterima:    total,
			YoYPercent:       yoyPercent,
			CurrentYearTotal: currentTotal,
			PrevYearTotal:    prevTotal,
		},
		TrendByYear: trend,
		Distribution: AdmissionAnalyticsDistribution{
			ByJalur: byJalur,
		},
		TopSchools:         topSchools,
		TrendByJalur:       trendByJalur,
		TopPerguruanTinggi: topPerguruanTinggi,
	}, nil
}
