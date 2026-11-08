package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type PublicHandler struct {
	DB *db.DB
}

type metaLastUpdatedResponse struct {
	PrestasiUpdatedAt    *time.Time `json:"prestasiUpdatedAt"`
	SiswaPTUpdatedAt     *time.Time `json:"siswaPTUpdatedAt"`
	DataSekolahUpdatedAt *time.Time `json:"dataSekolahUpdatedAt"`
}

type publicTopUniversitiesResponse struct {
	Data  []services.TopUniversity `json:"data"`
	Total int                      `json:"total"`
	Years []int                    `json:"years,omitempty"`
}

func buildTopUniversitiesResponse(rows []services.TopUniversity, years []int) publicTopUniversitiesResponse {
	data := rows
	if data == nil {
		data = []services.TopUniversity{}
	}
	return publicTopUniversitiesResponse{
		Data:  data,
		Total: len(data),
		Years: years,
	}
}

func formatDatabaseTarget() string {
	raw := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if raw == "" {
		return "unknown"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		parsed, err = url.Parse("postgres://" + raw)
		if err != nil {
			return "unknown"
		}
	}
	host := parsed.Host
	dbname := strings.TrimPrefix(parsed.Path, "/")
	if dbname == "" {
		dbname = "unknown"
	}
	if host == "" {
		host = "unknown"
	}
	return host + "/" + dbname
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func (h PublicHandler) Dashboard(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "public", "dashboard", "summary", publicSummarySlowThreshold)
	defer obs.Finish()
	jenjang := strings.TrimSpace(c.Query("jenjang"))
	status := strings.TrimSpace(c.Query("status"))
	tahun := 0
	if raw := strings.TrimSpace(c.Query("tahun")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid tahun")
			return
		}
		tahun = parsed
	}

	service := services.PublicDashboardService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, summaryRequestTimeout)
	defer cancel()

	payload, err := service.GetSummary(ctx, services.DashboardFilters{
		Jenjang: jenjang,
		Status:  status,
		Tahun:   tahun,
	})
	if err != nil {
		if handleOperationalError(c, h.DB, "public", "dashboard", err) {
			return
		}
		respondSafeSummaryFallback(c, "public", "dashboard", "Public dashboard summary is temporarily unavailable.", emptyDashboardSummary(), err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) LandingSummary(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "public", "landing_summary", "summary", publicSummarySlowThreshold)
	defer obs.Finish()
	service := services.LandingSummaryService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, summaryRequestTimeout)
	defer cancel()

	payload, err := service.Get(ctx, nil)
	if err != nil {
		if handleOperationalError(c, h.DB, "public", "landing_summary", err) {
			return
		}
		respondSafeSummaryFallback(c, "public", "landing_summary", "Public landing summary is temporarily unavailable.", emptyLandingSummary(), err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) MetaLastUpdated(c *gin.Context) {
	logging.Infof("public", "meta_last_updated", "event=hit")
	ctx, cancel := requestContextWithTimeout(c, summaryRequestTimeout)
	defer cancel()

	var prestasiUpdatedAt sql.NullTime
	var siswaPtUpdatedAt sql.NullTime
	var dataSekolahUpdatedAt sql.NullTime
	if err := h.DB.Pool.QueryRow(ctx, `
        SELECT
            MAX(last_update_prestasi) AS prestasi_updated_at,
            MAX(last_update_siswa_pt) AS siswa_pt_updated_at,
            MAX(updated_at) AS data_sekolah_updated_at
        FROM data_sekolah
    `).Scan(&prestasiUpdatedAt, &siswaPtUpdatedAt, &dataSekolahUpdatedAt); err != nil {
		if handleOperationalError(c, h.DB, "public", "meta_last_updated", err) {
			return
		}
		logging.Errorf("public", "meta_last_updated", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, metaLastUpdatedResponse{
		PrestasiUpdatedAt:    nullTimePtr(prestasiUpdatedAt),
		SiswaPTUpdatedAt:     nullTimePtr(siswaPtUpdatedAt),
		DataSekolahUpdatedAt: nullTimePtr(dataSekolahUpdatedAt),
	})
}

func (h PublicHandler) AchievementDashboard(c *gin.Context) {
	logging.Infof("public", "achievement_dashboard", "event=hit")
	service := services.PublicAchievementService{DB: h.DB}
	bentuk := strings.TrimSpace(c.Query("bentuk"))
	payload, err := service.GetDashboard(c, bentuk)
	if err != nil {
		logging.Errorf("public", "achievement_dashboard", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) AchievementSummary(c *gin.Context) {
	logging.Infof("public", "achievement_summary", "event=hit")
	category := strings.TrimSpace(c.Query("kategori"))
	year := strings.TrimSpace(c.Query("tahun"))
	bentuk := strings.TrimSpace(c.Query("bentukSekolah"))
	if bentuk == "" {
		bentuk = strings.TrimSpace(c.Query("bentuk"))
	}
	status := strings.TrimSpace(c.Query("status"))

	service := services.PublicAchievementService{DB: h.DB}
	payload, err := service.GetFullDashboard(c, category, year, bentuk, status)
	if err != nil {
		if handleOperationalError(c, h.DB, "public", "achievement_summary", err) {
			return
		}
		respondSafeSummaryFallback(c, "public", "achievement_summary", "Public achievement summary is temporarily unavailable.", emptyPublicAchievementSummary(), err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) AchievementFull(c *gin.Context) {
	logging.Infof("public", "achievement_full", "event=hit")
	category := strings.TrimSpace(c.Query("kategori"))
	year := strings.TrimSpace(c.Query("tahun"))
	bentuk := strings.TrimSpace(c.Query("bentuk"))
	status := strings.TrimSpace(c.Query("status"))

	service := services.PublicAchievementService{DB: h.DB}
	payload, err := service.GetFullDashboard(c, category, year, bentuk, status)
	if err != nil {
		logging.Errorf("public", "achievement_full", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) AchievementAnalytics(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "public", "achievement_analytics", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
	yearParam := strings.TrimSpace(c.Query("year"))
	if yearParam == "" {
		yearParam = strings.TrimSpace(c.Query("tahun"))
	}
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		parsed, convErr := strconv.Atoi(yearParam)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		yearParam = strconv.Itoa(parsed)
	}

	jenjang := strings.ToUpper(strings.TrimSpace(c.Query("jenjang")))
	if jenjang == "ALL" {
		jenjang = ""
	}
	if jenjang != "" && jenjang != "SMA" && jenjang != "SMK" && jenjang != "SLB" {
		utils.RespondBadRequest(c, "Invalid jenjang")
		return
	}

	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	if status == "ALL" {
		status = ""
	}
	if status != "" && status != "NEGERI" && status != "SWASTA" {
		utils.RespondBadRequest(c, "Invalid status")
		return
	}

	var schoolID *int
	if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid school_id")
			return
		}
		schoolID = &parsed
	}

	service := services.AchievementAnalyticsSummaryService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.GetPublic(ctx, services.AchievementAnalyticsPublicFilters{
		Year:     yearParam,
		Jenjang:  jenjang,
		Status:   status,
		SchoolID: schoolID,
	})
	if err != nil {
		switch {
		case handleOperationalError(c, h.DB, "public", "achievement_analytics", err):
			return
		case errors.Is(err, services.ErrInvalidFilter):
			logging.Warnf("public", "achievement_analytics", "event=reject reason=bad_request err=%v", err)
			utils.RespondBadRequest(c, "Invalid filter")
		case errors.Is(err, auth.ErrForbidden):
			logging.Warnf("public", "achievement_analytics", "event=reject reason=forbidden err=%v", err)
			utils.RespondForbidden(c)
		case errors.Is(err, auth.ErrUnauthorized):
			logging.Warnf("public", "achievement_analytics", "event=reject reason=unauthorized err=%v", err)
			utils.RespondUnauthorized(c)
		default:
			respondSafeAnalyticsFallback(c, "public", "achievement_analytics", "Public achievement analytics is temporarily unavailable.", emptyAchievementAnalyticsSummary(services.AchievementAnalyticsSummaryFilters{
				Role:     "PUBLIC",
				SchoolID: schoolID,
				Year:     yearParam,
				Jenjang:  jenjang,
				Status:   status,
			}), err)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) AdmissionDashboard(c *gin.Context) {
	logging.Infof("public", "admission_dashboard", "event=hit")
	service := services.PublicAdmissionService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, summaryRequestTimeout)
	defer cancel()
	yearParam := strings.TrimSpace(c.Query("year"))
	year := 0
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		parsed, convErr := strconv.Atoi(yearParam)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}
	jenjang := strings.TrimSpace(c.Query("jenjang"))
	status := strings.TrimSpace(c.Query("status"))
	payload, err := service.GetDashboard(ctx, year, jenjang, status)
	if err != nil {
		if handleOperationalError(c, h.DB, "public", "admission_dashboard", err) {
			return
		}
		logging.Errorf("public", "admission_dashboard", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) AdmissionTopUniversities(c *gin.Context) {
	logging.Infof("public", "admission_top_universities", "event=hit")
	service := services.PublicAdmissionService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, summaryRequestTimeout)
	defer cancel()
	yearParam := strings.TrimSpace(c.Query("year"))
	year := 0
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		parsed, convErr := strconv.Atoi(yearParam)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}
	jenjang := strings.TrimSpace(c.Query("jenjang"))
	status := strings.TrimSpace(c.Query("status"))

	if logging.DebugEnabled() {
		logging.Debugf("public", "admission_top_universities", "database=%s", formatDatabaseTarget())
	}

	if year == 0 {
		rows, years, err := service.GetTopUniversitiesByYear(ctx)
		if err != nil {
			if handleOperationalError(c, h.DB, "public", "admission_top_universities", err) {
				return
			}
			logging.Errorf("public", "admission_top_universities", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, buildTopUniversitiesResponse(rows, years))
		return
	}

	rows, err := service.GetTopUniversities(ctx, year, jenjang, status)
	if err != nil {
		if handleOperationalError(c, h.DB, "public", "admission_top_universities", err) {
			return
		}
		logging.Errorf("public", "admission_top_universities", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, buildTopUniversitiesResponse(rows, nil))
}

func (h PublicHandler) AdmissionAnalytics(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "public", "admission_analytics", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
	yearParam := strings.TrimSpace(c.Query("year"))
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		if _, convErr := strconv.Atoi(yearParam); convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
	}

	service := services.AdmissionAnalyticsSummaryService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()
	filters := services.AdmissionAnalyticsSummaryFilters{
		Role:    "PUBLIC",
		Year:    yearParam,
		Jenjang: strings.TrimSpace(c.Query("jenjang")),
		Status:  strings.TrimSpace(c.Query("status")),
	}
	payload, err := service.Get(ctx, filters)
	if err != nil {
		if handleOperationalError(c, h.DB, "public", "admission_analytics", err) {
			return
		}
		if c.Request.URL.Path == "/api/public/siswa/analytics/final" && isMissingRelation(err) {
			logging.Warnf("public", "admission_analytics", "event=final_schema_missing")
			filters.SkipFinal = true
			legacyPayload, legacyErr := service.Get(ctx, filters)
			if legacyErr != nil {
				if handleOperationalError(c, h.DB, "public", "admission_analytics", legacyErr) {
					return
				}
				respondSafeAnalyticsFallback(c, "public", "admission_analytics", "Public admission analytics is temporarily unavailable.", buildEmptyAdmissionAnalytics(filters), legacyErr)
				return
			}
			utils.RespondData(c, http.StatusOK, legacyPayload)
			return
		}
		if c.Request.URL.Path == "/api/public/siswa/analytics/final" {
			respondSafeAnalyticsFallback(c, "public", "admission_analytics", "Public admission analytics is temporarily unavailable.", buildEmptyAdmissionAnalytics(filters), err)
			return
		}
		respondSafeAnalyticsFallback(c, "public", "admission_analytics", "Public admission analytics is temporarily unavailable.", buildEmptyAdmissionAnalytics(filters), err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func isMissingRelation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42P01"
	}
	return false
}

func buildEmptyAdmissionAnalytics(filters services.AdmissionAnalyticsSummaryFilters) services.AdmissionAnalyticsSummary {
	return services.AdmissionAnalyticsSummary{
		FiltersEcho: services.AnalyticsFiltersEcho{
			Year:    filters.Year,
			Jenjang: filters.Jenjang,
			Status:  filters.Status,
		},
		AvailableYears: []int{},
		Kpis: services.AdmissionAnalyticsKpis{
			TotalDiterima:    0,
			YoYPercent:       nil,
			CurrentYearTotal: 0,
			PrevYearTotal:    0,
		},
		TrendByYear: []services.AnalyticsYearPoint{},
		Distribution: services.AdmissionAnalyticsDistribution{
			ByJalur: []services.AnalyticsKeyTotal{},
		},
		TopSchools:         []services.AdmissionTopSchool{},
		TrendByJalur:       []services.AdmissionTrendByJalurPoint{},
		TopPerguruanTinggi: []services.AdmissionTopPerguruanTinggi{},
	}
}

func isStatusLulusanPTAnalyticsEmptyError(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

func buildEmptyStatusLulusanPTAnalytics(startYear int, endYear int, jenjang string, page int, pageSize int) services.StatusLulusanPTAnalyticsResponse {
	nowYear := time.Now().Year()
	if endYear <= 0 {
		endYear = nowYear
	}
	if startYear <= 0 {
		startYear = endYear - 4
	}
	normalizedJenjang := strings.ToUpper(strings.TrimSpace(jenjang))
	if normalizedJenjang == "" {
		normalizedJenjang = "ALL"
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return services.StatusLulusanPTAnalyticsResponse{
		StartYear:    startYear,
		EndYear:      endYear,
		Jenjang:      normalizedJenjang,
		Series:       []services.StatusLulusanPTAnalyticsSeriesItem{},
		TotalsByYear: []services.StatusLulusanPTAnalyticsTotalByYear{},
		TopKampus:    []services.StatusLulusanPTAnalyticsDistributionItem{},
		TopProdi:     []services.StatusLulusanPTAnalyticsDistributionItem{},
		DetailRows:   []services.StatusLulusanPTAnalyticsDetailItem{},
		DetailMeta: services.StatusLulusanPTAnalyticsDetailMeta{
			Page:     page,
			PageSize: pageSize,
			Total:    0,
		},
	}
}

func (h PublicHandler) SiswaAnalyticsStatus(c *gin.Context) {
	logging.Infof("public", "siswa_analytics_status", "event=hit")
	service := services.SiswaService{DB: h.DB}
	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}

	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.ValidationSummary(ctx, auth.User{Role: "PUBLIC"}, nil, year)
	if err == nil {
		utils.RespondData(c, http.StatusOK, payload)
		return
	}
	if handleOperationalError(c, h.DB, "public", "siswa_analytics_status", err) {
		return
	}

	respondSafeSummaryFallback(c, "public", "siswa_analytics_status", "Public siswa validation summary is temporarily unavailable.", emptyValidationSummary(), err)
}

func (h PublicHandler) SiswaFinalOutcomeAnalytics(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "public", "siswa_final_outcome_analytics", "analytics", analyticsSlowThreshold)
	defer obs.Finish()

	year := 0
	if raw := strings.TrimSpace(c.Query("tahun_lulus")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid tahun_lulus")
			return
		}
		year = parsed
	} else if raw := strings.TrimSpace(c.Query("year")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}

	service := services.FinalOutcomeAnalyticsService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.Get(ctx, services.FinalOutcomeAnalyticsFilters{
		Year:    year,
		Jenjang: strings.TrimSpace(c.Query("jenjang")),
		Status:  strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		if handleOperationalError(c, h.DB, "public", "siswa_final_outcome_analytics", err) {
			return
		}
		respondSafeAnalyticsFallback(c, "public", "siswa_final_outcome_analytics", "Public outcome analytics is temporarily unavailable.", emptyFinalOutcomeAnalytics(services.FinalOutcomeAnalyticsFilters{
			Year:    year,
			Jenjang: strings.TrimSpace(c.Query("jenjang")),
			Status:  strings.TrimSpace(c.Query("status")),
		}), err)
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) StatusLulusanAnalyticsPT(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "public", "status_lulusan_analytics_pt", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
	parseYear := func(param string) (int, bool) {
		raw := strings.TrimSpace(c.Query(param))
		if raw == "" {
			return 0, true
		}
		year, err := strconv.Atoi(raw)
		if err != nil || year <= 0 {
			return 0, false
		}
		return year, true
	}

	startYear, ok := parseYear("start_year")
	if !ok {
		utils.RespondBadRequest(c, "Invalid start_year")
		return
	}
	endYear, ok := parseYear("end_year")
	if !ok {
		utils.RespondBadRequest(c, "Invalid end_year")
		return
	}
	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))

	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.GetPublicStatusLulusanPTAnalytics(ctx, services.StatusLulusanPTAnalyticsFilters{
		StartYear: startYear,
		EndYear:   endYear,
		Jenjang:   strings.TrimSpace(c.Query("jenjang")),
		Search:    strings.TrimSpace(c.Query("q")),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		switch {
		case handleOperationalError(c, h.DB, "public", "status_lulusan_analytics_pt", err):
			return
		case errors.Is(err, services.ErrInvalidOutcome):
			utils.RespondBadRequest(c, "Filter tidak valid.")
		case isMissingRelation(err):
			respondSafeAnalyticsFallback(c, "public", "status_lulusan_analytics_pt", "Public status lulusan PT analytics is temporarily unavailable.", buildEmptyStatusLulusanPTAnalytics(startYear, endYear, strings.TrimSpace(c.Query("jenjang")), page, pageSize), err)
		case isStatusLulusanPTAnalyticsEmptyError(err):
			utils.RespondData(c, http.StatusOK, buildEmptyStatusLulusanPTAnalytics(startYear, endYear, strings.TrimSpace(c.Query("jenjang")), page, pageSize))
		default:
			respondSafeAnalyticsFallback(c, "public", "status_lulusan_analytics_pt", "Public status lulusan PT analytics is temporarily unavailable.", buildEmptyStatusLulusanPTAnalytics(startYear, endYear, strings.TrimSpace(c.Query("jenjang")), page, pageSize), err)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) SiswaAnalyticsHigherEd(c *gin.Context) {
	h.respondFinalStatusAnalytics(c, "PT", true)
}

func (h PublicHandler) SiswaAnalyticsFinalHigherEd(c *gin.Context) {
	h.respondFinalStatusAnalytics(c, "PT", true)
}

func (h PublicHandler) SiswaAnalyticsFinalWork(c *gin.Context) {
	h.respondFinalStatusAnalytics(c, "KERJA", false)
}

func (h PublicHandler) SiswaAnalyticsFinalEntrepreneur(c *gin.Context) {
	h.respondFinalStatusAnalytics(c, "WIRAUSAHA", false)
}

func (h PublicHandler) SiswaAnalyticsFinalUndefined(c *gin.Context) {
	h.respondFinalStatusAnalytics(c, "BELUM", false)
}

func (h PublicHandler) respondFinalStatusAnalytics(c *gin.Context, finalType string, allowLegacy bool) {
	obs := beginEndpointObservation(c, h.DB, "public", "siswa_final_analytics", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()
	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}

	payload, err := service.AnalyticsFinalByType(ctx, auth.User{Role: "PUBLIC"}, nil, finalType, year)
	if err == nil {
		if payload == nil {
			payload = emptyFinalStatusAnalytics()
		}
		utils.RespondData(c, http.StatusOK, payload)
		return
	}
	if handleOperationalError(c, h.DB, "public", "siswa_final_analytics", err) {
		return
	}

	if isMissingRelation(err) {
		logging.Warnf(
			"public",
			"siswa_final_analytics",
			"event=final_schema_missing final_type=%s path=%s",
			finalType,
			c.Request.URL.Path,
		)
		if allowLegacy {
			legacyPayload, legacyErr := service.LegacyAnalyticsHigherEd(ctx, nil)
			if legacyErr == nil {
				if legacyPayload == nil {
					legacyPayload = emptyFinalStatusAnalytics()
				}
				utils.RespondData(c, http.StatusOK, legacyPayload)
				return
			}
			if handleOperationalError(c, h.DB, "public", "siswa_final_analytics", legacyErr) {
				return
			}
			respondSafeListFallback(c, "public", "siswa_final_analytics", "Public siswa final analytics is temporarily unavailable.", emptyFinalStatusAnalytics(), legacyErr)
			return
		}
	}
	respondSafeListFallback(c, "public", "siswa_final_analytics", "Public siswa final analytics is temporarily unavailable.", emptyFinalStatusAnalytics(), err)
}
