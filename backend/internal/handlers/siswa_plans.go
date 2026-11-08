package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type planListResponse struct {
	ID          int64      `json:"id"`
	SchoolID    int        `json:"school_id"`
	SchoolName  string     `json:"school_name"`
	NISN        string     `json:"nisn"`
	NamaSiswa   string     `json:"namaSiswa"`
	Rombel      string     `json:"rombel"`
	TahunLulus  int        `json:"tahunLulus"`
	Status      string     `json:"status"`
	FinalStatus string     `json:"final_status"`
	FinalPath   string     `json:"final_path"`
	FinalizedAt *time.Time `json:"finalized_at"`
}

type planDetailResponse struct {
	PlanJalur   []string            `json:"planJalur"`
	PlanChoices []planChoicePayload `json:"planChoices"`
	PlanSource  string              `json:"planSource"`
}

type planChoicePayload struct {
	Kampus string `json:"kampus"`
	Prodi  string `json:"prodi"`
}

type planUpdateRequest struct {
	PlanJalur   []string            `json:"planJalur"`
	PlanChoices []planChoicePayload `json:"planChoices"`
}

func (h SiswaHandler) ListPlans(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))
	if pageSize <= 0 {
		pageSize = 10
	}
	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}
	includeFinalized := false
	if raw := strings.TrimSpace(c.Query("includeFinalized")); raw != "" {
		includeFinalized = strings.EqualFold(raw, "true") || raw == "1"
	}
	if includeFinalized && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
		includeFinalized = false
	}

	filters := services.SiswaPlanListFilters{
		SchoolID:         requestedSchoolID,
		Page:             page,
		PageSize:         pageSize,
		Search:           strings.TrimSpace(c.Query("q")),
		Year:             year,
		IncludeFinalized: includeFinalized,
	}

	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListPlansV2(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]planListResponse, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, planListResponse{
			ID:          row.ID,
			SchoolID:    row.SchoolID,
			SchoolName:  row.SchoolName,
			NISN:        row.NISN,
			NamaSiswa:   row.NamaSiswa,
			Rombel:      row.Rombel,
			TahunLulus:  row.TahunLulus,
			Status:      row.Status,
			FinalStatus: row.FinalStatus,
			FinalPath:   row.FinalPath,
			FinalizedAt: row.FinalizedAt,
		})
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"data": payload,
		"meta": gin.H{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
	})
}

func (h SiswaHandler) GetPlans(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	cohortID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || cohortID <= 0 {
		utils.RespondBadRequest(c, "Invalid cohort id")
		return
	}

	service := services.SiswaService{DB: h.DB}
	cohort, plan, status, err := service.GetStudentPlan(c, user, cohortID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	var planPayload *planDetailResponse
	if plan != nil {
		choices := make([]planChoicePayload, 0, len(plan.PlanChoices))
		for _, choice := range plan.PlanChoices {
			choices = append(choices, planChoicePayload{
				Kampus: choice.Kampus,
				Prodi:  choice.Prodi,
			})
		}
		planPayload = &planDetailResponse{
			PlanJalur:   plan.PlanJalur,
			PlanChoices: choices,
			PlanSource:  plan.PlanSource,
		}
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"student": gin.H{
			"id":         cohort.ID,
			"nisn":       cohort.NISN,
			"namaSiswa":  cohort.NamaSiswa,
			"rombel":     cohort.Rombel,
			"tahunLulus": cohort.TahunLulus,
		},
		"status": status,
		"plan":   planPayload,
	})
}

