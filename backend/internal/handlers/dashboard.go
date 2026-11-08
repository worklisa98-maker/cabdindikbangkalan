package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type DashboardHandler struct {
	DB *db.DB
}

func (h DashboardHandler) Summary(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "dashboard", "summary", "summary", dashboardSummarySlowThreshold)
	defer obs.Finish()
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid school_id")
			return
		}
		requestedSchoolID = &id
	}

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

	service := services.InternalDashboardService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, summaryRequestTimeout)
	defer cancel()

	payload, err := service.GetSummary(ctx, user, services.InternalDashboardFilters{
		SchoolID: requestedSchoolID,
		Jenjang:  jenjang,
		Status:   status,
		Tahun:    tahun,
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			if handleOperationalError(c, h.DB, "dashboard", "summary", err) {
				return
			}
			respondSafeSummaryFallback(c, "dashboard", "summary", "Dashboard summary is temporarily unavailable.", emptyInternalDashboardSummary(), err)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h DashboardHandler) LandingSummary(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "dashboard", "landing_summary", "summary", dashboardSummarySlowThreshold)
	defer obs.Finish()
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	ctx, cancel := requestContextWithTimeout(c, summaryRequestTimeout)
	defer cancel()

	scope, err := auth.GetMultiSchoolScope(ctx, h.DB, user)
	if err != nil {
		utils.RespondForbidden(c)
		return
	}

	service := services.LandingSummaryService{DB: h.DB}
	var scopedSchoolIDs []int
	if !scope.All {
		scopedSchoolIDs = append([]int(nil), scope.SchoolIDs...)
	}

	payload, err := service.Get(ctx, scopedSchoolIDs)
	if err != nil {
		if handleOperationalError(c, h.DB, "dashboard", "landing_summary", err) {
			return
		}
		respondSafeSummaryFallback(c, "dashboard", "landing_summary", "Landing summary is temporarily unavailable.", emptyLandingSummary(), err)
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}
