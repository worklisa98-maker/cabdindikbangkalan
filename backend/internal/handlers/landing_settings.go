package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type LandingSettingsHandler struct {
	DB *db.DB
}

func (h LandingSettingsHandler) GetPublic(c *gin.Context) {
	h.respondSettings(c)
}

func (h LandingSettingsHandler) GetAdmin(c *gin.Context) {
	if _, ok := h.requireAdminMaster(c); !ok {
		return
	}
	h.respondSettings(c)
}

func (h LandingSettingsHandler) UpdateAdmin(c *gin.Context) {
	user, ok := h.requireAdminMaster(c)
	if !ok {
		return
	}

	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	patch := body
	if rawSettings, ok := body["settings"]; ok {
		settingsPatch, ok := rawSettings.(map[string]any)
		if !ok {
			utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
			return
		}
		patch = settingsPatch
	}

	service := services.LandingSettingsService{DB: h.DB}
	settings, err := service.Update(c, user, patch)
	if err != nil {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		logging.Errorf("landing_settings", "update_admin", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"settings": settings})
}

func (h LandingSettingsHandler) respondSettings(c *gin.Context) {
	service := services.LandingSettingsService{DB: h.DB}
	settings, err := service.Get(c)
	if err != nil {
		logging.Errorf("landing_settings", "get", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"settings": settings})
}

func (h LandingSettingsHandler) requireAdminMaster(c *gin.Context) (auth.User, bool) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return auth.User{}, false
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return auth.User{}, false
	}
	return user, true
}
