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
	"cabdin-bangkalan-backend/internal/logging"
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
	Perusahaan       string `json:"perusahaan"`
	BidangPerusahaan string `json:"bidangPerusahaan"`
	Jabatan          string `json:"jabatan"`
	TahunMulai       *int   `json:"tahunMulai"`
}

type outcomeBusinessPayload struct {
	Bidang        string `json:"bidang"`
	KategoriUsaha string `json:"kategoriUsaha"`
	NamaUsaha     string `json:"namaUsaha"`
	ProdukJasa    string `json:"produkJasa"`
	LokasiUsaha   string `json:"lokasiUsaha"`
	TahunMulai    *int   `json:"tahunMulai"`
	Keterangan    string `json:"keterangan"`
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

	filters := services.SiswaOutcomeListFilters{
		SchoolID:      requestedSchoolID,
		Status:        status,
		Jalur:         strings.TrimSpace(c.Query("jalur")),
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
		Page:          page,
		PageSize:      pageSize,
		Search:        strings.TrimSpace(c.Query("q")),
		Year:          year,
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
			logging.Errorf(
				"siswa_outcomes",
				"list",
				"err=%v status=%s school_id=%v year=%d page=%d page_size=%d",
				err,
				status,
				requestedSchoolID,
				year,
				page,
				pageSize,
			)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"items":     rows,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

func (h SiswaHandler) ListOutcomeYears(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}

	filters := services.SiswaOutcomeYearFilters{
		SchoolID:      requestedSchoolID,
		Status:        strings.TrimSpace(c.Query("status")),
		Jalur:         strings.TrimSpace(c.Query("jalur")),
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
	}

	service := services.SiswaService{DB: h.DB}
	years, err := service.ListOutcomeYears(c, user, filters)
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
			logging.Errorf(
				"siswa_outcomes",
				"list_years",
				"err=%v status=%s school_id=%v jenjang=%s status_sekolah=%s jalur=%s",
				err,
				filters.Status,
				requestedSchoolID,
				filters.Jenjang,
				filters.StatusSekolah,
				filters.Jalur,
			)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"years": years,
	})
}

