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

type AchievementAnalyticsSummaryFilters struct {
	Role       string
	SchoolID   *int
	Year       string
	Jenjang    string
	Status     string
	JenisLomba string
	AllowedLevels []string
}

type AnalyticsFiltersEcho struct {
	Year       string `json:"year"`
	Jenjang    string `json:"jenjang"`
	Status     string `json:"status"`
	SchoolID   *int   `json:"school_id,omitempty"`
	JenisLomba string `json:"jenis_lomba,omitempty"`
}

type AchievementAnalyticsKpis struct {
	Total            int      `json:"total"`
	YoYPercent       *float64 `json:"yoyPercent"`
	CurrentYearTotal int      `json:"currentYearTotal"`
	PrevYearTotal    int      `json:"prevYearTotal"`
}

type AnalyticsYearPoint struct {
	Year  int `json:"year"`
	Total int `json:"total"`
}

type AnalyticsKeyTotal struct {
	Key   string `json:"key"`
	Total int    `json:"total"`
}

type AnalyticsTingkatYearTotal struct {
	Tingkat string `json:"tingkat"`
	Year    int    `json:"year"`
	Total   int    `json:"total"`
}

type AnalyticsJenisYearTotal struct {
	Jenis string `json:"jenis"`
	Year  int    `json:"year"`
	Total int    `json:"total"`
}

type AnalyticsCategoryTotal struct {
	Kategori string `json:"kategori"`
	Total    int    `json:"total"`
}

type AchievementAnalyticsDistribution struct {
	ByTingkat    []AnalyticsKeyTotal `json:"byTingkat,omitempty"`
	ByJenisLomba []AnalyticsKeyTotal `json:"byJenisLomba,omitempty"`
}

type AchievementAnalyticsDistributionByYear struct {
	ByTingkat      []AnalyticsTingkatYearTotal `json:"byTingkat,omitempty"`
	ByJenisPeserta []AnalyticsJenisYearTotal   `json:"byJenisPeserta,omitempty"`
}

type AchievementTopSchool struct {
	SchoolID   int      `json:"school_id"`
	SchoolName string   `json:"school_name"`
	Total      int      `json:"total"`
	YoYPercent *float64 `json:"yoyPercent"`
}

type AchievementAnalyticsSummary struct {
	FiltersEcho        AnalyticsFiltersEcho                   `json:"filtersEcho"`
	AvailableYears     []int                                  `json:"availableYears"`
	LatestYear         *int                                   `json:"latestYear"`
	Kpis               AchievementAnalyticsKpis               `json:"kpis"`
	TrendByYear        []AnalyticsYearPoint                   `json:"trendByYear"`
	Distribution       AchievementAnalyticsDistribution       `json:"distribution"`
	DistributionByYear AchievementAnalyticsDistributionByYear `json:"distributionByYear,omitempty"`
	CategoryDonutSiswa []AnalyticsCategoryTotal               `json:"categoryDonutSiswa,omitempty"`
	TopSchools         []AchievementTopSchool                 `json:"topSchools,omitempty"`
}

type AchievementAnalyticsSummaryService struct {
	DB *db.DB
}

