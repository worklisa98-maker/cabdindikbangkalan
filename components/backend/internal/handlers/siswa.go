package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type SiswaHandler struct {
	DB *db.DB
}

const siswaTemplateMismatchMessage = "Header template tidak sesuai. Silakan download template resmi dan isi sesuai kolom."

type finalizeSiswaRequest struct {
	SchoolID   *int   `json:"school_id"`
	NISN       string `json:"nisn"`
	TahunLulus int    `json:"tahun_lulus"`
	OfferID    int64  `json:"offer_id"`
}

type validationFinalizeBulkRequest struct {
	SchoolID    *int     `json:"school_id"`
	FinalStatus string   `json:"final_status"`
	Slot        *int     `json:"slot"`
	TahunLulus  int      `json:"tahun_lulus"`
	NISNList    []string `json:"nisn_list"`
	StudentIDs  []int64  `json:"student_ids"`
}

type cohortListResponse struct {
	ID         int64  `json:"id"`
	NISN       string `json:"nisn"`
	NamaSiswa  string `json:"namaSiswa"`
	Rombel     string `json:"rombel"`
	TahunLulus int    `json:"tahunLulus"`
	Status     string `json:"status"`
}

type applicationListResponse struct {
	ID                  int64      `json:"id"`
	NISN                string     `json:"nisn"`
	NamaSiswa           string     `json:"namaSiswa"`
	Rombel              string     `json:"rombel"`
	JalurPenerimaan     string     `json:"jalurPenerimaan"`
	NamaPerguruanTinggi string     `json:"namaPerguruanTinggi"`
	Status              string     `json:"status"`
	FinalStatus         string     `json:"final_status"`
	FinalizedAt         *time.Time `json:"finalized_at"`
}