func (h SiswaHandler) UpdatePlans(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	cohortID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || cohortID <= 0 {
		utils.RespondBadRequest(c, "Invalid cohort id")
		return
	}

	var req planUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	choices := make([]services.SiswaPlanChoice, 0, len(req.PlanChoices))
	for _, choice := range req.PlanChoices {
		choices = append(choices, services.SiswaPlanChoice{
			Kampus: choice.Kampus,
			Prodi:  choice.Prodi,
		})
	}

	service := services.SiswaService{DB: h.DB}
	if err := service.UpsertStudentPlan(c, user, cohortID, services.SiswaPlanRecord{
		PlanJalur:   req.PlanJalur,
		PlanChoices: choices,
	}); err != nil {
		if message, ok := services.ValidationMessage(err); ok {
			utils.RespondBadRequest(c, message)
			return
		}
		switch {
		case errors.Is(err, services.ErrPlanYearLocked):
			utils.RespondBadRequest(c, "Rencana lulusan hanya dapat diubah pada tahun berjalan.")
		case errors.Is(err, services.ErrInvalidPlan):
			utils.RespondBadRequest(c, "Payload tidak valid.")
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{"status": "ok"})
}

func (h SiswaHandler) ExportPlans(c *gin.Context) {
	logging.Infof("siswa_plans", "export", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) && user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid school_id")
			return
		}
		requestedSchoolID = &id
	}
	if user.Role != "SCHOOL_ADMIN" && requestedSchoolID == nil {
		utils.RespondBadRequest(c, "school_id wajib diisi.")
		return
	}

	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}

	filters := services.SiswaPlanListFilters{
		SchoolID: requestedSchoolID,
		Year:     year,
	}

	const maxExportRows = 50000
	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ExportPlanTemplateRows(c, user, filters, maxExportRows)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrExportLimitExceeded):
			utils.RespondBadRequest(c, fmt.Sprintf("Export melebihi batas %d baris (total %d). Gunakan filter untuk memperkecil data.", maxExportRows, total))
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("siswa_plans", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	file := excelize.NewFile()
	sheet := "Rencana_Lulusan"
	file.SetSheetName("Sheet1", sheet)

	headers := parsers.SiswaPlanTemplateHeaders
	if err := setHeaderRow(file, sheet, headers); err != nil {
		logging.Errorf("siswa_plans", "export", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidation(file, sheet, headers, "Plan_Jalur", []string{"SNBP", "SNBT", "KEDINASAN", "MANDIRI", "LAINNYA"}); err != nil {
		logging.Errorf("siswa_plans", "export", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	campuses, err := loadCampusListFromDB(c.Request.Context(), h.DB)
	if err != nil {
		logging.Errorf("siswa_plans", "export", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if len(campuses) > 0 {
		if err := addCampusLookupNamedRange(file, "LOOKUP_KAMPUS", campuses); err != nil {
			logging.Errorf("siswa_plans", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		if err := addNamedRangeValidationRange(file, sheet, "F2:F2000", "KAMPUS_LIST"); err != nil {
			logging.Errorf("siswa_plans", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		if err := addNamedRangeValidationRange(file, sheet, "H2:H2000", "KAMPUS_LIST"); err != nil {
			logging.Errorf("siswa_plans", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		if err := addNamedRangeValidationRange(file, sheet, "J2:J2000", "KAMPUS_LIST"); err != nil {
			logging.Errorf("siswa_plans", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		if err := addNamedRangeValidationRange(file, sheet, "L2:L2000", "KAMPUS_LIST"); err != nil {
			logging.Errorf("siswa_plans", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	for i, row := range rows {
		rowValues := []interface{}{
			row.NISN,
			row.NamaSiswa,
			row.Rombel,
			row.TahunLulus,
			row.PlanJalur,
			row.Pilihan1Kampus,
			row.Pilihan1Prodi,
			row.Pilihan2Kampus,
			row.Pilihan2Prodi,
			row.Pilihan3Kampus,
			row.Pilihan3Prodi,
			row.Pilihan4Kampus,
			row.Pilihan4Prodi,
		}
		cell, cellErr := excelize.CoordinatesToCellName(1, i+2)
		if cellErr != nil {
			logging.Errorf("siswa_plans", "export", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			logging.Errorf("siswa_plans", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		logging.Errorf("siswa_plans", "export", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\"template-rencana-lulusan.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}
