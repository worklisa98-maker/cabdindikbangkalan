package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type DashboardHandler struct {
	DB *db.DB
}

func (h DashboardHandler) Summary(c *gin.Context) {
	logging.Infof("dashboard", "summary", "event=hit")
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
	payload, err := service.GetSummary(c, user, services.InternalDashboardFilters{
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
			logging.Errorf("dashboard", "summary", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}
