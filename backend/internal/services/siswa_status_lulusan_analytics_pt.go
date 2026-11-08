package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"github.com/jackc/pgx/v5"
)

type StatusLulusanPTAnalyticsFilters struct {
	StartYear int
	EndYear   int
	Jenjang   string
	SchoolID  *int
	Search    string
	Page      int
	PageSize  int
}

type StatusLulusanPTAnalyticsSeriesItem struct {
	TahunLulus     int      `json:"tahun_lulus"`
	Jalur          string   `json:"jalur"`
	Pendaftar      int      `json:"pendaftar"`
	Diterima       int      `json:"diterima"`
	AcceptanceRate float64  `json:"acceptance_rate"`
	YoYPendaftar   *float64 `json:"yoy_pendaftar"`
	YoYDiterima    *float64 `json:"yoy_diterima"`
}

type StatusLulusanPTAnalyticsTotalByYear struct {
	TahunLulus     int      `json:"tahun_lulus"`
	Pendaftar      int      `json:"pendaftar"`
	Diterima       int      `json:"diterima"`
	AcceptanceRate float64  `json:"acceptance_rate"`
	YoYPendaftar   *float64 `json:"yoy_pendaftar"`
	YoYDiterima    *float64 `json:"yoy_diterima"`
}

type StatusLulusanPTAnalyticsDistributionItem struct {
	Label        string  `json:"label"`
	Total        int     `json:"total"`
	SharePercent float64 `json:"share_percent"`
}

type StatusLulusanPTAnalyticsDetailItem struct {
	SchoolName    string `json:"school_name"`
	NISN          string `json:"nisn"`
	NamaSiswa     string `json:"nama_siswa"`
	Rombel        string `json:"rombel"`
	TahunLulus    int    `json:"tahun_lulus"`
	Jalur         string `json:"jalur"`
	NamaPT        string `json:"nama_pt"`
	Prodi         string `json:"prodi"`
	TahunDiterima *int   `json:"tahun_diterima,omitempty"`
}

type StatusLulusanPTAnalyticsDetailMeta struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type StatusLulusanPTAnalyticsResponse struct {
	StartYear    int                                        `json:"start_year"`
	EndYear      int                                        `json:"end_year"`
	Jenjang      string                                     `json:"jenjang"`
	Series       []StatusLulusanPTAnalyticsSeriesItem       `json:"series"`
	TotalsByYear []StatusLulusanPTAnalyticsTotalByYear      `json:"totals_by_year"`
	TopKampus    []StatusLulusanPTAnalyticsDistributionItem `json:"top_kampus"`
	TopProdi     []StatusLulusanPTAnalyticsDistributionItem `json:"top_prodi"`
	DetailRows   []StatusLulusanPTAnalyticsDetailItem       `json:"detail_rows"`
	DetailMeta   StatusLulusanPTAnalyticsDetailMeta         `json:"detail_meta"`
}

type statusLulusanPTAnalyticsScope struct {
	Role          string
	SchoolID      *int
	SchoolIDs     []int
	AllowedLevels []string
	JenjangFilter string
	DenyAll       bool
}

type statusLulusanPTAnalyticsKey struct {
	TahunLulus int
	Jalur      string
}

type statusLulusanPTAnalyticsCounts struct {
	Pendaftar int
	Diterima  int
}

type statusLulusanPTPrevCounts struct {
	Pendaftar int
	Diterima  int
	HasValue  bool
}

func normalizeStatusLulusanAnalyticsJenjang(raw string) (string, bool) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return "", true
	}
	switch value {
	case "SMA", "SMK", "SLB":
		return value, true
	default:
		return "", false
	}
}

