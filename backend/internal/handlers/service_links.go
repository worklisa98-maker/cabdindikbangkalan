package handlers

import (
	"errors"
	"net/http"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
	"github.com/gin-gonic/gin"
)

type ServiceLinksHandler struct{ DB *db.DB }

func privateResponse(c *gin.Context, status int, data any) {
	c.Header("Cache-Control", "private, no-store")
	utils.RespondData(c, status, data)
}

func (h ServiceLinksHandler) Public(c *gin.Context)        { h.list(c, false) }
func (h ServiceLinksHandler) Authenticated(c *gin.Context) { h.list(c, true) }
func (h ServiceLinksHandler) list(c *gin.Context, authenticated bool) {
	service := services.ServiceLinksService{DB: h.DB}
	halo, err := service.GetExternal(c, services.HaloSettingsKey)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	sinom, err := service.GetExternal(c, services.SinomSettingsKey)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	links, err := service.GetLinks(c)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if authenticated {
		privateResponse(c, http.StatusOK, gin.H{"halo": halo, "sinom": sinom, "pusat_tautan": services.FilterLinks(links, true)})
		return
	}
	c.Header("Cache-Control", "no-store")
	utils.RespondData(c, http.StatusOK, gin.H{"halo": halo, "sinom": sinom, "pusat_tautan": services.FilterLinks(links, false)})
}

func (h ServiceLinksHandler) Admin(c *gin.Context) {
	if _, ok := h.master(c); !ok {
		return
	}
	service := services.ServiceLinksService{DB: h.DB}
	halo, err := service.GetExternal(c, services.HaloSettingsKey)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	sinom, err := service.GetExternal(c, services.SinomSettingsKey)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	links, err := service.GetLinks(c)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	privateResponse(c, http.StatusOK, gin.H{"halo": halo, "sinom": sinom, "pusat_tautan": links})
}

func (h ServiceLinksHandler) SaveExternal(c *gin.Context) {
	user, ok := h.master(c)
	if !ok {
		return
	}
	key := services.HaloSettingsKey
	if c.Param("name") == "sinom" {
		key = services.SinomSettingsKey
	} else if c.Param("name") != "halo" {
		utils.RespondBadRequestMessage(c, "INVALID_SERVICE")
		return
	}
	var value services.ExternalService
	if err := c.ShouldBindJSON(&value); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := services.ServiceLinksService{DB: h.DB}
	if err := service.SaveExternal(c, key, value, user.ID); err != nil {
		if errors.Is(err, services.ErrInvalidSettings) {
			utils.RespondBadRequestMessage(c, "INVALID_SETTINGS")
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	privateResponse(c, http.StatusOK, gin.H{"service": value})
}

func (h ServiceLinksHandler) SaveLinks(c *gin.Context) {
	user, ok := h.master(c)
	if !ok {
		return
	}
	var request struct {
		services.LinkSettings
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := services.ServiceLinksService{DB: h.DB}
	saved, err := service.SaveLinks(c, request.LinkSettings, request.Version, user.ID)
	if errors.Is(err, services.ErrSettingsConflict) {
		privateResponse(c, http.StatusConflict, gin.H{"error": "SETTINGS_CHANGED"})
		return
	}
	if errors.Is(err, services.ErrInvalidSettings) {
		utils.RespondBadRequestMessage(c, "INVALID_SETTINGS")
		return
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	privateResponse(c, http.StatusOK, gin.H{"pusat_tautan": saved})
}

func (h ServiceLinksHandler) master(c *gin.Context) (auth.User, bool) {
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
