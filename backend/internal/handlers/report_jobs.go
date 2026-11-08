package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type reportJobResponse struct {
	JobID        string `json:"job_id"`
	Status       string `json:"status"`
	Progress     int    `json:"progress"`
	ErrorMessage string `json:"error_message,omitempty"`
	DownloadURL  string `json:"download_url,omitempty"`
}

func (h ReportHandler) CreateBundleJob(c *gin.Context) {
	service := h.ReportJobs
	if service == nil {
		utils.RespondInternalServerError(c)
		return
	}

	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !isRoleAllowedForReports(user.Role) {
		utils.RespondForbidden(c)
		return
	}

	query, err := h.buildReportQuery(c, user, bundleReportAllowedKeys())
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		default:
			utils.RespondBadRequest(c, err.Error())
		}
		return
	}

	params := map[string]string{}
	for key, values := range query {
		if len(values) == 0 {
			continue
		}
		value := strings.TrimSpace(values[len(values)-1])
		if value == "" {
			continue
		}
		params[key] = value
	}

	forceRegenerate := parseForceQuery(c.Query("force"))

	job, err := service.CreateOrReuseBundleJob(c.Request.Context(), user, params, forceRegenerate)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, toReportJobResponse(job))
}

func parseForceQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func (h ReportHandler) GetReportJobStatus(c *gin.Context) {
	service := h.ReportJobs
	if service == nil {
		utils.RespondInternalServerError(c)
		return
	}

	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	jobID := strings.TrimSpace(c.Param("id"))
	if jobID == "" {
		utils.RespondBadRequest(c, "Invalid job id")
		return
	}

	job, err := service.GetJobByID(c.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondNotFound(c)
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	if !service.CanUserAccessJob(user, job) {
		utils.RespondForbidden(c)
		return
	}

	utils.RespondData(c, http.StatusOK, toReportJobResponse(job))
}

func (h ReportHandler) DownloadReportJob(c *gin.Context) {
	service := h.ReportJobs
	if service == nil {
		utils.RespondInternalServerError(c)
		return
	}

	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	jobID := strings.TrimSpace(c.Param("id"))
	if jobID == "" {
		utils.RespondBadRequest(c, "Invalid job id")
		return
	}

	job, err := service.GetJobByID(c.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondNotFound(c)
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	if !service.CanUserAccessJob(user, job) {
		utils.RespondForbidden(c)
		return
	}

	if strings.TrimSpace(job.Status) != services.ReportJobStatusDone {
		utils.RespondConflictMessage(c, "JOB_NOT_READY")
		return
	}
	if job.ResultPath == nil || strings.TrimSpace(*job.ResultPath) == "" {
		utils.RespondNotFound(c)
		return
	}

	absolutePath, err := service.ResolveReportFilePath(*job.ResultPath)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if _, err := os.Stat(absolutePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			utils.RespondNotFound(c)
			return
		}
		utils.RespondInternalServerError(c)
		return
	}

	filename := fmt.Sprintf("sipres-bundle-%s.pdf", time.Now().Format("20060102-1504"))
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.File(absolutePath)
}

func bundleReportAllowedKeys() map[string]struct{} {
	return map[string]struct{}{
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
	}
}

func toReportJobResponse(job services.ReportJob) reportJobResponse {
	response := reportJobResponse{
		JobID:    strings.TrimSpace(job.ID),
		Status:   strings.TrimSpace(job.Status),
		Progress: job.Progress,
	}
	if errMsg := job.ErrorMessage; errMsg != nil && strings.TrimSpace(*errMsg) != "" {
		response.ErrorMessage = strings.TrimSpace(*errMsg)
	}
	if response.Status == services.ReportJobStatusDone {
		response.DownloadURL = "/api/reports/jobs/" + response.JobID + "/download"
	}
	return response
}