func normalizeStatusLulusanAnalyticsJalur(raw string) string {
	upper := strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case strings.Contains(upper, "SNBP"):
		return "SNBP"
	case strings.Contains(upper, "SNBT"):
		return "SNBT"
	case strings.Contains(upper, "MANDIRI"), strings.Contains(upper, "SNBM"):
		return "MANDIRI"
	case strings.Contains(upper, "KEDINAS"):
		return "KEDINASAN"
	default:
		return "LAINNYA"
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func statusLulusanPTAnalyticsIsNoRowsError(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

func normalizePositiveSchoolIDs(raw []int) []int {
	if len(raw) == 0 {
		return []int{}
	}
	seen := make(map[int]struct{}, len(raw))
	result := make([]int, 0, len(raw))
	for _, schoolID := range raw {
		if schoolID <= 0 {
			continue
		}
		if _, ok := seen[schoolID]; ok {
			continue
		}
		seen[schoolID] = struct{}{}
		result = append(result, schoolID)
	}
	return result
}

func containsSchoolID(ids []int, target int) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func resolveStatusLulusanPTAnalyticsYearRange(startYear int, endYear int) (int, int, error) {
	nowYear := time.Now().Year()
	if endYear <= 0 {
		endYear = nowYear
	}
	if startYear <= 0 {
		startYear = endYear - 4
	}
	if startYear > endYear {
		return 0, 0, ErrInvalidOutcome
	}
	return startYear, endYear, nil
}

func statusLulusanPTAnalyticsJenjangLabel(jenjangFilter string) string {
	jenjang := strings.TrimSpace(strings.ToUpper(jenjangFilter))
	if jenjang == "" {
		return "ALL"
	}
	return jenjang
}

func normalizeStatusLulusanPTAnalyticsPagination(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func buildStatusLulusanPTAnalyticsEmpty(startYear int, endYear int, jenjangFilter string) StatusLulusanPTAnalyticsResponse {
	page, pageSize := normalizeStatusLulusanPTAnalyticsPagination(1, 10)
	return StatusLulusanPTAnalyticsResponse{
		StartYear:    startYear,
		EndYear:      endYear,
		Jenjang:      statusLulusanPTAnalyticsJenjangLabel(jenjangFilter),
		Series:       []StatusLulusanPTAnalyticsSeriesItem{},
		TotalsByYear: []StatusLulusanPTAnalyticsTotalByYear{},
		TopKampus:    []StatusLulusanPTAnalyticsDistributionItem{},
		TopProdi:     []StatusLulusanPTAnalyticsDistributionItem{},
		DetailRows:   []StatusLulusanPTAnalyticsDetailItem{},
		DetailMeta: StatusLulusanPTAnalyticsDetailMeta{
			Page:     page,
			PageSize: pageSize,
			Total:    0,
		},
	}
}

func buildStatusLulusanPTAnalyticsScopeForUser(user auth.User, filters StatusLulusanPTAnalyticsFilters) (statusLulusanPTAnalyticsScope, error) {
	scope := statusLulusanPTAnalyticsScope{Role: strings.TrimSpace(user.Role)}

	requestedJenjang, ok := normalizeStatusLulusanAnalyticsJenjang(filters.Jenjang)
	if !ok {
		return scope, ErrInvalidOutcome
	}

	switch user.Role {
	case "SCHOOL_ADMIN":
		if user.SchoolID == nil {
			return scope, auth.ErrForbidden
		}
		scope.SchoolID = user.SchoolID
	case "ADMIN":
		scope.AllowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(scope.AllowedLevels) == 0 {
			if requestedJenjang != "" {
				return scope, auth.ErrForbidden
			}
			scope.DenyAll = true
			return scope, nil
		}
		if requestedJenjang != "" && !containsString(scope.AllowedLevels, requestedJenjang) {
			return scope, auth.ErrForbidden
		}
		scope.JenjangFilter = requestedJenjang
	case "ADMIN_MASTER":
		scope.JenjangFilter = requestedJenjang
	case "PENGAWAS":
		scope.JenjangFilter = requestedJenjang
		supervisedSchoolIDs := normalizePositiveSchoolIDs(user.SupervisedSchoolIDs)
		if len(supervisedSchoolIDs) == 0 {
			scope.DenyAll = true
			return scope, nil
		}
		if filters.SchoolID != nil {
			if *filters.SchoolID <= 0 || !containsSchoolID(supervisedSchoolIDs, *filters.SchoolID) {
				return scope, auth.ErrForbidden
			}
			scope.SchoolIDs = []int{*filters.SchoolID}
			return scope, nil
		}
		scope.SchoolIDs = supervisedSchoolIDs
	default:
		return scope, auth.ErrForbidden
	}

	return scope, nil
}

func buildStatusLulusanPTAnalyticsScopeForPublic(filters StatusLulusanPTAnalyticsFilters) (statusLulusanPTAnalyticsScope, error) {
	scope := statusLulusanPTAnalyticsScope{Role: "PUBLIC"}
	jenjang, ok := normalizeStatusLulusanAnalyticsJenjang(filters.Jenjang)
	if !ok {
		return scope, ErrInvalidOutcome
	}
	scope.JenjangFilter = jenjang
	return scope, nil
}

func compactStatusLulusanPTAnalyticsQuery(query string) string {
	return strings.Join(strings.Fields(query), " ")
}

func statusLulusanPTAnalyticsScopeSchoolID(scope statusLulusanPTAnalyticsScope) any {
	if scope.SchoolID != nil {
		return *scope.SchoolID
	}
	if len(scope.SchoolIDs) == 1 {
		return scope.SchoolIDs[0]
	}
	return nil
}

func logStatusLulusanPTAnalyticsDBError(
	source string,
	err error,
	filters StatusLulusanPTAnalyticsFilters,
	scope statusLulusanPTAnalyticsScope,
	query string,
) {
	logging.Errorf(
		"status_lulusan_analytics_pt",
		"analytics_pt",
		"event=db_query_failed source=%s err=%v start_year=%d end_year=%d jenjang=%s role=%s school_id=%v school_ids=%v allowed_levels=%v query=%q",
		source,
		err,
		filters.StartYear,
		filters.EndYear,
		statusLulusanPTAnalyticsJenjangLabel(filters.Jenjang),
		strings.ToUpper(strings.TrimSpace(scope.Role)),
		statusLulusanPTAnalyticsScopeSchoolID(scope),
		scope.SchoolIDs,
		scope.AllowedLevels,
		compactStatusLulusanPTAnalyticsQuery(query),
	)
}

func (s SiswaService) GetStatusLulusanPTAnalytics(
	ctx context.Context,
	user auth.User,
	filters StatusLulusanPTAnalyticsFilters,
) (StatusLulusanPTAnalyticsResponse, error) {
	scope, err := buildStatusLulusanPTAnalyticsScopeForUser(user, filters)
	if err != nil {
		return StatusLulusanPTAnalyticsResponse{}, err
	}
	return s.getStatusLulusanPTAnalytics(ctx, filters, scope)
}

func (s SiswaService) GetPublicStatusLulusanPTAnalytics(
	ctx context.Context,
	filters StatusLulusanPTAnalyticsFilters,
) (StatusLulusanPTAnalyticsResponse, error) {
	scope, err := buildStatusLulusanPTAnalyticsScopeForPublic(filters)
	if err != nil {
		return StatusLulusanPTAnalyticsResponse{}, err
	}
	return s.getStatusLulusanPTAnalytics(ctx, filters, scope)
}

func (s SiswaService) getStatusLulusanPTAnalytics(
	ctx context.Context,
	filters StatusLulusanPTAnalyticsFilters,
	scope statusLulusanPTAnalyticsScope,
) (StatusLulusanPTAnalyticsResponse, error) {
	startYear, endYear, yearErr := resolveStatusLulusanPTAnalyticsYearRange(filters.StartYear, filters.EndYear)
	if yearErr != nil {
		return StatusLulusanPTAnalyticsResponse{}, yearErr
	}
	filters.StartYear = startYear
	filters.EndYear = endYear
	filters.Page, filters.PageSize = normalizeStatusLulusanPTAnalyticsPagination(filters.Page, filters.PageSize)

	if scope.DenyAll {
		payload := buildStatusLulusanPTAnalyticsEmpty(filters.StartYear, filters.EndYear, scope.JenjangFilter)
		payload.DetailMeta = StatusLulusanPTAnalyticsDetailMeta{
			Page:     filters.Page,
			PageSize: filters.PageSize,
			Total:    0,
		}
		return payload, nil
	}

	pendaftarByKey, pErr := s.fetchStatusLulusanPTPendaftar(ctx, filters, scope)
	if pErr != nil {
		if statusLulusanPTAnalyticsIsNoRowsError(pErr) {
			pendaftarByKey = map[statusLulusanPTAnalyticsKey]int{}
		} else {
			return StatusLulusanPTAnalyticsResponse{}, pErr
		}
	}
	diterimaByKey, dErr := s.fetchStatusLulusanPTDiterima(ctx, filters, scope)
	if dErr != nil {
		if statusLulusanPTAnalyticsIsNoRowsError(dErr) {
			diterimaByKey = map[statusLulusanPTAnalyticsKey]int{}
		} else {
			return StatusLulusanPTAnalyticsResponse{}, dErr
		}
	}

	combined := make(map[statusLulusanPTAnalyticsKey]statusLulusanPTAnalyticsCounts)
	for key, total := range pendaftarByKey {
		counts := combined[key]
		counts.Pendaftar = total
		combined[key] = counts
	}
	for key, total := range diterimaByKey {
		counts := combined[key]
		counts.Diterima = total
		combined[key] = counts
	}

	if len(combined) == 0 {
		payload := buildStatusLulusanPTAnalyticsEmpty(filters.StartYear, filters.EndYear, scope.JenjangFilter)
		payload.DetailMeta = StatusLulusanPTAnalyticsDetailMeta{
			Page:     filters.Page,
			PageSize: filters.PageSize,
			Total:    0,
		}
		return payload, nil
	}

	jalurOrder := map[string]int{
		"SNBP":      1,
		"SNBT":      2,
		"MANDIRI":   3,
		"KEDINASAN": 4,
		"LAINNYA":   5,
	}
	keys := make([]statusLulusanPTAnalyticsKey, 0, len(combined))
	for key := range combined {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].TahunLulus != keys[j].TahunLulus {
			return keys[i].TahunLulus < keys[j].TahunLulus
		}
		left := jalurOrder[keys[i].Jalur]
		right := jalurOrder[keys[j].Jalur]
		if left != right {
			return left < right
		}
		return keys[i].Jalur < keys[j].Jalur
	})

	series := make([]StatusLulusanPTAnalyticsSeriesItem, 0, len(keys))
	prevByJalur := map[string]statusLulusanPTPrevCounts{}
	totalsByYearMap := map[int]statusLulusanPTAnalyticsCounts{}

	for _, key := range keys {
		counts := combined[key]
		rate := 0.0
		if counts.Pendaftar > 0 {
			rate = round2((float64(counts.Diterima) / float64(counts.Pendaftar)) * 100)
		}

		var yoyPendaftar *float64
		var yoyDiterima *float64
		if prev := prevByJalur[key.Jalur]; prev.HasValue {
			if prev.Pendaftar > 0 {
				value := round2((float64(counts.Pendaftar-prev.Pendaftar) / float64(prev.Pendaftar)) * 100)
				yoyPendaftar = &value
			}
			if prev.Diterima > 0 {
				value := round2((float64(counts.Diterima-prev.Diterima) / float64(prev.Diterima)) * 100)
				yoyDiterima = &value
			}
		}
		prevByJalur[key.Jalur] = statusLulusanPTPrevCounts{
			Pendaftar: counts.Pendaftar,
			Diterima:  counts.Diterima,
			HasValue:  true,
		}

		series = append(series, StatusLulusanPTAnalyticsSeriesItem{
			TahunLulus:     key.TahunLulus,
			Jalur:          key.Jalur,
			Pendaftar:      counts.Pendaftar,
			Diterima:       counts.Diterima,
			AcceptanceRate: rate,
			YoYPendaftar:   yoyPendaftar,
			YoYDiterima:    yoyDiterima,
		})

		yearTotals := totalsByYearMap[key.TahunLulus]
		yearTotals.Pendaftar += counts.Pendaftar
		yearTotals.Diterima += counts.Diterima
		totalsByYearMap[key.TahunLulus] = yearTotals
	}

	years := make([]int, 0, len(totalsByYearMap))
	for year := range totalsByYearMap {
		years = append(years, year)
	}
	sort.Ints(years)

	totalsByYear := make([]StatusLulusanPTAnalyticsTotalByYear, 0, len(years))
	var prevTotal statusLulusanPTPrevCounts
	for _, year := range years {
		counts := totalsByYearMap[year]
		rate := 0.0
		if counts.Pendaftar > 0 {
			rate = round2((float64(counts.Diterima) / float64(counts.Pendaftar)) * 100)
		}

		var yoyPendaftar *float64
		var yoyDiterima *float64
		if prevTotal.HasValue {
			if prevTotal.Pendaftar > 0 {
				value := round2((float64(counts.Pendaftar-prevTotal.Pendaftar) / float64(prevTotal.Pendaftar)) * 100)
				yoyPendaftar = &value
			}
			if prevTotal.Diterima > 0 {
				value := round2((float64(counts.Diterima-prevTotal.Diterima) / float64(prevTotal.Diterima)) * 100)
				yoyDiterima = &value
			}
		}
		prevTotal = statusLulusanPTPrevCounts{
			Pendaftar: counts.Pendaftar,
			Diterima:  counts.Diterima,
			HasValue:  true,
		}

		totalsByYear = append(totalsByYear, StatusLulusanPTAnalyticsTotalByYear{
			TahunLulus:     year,
			Pendaftar:      counts.Pendaftar,
			Diterima:       counts.Diterima,
			AcceptanceRate: rate,
			YoYPendaftar:   yoyPendaftar,
			YoYDiterima:    yoyDiterima,
		})
	}

	totalAcceptedRange := 0
	for _, row := range totalsByYear {
		totalAcceptedRange += row.Diterima
	}

	topKampus, topProdi, detailRows, detailMeta, insightsErr := s.fetchStatusLulusanPTAcceptedInsights(ctx, filters, scope, totalAcceptedRange)
	if insightsErr != nil {
		if statusLulusanPTAnalyticsIsNoRowsError(insightsErr) {
			topKampus = []StatusLulusanPTAnalyticsDistributionItem{}
			topProdi = []StatusLulusanPTAnalyticsDistributionItem{}
			detailRows = []StatusLulusanPTAnalyticsDetailItem{}
			detailMeta = StatusLulusanPTAnalyticsDetailMeta{
				Page:     filters.Page,
				PageSize: filters.PageSize,
				Total:    0,
			}
		} else {
			return StatusLulusanPTAnalyticsResponse{}, insightsErr
		}
	}

	return StatusLulusanPTAnalyticsResponse{
		StartYear:    startYear,
		EndYear:      endYear,
		Jenjang:      statusLulusanPTAnalyticsJenjangLabel(scope.JenjangFilter),
		Series:       series,
		TotalsByYear: totalsByYear,
		TopKampus:    topKampus,
		TopProdi:     topProdi,
		DetailRows:   detailRows,
		DetailMeta:   detailMeta,
	}, nil
}

func (s SiswaService) fetchStatusLulusanPTPendaftar(
	ctx context.Context,
	filters StatusLulusanPTAnalyticsFilters,
	scope statusLulusanPTAnalyticsScope,
) (map[statusLulusanPTAnalyticsKey]int, error) {
	result := map[statusLulusanPTAnalyticsKey]int{}
	requiresSchoolJoin := scope.JenjangFilter != "" || len(scope.AllowedLevels) > 0
	requiredTables := []string{"student_plans", "student_higher_ed_applications", "student_cohorts", "outcome_pt_offers"}
	if requiresSchoolJoin {
		requiredTables = append(requiredTables, "data_sekolah")
	}
	ok, missing, err := tablesExist(ctx, s.DB, requiredTables...)
	if err != nil {
		logStatusLulusanPTAnalyticsDBError("pendaftar.tables_exist", err, filters, scope, "")
		return nil, err
	}
	if !ok {
		logging.Warnf("status_lulusan_analytics_pt", "pendaftar", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return result, nil
	}

	conditions := []string{"d.tahun_lulus > 0"}
	args := make([]interface{}, 0)
	addClause := func(condition string, value interface{}) {
		conditions = append(conditions, fmt.Sprintf(condition, len(args)+1))
		args = append(args, value)
	}

	if filters.StartYear > 0 {
		addClause("d.tahun_lulus >= $%d", filters.StartYear)
	}
	if filters.EndYear > 0 {
		addClause("d.tahun_lulus <= $%d", filters.EndYear)
	}
	if len(scope.SchoolIDs) > 0 {
		addClause("d.school_id = ANY($%d)", scope.SchoolIDs)
	} else if scope.SchoolID != nil {
		addClause("d.school_id = $%d", *scope.SchoolID)
	}
	if requiresSchoolJoin {
		if scope.JenjangFilter != "" {
			addClause("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", scope.JenjangFilter)
		} else if len(scope.AllowedLevels) > 0 {
			addClause("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", scope.AllowedLevels)
		}
	}

	jalurExpr := `
		CASE
			WHEN UPPER(TRIM(COALESCE(u.jalur, ''))) LIKE '%SNBP%' THEN 'SNBP'
			WHEN UPPER(TRIM(COALESCE(u.jalur, ''))) LIKE '%SNBT%' THEN 'SNBT'
			WHEN UPPER(TRIM(COALESCE(u.jalur, ''))) LIKE '%MANDIRI%'
			  OR UPPER(TRIM(COALESCE(u.jalur, ''))) LIKE '%SNBM%' THEN 'MANDIRI'
			WHEN UPPER(TRIM(COALESCE(u.jalur, ''))) LIKE '%KEDINAS%' THEN 'KEDINASAN'
			ELSE 'LAINNYA'
		END
	`
	joinSekolah := ""
	if requiresSchoolJoin {
		joinSekolah = "\n\t\tJOIN data_sekolah ds ON ds.id = d.school_id"
	}
	query := `
		WITH plans AS (
			SELECT p.school_id,
			       p.tahun_lulus,
			       'NISN:' || TRIM(p.nisn) AS identity_key,
			       unnest(p.plan_jalur)::text AS jalur
			FROM student_plans p
			WHERE p.plan_source = 'MANUAL'
		),
		legacy AS (
			SELECT a.school_id,
			       sc.tahun_lulus,
			       'NISN:' || TRIM(sc.nisn) AS identity_key,
			       a.jalur_penerimaan::text AS jalur
			FROM student_higher_ed_applications a
			JOIN student_cohorts sc
			  ON sc.id = a.cohort_id
			 AND sc.school_id = a.school_id
		),
		offers AS (
			SELECT offer.school_id,
			       offer.tahun_lulus,
			       COALESCE(NULLIF('NISN:' || TRIM(COALESCE(offer.nisn, '')), 'NISN:'),
			                'NAME:' || offer.normalized_student_name) AS identity_key,
			       offer.jalur::text AS jalur
			FROM outcome_pt_offers offer
			WHERE offer.deleted_at IS NULL
		),
		unioned AS (
			SELECT * FROM plans
			UNION ALL
			SELECT * FROM legacy
			UNION ALL
			SELECT * FROM offers
		),
		normalized AS (
			SELECT u.school_id,
			       u.tahun_lulus,
			       u.identity_key,
			       ` + jalurExpr + ` AS jalur
			FROM unioned u
		),
		deduped AS (
			SELECT DISTINCT school_id,
			                tahun_lulus,
			                identity_key,
			                jalur
			FROM normalized
		)
		SELECT d.tahun_lulus,
		       d.jalur,
		       COUNT(*) AS total
		FROM deduped d` + joinSekolah + `
		WHERE ` + strings.Join(conditions, " AND ") + `
		GROUP BY d.tahun_lulus, d.jalur
	`

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		logStatusLulusanPTAnalyticsDBError("pendaftar.query", err, filters, scope, query)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var year int
		var jalur string
		var total int
		if scanErr := rows.Scan(&year, &jalur, &total); scanErr != nil {
			logStatusLulusanPTAnalyticsDBError("pendaftar.scan", scanErr, filters, scope, query)
			return nil, scanErr
		}
		key := statusLulusanPTAnalyticsKey{
			TahunLulus: year,
			Jalur:      normalizeStatusLulusanAnalyticsJalur(jalur),
		}
		result[key] = total
	}
	if err := rows.Err(); err != nil {
		logStatusLulusanPTAnalyticsDBError("pendaftar.rows", err, filters, scope, query)
		return nil, err
	}
	return result, nil
}

func (s SiswaService) fetchStatusLulusanPTDiterima(
	ctx context.Context,
	filters StatusLulusanPTAnalyticsFilters,
	scope statusLulusanPTAnalyticsScope,
) (map[statusLulusanPTAnalyticsKey]int, error) {
	result := map[statusLulusanPTAnalyticsKey]int{}
	{
		requiredTables := []string{"student_outcomes", "data_sekolah", "class12_students", "outcome_pt_details", "outcome_pt_offers", "student_higher_ed_applications", "student_cohorts"}
		ok, missing, err := tablesExist(ctx, s.DB, requiredTables...)
		if err != nil {
			logStatusLulusanPTAnalyticsDBError("diterima.tables_exist", err, filters, scope, "")
			return nil, err
		}
		if !ok {
			logging.Warnf("status_lulusan_analytics_pt", "diterima", "event=missing_tables tables=%s", strings.Join(missing, ","))
			return result, nil
		}

		conditions := []string{"src.tahun_lulus > 0", "src.final_status = 'PT'"}
		args := make([]interface{}, 0)
		addClause := func(condition string, value interface{}) {
			conditions = append(conditions, fmt.Sprintf(condition, len(args)+1))
			args = append(args, value)
		}

		if filters.StartYear > 0 {
			addClause("src.tahun_lulus >= $%d", filters.StartYear)
		}
		if filters.EndYear > 0 {
			addClause("src.tahun_lulus <= $%d", filters.EndYear)
		}
		if len(scope.SchoolIDs) > 0 {
			addClause("src.school_id = ANY($%d)", scope.SchoolIDs)
		} else if scope.SchoolID != nil {
			addClause("src.school_id = $%d", *scope.SchoolID)
		}
		if scope.JenjangFilter != "" {
			addClause("UPPER(TRIM(COALESCE(src.school_level, ''))) = $%d", scope.JenjangFilter)
		} else if len(scope.AllowedLevels) > 0 {
			addClause("UPPER(TRIM(COALESCE(src.school_level, ''))) = ANY($%d)", scope.AllowedLevels)
		}

		jalurExpr := `
			CASE
				WHEN UPPER(TRIM(COALESCE(src.pt_jalur, src.final_path, ''))) LIKE '%SNBP%' THEN 'SNBP'
				WHEN UPPER(TRIM(COALESCE(src.pt_jalur, src.final_path, ''))) LIKE '%SNBT%' THEN 'SNBT'
				WHEN UPPER(TRIM(COALESCE(src.pt_jalur, src.final_path, ''))) LIKE '%MANDIRI%'
				  OR UPPER(TRIM(COALESCE(src.pt_jalur, src.final_path, ''))) LIKE '%SNBM%' THEN 'MANDIRI'
				WHEN UPPER(TRIM(COALESCE(src.pt_jalur, src.final_path, ''))) LIKE '%KEDINAS%' THEN 'KEDINASAN'
				ELSE 'LAINNYA'
			END
		`
		query := buildFinalPTReconciledCTE() + `
			SELECT src.tahun_lulus,
			       ` + jalurExpr + ` AS jalur,
			       COUNT(*) AS total
			FROM final_pt src
			WHERE ` + strings.Join(conditions, " AND ") + `
			GROUP BY src.tahun_lulus, jalur
		`

		rows, err := s.DB.Pool.Query(ctx, query, args...)
		if err != nil {
			logStatusLulusanPTAnalyticsDBError("diterima.query", err, filters, scope, query)
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var year int
			var jalur string
			var total int
			if scanErr := rows.Scan(&year, &jalur, &total); scanErr != nil {
				logStatusLulusanPTAnalyticsDBError("diterima.scan", scanErr, filters, scope, query)
				return nil, scanErr
			}
			key := statusLulusanPTAnalyticsKey{
				TahunLulus: year,
				Jalur:      normalizeStatusLulusanAnalyticsJalur(jalur),
			}
			result[key] = total
		}
		if err := rows.Err(); err != nil {
			logStatusLulusanPTAnalyticsDBError("diterima.rows", err, filters, scope, query)
			return nil, err
		}
		return result, nil
	}
}

func (s SiswaService) fetchStatusLulusanPTAcceptedInsights(
	ctx context.Context,
	filters StatusLulusanPTAnalyticsFilters,
	scope statusLulusanPTAnalyticsScope,
	totalAcceptedRange int,
) (
	[]StatusLulusanPTAnalyticsDistributionItem,
	[]StatusLulusanPTAnalyticsDistributionItem,
	[]StatusLulusanPTAnalyticsDetailItem,
	StatusLulusanPTAnalyticsDetailMeta,
	error,
) {
	page, pageSize := normalizeStatusLulusanPTAnalyticsPagination(filters.Page, filters.PageSize)
	emptyMeta := StatusLulusanPTAnalyticsDetailMeta{
		Page:     page,
		PageSize: pageSize,
		Total:    0,
	}

	requiredTables := []string{"student_outcomes", "data_sekolah", "class12_students", "outcome_pt_details", "outcome_pt_offers", "student_higher_ed_applications", "student_cohorts"}
	ok, missing, err := tablesExist(ctx, s.DB, requiredTables...)
	if err != nil {
		logStatusLulusanPTAnalyticsDBError("accepted_insights.tables_exist", err, filters, scope, "")
		return nil, nil, nil, emptyMeta, err
	}
	if !ok {
		logging.Warnf("status_lulusan_analytics_pt", "accepted_insights", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []StatusLulusanPTAnalyticsDistributionItem{}, []StatusLulusanPTAnalyticsDistributionItem{}, []StatusLulusanPTAnalyticsDetailItem{}, emptyMeta, nil
	}

	conditions := []string{"x.tahun_lulus > 0", "x.final_status = 'PT'"}
	args := make([]interface{}, 0)
	addClause := func(condition string, value interface{}) {
		conditions = append(conditions, fmt.Sprintf(condition, len(args)+1))
		args = append(args, value)
	}

	if filters.StartYear > 0 {
		addClause("x.tahun_lulus >= $%d", filters.StartYear)
	}
	if filters.EndYear > 0 {
		addClause("x.tahun_lulus <= $%d", filters.EndYear)
	}
	if len(scope.SchoolIDs) > 0 {
		addClause("x.school_id = ANY($%d)", scope.SchoolIDs)
	} else if scope.SchoolID != nil {
		addClause("x.school_id = $%d", *scope.SchoolID)
	}
	if scope.JenjangFilter != "" {
		addClause("UPPER(TRIM(COALESCE(x.school_level, ''))) = $%d", scope.JenjangFilter)
	} else if len(scope.AllowedLevels) > 0 {
		addClause("UPPER(TRIM(COALESCE(x.school_level, ''))) = ANY($%d)", scope.AllowedLevels)
	}

	kampusExpr := "COALESCE(x.pt_kampus, '')"
	prodiExpr := "COALESCE(x.pt_prodi, '')"
	jalurSource := "COALESCE(x.pt_jalur, x.final_path, '')"
	tahunDiterimaExpr := "x.pt_tahun_diterima"
	namaSiswaExpr := "COALESCE(x.nama_siswa, '')"
	rombelExpr := "COALESCE(x.rombel, '')"
	jalurExpr := `
		CASE
			WHEN UPPER(TRIM(` + jalurSource + `)) LIKE '%SNBP%' THEN 'SNBP'
			WHEN UPPER(TRIM(` + jalurSource + `)) LIKE '%SNBT%' THEN 'SNBT'
			WHEN UPPER(TRIM(` + jalurSource + `)) LIKE '%MANDIRI%'
			  OR UPPER(TRIM(` + jalurSource + `)) LIKE '%SNBM%' THEN 'MANDIRI'
			WHEN UPPER(TRIM(` + jalurSource + `)) LIKE '%KEDINAS%' THEN 'KEDINASAN'
			ELSE 'LAINNYA'
		END
	`

	baseQuery := `
		FROM final_pt x
		WHERE ` + strings.Join(conditions, " AND ")

	topKampus := []StatusLulusanPTAnalyticsDistributionItem{}
	if totalAcceptedRange > 0 {
		query := buildFinalPTReconciledCTE() + `
			SELECT label, total
			FROM (
				SELECT TRIM(` + kampusExpr + `) AS label,
				       COUNT(*)::int AS total
				` + baseQuery + `
				AND TRIM(` + kampusExpr + `) <> ''
				GROUP BY label
			) ranked
			ORDER BY total DESC, label ASC
			LIMIT 10
		`
		rows, err := s.DB.Pool.Query(ctx, query, args...)
		if err != nil {
			logStatusLulusanPTAnalyticsDBError("accepted_insights.top_kampus", err, filters, scope, query)
			return nil, nil, nil, emptyMeta, err
		}
		defer rows.Close()
		for rows.Next() {
			var item StatusLulusanPTAnalyticsDistributionItem
			if scanErr := rows.Scan(&item.Label, &item.Total); scanErr != nil {
				logStatusLulusanPTAnalyticsDBError("accepted_insights.top_kampus_scan", scanErr, filters, scope, query)
				return nil, nil, nil, emptyMeta, scanErr
			}
			item.SharePercent = round2((float64(item.Total) / float64(totalAcceptedRange)) * 100)
			topKampus = append(topKampus, item)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			logStatusLulusanPTAnalyticsDBError("accepted_insights.top_kampus_rows", rowsErr, filters, scope, query)
			return nil, nil, nil, emptyMeta, rowsErr
		}
	}

	topProdi := []StatusLulusanPTAnalyticsDistributionItem{}
	if totalAcceptedRange > 0 {
		query := buildFinalPTReconciledCTE() + `
			SELECT label, total
			FROM (
				SELECT TRIM(` + prodiExpr + `) AS label,
				       COUNT(*)::int AS total
				` + baseQuery + `
				AND TRIM(` + prodiExpr + `) <> ''
				GROUP BY label
			) ranked
			ORDER BY total DESC, label ASC
			LIMIT 10
		`
		rows, err := s.DB.Pool.Query(ctx, query, args...)
		if err != nil {
			logStatusLulusanPTAnalyticsDBError("accepted_insights.top_prodi", err, filters, scope, query)
			return nil, nil, nil, emptyMeta, err
		}
		defer rows.Close()
		for rows.Next() {
			var item StatusLulusanPTAnalyticsDistributionItem
			if scanErr := rows.Scan(&item.Label, &item.Total); scanErr != nil {
				logStatusLulusanPTAnalyticsDBError("accepted_insights.top_prodi_scan", scanErr, filters, scope, query)
				return nil, nil, nil, emptyMeta, scanErr
			}
			item.SharePercent = round2((float64(item.Total) / float64(totalAcceptedRange)) * 100)
			topProdi = append(topProdi, item)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			logStatusLulusanPTAnalyticsDBError("accepted_insights.top_prodi_rows", rowsErr, filters, scope, query)
			return nil, nil, nil, emptyMeta, rowsErr
		}
	}

	if strings.EqualFold(strings.TrimSpace(scope.Role), "PUBLIC") {
		return topKampus, topProdi, []StatusLulusanPTAnalyticsDetailItem{}, emptyMeta, nil
	}

	searchConditions := append([]string{}, conditions...)
	searchArgs := append([]interface{}{}, args...)
	searchTerm := strings.ToLower(strings.TrimSpace(filters.Search))
	if searchTerm != "" {
		pattern := "%" + searchTerm + "%"
		searchFragments := []string{
			fmt.Sprintf("LOWER(COALESCE(x.school_name, '')) LIKE $%d", len(searchArgs)+1),
			fmt.Sprintf("LOWER(COALESCE(x.nisn, '')) LIKE $%d", len(searchArgs)+1),
			fmt.Sprintf("LOWER(COALESCE(x.nama_siswa, '')) LIKE $%d", len(searchArgs)+1),
			fmt.Sprintf("LOWER(COALESCE(x.rombel, '')) LIKE $%d", len(searchArgs)+1),
			fmt.Sprintf("LOWER(COALESCE(x.pt_kampus, '')) LIKE $%d", len(searchArgs)+1),
			fmt.Sprintf("LOWER(COALESCE(x.pt_prodi, '')) LIKE $%d", len(searchArgs)+1),
		}
		searchConditions = append(searchConditions, "("+strings.Join(searchFragments, " OR ")+")")
		searchArgs = append(searchArgs, pattern)
	}

	searchBaseQuery := `
		FROM final_pt x
		WHERE ` + strings.Join(searchConditions, " AND ")

	countQuery := buildFinalPTReconciledCTE() + ` SELECT COUNT(*) ` + searchBaseQuery
	meta := emptyMeta
	if err := s.DB.Pool.QueryRow(ctx, countQuery, searchArgs...).Scan(&meta.Total); err != nil {
		logStatusLulusanPTAnalyticsDBError("accepted_insights.detail_count", err, filters, scope, countQuery)
		return nil, nil, nil, emptyMeta, err
	}

	listArgs := append([]interface{}{}, searchArgs...)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)
	listQuery := `
		SELECT
			COALESCE(x.school_name, '') AS school_name,
			COALESCE(x.nisn, '') AS nisn,
			` + namaSiswaExpr + ` AS nama_siswa,
			` + rombelExpr + ` AS rombel,
			x.tahun_lulus,
			` + jalurExpr + ` AS jalur,
			` + kampusExpr + ` AS nama_pt,
			` + prodiExpr + ` AS prodi,
			` + tahunDiterimaExpr + ` AS tahun_diterima
		` + searchBaseQuery + `
		ORDER BY x.tahun_lulus DESC, COALESCE(x.school_name, '') ASC, ` + namaSiswaExpr + ` ASC, COALESCE(x.nisn, '') ASC
		LIMIT $` + fmt.Sprint(len(searchArgs)+1) + ` OFFSET $` + fmt.Sprint(len(searchArgs)+2)
	listQuery = buildFinalPTReconciledCTE() + listQuery

	rows, err := s.DB.Pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		logStatusLulusanPTAnalyticsDBError("accepted_insights.detail_rows", err, filters, scope, listQuery)
		return nil, nil, nil, emptyMeta, err
	}
	defer rows.Close()

	detailRows := make([]StatusLulusanPTAnalyticsDetailItem, 0, pageSize)
	for rows.Next() {
		var item StatusLulusanPTAnalyticsDetailItem
		if scanErr := rows.Scan(
			&item.SchoolName,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.Jalur,
			&item.NamaPT,
			&item.Prodi,
			&item.TahunDiterima,
		); scanErr != nil {
			logStatusLulusanPTAnalyticsDBError("accepted_insights.detail_scan", scanErr, filters, scope, listQuery)
			return nil, nil, nil, emptyMeta, scanErr
		}
		detailRows = append(detailRows, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		logStatusLulusanPTAnalyticsDBError("accepted_insights.detail_rows_iter", rowsErr, filters, scope, listQuery)
		return nil, nil, nil, emptyMeta, rowsErr
	}

	return topKampus, topProdi, detailRows, meta, nil
}
