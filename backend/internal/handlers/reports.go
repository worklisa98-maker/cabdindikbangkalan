package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type ReportHandler struct {
	DB                    *db.DB
	JWTSecret             string
	FrontendOrigin        string
	ReportTokenTTLSeconds int
	ChromeExecutablePath  string
	ReportJobs            *services.ReportJobService
}

func (h ReportHandler) DownloadAkreditasiPDF(c *gin.Context) {
	h.downloadPDF(c, reportDownloadConfig{
		reportPath:  "/akreditasi?export=1",
		filenameKey: "akreditasi",
		allowedKeys: map[string]struct{}{
			"jenjang":   {},
			"status":    {},
			"school_id": {},
			"watermark": {},
		},
	})
}

func (h ReportHandler) DownloadPrestasiPDF(c *gin.Context) {
	h.downloadPDF(c, reportDownloadConfig{
		reportPath:  "/reports/prestasi",
		filenameKey: "prestasi",
		allowedKeys: map[string]struct{}{
			"year":           {},
			"jenjang":        {},
			"status_sekolah": {},
			"school_id":      {},
			"jenis_lomba":    {},
			"watermark":      {},
		},
	})
}

func (h ReportHandler) DownloadSiswaPDF(c *gin.Context) {
	tab := strings.ToLower(strings.TrimSpace(c.Query("tab")))
	if tab == "" {
		tab = "analytics"
	}
	switch tab {
	case "analytics", "pt", "kerja", "wirausaha", "belum":
	default:
		utils.RespondBadRequest(c, "Invalid tab")
		return
	}

	h.downloadPDF(c, reportDownloadConfig{
		reportPath:  "/reports/siswa",
		filenameKey: "siswa-" + tab,
		allowedKeys: map[string]struct{}{
			"tab":            {},
			"year":           {},
			"jenjang":        {},
			"status_sekolah": {},
			"school_id":      {},
			"jalur":          {},
			"q":              {},
			"start_year":     {},
			"end_year":       {},
			"watermark":      {},
		},
	})
}

func (h ReportHandler) DownloadPTRankingPDF(c *gin.Context) {
	h.downloadPDF(c, reportDownloadConfig{
		reportPath:  "/reports/pt-ranking",
		filenameKey: "pt-ranking",
		allowedKeys: map[string]struct{}{
			"year":           {},
			"jenjang":        {},
			"status_sekolah": {},
			"school_id":      {},
			"watermark":      {},
		},
	})
}

func (h ReportHandler) DownloadBundlePDF(c *gin.Context) {
	h.downloadPDF(c, reportDownloadConfig{
		reportPaths: []string{
			"/akreditasi?export=1",
			"/prestasi?export=1",
			"/siswa?export=1",
		},
		filenameKey: "bundle",
		paperWidth:  11.69,
		paperHeight: 8.27,
		allowedKeys: map[string]struct{}{
			"tab":            {},
			"status":         {},
			"year":           {},
			"jenjang":        {},
			"status_sekolah": {},
			"school_id":      {},
			"jenis_lomba":    {},
			"jalur":          {},
			"q":              {},
			"start_year":     {},
			"end_year":       {},
			"watermark":      {},
		},
	})
}

type reportDownloadConfig struct {
	reportPath  string
	reportPaths []string
	filenameKey string
	allowedKeys map[string]struct{}
	paperWidth  float64
	paperHeight float64
}

