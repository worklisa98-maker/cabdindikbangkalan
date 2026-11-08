package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type SiswaOutcomeDashboardFilters struct {
	SchoolID      *int
	Status        string
	Year          int
	Jenjang       string
	StatusSekolah string
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

type SiswaOutcomeDashboardSummary struct {
	PT        int `json:"pt"`
	Kerja     int `json:"kerja"`
	Wirausaha int `json:"wirausaha"`
	Belum     int `json:"belum"`
}

type SiswaOutcomeDashboardPayload struct {
	Summary        SiswaOutcomeDashboardSummary `json:"summary"`
	Total          int                          `json:"total"`
	Kpis           SiswaOutcomeDashboardKpis    `json:"kpis"`
	TrendByYear    []SiswaOutcomeDashboardPoint `json:"trendByYear"`
	BreakdownJalur []SiswaOutcomeDashboardItem  `json:"breakdownJalur"`
	TopKampus      []SiswaOutcomeDashboardItem  `json:"topKampus"`
	TopProdi       []SiswaOutcomeDashboardItem  `json:"topProdi"`
}

func emptyOutcomeDashboard() SiswaOutcomeDashboardPayload {
	return SiswaOutcomeDashboardPayload{
		Summary: SiswaOutcomeDashboardSummary{
			PT:        0,
			Kerja:     0,
			Wirausaha: 0,
			Belum:     0,
		},
		Total: 0,
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

func roundPercent(value float64) float64 {
	return math.Round(value*10) / 10
}

func trimOutcomeDashboardTrend(points []SiswaOutcomeDashboardPoint, limit int) []SiswaOutcomeDashboardPoint {
	if limit <= 0 || len(points) <= limit {
		cloned := make([]SiswaOutcomeDashboardPoint, len(points))
		copy(cloned, points)
		return cloned
	}
	start := len(points) - limit
	cloned := make([]SiswaOutcomeDashboardPoint, len(points[start:]))
	copy(cloned, points[start:])
	return cloned
}

func buildOutcomeDashboardKpis(
	points []SiswaOutcomeDashboardPoint,
	targetYear int,
	aggregateTotal int,
	aggregateAllYears bool,
) SiswaOutcomeDashboardKpis {
	totalsByYear := make(map[int]int, len(points))
	for _, point := range points {
		totalsByYear[point.Year] = point.Total
	}

	currentYear := 0
	prevYear := 0
	switch {
	case targetYear > 0:
		currentYear = targetYear
		prevYear = targetYear - 1
	case len(points) > 0:
		currentYear = points[len(points)-1].Year
		if len(points) > 1 {
			prevYear = points[len(points)-2].Year
		}
	}

	currentTotal := totalsByYear[currentYear]
	prevTotal := totalsByYear[prevYear]
	total := currentTotal
	if aggregateAllYears {
		total = aggregateTotal
	}

	var yoy *float64
	if prevTotal > 0 {
		value := roundPercent((float64(currentTotal-prevTotal) / float64(prevTotal)) * 100)
		yoy = &value
	}

	return SiswaOutcomeDashboardKpis{
		Total:            total,
		CurrentYear:      currentYear,
		PrevYear:         prevYear,
		CurrentYearTotal: currentTotal,
		PrevYearTotal:    prevTotal,
		YoYPercent:       yoy,
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

	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return payload, err
	}
	var selectedSchoolID *int
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return payload, auth.ErrForbidden
		}
		schoolID := *filters.SchoolID
		selectedSchoolID = &schoolID
	}

	requiredTables := []string{"student_outcomes", "data_sekolah"}
	if statusFilter == "PT" {
		requiredTables = append(requiredTables, "class12_students", "outcome_pt_details", "outcome_pt_offers", "student_higher_ed_applications", "student_cohorts")
	}
	ok, missing, err := tablesExist(ctx, s.DB, requiredTables...)
	if err != nil {
		return payload, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "dashboard", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return payload, nil
	}

	legacyOK, _, legacyErr := tablesExist(ctx, s.DB, "student_cohorts", "student_final_outcomes")
	if legacyErr != nil {
		return payload, legacyErr
	}
	legacyOfferOK, _, legacyOfferErr := tablesExist(ctx, s.DB, "student_higher_ed_acceptance_offers")
	if legacyOfferErr != nil {
		return payload, legacyOfferErr
	}

	baseCTE := buildUnifiedFinalOutcomeCTE(legacyOK, legacyOfferOK)
	baseConditions := []string{"fo.finalized_at IS NOT NULL"}
	if statusFilter == "PT" {
		baseCTE = buildFinalPTReconciledCTE() + `,
			final_outcomes AS (
				SELECT id,
				       school_id,
				       school_name,
				       school_level,
				       school_status,
				       nisn,
				       nama_siswa,
				       rombel,
				       tahun_lulus,
				       final_status,
				       final_path,
				       source,
				       finalized_at,
				       pt_kampus,
				       pt_prodi,
				       pt_jalur,
				       pt_tahun_diterima,
				       ''::text AS kerja_perusahaan,
				       ''::text AS usaha_bidang
				FROM final_pt
			)
		`
		baseConditions = []string{}
	}
	baseArgs := make([]interface{}, 0)
	addClause := func(conditions *[]string, args *[]interface{}, clause string, value interface{}) {
		*conditions = append(*conditions, fmt.Sprintf(clause, len(*args)+1))
		*args = append(*args, value)
	}

	if statusFilter != "ALL" {
		addClause(&baseConditions, &baseArgs, "fo.final_status = $%d", statusFilter)
	}
	if selectedSchoolID != nil {
		addClause(&baseConditions, &baseArgs, "fo.school_id = $%d", *selectedSchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return payload, nil
		}
		addClause(&baseConditions, &baseArgs, "fo.school_id = ANY($%d)", scope.SchoolIDs)
	}
	if jenjangFilter != "" {
		addClause(&baseConditions, &baseArgs, "UPPER(TRIM(COALESCE(fo.school_level, ''))) = $%d", jenjangFilter)
	}
	if auth.IsAdmin(user) {
		statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if statusSekolah != "" && statusSekolah != "ALL" {
			addClause(&baseConditions, &baseArgs, "UPPER(TRIM(COALESCE(fo.school_status, ''))) = $%d", statusSekolah)
		}
	}

	aggregateAllYears := statusFilter == "PT" && filters.Year <= 0
	targetYear := filters.Year
	if !aggregateAllYears && targetYear <= 0 {
		maxQuery := baseCTE + `
            SELECT COALESCE(MAX(fo.tahun_lulus), 0)
            FROM final_outcomes fo
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
		if !aggregateAllYears {
			return payload, nil
		}
	}

	yearlyTotalsConditions := append([]string{}, baseConditions...)
	yearlyTotalsArgs := append([]interface{}{}, baseArgs...)
	if !aggregateAllYears {
		addClause(&yearlyTotalsConditions, &yearlyTotalsArgs, "fo.tahun_lulus <= $%d", targetYear)
	}
	yearlyTotalsQuery := baseCTE + `
        SELECT fo.tahun_lulus,
               COUNT(*) AS total
        FROM final_outcomes fo
    `
	yearlyTotalsWhere := ""
	if len(yearlyTotalsConditions) > 0 {
		yearlyTotalsWhere = " WHERE " + strings.Join(yearlyTotalsConditions, " AND ")
	}
	yearlyTotalsQuery += yearlyTotalsWhere + " GROUP BY fo.tahun_lulus ORDER BY fo.tahun_lulus ASC"

	yearlyTotalsRows, err := s.DB.Pool.Query(ctx, yearlyTotalsQuery, yearlyTotalsArgs...)
	if err != nil {
		return payload, err
	}
	defer yearlyTotalsRows.Close()

	yearlyTotals := make([]SiswaOutcomeDashboardPoint, 0)
	for yearlyTotalsRows.Next() {
		var point SiswaOutcomeDashboardPoint
		if scanErr := yearlyTotalsRows.Scan(&point.Year, &point.Total); scanErr != nil {
			return payload, scanErr
		}
		yearlyTotals = append(yearlyTotals, point)
	}
	if err := yearlyTotalsRows.Err(); err != nil {
		return payload, err
	}
	if len(yearlyTotals) == 0 {
		return payload, nil
	}

	summaryConditions := make([]string, 0, len(baseConditions)+1)
	summaryConditions = append(summaryConditions, baseConditions...)
	summaryArgs := append([]interface{}{}, baseArgs...)
	if !aggregateAllYears {
		addClause(&summaryConditions, &summaryArgs, "fo.tahun_lulus = $%d", targetYear)
	}
	summaryQuery := baseCTE + `
		SELECT
			COALESCE(SUM(CASE WHEN fo.final_status = 'PT' THEN 1 ELSE 0 END), 0) AS pt,
			COALESCE(SUM(CASE WHEN fo.final_status = 'KERJA' THEN 1 ELSE 0 END), 0) AS kerja,
			COALESCE(SUM(CASE WHEN fo.final_status = 'WIRAUSAHA' THEN 1 ELSE 0 END), 0) AS wirausaha,
			COALESCE(SUM(CASE WHEN fo.final_status = 'BELUM' THEN 1 ELSE 0 END), 0) AS belum
		FROM final_outcomes fo
	`
	if err := s.DB.Pool.QueryRow(
		ctx,
		summaryQuery+" WHERE "+strings.Join(summaryConditions, " AND "),
		summaryArgs...,
	).Scan(
		&payload.Summary.PT,
		&payload.Summary.Kerja,
		&payload.Summary.Wirausaha,
		&payload.Summary.Belum,
	); err != nil {
		return payload, err
	}
	payload.Total = payload.Summary.PT + payload.Summary.Kerja + payload.Summary.Wirausaha + payload.Summary.Belum

	payload.Kpis = buildOutcomeDashboardKpis(yearlyTotals, targetYear, payload.Total, aggregateAllYears)
	payload.TrendByYear = trimOutcomeDashboardTrend(yearlyTotals, 5)

	includePT := statusFilter == "ALL" || statusFilter == "PT"
	if !includePT {
		return payload, nil
	}

	ptConditions := append([]string{}, baseConditions...)
	ptArgs := append([]interface{}{}, baseArgs...)
	if statusFilter == "ALL" {
		addClause(&ptConditions, &ptArgs, "fo.final_status = $%d", "PT")
	}
	if !aggregateAllYears {
		addClause(&ptConditions, &ptArgs, "fo.tahun_lulus = $%d", targetYear)
	}
	ptWhere := " WHERE " + strings.Join(ptConditions, " AND ")

	var ptTotal int
	totalQuery := baseCTE + `
        SELECT COUNT(*)
        FROM final_outcomes fo
    ` + ptWhere
	if err := s.DB.Pool.QueryRow(ctx, totalQuery, ptArgs...).Scan(&ptTotal); err != nil {
		return payload, err
	}
	if ptTotal == 0 {
		return payload, nil
	}

	jalurCase := `
        CASE
          WHEN LOWER(COALESCE(fo.pt_jalur, '')) LIKE '%snbp%' THEN 'SNBP'
          WHEN LOWER(COALESCE(fo.pt_jalur, '')) LIKE '%snbt%' THEN 'SNBT'
          WHEN LOWER(COALESCE(fo.pt_jalur, '')) LIKE '%mandiri%' OR LOWER(COALESCE(fo.pt_jalur, '')) LIKE '%snbm%' THEN 'SNBM (Mandiri)'
          WHEN LOWER(COALESCE(fo.pt_jalur, '')) LIKE '%kedinas%' THEN 'Kedinasan'
          ELSE 'Lainnya'
        END
    `

	breakdownQuery := baseCTE + `
        SELECT ` + jalurCase + ` AS label,
               COUNT(*) AS total
        FROM final_outcomes fo
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

	topQuery := baseCTE + `
        SELECT COALESCE(NULLIF(TRIM(fo.%s), ''), '') AS label,
               COUNT(*) AS total
        FROM final_outcomes fo
    ` + ptWhere + `
        AND COALESCE(NULLIF(TRIM(fo.%s), ''), '') <> ''
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

	if items, err := fetchTop("pt_kampus"); err != nil {
		return payload, err
	} else {
		payload.TopKampus = items
	}
	if items, err := fetchTop("pt_prodi"); err != nil {
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
	scope auth.MultiSchoolScope,
	selectedSchoolID *int,
	jenjangFilter string,
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

	legacyOK, _, legacyErr := tablesExist(ctx, s.DB, "student_cohorts", "student_final_outcomes")
	if legacyErr != nil {
		return payload, legacyErr
	}
	legacyOfferOK, _, legacyOfferErr := tablesExist(ctx, s.DB, "student_higher_ed_acceptance_offers")
	if legacyOfferErr != nil {
		return payload, legacyOfferErr
	}
	legacyCTE := buildUnifiedFinalOutcomeCTE(legacyOK, legacyOfferOK)

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
	addLegacyClause := func(conditions *[]string, args *[]interface{}, clause string, value interface{}) {
		*conditions = append(*conditions, fmt.Sprintf(clause, len(*args)+1))
		*args = append(*args, value)
	}

	if selectedSchoolID != nil {
		addClause(&baseConditions, &baseArgs, "c.school_id = $%d", *selectedSchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return payload, nil
		}
		addClause(&baseConditions, &baseArgs, "c.school_id = ANY($%d)", scope.SchoolIDs)
	}
	if jenjangFilter != "" {
		addClause(&baseConditions, &baseArgs, "UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", jenjangFilter)
	}
	if auth.IsAdmin(user) {
		statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if statusSekolah != "" && statusSekolah != "ALL" {
			addClause(&baseConditions, &baseArgs, "UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $%d", statusSekolah)
		}
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
		if targetYear <= 0 && legacyOK {
			legacyConditions := []string{"fo.final_status = 'BELUM'"}
			legacyArgs := make([]interface{}, 0)
			if selectedSchoolID != nil {
				addLegacyClause(&legacyConditions, &legacyArgs, "fo.school_id = $%d", *selectedSchoolID)
			} else if !scope.All {
				if len(scope.SchoolIDs) == 0 {
					return payload, nil
				}
				addLegacyClause(&legacyConditions, &legacyArgs, "fo.school_id = ANY($%d)", scope.SchoolIDs)
			}
			if jenjangFilter != "" {
				addLegacyClause(&legacyConditions, &legacyArgs, "UPPER(TRIM(COALESCE(fo.school_level, ''))) = $%d", jenjangFilter)
			}
			if auth.IsAdmin(user) {
				statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
				if statusSekolah != "" && statusSekolah != "ALL" {
					addLegacyClause(&legacyConditions, &legacyArgs, "UPPER(TRIM(COALESCE(fo.school_status, ''))) = $%d", statusSekolah)
				}
			}
			legacyWhere := ""
			if len(legacyConditions) > 0 {
				legacyWhere = " WHERE " + strings.Join(legacyConditions, " AND ")
			}
			maxLegacyQuery := legacyCTE + `
				SELECT COALESCE(MAX(fo.tahun_lulus), 0)
				FROM final_outcomes fo
			` + legacyWhere
			if err := s.DB.Pool.QueryRow(ctx, maxLegacyQuery, legacyArgs...).Scan(&targetYear); err != nil {
				return payload, err
			}
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
		if legacyOK {
			legacyConditions := []string{"fo.final_status = 'BELUM'"}
			legacyArgs := make([]interface{}, 0)
			if selectedSchoolID != nil {
				addLegacyClause(&legacyConditions, &legacyArgs, "fo.school_id = $%d", *selectedSchoolID)
			} else if !scope.All {
				if len(scope.SchoolIDs) == 0 {
					return total, nil
				}
				addLegacyClause(&legacyConditions, &legacyArgs, "fo.school_id = ANY($%d)", scope.SchoolIDs)
			}
			if jenjangFilter != "" {
				addLegacyClause(&legacyConditions, &legacyArgs, "UPPER(TRIM(COALESCE(fo.school_level, ''))) = $%d", jenjangFilter)
			}
			if auth.IsAdmin(user) {
				statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
				if statusSekolah != "" && statusSekolah != "ALL" {
					addLegacyClause(&legacyConditions, &legacyArgs, "UPPER(TRIM(COALESCE(fo.school_status, ''))) = $%d", statusSekolah)
				}
			}
			addLegacyClause(&legacyConditions, &legacyArgs, "fo.tahun_lulus = $%d", year)
			legacyWhere := " WHERE " + strings.Join(legacyConditions, " AND ")
			legacyQuery := legacyCTE + `
				SELECT COUNT(*)
				FROM final_outcomes fo
			` + legacyWhere
			var legacyTotal int
			if err := s.DB.Pool.QueryRow(ctx, legacyQuery, legacyArgs...).Scan(&legacyTotal); err != nil {
				return 0, err
			}
			total += legacyTotal
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
	if legacyOK {
		legacyConditions := []string{"fo.final_status = 'BELUM'"}
		legacyArgs := make([]interface{}, 0)
		if selectedSchoolID != nil {
			addLegacyClause(&legacyConditions, &legacyArgs, "fo.school_id = $%d", *selectedSchoolID)
		} else if !scope.All {
			if len(scope.SchoolIDs) != 0 {
				addLegacyClause(&legacyConditions, &legacyArgs, "fo.school_id = ANY($%d)", scope.SchoolIDs)
			}
		}
		if jenjangFilter != "" {
			addLegacyClause(&legacyConditions, &legacyArgs, "UPPER(TRIM(COALESCE(fo.school_level, ''))) = $%d", jenjangFilter)
		}
		if auth.IsAdmin(user) {
			statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
			if statusSekolah != "" && statusSekolah != "ALL" {
				addLegacyClause(&legacyConditions, &legacyArgs, "UPPER(TRIM(COALESCE(fo.school_status, ''))) = $%d", statusSekolah)
			}
		}
		addLegacyClause(&legacyConditions, &legacyArgs, "fo.tahun_lulus <= $%d", targetYear)
		legacyTrendQuery := legacyCTE + `
			SELECT fo.tahun_lulus,
			       COUNT(*) AS total
			FROM final_outcomes fo
			WHERE ` + strings.Join(legacyConditions, " AND ") + `
			GROUP BY fo.tahun_lulus
			ORDER BY fo.tahun_lulus DESC
			LIMIT 5
		`
		legacyTrendRows, err := s.DB.Pool.Query(ctx, legacyTrendQuery, legacyArgs...)
		if err != nil {
			return payload, err
		}
		defer legacyTrendRows.Close()

		trendMap := make(map[int]int, len(trend))
		for _, point := range trend {
			trendMap[point.Year] = point.Total
		}
		for legacyTrendRows.Next() {
			var year int
			var total int
			if scanErr := legacyTrendRows.Scan(&year, &total); scanErr != nil {
				return payload, scanErr
			}
			trendMap[year] += total
		}

		merged := make([]SiswaOutcomeDashboardPoint, 0, len(trendMap))
		for year, total := range trendMap {
			merged = append(merged, SiswaOutcomeDashboardPoint{Year: year, Total: total})
		}
		sort.Slice(merged, func(i, j int) bool { return merged[i].Year < merged[j].Year })
		if len(merged) > 5 {
			merged = merged[len(merged)-5:]
		}
		trend = merged
	}
	payload.TrendByYear = trend

	return payload, nil
}
