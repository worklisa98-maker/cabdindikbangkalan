package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type SystemTimeHandler struct {
	Clock services.SystemClock
}

type setSystemTimeOverrideRequest struct {
	OverrideDatetime string `json:"overrideDatetime"`
}

func (h SystemTimeHandler) GetCurrentStatus(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !h.Clock.FeatureEnabled() {
		utils.RespondForbidden(c)
		return
	}

	status, err := h.Clock.Status(c, auth.IsAdminMaster(user), false)
	if err != nil {
		logging.Errorf("system_time", "get_current_status", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, status)
}

func (h SystemTimeHandler) GetAdminStatus(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) || !h.Clock.FeatureEnabled() {
		utils.RespondForbidden(c)
		return
	}

	status, err := h.Clock.Status(c, true, true)
	if err != nil {
		logging.Errorf("system_time", "get_admin_status", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, status)
}

func (h SystemTimeHandler) SetOverride(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	var req setSystemTimeOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	status, err := h.Clock.SetOverride(c, user, req.OverrideDatetime)
	if err != nil {
		switch err.Error() {
		case "INVALID_OVERRIDE_DATETIME":
			utils.RespondBadRequestMessage(c, err.Error())
		default:
			if err == auth.ErrForbidden {
				utils.RespondForbidden(c)
				return
			}
			logging.Errorf("system_time", "set_override", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, status)
}

func (h SystemTimeHandler) ResetOverride(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	status, err := h.Clock.ResetOverride(c, user)
	if err != nil {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		logging.Errorf("system_time", "reset_override", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, status)
}
