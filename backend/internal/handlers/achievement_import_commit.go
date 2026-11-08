package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

// CommitImport validates and inserts accepted Achievement (prestasi) rows.
// Safety: re-validates on every request, does not rely on FE.
func (h AchievementHandler) CommitImport(c *gin.Context) {
	logging.Infof("achievement", "commit_import", "event=hit")
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

	var jenjangPendidikan string
	if err := h.DB.Pool.QueryRow(c, `
        SELECT COALESCE(bentuk_pendidikan, '')
        FROM data_sekolah
        WHERE id = $1
    `, *user.SchoolID).Scan(&jenjangPendidikan); err != nil {
		utils.RespondBadRequestMessage(c, "Sekolah tidak ditemukan")
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
		logging.Errorf("achievement", "commit_import", "err=%v", err)
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

	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		logging.Errorf("achievement", "commit_import", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	defer func() { _ = tx.Rollback(c) }()

	result, err := services.CommitPrestasiImport(
		c,
		tx,
		buffer,
		*user.SchoolID,
		strings.TrimSpace(jenjangPendidikan),
		services.ValidateOptions{IgnoreIDPrestasi: true},
		services.NewPrestasiDBChecker(h.DB),
	)
	if err != nil {
		// Most validation errors are returned as 200 with rejected_rows; here only hard failures.
		if errors.Is(err, auth.ErrForbidden) || errors.Is(err, auth.ErrUnauthorized) {
			utils.RespondForbidden(c)
			return
		}
		logging.Errorf("achievement", "commit_import", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	if err := tx.Commit(c); err != nil {
		logging.Errorf("achievement", "commit_import", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.JSON(http.StatusOK, result)
}
