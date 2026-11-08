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

type upsertStudentYearSummaryRequest struct {
	SchoolID        *int `json:"school_id"`
	TahunLulus      int  `json:"tahun_lulus"`
	TotalSiswaAkhir int  `json:"total_siswa_akhir"`
}

type upsertPTRegistrationCountsRequest struct {
	SchoolID   *int           `json:"school_id"`
	TahunLulus int            `json:"tahun_lulus"`
	Counts     map[string]int `json:"counts"`
}

type upsertEntrepreneurPlanCountRequest struct {
	SchoolID     *int `json:"school_id"`
	TahunLulus   int  `json:"tahun_lulus"`
	PlannedCount int  `json:"planned_count"`
}

func parseOptionalYearQuery(c *gin.Context) (int, bool) {
	raw := strings.TrimSpace(c.Query("year"))
	if raw == "" {
		raw = strings.TrimSpace(c.Query("tahun_lulus"))
	}
	if raw == "" {
		return 0, true
	}
	year, err := strconv.Atoi(raw)
	if err != nil || year <= 0 {
		utils.RespondBadRequest(c, "Invalid year")
		return 0, false
	}
	return year, true
}

func schoolIDForWrite(user auth.User, requested *int) (int, error) {
	scope, err := auth.GetSingleSchoolScopeV2(user, requested)
	if err != nil {
		return 0, err
	}
	if scope == nil {
		return 0, auth.ErrForbidden
	}
	return *scope, nil
}

func (h SiswaHandler) ListStudentYearSummaries(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}
	year, ok := parseOptionalYearQuery(c)
	if !ok {
		return
	}

	service := services.SiswaService{DB: h.DB}
	rows, err := service.ListStudentYearSummaries(c, user, requestedSchoolID, year)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		case errors.Is(err, auth.ErrUnauthorized):
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"items": rows})
}

func (h SiswaHandler) UpsertStudentYearSummary(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	var req upsertStudentYearSummaryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	schoolID, err := schoolIDForWrite(user, req.SchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
		} else {
			utils.RespondUnauthorized(c)
		}
		return
	}

	service := services.SiswaService{DB: h.DB}
	item, err := service.UpsertStudentYearSummary(c, user, services.UpsertStudentYearSummaryInput{
		SchoolID:        schoolID,
		TahunLulus:      req.TahunLulus,
		TotalSiswaAkhir: req.TotalSiswaAkhir,
		UpdatedBy:       user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidPlan):
			utils.RespondBadRequest(c, "Tahun lulus dan jumlah siswa akhir wajib valid.")
		case errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, item)
}

func (h SiswaHandler) ListPTRegistrationCounts(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}
	year, ok := parseOptionalYearQuery(c)
	if !ok {
		return
	}

	service := services.SiswaService{DB: h.DB}
	rows, err := service.ListPTRegistrationSummary(c, user, requestedSchoolID, year)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		case errors.Is(err, auth.ErrUnauthorized):
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"items": rows})
}

func (h SiswaHandler) UpsertPTRegistrationCounts(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	var req upsertPTRegistrationCountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	schoolID, err := schoolIDForWrite(user, req.SchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
		} else {
			utils.RespondUnauthorized(c)
		}
		return
	}

	service := services.SiswaService{DB: h.DB}
	item, err := service.UpsertPTRegistrationCounts(c, user, services.UpsertPTRegistrationCountsInput{
		SchoolID:   schoolID,
		TahunLulus: req.TahunLulus,
		Counts:     req.Counts,
		UpdatedBy:  user.ID,
	})
	if err != nil {
		if message, ok := services.ValidationMessage(err); ok {
			utils.RespondBadRequest(c, message)
			return
		}
		switch {
		case errors.Is(err, services.ErrInvalidPlan):
			utils.RespondBadRequest(c, "Jumlah pendaftar PT wajib valid.")
		case errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, item)
}

func (h SiswaHandler) ListEntrepreneurPlanCounts(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}
	year, ok := parseOptionalYearQuery(c)
	if !ok {
		return
	}

	service := services.SiswaService{DB: h.DB}
	rows, err := service.ListEntrepreneurPlanSummaries(c, user, requestedSchoolID, year)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		case errors.Is(err, auth.ErrUnauthorized):
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"items": rows})
}

func (h SiswaHandler) UpsertEntrepreneurPlanCount(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	var req upsertEntrepreneurPlanCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	schoolID, err := schoolIDForWrite(user, req.SchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
		} else {
			utils.RespondUnauthorized(c)
		}
		return
	}

	service := services.SiswaService{DB: h.DB}
	item, err := service.UpsertEntrepreneurPlanCount(c, user, services.UpsertEntrepreneurPlanCountInput{
		SchoolID:     schoolID,
		TahunLulus:   req.TahunLulus,
		PlannedCount: req.PlannedCount,
		UpdatedBy:    user.ID,
	})
	if err != nil {
		if message, ok := services.ValidationMessage(err); ok {
			utils.RespondBadRequest(c, message)
			return
		}
		switch {
		case errors.Is(err, services.ErrInvalidPlan):
			utils.RespondBadRequest(c, "Tahun lulus dan jumlah rencana wirausaha wajib valid.")
		case errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, item)
}
