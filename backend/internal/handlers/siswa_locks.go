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

type graduationYearLockRequest struct {
	SchoolID   *int `json:"school_id"`
	TahunLulus int  `json:"tahun_lulus"`
}

func (h SiswaHandler) GetGraduationYearLock(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	switch user.Role {
	case auth.RoleAdminCabdin, auth.RoleAdminMaster, auth.RoleSchoolAdmin, auth.RolePengawas:
	default:
		utils.RespondForbidden(c)
		return
	}

	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid school_id")
			return
		}
		requestedSchoolID = &parsed
	}

	rawYear := strings.TrimSpace(c.Query("tahun_lulus"))
	if rawYear == "" {
		utils.RespondBadRequest(c, "tahun_lulus wajib diisi.")
		return
	}
	tahunLulus, convErr := strconv.Atoi(rawYear)
	if convErr != nil || tahunLulus <= 0 {
		utils.RespondBadRequest(c, "Invalid tahun_lulus")
		return
	}

	scope, scopeErr := auth.GetMultiSchoolScope(c, h.DB, user)
	if scopeErr != nil {
		if errors.Is(scopeErr, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	if requestedSchoolID != nil {
		if ensureErr := auth.EnsureSchoolInScope(scope, *requestedSchoolID); ensureErr != nil {
			utils.RespondForbidden(c)
			return
		}
	}

	service := services.SiswaService{DB: h.DB}
	lock, err := service.GetGraduationYearLockSummary(c, scope, requestedSchoolID, tahunLulus)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, lock)
}

func (h SiswaHandler) CloseGraduationYear(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	var req graduationYearLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	if req.SchoolID != nil && *req.SchoolID <= 0 {
		utils.RespondBadRequest(c, "Invalid school_id")
		return
	}
	if req.TahunLulus <= 0 {
		utils.RespondBadRequest(c, "tahun_lulus wajib diisi.")
		return
	}

	service := services.SiswaService{DB: h.DB}
	lock, err := service.CloseGraduationYear(c, req.SchoolID, req.TahunLulus, user.ID)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, lock)
}
