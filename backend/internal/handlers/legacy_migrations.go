package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type LegacyMigrationHandler struct {
	DB *db.DB
}

func (h LegacyMigrationHandler) LegacyPTStatus(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	run := false
	if raw := strings.TrimSpace(c.Query("run")); raw != "" {
		if raw == "1" || strings.EqualFold(raw, "true") {
			run = true
		}
	}

	service := services.LegacyPTMigrationService{DB: h.DB}
	status, err := service.GetStatus(c, run)
	if err != nil {
		logging.Errorf("legacy_pt_migration", "status", "err=%v", err)
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusInternalServerError, utils.APIResponse{
			Error:   "INTERNAL_SERVER_ERROR",
			Message: "Legacy PT migration status failed",
		})
		return
	}
	utils.RespondData(c, http.StatusOK, status)
}

func (h LegacyMigrationHandler) LegacyPTReport(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	service := services.LegacyPTMigrationService{DB: h.DB}
	status, err := service.GetStatus(c, false)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if strings.TrimSpace(status.ReportPath) == "" {
		utils.RespondNotFound(c)
		return
	}

	if _, statErr := os.Stat(status.ReportPath); statErr != nil {
		utils.RespondNotFound(c)
		return
	}

	filename := filepath.Base(status.ReportPath)
	if filename == "" {
		filename = "legacy-pt-migration-report.xlsx"
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.File(status.ReportPath)
}

func (h LegacyMigrationHandler) PTOutcomeOfferAudit(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.AuditPTOutcomeOfferBackfill(c)
	if err != nil {
		logging.Errorf("pt_outcome_offer_backfill", "audit", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, result)
}

func (h LegacyMigrationHandler) PTOutcomeOfferBackfill(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.BackfillLegacyPTOutcomeOffers(c)
	if err != nil {
		logging.Errorf("pt_outcome_offer_backfill", "run", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, result)
}
