package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type refKampusCreateRequest struct {
	Nama string `json:"nama"`
}

type refKampusUpdateRequest struct {
	Nama     *string `json:"nama"`
	IsActive *bool   `json:"isActive"`
}

func (h AdminHandler) ListRefKampus(c *gin.Context) {
	logging.Infof("admin", "list_ref_kampus", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	limit := 200
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, convErr := strconv.Atoi(raw); convErr == nil {
			limit = parsed
		}
	}
	includeInactive := strings.EqualFold(strings.TrimSpace(c.Query("includeInactive")), "true") ||
		strings.TrimSpace(c.Query("includeInactive")) == "1"

	service := services.AdminService{DB: h.DB}
	items, err := service.ListManagedRefKampus(c, services.RefKampusAdminFilters{
		Query:           strings.TrimSpace(c.Query("q")),
		Limit:           limit,
		IncludeInactive: includeInactive,
	})
	if err != nil {
		logging.Errorf("admin", "list_ref_kampus", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, items)
}

func (h AdminHandler) CreateRefKampus(c *gin.Context) {
	logging.Infof("admin", "create_ref_kampus", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	var req refKampusCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	service := services.AdminService{DB: h.DB}
	item, err := service.CreateManagedRefKampus(c, services.RefKampusCreateInput{
		Nama: req.Nama,
	})
	if err != nil {
		switch err.Error() {
		case "REF_KAMPUS_NAME_REQUIRED", "INVALID_PAYLOAD":
			utils.RespondBadRequestMessage(c, err.Error())
		case "REF_KAMPUS_NAME_EXISTS":
			utils.RespondConflictMessage(c, err.Error())
		default:
			logging.Errorf("admin", "create_ref_kampus", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, item)
}

func (h AdminHandler) UpdateRefKampus(c *gin.Context) {
	logging.Infof("admin", "update_ref_kampus", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || id <= 0 {
		utils.RespondBadRequestMessage(c, "INVALID_REF_KAMPUS_ID")
		return
	}

	var req refKampusUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	service := services.AdminService{DB: h.DB}
	item, err := service.UpdateManagedRefKampus(c, id, services.RefKampusUpdateInput{
		Nama:     req.Nama,
		IsActive: req.IsActive,
	})
	if err != nil {
		switch err.Error() {
		case "INVALID_REF_KAMPUS_ID", "INVALID_PAYLOAD", "REF_KAMPUS_NAME_REQUIRED":
			utils.RespondBadRequestMessage(c, err.Error())
		case "REF_KAMPUS_NAME_EXISTS":
			utils.RespondConflictMessage(c, err.Error())
		case "REF_KAMPUS_NOT_FOUND":
			utils.RespondError(c, http.StatusNotFound, err.Error())
		default:
			logging.Errorf("admin", "update_ref_kampus", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, item)
}