func (h ReportHandler) downloadPDF(c *gin.Context, cfg reportDownloadConfig) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !isRoleAllowedForReports(user.Role) {
		utils.RespondForbidden(c)
		return
	}

	query, err := h.buildReportQuery(c, user, cfg.allowedKeys)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		default:
			utils.RespondBadRequest(c, err.Error())
		}
		return
	}

	tokenTTL := time.Duration(h.ReportTokenTTLSeconds) * time.Second
	reportPath := cfg.reportPath
	if len(cfg.reportPaths) > 0 {
		reportPath = strings.Join(cfg.reportPaths, ",")
	}
	reportToken, err := utils.GenerateTokenWithTTL(
		h.JWTSecret,
		user.ID,
		user.Role,
		user.SchoolID,
		user.NamaInstansi,
		user.TokenVersion,
		tokenTTL,
	)
	if err != nil {
		logging.Errorf(
			"reports",
			"generate_report_token",
			"request_url=%s path=%s tab=%s err=%v",
			c.Request.URL.String(),
			reportPath,
			c.Query("tab"),
			err,
		)
		utils.RespondInternalServerError(c)
		return
	}

	wib, _ := time.LoadLocation("Asia/Jakarta")
	nowWIB := time.Now().In(wib)
	tab := strings.TrimSpace(query.Get("tab"))

	renderer := services.PDFReportRenderer{
		FrontendOrigin:       h.FrontendOrigin,
		ChromeExecutablePath: h.ChromeExecutablePath,
	}
	var content []byte
	if len(cfg.reportPaths) > 0 {
		inputs := make([]services.PDFReportInput, 0, len(cfg.reportPaths))
		for _, path := range cfg.reportPaths {
			trimmedPath := strings.TrimSpace(path)
			if trimmedPath == "" {
				continue
			}
			inputs = append(inputs, services.PDFReportInput{
				Path:        trimmedPath,
				Tab:         tab,
				Query:       query,
				Token:       reportToken,
				PaperWidth:  cfg.paperWidth,
				PaperHeight: cfg.paperHeight,
			})
		}
		content, err = renderer.RenderScreenshotPDFMulti(c.Request.Context(), inputs)
	} else {
		content, err = renderer.RenderScreenshotPDF(c.Request.Context(), services.PDFReportInput{
			Path:        cfg.reportPath,
			Tab:         tab,
			Query:       query,
			Token:       reportToken,
			PaperWidth:  cfg.paperWidth,
			PaperHeight: cfg.paperHeight,
		})
	}
	if err != nil {
		logging.Errorf(
			"reports",
			"render_pdf",
			"request_url=%s report_path=%s tab=%s query=%s err=%v",
			c.Request.URL.String(),
			reportPath,
			c.Query("tab"),
			query.Encode(),
			err,
		)
		utils.RespondInternalServerError(c)
		return
	}

	filename := fmt.Sprintf("%s-%d.pdf", cfg.filenameKey, nowWIB.Year())
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/pdf", content)
}

func (h ReportHandler) buildReportQuery(
	c *gin.Context,
	user auth.User,
	allowed map[string]struct{},
) (url.Values, error) {
	values := url.Values{}
	rawQuery := c.Request.URL.Query()

	scope, err := auth.GetMultiSchoolScope(c, h.DB, user)
	if err != nil {
		return nil, err
	}

	for key, rawValues := range rawQuery {
		if _, ok := allowed[key]; !ok {
			continue
		}
		if len(rawValues) == 0 {
			continue
		}
		value := strings.TrimSpace(rawValues[len(rawValues)-1])
		if value == "" {
			continue
		}

		if key == "school_id" {
			if auth.IsSchoolAdmin(user) {
				return nil, errors.New("school_id tidak diperbolehkan untuk SCHOOL_ADMIN")
			}
			schoolID, convErr := strconv.Atoi(value)
			if convErr != nil || schoolID <= 0 {
				return nil, errors.New("Invalid school_id")
			}
			if err := auth.EnsureSchoolInScope(scope, schoolID); err != nil {
				return nil, auth.ErrForbidden
			}
		}

		if (key == "jenjang" || key == "status_sekolah") && !auth.IsAdmin(user) {
			continue
		}

		values.Set(key, value)
	}

	return values, nil
}

func isRoleAllowedForReports(role string) bool {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case auth.RoleAdminCabdin, auth.RoleAdminMaster, auth.RoleSchoolAdmin, auth.RolePengawas:
		return true
	default:
		return false
	}
}

func resolveReportUserName(user auth.User) string {
	name := strings.TrimSpace(user.NamaInstansi)
	if name != "" {
		return name
	}
	return fmt.Sprintf("User-%d", user.ID)
}