func (h SiswaHandler) ImportCohorts(c *gin.Context) {
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

	buffer, filename, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	uploadService := services.UploadService{DB: h.DB}
	storageKey := fmt.Sprintf("inline/%d/cohorts/%d", schoolID, time.Now().UnixNano())
	uploadID, _, err := uploadService.Create(c, user, "COHORTS", time.Now().Year(), nil, storageKey, filename)
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
	markFailed := func(err error) {
		_ = uploadService.MarkFailed(c, uploadID, err)
	}

	foundHeaders, ok, err := parsers.ValidateExactHeaders(buffer, "", parsers.SiswaCohortTemplateHeaders)
	if err != nil {
		markFailed(err)
		utils.RespondInternalServerError(c)
		return
	}
	if !ok {
		markFailed(errors.New("template mismatch"))
		respondSiswaHeaderMismatch(c, parsers.SiswaCohortTemplateHeaders, foundHeaders)
		return
	}
	parsed, err := parsers.ParseSiswaCohorts(buffer)
	if err != nil {
		markFailed(err)
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		markFailed(errors.New("missing columns"))
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportCohorts(c, schoolID, uploadID, parsed.Rows)
	if err != nil {
		markFailed(err)
		utils.RespondInternalServerError(c)
		return
	}
	if err := uploadService.MarkSuccess(c, uploadID); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportApplications(c *gin.Context) {
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

	buffer, filename, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	uploadService := services.UploadService{DB: h.DB}
	storageKey := fmt.Sprintf("inline/%d/applications/%d", schoolID, time.Now().UnixNano())
	uploadID, _, err := uploadService.Create(c, user, "APPLICATIONS", time.Now().Year(), nil, storageKey, filename)
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
	markFailed := func(err error) {
		_ = uploadService.MarkFailed(c, uploadID, err)
	}

	foundHeaders, ok, err := parsers.ValidateExactHeaders(buffer, "", parsers.SiswaApplicationTemplateHeaders)
	if err != nil {
		markFailed(err)
		utils.RespondInternalServerError(c)
		return
	}
	if !ok {
		markFailed(errors.New("template mismatch"))
		respondSiswaHeaderMismatch(c, parsers.SiswaApplicationTemplateHeaders, foundHeaders)
		return
	}
	parsed, err := parsers.ParseSiswaApplications(buffer)
	if err != nil {
		markFailed(err)
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		markFailed(errors.New("missing columns"))
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportApplications(c, schoolID, uploadID, parsed.Rows)
	if err != nil {
		markFailed(err)
		utils.RespondInternalServerError(c)
		return
	}
	if err := uploadService.MarkSuccess(c, uploadID); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportAcceptanceOffers(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, true)
	if scope == nil {
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}
	schoolID := *scope

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}
	parsed, err := parsers.ParseSiswaAcceptanceOffers(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportAcceptanceOffers(c, schoolID, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportUKK(c *gin.Context) {
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
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}
	schoolID := *scope

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}
	parsed, err := parsers.ParseSiswaUKK(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportUKK(c, schoolID, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportHistoricalFinal(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, true)
	if scope == nil {
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}
	schoolID := *scope

	settings, _, ok := h.ensureLegacyImportEnabled(c, user, schoolID)
	if !ok {
		return
	}

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}
	parsed, err := parsers.ParseSiswaHistoricalFinals(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportHistoricalFinals(c, schoolID, settings.MaxYear, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportLegacyWork(c *gin.Context) {
	h.importLegacyFinalSimple(c, "WORK")
}

func (h SiswaHandler) ImportLegacyEntrepreneur(c *gin.Context) {
	h.importLegacyFinalSimple(c, "ENTREPRENEUR")
}

func (h SiswaHandler) ImportLegacyUndefined(c *gin.Context) {
	h.importLegacyFinalSimple(c, "UNDEFINED")
}

func (h SiswaHandler) importLegacyFinalSimple(c *gin.Context, finalType string) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, true)
	if scope == nil {
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}
	schoolID := *scope

	settings, _, ok := h.ensureLegacyImportEnabled(c, user, schoolID)
	if !ok {
		return
	}

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	parsed, err := parsers.ParseSiswaCohorts(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportLegacyFinals(c, schoolID, finalType, settings.MaxYear, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ListCohorts(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
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
	includePlanned := false
	if raw := strings.TrimSpace(c.Query("includePlanned")); raw != "" {
		includePlanned = strings.EqualFold(raw, "true") || raw == "1"
	}
	if includePlanned && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
		includePlanned = false
	}

	filters := services.SiswaCohortListFilters{
		SchoolID:       scope,
		Page:           page,
		PageSize:       pageSize,
		Search:         strings.TrimSpace(c.Query("q")),
		Year:           year,
		IncludePlanned: includePlanned,
	}

	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListCohorts(c, user, filters)
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

	payload := make([]cohortListResponse, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, cohortListResponse{
			ID:         row.ID,
			NISN:       row.NISN,
			NamaSiswa:  row.NamaSiswa,
			Rombel:     row.Rombel,
			TahunLulus: row.TahunLulus,
			Status:     row.Status,
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

func (h SiswaHandler) ListApplications(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
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

	filters := services.SiswaApplicationListFilters{
		SchoolID: scope,
		Page:     page,
		PageSize: pageSize,
		Search:   strings.TrimSpace(c.Query("q")),
		Year:     year,
	}

	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListApplications(c, user, filters)
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

	payload := make([]applicationListResponse, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, applicationListResponse{
			ID:                  row.ID,
			NISN:                row.NISN,
			NamaSiswa:           row.NamaSiswa,
			Rombel:              row.Rombel,
			JalurPenerimaan:     row.JalurPenerimaan,
			NamaPerguruanTinggi: row.NamaPerguruanTinggi,
			Status:              row.Status,
			FinalStatus:         row.FinalStatus,
			FinalizedAt:         row.FinalizedAt,
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

func (h SiswaHandler) ValidationPending(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
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
	result, err := service.ValidationPending(c, user, scope, year)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, result)
}

func (h SiswaHandler) ValidationSummary(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
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
	result, err := service.ValidationSummary(c, user, scope, year)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, result)
}

func (h SiswaHandler) ValidationFinalizeBulk(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	var req validationFinalizeBulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	status := strings.ToUpper(strings.TrimSpace(req.FinalStatus))
	if status == "" {
		utils.RespondBadRequest(c, "final_status wajib diisi.")
		return
	}

	if status == "PT" || status == "WIRAUSAHA" {
		if req.Slot == nil || *req.Slot < 1 || *req.Slot > 3 {
			utils.RespondBadRequest(c, "slot wajib diisi (1-3) untuk PT/WIRAUSAHA.")
			return
		}
	} else {
		if req.Slot != nil {
			utils.RespondBadRequest(c, "slot harus null untuk KERJA/BELUM.")
			return
		}
	}

	targetCount := len(req.NISNList)
	if len(req.StudentIDs) > 0 {
		targetCount = len(req.StudentIDs)
	}
	if targetCount == 0 {
		utils.RespondBadRequest(c, "nisn_list atau student_ids wajib diisi.")
		return
	}
	if targetCount > 500 {
		utils.RespondBadRequest(c, "Maksimal 500 siswa per request.")
		return
	}

	scope, err := auth.GetSchoolScope(user, req.SchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondUnauthorized(c)
		return
	}
	if scope == nil {
		utils.RespondBadRequest(c, "school_id wajib diisi.")
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ValidationFinalizeBulk(c, user, services.ValidationFinalizeBulkInput{
		SchoolID:    *scope,
		FinalStatus: status,
		Slot:        req.Slot,
		Year:        req.TahunLulus,
		NISNList:    req.NISNList,
		StudentIDs:  req.StudentIDs,
		ChangedBy:   user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidOutcome):
			utils.RespondBadRequest(c, "Payload tidak valid.")
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, result)
}

func (h SiswaHandler) Finalize(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	var req finalizeSiswaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	requestedSchoolID := req.SchoolID
	scope, err := auth.GetSchoolScope(user, requestedSchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondUnauthorized(c)
		return
	}
	if scope == nil {
		utils.RespondBadRequest(c, "school_id wajib diisi.")
		return
	}

	nisn := strings.TrimSpace(req.NISN)
	if nisn == "" || req.TahunLulus <= 0 || req.OfferID <= 0 {
		utils.RespondBadRequest(c, "nisn, tahun_lulus, dan offer_id wajib diisi.")
		return
	}

	service := services.SiswaService{DB: h.DB}
	if err := service.FinalizeHigherEd(c, *scope, nisn, req.TahunLulus, req.OfferID); err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
		case errors.Is(err, services.ErrSiswaOfferNotFound):
			utils.RespondBadRequest(c, "Offer tidak valid untuk siswa tersebut.")
		case errors.Is(err, services.ErrSiswaFinalized):
			utils.RespondBadRequest(c, "Data sudah final dan terkunci.")
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"nisn":        nisn,
		"tahun_lulus": req.TahunLulus,
		"offer_id":    req.OfferID,
		"final_type":  "HIGHER_ED",
	})
}

func (h SiswaHandler) AnalyticsHigherEd(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.AnalyticsHigherEd(c, user, scope)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, result)
}

func (h SiswaHandler) DownloadRejectedImportReport(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	reportID := strings.TrimSpace(c.Param("id"))
	if reportID == "" {
		utils.RespondBadRequest(c, "Invalid report id")
		return
	}

	service := services.SiswaService{DB: h.DB}
	schoolID, filename, content, err := service.GetImportReport(c, reportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondBadRequest(c, "Laporan tidak ditemukan.")
		} else {
			utils.RespondInternalServerError(c)
		}
		return
	}

	scope, scopeErr := auth.GetSchoolScope(user, nil)
	if scopeErr != nil {
		if errors.Is(scopeErr, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondUnauthorized(c)
		return
	}
	if scope != nil && schoolID != *scope {
		utils.RespondForbidden(c)
		return
	}

	if filename == "" {
		filename = "rejected.xlsx"
	}
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func siswaImportResponsePayload(result services.SiswaImportResult) gin.H {
	payload := gin.H{
		"total_rows": result.TotalRows,
		"accepted":   result.Accepted,
		"rejected":   result.Rejected,
	}
	if result.RejectedReportID != "" {
		payload["rejected_excel_url"] = "/api/siswa/import-reports/" + result.RejectedReportID + "/rejected.xlsx"
		payload["rejected_excel_filename"] = result.RejectedFilename
	}
	if len(result.Rejections) > 0 {
		items := make([]gin.H, 0, len(result.Rejections))
		for _, rejection := range result.Rejections {
			items = append(items, gin.H{
				"row_number": rejection.RowNumber,
				"reason":     rejection.Reason,
			})
		}
		payload["rejections"] = items
	}
	return payload
}

func siswaAuthorizedUser(c *gin.Context) (auth.User, bool) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return auth.User{}, false
	}
	if user.Role != "ADMIN" && user.Role != "SCHOOL_ADMIN" && user.Role != "ADMIN_MASTER" {
		utils.RespondForbidden(c)
		return auth.User{}, false
	}
	return user, true
}

func siswaResolveScope(c *gin.Context, user auth.User, includeForm bool) *int {
	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid school_id")
			return nil
		}
		requestedSchoolID = &id
	} else if includeForm {
		if raw := strings.TrimSpace(c.PostForm("school_id")); raw != "" {
			id, err := strconv.Atoi(raw)
			if err != nil || id <= 0 {
				utils.RespondBadRequest(c, "Invalid school_id")
				return nil
			}
			requestedSchoolID = &id
		}
	}

	scope, err := auth.GetSchoolScope(user, requestedSchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
		} else {
			utils.RespondUnauthorized(c)
		}
		return nil
	}
	return scope
}

func (h SiswaHandler) fetchSchoolLevel(c *gin.Context, schoolID int) (string, error) {
	var level string
	if err := h.DB.Pool.QueryRow(c, `
		SELECT COALESCE(UPPER(TRIM(bentuk_pendidikan)), '')
		FROM data_sekolah
		WHERE id = $1
	`, schoolID).Scan(&level); err != nil {
		return "", err
	}
	if strings.TrimSpace(level) == "" {
		return "", errors.New("LEVEL_NOT_SET")
	}
	return level, nil
}

func (h SiswaHandler) ensureLegacyImportEnabled(c *gin.Context, user auth.User, schoolID int) (services.LegacyImportSettings, string, bool) {
	level, err := h.fetchSchoolLevel(c, schoolID)
	if err != nil {
		utils.RespondBadRequest(c, "Invalid school level")
		return services.LegacyImportSettings{}, "", false
	}

	var cabdinUserID *int
	if user.Role == "ADMIN" {
		cabdinUserID = &user.ID
	}
	legacyService := services.LegacyImportService{DB: h.DB}
	enabled, settings, source, err := legacyService.EffectiveLegacyEnabled(c, level, cabdinUserID, time.Now())
	if err != nil {
		utils.RespondInternalServerError(c)
		return services.LegacyImportSettings{}, "", false
	}
	if !enabled {
		c.JSON(http.StatusForbidden, utils.APIResponse{
			Error:   "FORBIDDEN",
			Message: "LEGACY_IMPORT_DISABLED",
			Details: gin.H{
				"level":  level,
				"source": source,
			},
		})
		return settings, level, false
	}
	return settings, level, true
}

func readExcelUpload(c *gin.Context) ([]byte, string, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, "", errors.New("file Excel wajib diunggah")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, "", errors.New("gagal membuka file Excel")
	}
	defer file.Close()
	buffer, err := io.ReadAll(file)
	if err != nil {
		return nil, "", errors.New("gagal membaca file Excel")
	}
	return buffer, fileHeader.Filename, nil
}

func respondSiswaHeaderMismatch(c *gin.Context, expected, found []string) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusBadRequest, utils.APIResponse{
		Error:   "TEMPLATE_MISMATCH",
		Message: siswaTemplateMismatchMessage,
		Details: gin.H{
			"expected_headers": expected,
			"found_headers":    found,
		},
	})
}
