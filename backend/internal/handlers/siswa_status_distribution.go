package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

func (h SiswaHandler) StatusDistribution(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	year, _ := strconv.Atoi(strings.TrimSpace(c.Query("year")))
	var schoolID *int
	if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
		value, convErr := strconv.Atoi(raw)
		if convErr != nil || value <= 0 {
			utils.RespondBadRequest(c, "school_id tidak valid.")
			return
		}
		schoolID = &value
	}
	payload, err := (services.SiswaService{DB: h.DB}).StatusDistribution(c, user, services.StatusDistributionFilters{
		Year: year, Jenjang: c.Query("jenjang"), StatusSekolah: c.Query("status_sekolah"), SchoolID: schoolID,
	})
	if errors.Is(err, auth.ErrForbidden) {
		utils.RespondForbidden(c)
		return
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}
