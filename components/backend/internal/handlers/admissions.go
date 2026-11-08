package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type AdmissionHandler struct {
	DB *db.DB
}

type admissionListResponse struct {
	ID                  string `json:"id"`
	SchoolID            int    `json:"schoolId"`
	NamaSekolah         string `json:"namaSekolah"`
	NISNSiswa           string `json:"nisnSiswa"`
	NamaSiswa           string `json:"namaSiswa"`
	NamaPerguruanTinggi string `json:"namaPerguruanTinggi"`
	Jurusan             string `json:"jurusanPerguruanTinggi"`
	JalurPenerimaan     string `json:"jalurPenerimaan"`
	TahunDiterima       int    `json:"tahunDiterima"`
}

type admissionUpdateRequest struct {
	Password string `json:"password"`
	Payload  struct {
		NISNSiswa           string `json:"nisnSiswa"`
		NamaSiswa           string `json:"namaSiswa"`
		NamaPerguruanTinggi string `json:"namaPerguruanTinggi"`
		Jurusan             string `json:"jurusanPerguruanTinggi"`
		JalurPenerimaan     string `json:"jalurPenerimaan"`
		TahunDiterima       int    `json:"tahunDiterima"`
	} `json:"payload"`
}

type universitySuggestResponse struct {
	Nama string `json:"nama"`
}

