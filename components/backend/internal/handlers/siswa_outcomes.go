package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type outcomeUpdateRequest struct {
	FinalStatus    string                  `json:"finalStatus"`
	FinalPath      string                  `json:"finalPath"`
	PtDetail       *outcomePTPayload       `json:"ptDetail"`
	WorkDetail     *outcomeWorkPayload     `json:"workDetail"`
	BusinessDetail *outcomeBusinessPayload `json:"businessDetail"`
	Reason         string                  `json:"reason"`
}

type outcomePTPayload struct {
	Kampus        string `json:"kampus"`
	Prodi         string `json:"prodi"`
	TahunDiterima *int   `json:"tahunDiterima"`
}

type outcomeWorkPayload struct {
	Perusahaan string `json:"perusahaan"`
	Jabatan    string `json:"jabatan"`
	TahunMulai *int   `json:"tahunMulai"`
}

type outcomeBusinessPayload struct {
	Bidang     string `json:"bidang"`
	NamaUsaha  string `json:"namaUsaha"`
	TahunMulai *int   `json:"tahunMulai"`
}

func (h SiswaHandler) ImportPlans(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	scope := siswaResolveScope(c, user, true)
	if scope == nil {
		return
	}
	schoolID := *scope

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	foundHeaders, ok, err := parsers.ValidateExactHeaders(buffer, "", parsers.SiswaPlanTemplateHeaders)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if !ok {
		respondSiswaHeaderMismatch(c, parsers.SiswaPlanTemplateHeaders, foundHeaders)
		return
	}

	parsed, err := parsers.ParseSiswaPlans(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportPlans(c, schoolID, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportOutcomes(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	scope := siswaResolveScope(c, user, true)
	if scope == nil {
		return
	}
	schoolID := *scope

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	foundHeaders, ok, err := parsers.ValidateExactHeaders(buffer, "", parsers.SiswaOutcomeTemplateHeaders)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if !ok {
		respondSiswaHeaderMismatch(c, parsers.SiswaOutcomeTemplateHeaders, foundHeaders)
		return
	}

	parsed, err := parsers.ParseSiswaOutcomes(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportOutcomes(c, user, schoolID, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ListOutcomes(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status == "" {
		utils.RespondBadRequest(c, "status wajib diisi.")
		return
	}

	rawSchoolID := strings.TrimSpace(c.Query("school_id"))
	scope := siswaResolveScope(c, user, false)
	if rawSchoolID != "" && scope == nil {
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

	filters := services.SiswaOutcomeListFilters{
		SchoolID: scope,
		Status:   status,
		Jalur:    strings.TrimSpace(c.Query("jalur")),
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
		Page:     page,
		PageSize: pageSize,
		Search:   strings.TrimSpace(c.Query("q")),
		Year:     year,
	}

	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListOutcomes(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			if errors.Is(err, services.ErrInvalidOutcome) {
				utils.RespondBadRequest(c, "Filter tidak valid.")
				return
			}
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"data": rows,
		"meta": gin.H{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
	})
}

func (h SiswaHandler) OutcomesImportHistory(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	rawSchoolID := strings.TrimSpace(c.Query("school_id"))
	scope := siswaResolveScope(c, user, false)
	if rawSchoolID != "" && scope == nil {
		return
	}
	if scope == nil && user.Role == "SCHOOL_ADMIN" {
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

	filters := services.SiswaOutcomeImportHistoryFilters{
		SchoolID: scope,
		Page:     page,
		PageSize: pageSize,
		Search:   strings.TrimSpace(c.Query("q")),
		Year:     year,
	}

	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListOutcomeImportHistory(c, user, filters)
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

	utils.RespondData(c, http.StatusOK, gin.H{
		"data": rows,
		"meta": gin.H{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
	})
}

func (h SiswaHandler) GetOutcome(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	studentID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || studentID <= 0 {
		utils.RespondBadRequest(c, "Invalid student id")
		return
	}

	service := services.SiswaService{DB: h.DB}
	student, outcome, err := service.GetOutcomeDetail(c, user, studentID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondNotFound(c)
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"student": gin.H{
			"id":         student.ID,
			"nisn":       student.NISN,
			"namaSiswa":  student.NamaSiswa,
			"rombel":     student.Rombel,
			"tahunLulus": student.TahunLulus,
		},
		"outcome": outcome,
	})
}

func (h SiswaHandler) UpdateOutcome(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	studentID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || studentID <= 0 {
		utils.RespondBadRequest(c, "Invalid student id")
		return
	}

	var req outcomeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	var ptDetail *services.OutcomePTDetail
	if req.PtDetail != nil {
		ptDetail = &services.OutcomePTDetail{
			Kampus:        req.PtDetail.Kampus,
			Prodi:         req.PtDetail.Prodi,
			TahunDiterima: req.PtDetail.TahunDiterima,
		}
	}
	var workDetail *services.OutcomeWorkDetail
	if req.WorkDetail != nil {
		workDetail = &services.OutcomeWorkDetail{
			Perusahaan: req.WorkDetail.Perusahaan,
			Jabatan:    req.WorkDetail.Jabatan,
			TahunMulai: req.WorkDetail.TahunMulai,
		}
	}
	var businessDetail *services.OutcomeBusinessDetail
	if req.BusinessDetail != nil {
		businessDetail = &services.OutcomeBusinessDetail{
			Bidang:     req.BusinessDetail.Bidang,
			NamaUsaha:  req.BusinessDetail.NamaUsaha,
			TahunMulai: req.BusinessDetail.TahunMulai,
		}
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.UpsertOutcome(c, user, studentID, services.SiswaOutcomePayload{
		FinalStatus:    req.FinalStatus,
		FinalPath:      req.FinalPath,
		PtDetail:       ptDetail,
		WorkDetail:     workDetail,
		BusinessDetail: businessDetail,
		Reason:         req.Reason,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrOutcomeYearLocked):
			utils.RespondBadRequest(c, "Perubahan status akhir hanya diperbolehkan pada tahun berjalan.")
		case errors.Is(err, services.ErrOutcomeFinalLocked):
			utils.RespondBadRequest(c, "Data sudah final dan terkunci.")
		case errors.Is(err, services.ErrInvalidOutcome):
			utils.RespondBadRequest(c, "Payload tidak valid.")
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondNotFound(c)
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"status":            "ok",
		"auto_plan_updated": result.AutoPlanUpdated,
		"warning":           result.Warning,
	})
}

func (h SiswaHandler) ExportOutcomeTemplate(c *gin.Context) {
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
	rows, total, err := service.ExportOutcomeTemplateRows(c, user, filters, maxExportRows)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrExportLimitExceeded):
			utils.RespondBadRequest(c, fmt.Sprintf("Export melebihi batas %d baris (total %d). Gunakan filter untuk memperkecil data.", maxExportRows, total))
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	file := excelize.NewFile()
	sheet := "Status_Akhir"
	file.SetSheetName("Sheet1", sheet)

	headers := parsers.SiswaOutcomeTemplateHeaders
	if err := setHeaderRow(file, sheet, headers); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidation(file, sheet, headers, "Final_Status", []string{"PT", "KERJA", "WIRAUSAHA", "BELUM"}); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidation(file, sheet, headers, "Final_Path", []string{"SNBP", "SNBT", "KEDINASAN", "MANDIRI", "LAINNYA"}); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	for i, row := range rows {
		rowValues := []interface{}{
			row.NISN,
			row.NamaSiswa,
			row.Rombel,
			row.TahunLulus,
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
		}
		cell, cellErr := excelize.CoordinatesToCellName(1, i+2)
		if cellErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\"template-status-akhir.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h SiswaHandler) OutcomesAnalytics(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status == "" {
		utils.RespondBadRequest(c, "status wajib diisi.")
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil && user.Role == "SCHOOL_ADMIN" {
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

	service := services.SiswaService{DB: h.DB}
	payload, err := service.AnalyticsFinalByType(c, user, scope, status, year)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	if payload == nil {
		payload = []services.SiswaFinalStatusAnalytics{}
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) OutcomesDashboard(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil && user.Role == "SCHOOL_ADMIN" {
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

	filters := services.SiswaOutcomeDashboardFilters{
		SchoolID: scope,
		Status:   strings.TrimSpace(c.Query("status")),
		Year:     year,
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
	}

	service := services.SiswaService{DB: h.DB}
	payload, err := service.OutcomeDashboard(c, user, filters)
	if err != nil {
		if errors.Is(err, services.ErrInvalidOutcome) {
			utils.RespondBadRequest(c, "Filter tidak valid.")
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}