func (h SiswaHandler) OutcomesImportHistory(c *gin.Context) {
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

	filters := services.SiswaOutcomeImportHistoryFilters{
		SchoolID: requestedSchoolID,
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
			Perusahaan:       req.WorkDetail.Perusahaan,
			BidangPerusahaan: req.WorkDetail.BidangPerusahaan,
			Jabatan:          req.WorkDetail.Jabatan,
			TahunMulai:       req.WorkDetail.TahunMulai,
		}
	}
	var businessDetail *services.OutcomeBusinessDetail
	if req.BusinessDetail != nil {
		businessDetail = &services.OutcomeBusinessDetail{
			Bidang:        req.BusinessDetail.Bidang,
			KategoriUsaha: req.BusinessDetail.KategoriUsaha,
			NamaUsaha:     req.BusinessDetail.NamaUsaha,
			ProdukJasa:    req.BusinessDetail.ProdukJasa,
			LokasiUsaha:   req.BusinessDetail.LokasiUsaha,
			TahunMulai:    req.BusinessDetail.TahunMulai,
			Keterangan:    req.BusinessDetail.Keterangan,
		}
	}

	service := newOutcomeUpsertService(h.DB)
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
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondNotFound(c)
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			if message, handled := mapOutcomeUpdateError(err); handled {
				utils.RespondBadRequest(c, message)
				return
			}
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
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	status := strings.ToUpper(strings.TrimSpace(c.Query("status")))
	filename := "template-final-siswa-akhir.xlsx"

	renderBaseRows := func() ([]services.SiswaPlanCohort, bool) {
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
			return nil, false
		}
		return rows, true
	}

	switch status {
	case "":
		rows, ok := renderBaseRows()
		if !ok {
			return
		}
		sheet := "Status_Akhir"
		file.SetSheetName("Sheet1", sheet)
		headers := parsers.SiswaOutcomeTemplateHeaders
		if err := setHeaderRow(file, sheet, headers); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addFixedListValidation(file, sheet, headers, "Status_Final", []string{"PT", "KERJA", "WIRAUSAHA", "BELUM"}); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addFixedListValidation(file, sheet, headers, "Jalur_Final", []string{"SNBP", "SNBT", "KEDINASAN", "MANDIRI", "LAINNYA"}); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		for i, row := range rows {
			rowValues := []interface{}{row.NISN, row.NamaSiswa, row.Rombel, row.TahunLulus, "", "", ""}
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
		applyTemplateSheetLayout(file, sheet, headers)
	case "PT":
		rows, ok := renderBaseRows()
		if !ok {
			return
		}
		sheet := "Final_PT"
		file.SetSheetName("Sheet1", sheet)
		headers := parsers.SiswaOutcomeTemplateHeaders
		if err := setHeaderRow(file, sheet, headers); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addFixedListValidation(file, sheet, headers, "Status_Final", []string{"PT"}); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addFixedListValidation(file, sheet, headers, "Jalur_Final", []string{"SNBP", "SNBT", "KEDINASAN", "MANDIRI", "LAINNYA"}); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		for i, row := range rows {
			rowValues := []interface{}{row.NISN, row.NamaSiswa, row.Rombel, row.TahunLulus, "PT", "", ""}
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
		applyTemplateSheetLayout(file, sheet, headers)
		filename = "template-final-pt.xlsx"
	case "KERJA":
		rows, ok := renderBaseRows()
		if !ok {
			return
		}
		sheet := "Final_Kerja"
		file.SetSheetName("Sheet1", sheet)
		headers := parsers.SiswaOutcomeWorkTemplateHeaders
		if err := setHeaderRow(file, sheet, headers); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		for i, row := range rows {
			rowValues := []interface{}{row.TahunLulus, row.NISN, row.NamaSiswa, row.Rombel, "", "", "", ""}
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
		applyTemplateSheetLayout(file, sheet, headers)
		filename = "template-final-kerja.xlsx"
	case "WIRAUSAHA":
		rows, total, err := service.ExportOutcomeEntrepreneurTemplateRows(c, user, filters, maxExportRows)
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
		sheet := "Final_Wirausaha"
		file.SetSheetName("Sheet1", sheet)
		headers := parsers.SiswaOutcomeEntrepreneurTemplateHeaders
		if err := setHeaderRow(file, sheet, headers); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addFixedListValidation(file, sheet, headers, "Pilihan_Final", []string{"1", "2"}); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		for i, row := range rows {
			rowValues := []interface{}{
				row.TahunLulus,
				row.NISN,
				row.NamaSiswa,
				row.Rombel,
				row.Pilihan1NamaUsaha,
				row.Pilihan1Kategori,
				row.Pilihan1ProdukJasa,
				row.Pilihan1LokasiUsaha,
				row.Pilihan2NamaUsaha,
				row.Pilihan2Kategori,
				row.Pilihan2ProdukJasa,
				row.Pilihan2LokasiUsaha,
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
		applyTemplateSheetLayout(file, sheet, headers)
		_ = addTemplateInfoSheet(file, "Isi kolom Pilihan_Final dengan 1 atau 2 sesuai usaha yang menjadi hasil final. Kosongkan jika belum ada pilihan final.")
		filename = "template-final-wirausaha.xlsx"
	case "BELUM":
		rows, ok := renderBaseRows()
		if !ok {
			return
		}
		sheet := "Final_Belum_Melanjutkan"
		file.SetSheetName("Sheet1", sheet)
		headers := parsers.SiswaOutcomeUndefinedTemplateHeaders
		if err := setHeaderRow(file, sheet, headers); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		for i, row := range rows {
			rowValues := []interface{}{row.TahunLulus, row.NISN, row.NamaSiswa, row.Rombel, ""}
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
		applyTemplateSheetLayout(file, sheet, headers)
		filename = "template-final-belum-melanjutkan.xlsx"
	default:
		utils.RespondBadRequest(c, "status wajib PT/KERJA/WIRAUSAHA/BELUM.")
		return
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h SiswaHandler) ExportOutcomes(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status == "" {
		utils.RespondBadRequest(c, "status wajib diisi.")
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}

	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}

	filters := services.SiswaOutcomeExportFilters{
		SchoolID:      requestedSchoolID,
		Status:        status,
		Year:          year,
		Search:        strings.TrimSpace(c.Query("q")),
		Jalur:         strings.TrimSpace(c.Query("jalur")),
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
	}

	const maxExportRows = 50000
	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ExportOutcomesRows(c, user, filters, maxExportRows)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrExportLimitExceeded):
			utils.RespondBadRequest(c, fmt.Sprintf("Export melebihi batas %d baris (total %d). Gunakan filter untuk memperkecil data.", maxExportRows, total))
		case errors.Is(err, services.ErrInvalidOutcome):
			utils.RespondBadRequest(c, "Filter tidak valid.")
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

	headers := []string{
		"NPSN",
		"Nama Sekolah",
		"Bentuk Pendidikan",
		"Status Sekolah",
		"NISN",
		"Nama Siswa",
		"Rombel",
		"Tahun Lulus",
		"Status Final",
		"Jalur Final",
		"PTN",
		"Prodi",
		"Jalur PT",
		"Tahun Diterima",
		"Perusahaan",
		"Jabatan",
		"Tahun Mulai Kerja",
		"Bidang Usaha",
		"Nama Usaha",
		"Tahun Mulai Usaha",
		"Finalized At",
	}

	for i, header := range headers {
		cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
		if cellErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
	}

	for i, row := range rows {
		finalizedAt := ""
		if row.FinalizedAt != nil {
			finalizedAt = row.FinalizedAt.Format("2006-01-02 15:04:05")
		}
		rowValues := []interface{}{
			row.NPSN,
			row.NamaSekolah,
			row.BentukPendidikan,
			row.StatusSekolah,
			row.NISN,
			row.NamaSiswa,
			row.Rombel,
			row.TahunLulus,
			row.StatusFinal,
			row.JalurFinal,
			row.PTN,
			row.Prodi,
			row.JalurPT,
			row.TahunDiterimaPT,
			row.Perusahaan,
			row.Jabatan,
			row.TahunMulaiKerja,
			row.BidangUsaha,
			row.NamaUsaha,
			row.TahunMulaiUsaha,
			finalizedAt,
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

	c.Header("Content-Disposition", "attachment; filename=\"siswa-outcomes.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h SiswaHandler) PTRanking(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}

	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}

	service := services.SiswaService{DB: h.DB}
	payload, err := service.PTRanking(c, user, services.SiswaPTRankingFilters{
		SchoolID:      requestedSchoolID,
		Year:          year,
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
		Limit:         2000,
	})
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

	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) OutcomesAnalytics(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "siswa_outcomes", "analytics", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	if status == "" {
		utils.RespondBadRequest(c, "status wajib diisi.")
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
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
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.AnalyticsFinalByType(ctx, user, requestedSchoolID, status, year)
	if err != nil {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		if err == auth.ErrUnauthorized {
			utils.RespondUnauthorized(c)
			return
		}
		if handleOperationalError(c, h.DB, "siswa_outcomes", "analytics", err) {
			return
		}
		respondSafeListFallback(c, "siswa_outcomes", "analytics", "Outcome analytics is temporarily unavailable.", emptyFinalStatusAnalytics(), err)
		return
	}

	if payload == nil {
		payload = emptyFinalStatusAnalytics()
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) OutcomesDashboard(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "siswa_outcomes", "dashboard", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
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
		SchoolID:      requestedSchoolID,
		Status:        strings.TrimSpace(c.Query("status")),
		Year:          year,
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
	}

	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.OutcomeDashboard(ctx, user, filters)
	if err != nil {
		if errors.Is(err, services.ErrInvalidOutcome) {
			utils.RespondBadRequest(c, "Filter tidak valid.")
			return
		}
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		if err == auth.ErrUnauthorized {
			utils.RespondUnauthorized(c)
			return
		}
		if handleOperationalError(c, h.DB, "siswa_outcomes", "dashboard", err) {
			return
		}
		logging.Errorf(
			"siswa_outcomes",
			"dashboard",
			"err=%v status=%s school_id=%v year=%d jenjang=%s status_sekolah=%s",
			err,
			filters.Status,
			filters.SchoolID,
			filters.Year,
			filters.Jenjang,
			filters.StatusSekolah,
		)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) StatusLulusanAnalyticsPT(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "siswa_outcomes", "status_lulusan_analytics_pt", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	parseYear := func(param string) (int, bool) {
		raw := strings.TrimSpace(c.Query(param))
		if raw == "" {
			return 0, true
		}
		year, err := strconv.Atoi(raw)
		if err != nil || year <= 0 {
			return 0, false
		}
		return year, true
	}

	startYear, ok := parseYear("start_year")
	if !ok {
		utils.RespondBadRequest(c, "Invalid start_year")
		return
	}
	endYear, ok := parseYear("end_year")
	if !ok {
		utils.RespondBadRequest(c, "Invalid end_year")
		return
	}
	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))
	var requestedSchoolID *int
	if user.Role == "PENGAWAS" {
		requestedSchoolID, ok = siswaRequestedSchoolIDFromQuery(c)
		if !ok {
			return
		}
	}

	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.GetStatusLulusanPTAnalytics(ctx, user, services.StatusLulusanPTAnalyticsFilters{
		StartYear: startYear,
		EndYear:   endYear,
		Jenjang:   strings.TrimSpace(c.Query("jenjang")),
		SchoolID:  requestedSchoolID,
		Search:    strings.TrimSpace(c.Query("q")),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		switch {
		case handleOperationalError(c, h.DB, "siswa_outcomes", "status_lulusan_analytics_pt", err):
			return
		case errors.Is(err, services.ErrInvalidOutcome):
			utils.RespondBadRequest(c, "Filter tidak valid.")
		case isMissingRelation(err):
			jenjang := strings.TrimSpace(c.Query("jenjang"))
			if user.Role == "SCHOOL_ADMIN" {
				jenjang = ""
			}
			respondSafeAnalyticsFallback(c, "siswa_outcomes", "status_lulusan_analytics_pt", "Status lulusan PT analytics is temporarily unavailable.", buildEmptyStatusLulusanPTAnalytics(startYear, endYear, jenjang, page, pageSize), err)
		case isStatusLulusanPTAnalyticsEmptyError(err):
			jenjang := strings.TrimSpace(c.Query("jenjang"))
			if user.Role == "SCHOOL_ADMIN" {
				jenjang = ""
			}
			utils.RespondData(c, http.StatusOK, buildEmptyStatusLulusanPTAnalytics(startYear, endYear, jenjang, page, pageSize))
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			jenjang := strings.TrimSpace(c.Query("jenjang"))
			if user.Role == "SCHOOL_ADMIN" {
				jenjang = ""
			}
			respondSafeAnalyticsFallback(c, "siswa_outcomes", "status_lulusan_analytics_pt", "Status lulusan PT analytics is temporarily unavailable.", buildEmptyStatusLulusanPTAnalytics(startYear, endYear, jenjang, page, pageSize), err)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}
