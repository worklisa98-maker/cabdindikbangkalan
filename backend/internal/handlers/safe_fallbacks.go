package handlers

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

func respondSafeSummaryFallback(c *gin.Context, module string, action string, message string, data interface{}, err error) {
	logging.Errorf(module, action, "event=safe_summary_fallback path=%s err=%v", requestRoutePath(c), err)
	utils.RespondSafeSummaryFallback(c, message, data)
}

func respondSafeListFallback(c *gin.Context, module string, action string, message string, data interface{}, err error) {
	logging.Errorf(module, action, "event=safe_list_fallback path=%s err=%v", requestRoutePath(c), err)
	utils.RespondSafeListFallback(c, message, data)
}

func respondSafeAnalyticsFallback(c *gin.Context, module string, action string, message string, data interface{}, err error) {
	logging.Errorf(module, action, "event=safe_analytics_fallback path=%s err=%v", requestRoutePath(c), err)
	utils.RespondSafeAnalyticsFallback(c, message, data)
}

func emptyDashboardSummary() services.DashboardSummary {
	return services.DashboardSummary{
		Kpi:                   services.DashboardKPI{},
		Akreditasi:            services.AkreditasiCounts{},
		PctAB:                 0,
		TopSekolah:            []services.TopSekolahItem{},
		Totals:                services.DashboardTotals{},
		AkreditasiPerJenjang:  []services.AkreditasiByJenjang{},
		AkreditasiComposition: []services.AkreditasiComposition{},
		StatusByJenjang:       []services.StatusByJenjang{},
		GuruVsTendik:          []map[string]interface{}{},
		JenjangSummaries:      []services.JenjangSummary{},
		LastUpdated:           nil,
	}
}

func emptyInternalDashboardSummary() services.InternalDashboardSummary {
	return services.InternalDashboardSummary{
		DashboardSummary: emptyDashboardSummary(),
		Extras:           services.InternalDashboardExtras{},
		InternalIndex:    nil,
	}
}

func emptyLandingSummary() services.LandingSummary {
	return services.LandingSummary{
		PrestasiTotal:         0,
		ActiveSchools:         0,
		YearMin:               nil,
		YearMax:               nil,
		AkreditasiABPercent:   0,
		LatestYear:            nil,
		PrevYear:              nil,
		YoYSiswaPercent:       0,
		YoYGuruTendikPercent:  0,
		CurrentYearSiswa:      0,
		PrevYearSiswa:         0,
		CurrentYearGuruTendik: 0,
		PrevYearGuruTendik:    0,
	}
}

func emptyAchievementAnalyticsSummary(filters services.AchievementAnalyticsSummaryFilters) services.AchievementAnalyticsSummary {
	return services.AchievementAnalyticsSummary{
		FiltersEcho: services.AnalyticsFiltersEcho{
			Year:       filters.Year,
			Jenjang:    filters.Jenjang,
			Status:     filters.Status,
			SchoolID:   filters.SchoolID,
			JenisLomba: filters.JenisLomba,
		},
		AvailableYears: []int{},
		LatestYear:     nil,
		Kpis:           services.AchievementAnalyticsKpis{},
		TrendByYear:    []services.AnalyticsYearPoint{},
		Distribution: services.AchievementAnalyticsDistribution{
			ByTingkat:    []services.AnalyticsKeyTotal{},
			ByJenisLomba: []services.AnalyticsKeyTotal{},
		},
		DistributionByYear: services.AchievementAnalyticsDistributionByYear{
			ByTingkat:      []services.AnalyticsTingkatYearTotal{},
			ByJenisPeserta: []services.AnalyticsJenisYearTotal{},
		},
		CategoryDonutSiswa: []services.AnalyticsCategoryTotal{},
		TopSchools:         []services.AchievementTopSchool{},
	}
}

func emptyAchievementAdminSummary() achievementAdminSummaryResponse {
	return achievementAdminSummaryResponse{
		RekapJenjangTingkat: []services.AchievementJenjangTingkat{},
		DetailSekolah:       []services.AchievementSchoolDetail{},
	}
}

func emptyPublicAchievementSummary() services.PublicAchievementFull {
	return services.PublicAchievementFull{
		CategoryTrends:           []services.PublicAchievementTrend{},
		DistribusiJenjangTingkat: []services.PublicJenjangTingkatCount{},
		TopSchools:               []services.PublicTopSchool{},
		SchoolDetails:            []services.PublicSchoolDetail{},
		AvailableYears:           []string{},
	}
}

func emptyAcceptanceOfferAnalyticsPayload(page int, pageSize int) services.AcceptanceOfferAnalyticsPayload {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return services.AcceptanceOfferAnalyticsPayload{
		AvailableYears: []int{},
		Summary: services.AcceptanceOfferAnalyticsSummary{
			TotalPendaftar: 0,
			TotalDiterima:  0,
			AcceptanceRate: 0,
			BelumDiumumkan: 0,
		},
		BreakdownJalur: []services.AcceptanceOfferAnalyticsJalurItem{
			{Jalur: "SNBP", Pendaftar: 0, Diterima: 0, AcceptanceRate: 0},
			{Jalur: "SNBT", Pendaftar: 0, Diterima: 0, AcceptanceRate: 0},
			{Jalur: "MANDIRI", Pendaftar: 0, Diterima: 0, AcceptanceRate: 0},
			{Jalur: "KEDINASAN", Pendaftar: 0, Diterima: 0, AcceptanceRate: 0},
		},
		TrendTahunan:  []services.AcceptanceOfferAnalyticsTrendItem{},
		AcceptedItems: []services.AcceptanceOfferAcceptedStudentItem{},
		Pagination: services.AcceptanceOfferAnalyticsPagination{
			Page:     page,
			PageSize: pageSize,
			Total:    0,
		},
	}
}

func emptyFinalOutcomeAnalytics(filters services.FinalOutcomeAnalyticsFilters) services.FinalOutcomeAnalyticsSummary {
	yearLabel := ""
	if filters.Year > 0 {
		yearLabel = strconv.Itoa(filters.Year)
	}

	jenjang := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
	switch jenjang {
	case "SMA", "SMK", "SLB":
	default:
		jenjang = "ALL"
	}

	status := strings.ToUpper(strings.TrimSpace(filters.Status))
	switch status {
	case "PT", "KERJA", "WIRAUSAHA", "BELUM":
	default:
		status = "ALL"
	}

	return services.FinalOutcomeAnalyticsSummary{
		FiltersEcho: services.AnalyticsFiltersEcho{
			Year:    yearLabel,
			Jenjang: jenjang,
			Status:  status,
		},
		AvailableYears:   []int{},
		LatestYear:       nil,
		Kpis:             services.FinalOutcomeAnalyticsKpis{},
		TrendByYear:      []services.AnalyticsYearPoint{},
		BreakdownByJalur: []services.AnalyticsKeyTotal{},
		TopKampus:        []services.FinalOutcomeTopItem{},
		TopProdi:         []services.FinalOutcomeTopItem{},
	}
}

func emptyAdmissionSummaryList() []map[string]interface{} {
	return []map[string]interface{}{}
}

func emptyValidationSummary() services.SiswaValidationSummary {
	return services.SiswaValidationSummary{}
}

func emptyHigherEdAnalytics() []services.SiswaHigherEdAnalytics {
	return []services.SiswaHigherEdAnalytics{}
}

func emptyFinalStatusAnalytics() []services.SiswaFinalStatusAnalytics {
	return []services.SiswaFinalStatusAnalytics{}
}
