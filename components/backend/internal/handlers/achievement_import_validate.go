package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

// ValidateImport validates an Achievement (prestasi) Excel without inserting any row.
// This is intended for SCHOOL_ADMIN to preview accepted/rejected rows + reasons.
func (h AchievementHandler) ValidateImport(c *gin.Context) {
	logging.Infof("achievement", "validate_import", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}
	if user.SchoolID == nil {
		utils.RespondUnauthorized(c)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "BAD_REQUEST",
			"message": "File Excel wajib diunggah.",
		})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		utils.RespondBadRequestMessage(c, "Gagal membaca file Excel.")
		return
	}
	defer file.Close()

	buffer, err := io.ReadAll(file)
	if err != nil {
		utils.RespondBadRequestMessage(c, "Gagal membaca file Excel.")
		return
	}

	_, ok, err := parsers.ValidateExactHeaders(
		buffer,
		"Achievement",
		parsers.AchievementTemplateHeaders,
	)
	if err != nil {
		logging.Errorf("achievement", "validate_import", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "HEADER_MISMATCH",
			"message": "Header tidak sesuai template. Silakan unduh template dan sesuaikan.",
		})
		return
	}

	result, err := services.ValidatePrestasiImport(
		c,
		buffer,
		*user.SchoolID,
		services.ValidateOptions{IgnoreIDPrestasi: true},
		services.NewPrestasiDBChecker(h.DB),
	)
	if err != nil {
		logging.Errorf("achievement", "validate_import", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.JSON(http.StatusOK, result)
}
