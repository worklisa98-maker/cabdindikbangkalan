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

type ptChoiceRuleRequest struct {
	TahunLulus *int   `json:"tahun_lulus"`
	Jalur      string `json:"jalur"`
	MaxChoices int    `json:"max_choices"`
	IsActive   *bool  `json:"is_active"`
}

type ptChoiceRuleUpdateRequest struct {
	TahunLulus *int    `json:"tahun_lulus"`
	Jalur      *string `json:"jalur"`
	MaxChoices *int    `json:"max_choices"`
	IsActive   *bool   `json:"is_active"`
}

func (h AdminHandler) ListPTChoiceRules(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	service := services.AdminService{DB: h.DB}
	payload, err := service.ListPTChoiceRules(c)
	if err != nil {
		logging.Errorf("admin", "list_pt_choice_rules", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdminHandler) CreatePTChoiceRule(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	var req ptChoiceRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	service := services.AdminService{DB: h.DB}
	item, err := service.CreatePTChoiceRule(c, services.PTChoiceRuleCreateInput{
		TahunLulus: req.TahunLulus,
		Jalur:      req.Jalur,
		MaxChoices: req.MaxChoices,
		IsActive:   req.IsActive,
	})
	if err != nil {
		switch err.Error() {
		case "INVALID_PAYLOAD", "PT_CHOICE_RULE_JALUR_INVALID", "PT_CHOICE_RULE_MAX_CHOICES_INVALID":
			utils.RespondBadRequestMessage(c, err.Error())
		case "PT_CHOICE_RULE_ALREADY_EXISTS":
			utils.RespondConflictMessage(c, err.Error())
		default:
			logging.Errorf("admin", "create_pt_choice_rule", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, item)
}

func (h AdminHandler) UpdatePTChoiceRule(c *gin.Context) {
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
		utils.RespondBadRequestMessage(c, "INVALID_PT_CHOICE_RULE_ID")
		return
	}

	var req ptChoiceRuleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	service := services.AdminService{DB: h.DB}
	item, err := service.UpdatePTChoiceRule(c, int64(id), services.PTChoiceRuleUpdateInput{
		TahunLulus: req.TahunLulus,
		Jalur:      req.Jalur,
		MaxChoices: req.MaxChoices,
		IsActive:   req.IsActive,
	})
	if err != nil {
		switch err.Error() {
		case "INVALID_PT_CHOICE_RULE_ID", "INVALID_PAYLOAD", "PT_CHOICE_RULE_JALUR_INVALID", "PT_CHOICE_RULE_MAX_CHOICES_INVALID":
			utils.RespondBadRequestMessage(c, err.Error())
		case "PT_CHOICE_RULE_ALREADY_EXISTS":
			utils.RespondConflictMessage(c, err.Error())
		case "PT_CHOICE_RULE_NOT_FOUND":
			utils.RespondError(c, http.StatusNotFound, err.Error())
		default:
			logging.Errorf("admin", "update_pt_choice_rule", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, item)
}

func (h SiswaHandler) GetPTChoiceRules(c *gin.Context) {
	_, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	tahunLulus := 0
	if raw := strings.TrimSpace(c.Query("tahun_lulus")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "tahun_lulus tidak valid.")
			return
		}
		tahunLulus = parsed
	}

	service := services.SiswaService{DB: h.DB}
	payload, err := service.GetPTChoiceRuleMap(c, tahunLulus)
	if err != nil {
		logging.Errorf("siswa", "get_pt_choice_rules", "tahun_lulus=%d err=%v", tahunLulus, err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}
