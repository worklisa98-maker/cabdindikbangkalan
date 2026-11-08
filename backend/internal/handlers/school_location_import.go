package handlers

import (
	"context"
	"net/http"
	"time"

	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
	"github.com/gin-gonic/gin"
)

func (h SchoolHandler) PreviewLocationImport(c *gin.Context) { h.locationImport(c, false) }
func (h SchoolHandler) ApplyLocationImport(c *gin.Context)   { h.locationImport(c, true) }

func (h SchoolHandler) locationImport(c *gin.Context, apply bool) {
	user, ok := (LandingSettingsHandler{}).requireAdminMaster(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	report, err := services.RunLocationImport(ctx, h.DB.Pool, services.LocationSeedPath(), apply, &user.ID)
	if err != nil {
		logging.Errorf("school_locations", "initial_import", "actor=%d apply=%t err=%v", user.ID, apply, err)
		report.Errors = append(report.Errors, "Data awal lokasi gagal diproses. Periksa file referensi dan migration 066 pada server; tidak ada perubahan yang disimpan.")
		c.JSON(http.StatusInternalServerError, gin.H{"data": report, "message": report.Errors[0]})
		return
	}
	if apply {
		logging.Infof("school_locations", "initial_import", "actor=%d imported=%d", user.ID, report.Imported)
	}
	c.Header("Cache-Control", "no-store")
	utils.RespondData(c, http.StatusOK, report)
}
