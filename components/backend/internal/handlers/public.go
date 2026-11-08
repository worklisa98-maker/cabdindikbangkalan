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
	logging.Infof("public", "dashboard", "event=hit")
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
	payload, err := service.GetSummary(c, services.DashboardFilters{
		Jenjang: jenjang,
		Status:  status,
		Tahun:   tahun,
	})
	if err != nil {
		logging.Errorf("public", "dashboard", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) MetaLastUpdated(c *gin.Context) {
	logging.Infof("public", "meta_last_updated", "event=hit")
	var prestasiUpdatedAt sql.NullTime
	var siswaPtUpdatedAt sql.NullTime
	var dataSekolahUpdatedAt sql.NullTime
	if err := h.DB.Pool.QueryRow(c, `
        SELECT
            MAX(last_update_prestasi) AS prestasi_updated_at,
            MAX(last_update_siswa_pt) AS siswa_pt_updated_at,
            MAX(updated_at) AS data_sekolah_updated_at
        FROM data_sekolah
    `).Scan(&prestasiUpdatedAt, &siswaPtUpdatedAt, &dataSekolahUpdatedAt); err != nil {
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
		logging.Errorf("public", "achievement_summary", "err=%v", err)
		utils.RespondInternalServerError(c)
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
	logging.Infof("public", "achievement_analytics", "event=hit")
	yearParam := strings.TrimSpace(c.Query("year"))
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		if _, convErr := strconv.Atoi(yearParam); convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
	}

	service := services.AchievementAnalyticsSummaryService{DB: h.DB}
	payload, err := service.Get(c, services.AchievementAnalyticsSummaryFilters{
		Role:       "PUBLIC",
		Year:       yearParam,
		Jenjang:    strings.TrimSpace(c.Query("jenjang")),
		Status:     strings.TrimSpace(c.Query("status")),
		JenisLomba: strings.TrimSpace(c.Query("jenis_lomba")),
	})
	if err != nil {
		logging.Errorf("public", "achievement_analytics", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) AdmissionDashboard(c *gin.Context) {
	logging.Infof("public", "admission_dashboard", "event=hit")
	service := services.PublicAdmissionService{DB: h.DB}
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
	payload, err := service.GetDashboard(c, year, jenjang, status)
	if err != nil {
		logging.Errorf("public", "admission_dashboard", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h PublicHandler) AdmissionTopUniversities(c *gin.Context) {
	logging.Infof("public", "admission_top_universities", "event=hit")
	service := services.PublicAdmissionService{DB: h.DB}
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
		rows, years, err := service.GetTopUniversitiesByYear(c)
		if err != nil {
			logging.Errorf("public", "admission_top_universities", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, buildTopUniversitiesResponse(rows, years))
		return
	}

	rows, err := service.GetTopUniversities(c, year, jenjang, status)
	if err != nil {
		logging.Errorf("public", "admission_top_universities", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, buildTopUniversitiesResponse(rows, nil))
}

func (h PublicHandler) AdmissionAnalytics(c *gin.Context) {
	yearParam := strings.TrimSpace(c.Query("year"))
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		if _, convErr := strconv.Atoi(yearParam); convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
	}

	service := services.AdmissionAnalyticsSummaryService{DB: h.DB}
	filters := services.AdmissionAnalyticsSummaryFilters{
		Role:    "PUBLIC",
		Year:    yearParam,
		Jenjang: strings.TrimSpace(c.Query("jenjang")),
		Status:  strings.TrimSpace(c.Query("status")),
	}
	payload, err := service.Get(c, filters)
	if err != nil {
		if c.Request.URL.Path == "/api/public/siswa/analytics/final" && isMissingRelation(err) {
			logging.Warnf("public", "admission_analytics", "event=final_schema_missing")
			filters.SkipFinal = true
			legacyPayload, legacyErr := service.Get(c, filters)
			if legacyErr != nil {
				utils.RespondData(c, http.StatusOK, buildEmptyAdmissionAnalytics(filters))
				return
			}
			utils.RespondData(c, http.StatusOK, legacyPayload)
			return
		}
		if c.Request.URL.Path == "/api/public/siswa/analytics/final" {
			logging.Warnf("public", "admission_analytics", "event=analytics_query_failed err=%v", err)
			utils.RespondData(c, http.StatusOK, buildEmptyAdmissionAnalytics(filters))
			return
		}
		logging.Errorf("public", "admission_analytics", "err=%v", err)
		utils.RespondInternalServerError(c)
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

	payload, err := service.ValidationSummary(c.Request.Context(), auth.User{Role: "PUBLIC"}, nil, year)
	if err == nil {
		utils.RespondData(c, http.StatusOK, payload)
		return
	}

	if isMissingRelation(err) {
		logging.Warnf(
			"public",
			"siswa_analytics_status",
			"event=final_schema_missing_fallback path=%s",
			c.Request.URL.Path,
		)
	} else {
		logging.Warnf(
			"public",
			"siswa_analytics_status",
			"event=analytics_query_failed path=%s err=%v",
			c.Request.URL.Path,
			err,
		)
	}

	utils.RespondData(c, http.StatusOK, services.SiswaValidationSummary{})
}

func (h PublicHandler) SiswaFinalOutcomeAnalytics(c *gin.Context) {
	logging.Infof("public", "siswa_final_outcome_analytics", "event=hit")

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
	payload, err := service.Get(c, services.FinalOutcomeAnalyticsFilters{
		Year:    year,
		Jenjang: strings.TrimSpace(c.Query("jenjang")),
		Status:  strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		logging.Errorf("public", "siswa_final_outcome_analytics", "err=%v", err)
		utils.RespondInternalServerError(c)
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

	payload, err := service.AnalyticsFinalByType(c, auth.User{Role: "PUBLIC"}, nil, finalType, year)
	if err == nil {
		if payload == nil {
			payload = []services.SiswaFinalStatusAnalytics{}
		}
		utils.RespondData(c, http.StatusOK, payload)
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
			legacyPayload, legacyErr := service.LegacyAnalyticsHigherEd(c, nil)
			if legacyErr == nil {
				if legacyPayload == nil {
					legacyPayload = []services.SiswaFinalStatusAnalytics{}
				}
				utils.RespondData(c, http.StatusOK, legacyPayload)
				return
			}
		}
	} else {
		logging.Warnf(
			"public",
			"siswa_final_analytics",
			"event=analytics_query_failed final_type=%s path=%s err=%v",
			finalType,
			c.Request.URL.Path,
			err,
		)
	}

	utils.RespondData(c, http.StatusOK, []services.SiswaFinalStatusAnalytics{})
}