func (s AchievementAnalyticsSummaryService) Get(
	ctx context.Context,
	filters AchievementAnalyticsSummaryFilters,
) (AchievementAnalyticsSummary, error) {
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
			return AchievementAnalyticsSummary{}, auth.ErrForbidden
		}
		scopeSchoolID = filters.SchoolID
	case "ADMIN":
		scopeSchoolID = filters.SchoolID
	case "PUBLIC":
		scopeSchoolID = nil
	default:
		return AchievementAnalyticsSummary{}, auth.ErrForbidden
	}

	normalizedJenjang := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
	if normalizedJenjang == "ALL" {
		normalizedJenjang = ""
	}
	normalizedStatus := strings.ToUpper(strings.TrimSpace(filters.Status))
	if normalizedStatus == "ALL" {
		normalizedStatus = ""
	}
	normalizedJenisLomba := strings.ToUpper(strings.TrimSpace(filters.JenisLomba))
	if normalizedJenisLomba == "ALL" {
		normalizedJenisLomba = ""
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
		return AchievementAnalyticsSummary{}, err
	}

	base := `
        WITH normalized AS (
            SELECT ps.school_id,
                   ds.nama_sekolah,
                   CASE
                       WHEN ps.tahun_lomba ~ '^\d{4}$' THEN ps.tahun_lomba::int
                       ELSE NULL
                   END AS year,
                   CASE
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('KAB', 'KABUPATEN', 'KOTA', 'KABUPATEN/KOTA', 'KOTA/KABUPATEN') THEN 'KABUPATEN'
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('PROV', 'PROVINSI') THEN 'PROVINSI'
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('NAS', 'NASIONAL') THEN 'NASIONAL'
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('INT', 'INTERNASIONAL') THEN 'INTERNASIONAL'
                       ELSE NULL
                   END AS tingkat,
                   CASE
                       WHEN UPPER(TRIM(COALESCE(ps.jenis_lomba, ''))) IN ('SISWA', 'MURID') THEN 'SISWA'
                       WHEN UPPER(TRIM(COALESCE(ps.jenis_lomba, ''))) IN ('GURU', 'PENDIDIK') THEN 'GURU'
                       WHEN UPPER(TRIM(COALESCE(ps.jenis_lomba, ''))) IN ('TENDIK', 'TENAGA KEPENDIDIKAN') THEN 'TENDIK'
                       ELSE 'LAINNYA'
                   END AS jenis_lomba,
                   CASE
                       WHEN UPPER(TRIM(COALESCE(ps.kategori_prestasi, ''))) IN ('OSN', 'O2SN', 'FLS3N', 'LKSN')
                           THEN UPPER(TRIM(COALESCE(ps.kategori_prestasi, '')))
                       ELSE 'LAINNYA'
                   END AS kategori,
                   UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) AS jenjang,
                   UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) AS status
            FROM prestasi_siswa ps
            JOIN data_sekolah ds ON ds.id = ps.school_id
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
		if normalizedJenisLomba != "" {
			addClause(fmt.Sprintf("jenis_lomba = $%d", idx), normalizedJenisLomba)
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

	availableWhere, availableArgs := buildWhere(false, nil)
	availableQuery := base + `
        SELECT DISTINCT year
        FROM normalized
    ` + availableWhere + `
        ORDER BY year DESC
    `
	yearRows, err := s.DB.Pool.Query(ctx, availableQuery, availableArgs...)
	if err != nil {
		return AchievementAnalyticsSummary{}, err
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
		return AchievementAnalyticsSummary{}, err
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
			return AchievementAnalyticsSummary{}, err
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
		return AchievementAnalyticsSummary{}, err
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
	tingkatQuery := base + `
        SELECT tingkat, COUNT(*)
        FROM normalized
    ` + distributionWhere + `
        AND tingkat IS NOT NULL
        GROUP BY tingkat
    `
	tingkatRows, err := s.DB.Pool.Query(ctx, tingkatQuery, distributionArgs...)
	if err != nil {
		return AchievementAnalyticsSummary{}, err
	}
	defer tingkatRows.Close()

	var byTingkat []AnalyticsKeyTotal
	for tingkatRows.Next() {
		var tingkat string
		var count int
		if scanErr := tingkatRows.Scan(&tingkat, &count); scanErr == nil {
			byTingkat = append(byTingkat, AnalyticsKeyTotal{Key: tingkat, Total: count})
		}
	}

	jenisQuery := base + `
        SELECT jenis_lomba, COUNT(*)
        FROM normalized
    ` + distributionWhere + `
        GROUP BY jenis_lomba
    `
	jenisRows, err := s.DB.Pool.Query(ctx, jenisQuery, distributionArgs...)
	if err != nil {
		return AchievementAnalyticsSummary{}, err
	}
	defer jenisRows.Close()

	jenisOrder := []string{"SISWA", "GURU", "TENDIK", "LAINNYA"}
	jenisLabel := map[string]string{
		"SISWA":   "Siswa",
		"GURU":    "Guru",
		"TENDIK":  "Tendik",
		"LAINNYA": "Lainnya",
	}
	jenisMap := map[string]int{}
	for jenisRows.Next() {
		var jenis string
		var count int
		if scanErr := jenisRows.Scan(&jenis, &count); scanErr == nil {
			jenisMap[jenis] = count
		}
	}
	var byJenis []AnalyticsKeyTotal
	for _, key := range jenisOrder {
		label := jenisLabel[key]
		if label == "" {
			label = key
		}
		byJenis = append(byJenis, AnalyticsKeyTotal{Key: label, Total: jenisMap[key]})
	}

	const maxYearSeries = 5
	selectedYears := []int{}
	if yearValue != nil {
		selectedYears = append(selectedYears, *yearValue)
	} else {
		selectedYears = append(selectedYears, availableYears...)
		if len(selectedYears) > maxYearSeries {
			selectedYears = selectedYears[:maxYearSeries]
		}
	}

	var distributionByYear AchievementAnalyticsDistributionByYear
	if len(selectedYears) > 0 {
		sort.Ints(selectedYears)
		yearWhere, yearArgs := buildWhere(false, selectedYears)

		tingkatYearQuery := base + `
        SELECT tingkat, year, COUNT(*)
        FROM normalized
    ` + yearWhere + `
        AND tingkat IS NOT NULL
        GROUP BY tingkat, year
        ORDER BY year ASC
    `
		tingkatYearRows, err := s.DB.Pool.Query(ctx, tingkatYearQuery, yearArgs...)
		if err != nil {
			return AchievementAnalyticsSummary{}, err
		}
		defer tingkatYearRows.Close()

		for tingkatYearRows.Next() {
			var tingkat string
			var year int
			var count int
			if scanErr := tingkatYearRows.Scan(&tingkat, &year, &count); scanErr == nil {
				distributionByYear.ByTingkat = append(distributionByYear.ByTingkat, AnalyticsTingkatYearTotal{
					Tingkat: tingkat,
					Year:    year,
					Total:   count,
				})
			}
		}

		jenisYearQuery := base + `
        SELECT jenis_lomba, year, COUNT(*)
        FROM normalized
    ` + yearWhere + `
        GROUP BY jenis_lomba, year
        ORDER BY year ASC
    `
		jenisYearRows, err := s.DB.Pool.Query(ctx, jenisYearQuery, yearArgs...)
		if err != nil {
			return AchievementAnalyticsSummary{}, err
		}
		defer jenisYearRows.Close()

		for jenisYearRows.Next() {
			var jenis string
			var year int
			var count int
			if scanErr := jenisYearRows.Scan(&jenis, &year, &count); scanErr == nil {
				label := jenisLabel[jenis]
				if label == "" {
					label = jenis
				}
				distributionByYear.ByJenisPeserta = append(distributionByYear.ByJenisPeserta, AnalyticsJenisYearTotal{
					Jenis: label,
					Year:  year,
					Total: count,
				})
			}
		}
	}

	categoryOrder := []string{"OSN", "O2SN", "FLS3N", "LKSN", "LAINNYA"}
	categoryLabel := map[string]string{
		"OSN":     "OSN",
		"O2SN":    "O2SN",
		"FLS3N":   "FLS3N",
		"LKSN":    "LKSN",
		"LAINNYA": "Lainnya",
	}
	donutWhere, donutArgs := buildWhere(true, nil)
	donutQuery := base + `
        SELECT kategori, COUNT(*)
        FROM normalized
    ` + donutWhere + `
        AND jenis_lomba = 'SISWA'
        GROUP BY kategori
    `
	donutRows, err := s.DB.Pool.Query(ctx, donutQuery, donutArgs...)
	if err != nil {
		return AchievementAnalyticsSummary{}, err
	}
	defer donutRows.Close()

	categoryTotals := map[string]int{}
	for donutRows.Next() {
		var kategori string
		var count int
		if scanErr := donutRows.Scan(&kategori, &count); scanErr == nil {
			categoryTotals[kategori] = count
		}
	}

	var categoryDonutSiswa []AnalyticsCategoryTotal
	for _, key := range categoryOrder {
		label := categoryLabel[key]
		if label == "" {
			label = key
		}
		total := categoryTotals[key]
		if total <= 0 {
			continue
		}
		categoryDonutSiswa = append(categoryDonutSiswa, AnalyticsCategoryTotal{
			Kategori: label,
			Total:    total,
		})
	}

	var topSchools []AchievementTopSchool
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
			return AchievementAnalyticsSummary{}, err
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
				topSchools = append(topSchools, AchievementTopSchool{
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

	return AchievementAnalyticsSummary{
		FiltersEcho: AnalyticsFiltersEcho{
			Year:       filters.Year,
			Jenjang:    filters.Jenjang,
			Status:     filters.Status,
			SchoolID:   scopeSchoolID,
			JenisLomba: filters.JenisLomba,
		},
		AvailableYears: availableYears,
		LatestYear:     latestYear,
		Kpis: AchievementAnalyticsKpis{
			Total:            total,
			YoYPercent:       yoyPercent,
			CurrentYearTotal: currentTotal,
			PrevYearTotal:    prevTotal,
		},
		TrendByYear: trend,
		Distribution: AchievementAnalyticsDistribution{
			ByTingkat:    byTingkat,
			ByJenisLomba: byJenis,
		},
		DistributionByYear: distributionByYear,
		CategoryDonutSiswa: categoryDonutSiswa,
		TopSchools:         topSchools,
	}, nil
}