func (h AdmissionHandler) List(c *gin.Context) {
	logging.Infof("admission", "list", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
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

	tahun := 0
	if raw := strings.TrimSpace(c.Query("tahun")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid tahun")
			return
		}
		tahun = parsed
	}

	filters := services.AdmissionFilters{
		SchoolID: requestedSchoolID,
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
		Tahun:    tahun,
		Jalur:    strings.TrimSpace(c.Query("jalur")),
	}
	if user.Role == "SCHOOL_ADMIN" {
		filters.Jenjang = ""
	}

	service := services.AdmissionService{DB: h.DB}
	result, err := service.List(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("admission", "list", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]map[string]interface{}, 0, len(result))
	for _, item := range result {
		payload = append(payload, map[string]interface{}{
			"id":                     item.ID,
			"schoolId":               item.SchoolID,
			"namaSekolah":            item.NamaSekolah,
			"jenjangPendidikan":      item.JenjangPendidikan,
			"namaSiswa":              item.NamaSiswa,
			"rombelSiswa":            item.RombelSiswa,
			"tahunLulus":             item.TahunLulus,
			"namaPerguruanTinggi":    item.NamaPerguruanTinggi,
			"jurusanPerguruanTinggi": item.Jurusan,
			"tahunDiterima":          item.TahunDiterima,
			"jalurPenerimaan":        item.JalurPenerimaan,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdmissionHandler) ListTable(c *gin.Context) {
	logging.Infof("admission", "list_table", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("sekolahId")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid sekolahId")
			return
		}
		requestedSchoolID = &id
	}

	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" && strings.ToUpper(raw) != "ALL" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}

	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))
	if pageSize <= 0 {
		pageSize = 20
	}

	filters := services.AdmissionListFilters{
		SchoolID: requestedSchoolID,
		Year:     year,
		PtName:   strings.TrimSpace(c.Query("pt")),
		Search:   strings.TrimSpace(c.Query("q")),
		Page:     page,
		PageSize: pageSize,
	}

	service := services.AdmissionService{DB: h.DB}
	rows, total, err := service.ListTable(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("admission", "list_table", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]admissionListResponse, 0, len(rows))
	for _, item := range rows {
		payload = append(payload, admissionListResponse{
			ID:                  item.ID,
			SchoolID:            item.SchoolID,
			NamaSekolah:         item.NamaSekolah,
			NISNSiswa:           item.NISNSiswa,
			NamaSiswa:           item.NamaSiswa,
			NamaPerguruanTinggi: item.NamaPerguruanTinggi,
			Jurusan:             item.Jurusan,
			JalurPenerimaan:     item.JalurPenerimaan,
			TahunDiterima:       item.TahunDiterima,
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

func (h AdmissionHandler) Export(c *gin.Context) {
	logging.Infof("admission", "export", "event=hit")
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
	if raw := strings.TrimSpace(c.Query("sekolahId")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid sekolahId")
			return
		}
		requestedSchoolID = &id
	}

	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" && strings.ToUpper(raw) != "ALL" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}

	filters := services.AdmissionExportFilters{
		SchoolID: requestedSchoolID,
		Year:     year,
		PtName:   strings.TrimSpace(c.Query("pt")),
		Search:   strings.TrimSpace(c.Query("q")),
		Jalur:    strings.TrimSpace(c.Query("jalur")),
	}

	const maxExportRows = 50000
	service := services.AdmissionService{DB: h.DB}
	rows, total, err := service.ExportRows(c, user, filters, maxExportRows)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrExportLimitExceeded):
			utils.RespondBadRequest(c, fmt.Sprintf("Export melebihi batas %d baris (total %d). Gunakan filter untuk memperkecil data.", maxExportRows, total))
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("admission", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	file := excelize.NewFile()
	sheet := "Siswa_Perguruan_Tinggi"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Nama_Sekolah",
		"NISN_Siswa",
		"Nama_Siswa",
		"Nama_Perguruan_Tinggi",
		"Jurusan_Perguruan_Tinggi",
		"Jalur_Penerimaan",
		"Tahun_Diterima",
	}

	for i, header := range headers {
		cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
		if cellErr != nil {
			logging.Errorf("admission", "export", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			logging.Errorf("admission", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	for i, row := range rows {
		rowValues := []interface{}{
			row.NamaSekolah,
			row.NISNSiswa,
			row.NamaSiswa,
			row.NamaPerguruanTinggi,
			row.Jurusan,
			row.JalurPenerimaan,
			row.TahunDiterima,
		}
		cell, cellErr := excelize.CoordinatesToCellName(1, i+2)
		if cellErr != nil {
			logging.Errorf("admission", "export", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			logging.Errorf("admission", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		logging.Errorf("admission", "export", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\"siswa-perguruan-tinggi.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h AdmissionHandler) Update(c *gin.Context) {
	logging.Infof("admission", "update", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		utils.RespondBadRequest(c, "Invalid id")
		return
	}

	var req admissionUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		utils.RespondBadRequestMessage(c, "PASSWORD_REQUIRED")
		return
	}

	authService := services.AuthService{DB: h.DB}
	record, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("admission", "update", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !utils.VerifyPassword(req.Password, record.PasswordHash) {
		utils.RespondBadRequestMessage(c, "PASSWORD_SALAH")
		return
	}
	if user.SchoolID == nil {
		utils.RespondForbidden(c)
		return
	}

	result, err := h.DB.Pool.Exec(c, `
        UPDATE siswa_perguruan_tinggi
        SET nisn_siswa = $1,
            nama_siswa = $2,
            nama_perguruan_tinggi = $3,
            jurusan_perguruan_tinggi = $4,
            jalur_penerimaan = $5,
            tahun_diterima = $6,
            updated_at = NOW()
        WHERE id = $7
          AND school_id = $8
    `,
		strings.TrimSpace(req.Payload.NISNSiswa),
		strings.TrimSpace(req.Payload.NamaSiswa),
		strings.TrimSpace(req.Payload.NamaPerguruanTinggi),
		strings.TrimSpace(req.Payload.Jurusan),
		strings.TrimSpace(strings.ToUpper(req.Payload.JalurPenerimaan)),
		req.Payload.TahunDiterima,
		id,
		*user.SchoolID,
	)
	if err != nil {
		logging.Errorf("admission", "update", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if result.RowsAffected() == 0 {
		utils.RespondForbidden(c)
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{"id": id})
}

func (h AdmissionHandler) SuggestUniversities(c *gin.Context) {
	logging.Infof("admission", "suggest_universities", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) && user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		utils.RespondData(c, http.StatusOK, []universitySuggestResponse{})
		return
	}

	limit := 10
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, convErr := strconv.Atoi(raw); convErr == nil {
			limit = parsed
		}
	}

	service := services.AdmissionService{DB: h.DB}
	records, err := service.SuggestUniversities(c, user, query, limit)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("admission", "suggest_universities", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]universitySuggestResponse, 0, len(records))
	for _, record := range records {
		payload = append(payload, universitySuggestResponse{Nama: record.Nama})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdmissionHandler) Summary(c *gin.Context) {
	logging.Infof("admission", "summary", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
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

	yearParam := strings.TrimSpace(c.Query("year"))
	if yearParam == "" {
		yearParam = strings.TrimSpace(c.Query("tahun"))
	}
	year := 0
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		parsed, convErr := strconv.Atoi(yearParam)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}

	filters := services.AdmissionFilters{
		SchoolID: requestedSchoolID,
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
		Status:   strings.TrimSpace(c.Query("status")),
		Tahun:    year,
	}
	if user.Role == "SCHOOL_ADMIN" {
		filters.Jenjang = ""
	}

	service := services.AdmissionService{DB: h.DB}
	result, err := service.SummaryByYear(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("admission", "summary", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]map[string]interface{}, 0, len(result))
	for _, item := range result {
		payload = append(payload, map[string]interface{}{
			"year":      item.Year,
			"SNBP":      item.SNBP,
			"SNBT":      item.SNBT,
			"MANDIRI":   item.MANDIRI,
			"KEDINASAN": item.KEDINASAN,
			"LAINNYA":   item.LAINNYA,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdmissionHandler) SummaryByUniversity(c *gin.Context) {
	logging.Infof("admission", "summary_by_university", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
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

	tahun := 0
	if raw := strings.TrimSpace(c.Query("tahun")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid tahun")
			return
		}
		tahun = parsed
	}

	filters := services.AdmissionFilters{
		SchoolID: requestedSchoolID,
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
		Tahun:    tahun,
		Status:   strings.TrimSpace(c.Query("status")),
	}
	if user.Role == "SCHOOL_ADMIN" {
		filters.Jenjang = ""
	}

	service := services.AdmissionService{DB: h.DB}
	result, err := service.SummaryByUniversity(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("admission", "summary_by_university", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]map[string]interface{}, 0, len(result))
	for _, item := range result {
		payload = append(payload, map[string]interface{}{
			"university_name": item.UniversityName,
			"SNBP":            item.SNBP,
			"SNBT":            item.SNBT,
			"MANDIRI":         item.MANDIRI,
			"KEDINASAN":       item.KEDINASAN,
			"LAINNYA":         item.LAINNYA,
			"TOTAL":           item.Total,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdmissionHandler) Analytics(c *gin.Context) {
	logging.Infof("admission", "analytics", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) && user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	yearParam := strings.TrimSpace(c.Query("year"))
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		if _, convErr := strconv.Atoi(yearParam); convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
	}

	var requestedSchoolID *int
	if auth.IsAdmin(user) {
		if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
			id, convErr := strconv.Atoi(raw)
			if convErr != nil || id <= 0 {
				utils.RespondBadRequest(c, "Invalid school_id")
				return
			}
			requestedSchoolID = &id
		}
	}

	filters := services.AdmissionAnalyticsSummaryFilters{
		Role:     user.Role,
		SchoolID: requestedSchoolID,
		Year:     yearParam,
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
		Status:   strings.TrimSpace(c.Query("status")),
	}
	if user.Role == "ADMIN" {
		filters.AllowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if filters.AllowedLevels == nil {
			filters.AllowedLevels = []string{}
		}
	}
	if user.Role == "SCHOOL_ADMIN" {
		filters.SchoolID = user.SchoolID
	}

	service := services.AdmissionAnalyticsSummaryService{DB: h.DB}
	payload, err := service.Get(c, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("admission", "analytics", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdmissionHandler) AnalyticsFinal(c *gin.Context) {
	logging.Infof("admission", "analytics_final", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) && user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	yearParam := strings.TrimSpace(c.Query("year"))
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		if _, convErr := strconv.Atoi(yearParam); convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
	}

	var requestedSchoolID *int
	if auth.IsAdmin(user) {
		if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
			id, convErr := strconv.Atoi(raw)
			if convErr != nil || id <= 0 {
				utils.RespondBadRequest(c, "Invalid school_id")
				return
			}
			requestedSchoolID = &id
		}
	}

	filters := services.AdmissionAnalyticsSummaryFilters{
		Role:     user.Role,
		SchoolID: requestedSchoolID,
		Year:     yearParam,
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
		Status:   strings.TrimSpace(c.Query("status")),
	}
	if user.Role == "ADMIN" {
		filters.AllowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if filters.AllowedLevels == nil {
			filters.AllowedLevels = []string{}
		}
	}
	if user.Role == "SCHOOL_ADMIN" {
		filters.SchoolID = user.SchoolID
	}

	service := services.AdmissionAnalyticsSummaryService{DB: h.DB}
	payload, err := service.Get(c, filters)
	if err != nil {
		if isMissingRelation(err) {
			logging.Warnf("admission", "analytics_final", "event=final_schema_missing")
			filters.SkipFinal = true
			legacyPayload, legacyErr := service.Get(c, filters)
			if legacyErr != nil {
				utils.RespondData(c, http.StatusOK, buildEmptyAdmissionAnalytics(filters))
				return
			}
			utils.RespondData(c, http.StatusOK, legacyPayload)
			return
		}
		logging.Warnf("admission", "analytics_final", "event=analytics_query_failed err=%v", err)
		utils.RespondData(c, http.StatusOK, buildEmptyAdmissionAnalytics(filters))
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}
