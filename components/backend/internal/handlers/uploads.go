package handlers

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgconn"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/storage"
	"cabdin-bangkalan-backend/internal/utils"
)

type UploadHandler struct {
	DB      *db.DB
	Storage storage.Provider
}

func (h UploadHandler) List(c *gin.Context) {
	logging.Infof("upload", "list", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role == "SCHOOL_ADMIN" && user.SchoolID == nil {
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

	service := services.UploadService{DB: h.DB}
	result, err := service.List(c, user, services.UploadFilters{SchoolID: requestedSchoolID})
	if err != nil {

		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("upload", "list", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]map[string]interface{}, 0, len(result))
	for _, row := range result {
		payload = append(payload, map[string]interface{}{
			"id":               row.ID,
			"schoolId":         row.SchoolID,
			"userId":           row.UserID,
			"type":             row.Type,
			"year":             row.Year,
			"storageKey":       row.StorageKey,
			"originalFilename": row.OriginalFilename,
			"status":           row.Status,
			"errorLog":         row.ErrorLog,
			"createdAt":        row.CreatedAt,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

type uploadHistoryResponse struct {
	ID               int       `json:"id"`
	SchoolID         int       `json:"schoolId"`
	SchoolName       string    `json:"schoolName"`
	UserID           int       `json:"userId"`
	UploaderName     string    `json:"uploaderName"`
	Type             string    `json:"type"`
	OriginalFilename string    `json:"originalFilename"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"createdAt"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"`
	DeletedByName    string    `json:"deletedByName,omitempty"`
	Accepted         int       `json:"accepted"`
	Rejected         int       `json:"rejected"`
}

func (h UploadHandler) ListHistory(c *gin.Context) {
	logging.Infof("histori", "list_history", "event=hit")
	defer func() {
		if r := recover(); r != nil {
			logging.Errorf("histori", "list_history_panic", "err=%v", r)
			if logging.DebugEnabled() {
				logging.Debugf("histori", "list_history_panic", "stack=%s", debug.Stack())
			}
			c.JSON(500, gin.H{"error": "INTERNAL_SERVER_ERROR"})
		}
	}()

	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role == "SCHOOL_ADMIN" && user.SchoolID == nil {
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

	module := strings.TrimSpace(c.Query("module"))
	if module == "" {
		module = strings.TrimSpace(c.Query("modul"))
	}

	parseDate := func(raw string) (*time.Time, error) {
		if strings.TrimSpace(raw) == "" {
			return nil, nil
		}
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	}

	var dateFrom *time.Time
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		parsed, convErr := parseDate(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid from date")
			return
		}
		dateFrom = parsed
	} else if raw := strings.TrimSpace(c.Query("date_from")); raw != "" {
		parsed, convErr := parseDate(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid date_from")
			return
		}
		dateFrom = parsed
	}

	var dateTo *time.Time
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		parsed, convErr := parseDate(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid to date")
			return
		}
		if parsed != nil {
			nextDay := parsed.AddDate(0, 0, 1)
			dateTo = &nextDay
		}
	} else if raw := strings.TrimSpace(c.Query("date_to")); raw != "" {
		parsed, convErr := parseDate(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid date_to")
			return
		}
		if parsed != nil {
			nextDay := parsed.AddDate(0, 0, 1)
			dateTo = &nextDay
		}
	}

	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	service := services.UploadService{DB: h.DB}
	rows, total, err := service.ListHistory(c, user, services.UploadHistoryFilters{
		SchoolID: requestedSchoolID,
		Module:   module,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Limit:    pageSize,
		Offset:   offset,
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("histori", "list_history", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]uploadHistoryResponse, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, uploadHistoryResponse{
			ID:               row.ID,
			SchoolID:         row.SchoolID,
			SchoolName:       row.SchoolName,
			UserID:           row.UserID,
			UploaderName:     row.UploaderName,
			Type:             row.Type,
			OriginalFilename: row.OriginalFilename,
			Status:           row.Status,
			CreatedAt:        row.CreatedAt,
			DeletedAt:        row.DeletedAt,
			DeletedByName:    row.DeletedByName,
			Accepted:         row.AcceptedCount,
			Rejected:         row.RejectedCount,
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

func (h UploadHandler) Create(c *gin.Context) {
	logging.Infof("upload", "create", "event=hit")
	defer func() {
		if r := recover(); r != nil {
			logging.Errorf("upload", "create_panic", "err=%v", r)
			if logging.DebugEnabled() {
				logging.Debugf("upload", "create_panic", "stack=%s", debug.Stack())
			}
			c.JSON(500, gin.H{"error": "INTERNAL_SERVER_ERROR"})
		}
	}()
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}
	if user.SchoolID == nil {
		utils.RespondUnauthorized(c)
		return
	}

	uploadType := strings.ToUpper(c.PostForm("type"))
	if uploadType != "ACHIEVEMENT" && uploadType != "ACCEPTANCE" {
		respondWithUploadError(c, http.StatusBadRequest, "INVALID_TYPE", "Tipe import tidak valid", nil, nil)
		return
	}

	year, err := strconv.Atoi(c.PostForm("year"))
	if err != nil || year <= 0 {
		respondWithUploadError(c, http.StatusBadRequest, "INVALID_YEAR", "Tahun tidak valid", err, nil)
		return
	}

	password := strings.TrimSpace(c.PostForm("password"))
	if password == "" {
		respondWithUploadError(c, http.StatusBadRequest, "PASSWORD_REQUIRED", "Password wajib diisi.", nil, nil)
		return
	}
	authService := services.AuthService{DB: h.DB}
	record, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		respondWithUploadError(c, http.StatusInternalServerError, "USER_LOOKUP_FAILED", "Gagal memverifikasi user", err, nil)
		return
	}
	if !utils.VerifyPassword(password, record.PasswordHash) {
		respondWithUploadError(c, http.StatusBadRequest, "PASSWORD_INVALID", "Password salah.", nil, nil)
		return
	}

	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.PostForm("school_id")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id == 0 {
			respondWithUploadError(c, http.StatusBadRequest, "INVALID_SCHOOL_ID", "school_id tidak valid", convErr, nil)
			return
		}
		requestedSchoolID = &id
	}
	requestedSchoolID = nil

	scope, err := auth.GetSchoolScope(user, requestedSchoolID)
	if err != nil {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
		} else {
			utils.RespondUnauthorized(c)
		}
		return
	}
	if scope == nil {
		utils.RespondForbidden(c)
		return
	}
	schoolID := *scope
	var bentukPendidikan string
	if err := h.DB.Pool.QueryRow(c, `
        SELECT COALESCE(bentuk_pendidikan, '')
        FROM data_sekolah
        WHERE id = $1
    `, schoolID).Scan(&bentukPendidikan); err != nil {
		respondWithUploadError(c, http.StatusBadRequest, "SCHOOL_NOT_FOUND", "Sekolah tidak ditemukan", err, nil)
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		respondWithUploadError(c, http.StatusBadRequest, "FILE_REQUIRED", "File Excel wajib diunggah.", err, nil)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		respondWithUploadError(c, http.StatusBadRequest, "FILE_READ_FAILED", "Gagal membaca file Excel.", err, nil)
		return
	}
	defer file.Close()

	buffer, err := io.ReadAll(file)
	if err != nil {
		respondWithUploadError(c, http.StatusBadRequest, "FILE_READ_FAILED", "Gagal membaca file Excel.", err, nil)
		return
	}

	storageKey, err := h.Storage.Save(schoolID, fileHeader.Filename, buffer)
	if err != nil {
		respondWithUploadError(c, http.StatusInternalServerError, "STORAGE_FAILED", "Gagal menyimpan file import.", err, nil)
		return
	}

	service := services.UploadService{DB: h.DB}
	uploadID, _, err := service.Create(c, user, uploadType, year, requestedSchoolID, storageKey, fileHeader.Filename)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			respondWithUploadError(c, http.StatusInternalServerError, "UPLOAD_CREATE_FAILED", "Gagal membuat record import.", err, nil)
		}
		return
	}

	if uploadType == "ACHIEVEMENT" {
		actualHeaders, ok, err := parsers.ValidateExactHeaders(
			buffer,
			"Achievement",
			parsers.AchievementTemplateHeaders,
		)
		if err != nil {
			_ = service.MarkFailed(c, uploadID, err)
			respondWithUploadError(c, http.StatusInternalServerError, "HEADER_VALIDATION_FAILED", "Gagal memeriksa header template prestasi.", err, nil)
			return
		}
		if !ok {
			_ = service.MarkFailed(c, uploadID, errors.New("Template Excel tidak sesuai"))
			respondWithUploadError(c, http.StatusBadRequest, "TEMPLATE_MISMATCH", "Template Excel tidak sesuai.", errors.New("template mismatch"), gin.H{
				"expected_headers": parsers.AchievementTemplateHeaders,
				"actual_headers":   actualHeaders,
			})
			return
		}

		parseResult, err := parsers.ParseAchievements(buffer)
		if err != nil {
			_ = service.MarkFailed(c, uploadID, err)
			respondWithUploadError(c, http.StatusInternalServerError, "PARSE_FAILED", "Gagal membaca data Excel prestasi.", err, nil)
			return
		}
		if len(parseResult.MissingColumns) > 0 {
			missingMessage := "Kolom wajib tidak terbaca: " + strings.Join(parseResult.MissingColumns, ", ")
			for _, key := range parseResult.MissingColumns {
				if key == "kategori_prestasi" || key == "peringkat" {
					friendly := "Kolom Kategori/Peringkat tidak terbaca, kemungkinan template tidak sesuai."
					_ = service.MarkFailed(c, uploadID, errors.New(friendly))
					respondWithUploadError(c, http.StatusBadRequest, "MISSING_COLUMNS", friendly, errors.New(friendly), gin.H{
						"missing_columns": parseResult.MissingColumns,
					})
					return
				}
			}
			_ = service.MarkFailed(c, uploadID, errors.New(missingMessage))
			respondWithUploadError(c, http.StatusBadRequest, "MISSING_COLUMNS", "Kolom wajib tidak lengkap.", errors.New(missingMessage), gin.H{
				"missing_columns": parseResult.MissingColumns,
			})
			return
		}

		for i := range parseResult.Rows {
			parseResult.Rows[i].JenjangPendidikan = bentukPendidikan
		}
		result, err := service.StoreAchievements(c, uploadID, schoolID, parseResult.Rows)
		if err != nil {
			_ = service.MarkFailed(c, uploadID, err)
			respondWithUploadError(c, http.StatusInternalServerError, "STORE_FAILED", "Gagal menyimpan data prestasi.", err, nil)
			return
		}
		if err := service.MarkSuccess(c, uploadID); err != nil {
			respondWithUploadError(c, http.StatusInternalServerError, "MARK_SUCCESS_FAILED", "Gagal menandai import prestasi berhasil.", err, nil)
			return
		}
		if _, err := h.DB.Pool.Exec(c, `
            UPDATE data_sekolah
            SET last_update_prestasi = NOW(),
                updated_at = NOW()
            WHERE id = $1
        `, schoolID); err != nil {
			respondWithUploadError(c, http.StatusInternalServerError, "UPDATE_METADATA_FAILED", "Gagal memperbarui metadata sekolah.", err, nil)
			return
		}
		respondUploadResult(c, uploadID, uploadType, user.Role == "SCHOOL_ADMIN", result)
		return
	} else {
		actualHeaders, ok, err := parsers.ValidateExactHeaders(
			buffer,
			"Siswa_Perguruan_Tinggi",
			parsers.AcceptanceTemplateHeaders,
		)
		if err != nil {
			_ = service.MarkFailed(c, uploadID, err)
			respondWithUploadError(c, http.StatusInternalServerError, "HEADER_VALIDATION_FAILED", "Gagal memeriksa header template siswa perguruan tinggi.", err, nil)
			return
		}
		if !ok {
			_ = service.MarkFailed(c, uploadID, errors.New("Template Excel tidak sesuai"))
			respondWithUploadError(c, http.StatusBadRequest, "TEMPLATE_MISMATCH", "Template Excel tidak sesuai.", errors.New("template mismatch"), gin.H{
				"expected_headers": parsers.AcceptanceTemplateHeaders,
				"actual_headers":   actualHeaders,
			})
			return
		}

		parseResult, err := parsers.ParseAdmissions(buffer)
		if err != nil {
			_ = service.MarkFailed(c, uploadID, err)
			respondWithUploadError(c, http.StatusInternalServerError, "PARSE_FAILED", "Gagal membaca data Excel siswa perguruan tinggi.", err, nil)
			return
		}
		if len(parseResult.MissingColumns) > 0 {
			missingMessage := "Kolom wajib tidak lengkap: " + strings.Join(parseResult.MissingColumns, ", ")
			_ = service.MarkFailed(c, uploadID, errors.New(missingMessage))
			respondWithUploadError(c, http.StatusBadRequest, "MISSING_COLUMNS", "Kolom wajib tidak lengkap.", errors.New(missingMessage), gin.H{
				"missing_columns": parseResult.MissingColumns,
			})
			return
		}

		for i := range parseResult.Rows {
			parseResult.Rows[i].JenjangPendidikan = bentukPendidikan
		}
		result, err := service.StoreAdmissions(c, uploadID, schoolID, parseResult.Rows)
		if err != nil {
			_ = service.MarkFailed(c, uploadID, err)
			respondWithUploadError(c, http.StatusInternalServerError, "STORE_FAILED", "Gagal menyimpan data siswa perguruan tinggi.", err, nil)
			return
		}
		if err := service.MarkSuccess(c, uploadID); err != nil {
			respondWithUploadError(c, http.StatusInternalServerError, "MARK_SUCCESS_FAILED", "Gagal menandai import siswa perguruan tinggi berhasil.", err, nil)
			return
		}
		if _, err := h.DB.Pool.Exec(c, `
            UPDATE data_sekolah
            SET last_update_siswa_pt = NOW(),
                updated_at = NOW()
            WHERE id = $1
        `, schoolID); err != nil {
			respondWithUploadError(c, http.StatusInternalServerError, "UPDATE_METADATA_FAILED", "Gagal memperbarui metadata sekolah.", err, nil)
			return
		}
		respondUploadResult(c, uploadID, uploadType, user.Role == "SCHOOL_ADMIN", result)
		return
	}

	respondWithUploadError(c, http.StatusInternalServerError, "UNKNOWN_IMPORT_ERROR", "Terjadi kesalahan saat memproses import.", nil, nil)
}

type uploadResponse struct {
	Status                string                   `json:"status"`
	TotalRows             int                      `json:"totalRows"`
	Inserted              int                      `json:"inserted"`
	Rejected              int                      `json:"rejected"`
	RejectedReportUrl     string                   `json:"rejectedReportUrl,omitempty"`
	RejectedExcelFilename string                   `json:"rejected_excel_filename,omitempty"`
	RejectedExcelURL      string                   `json:"rejected_excel_url,omitempty"`
	Rejections            []uploadRejectionSummary `json:"rejections,omitempty"`
}

type uploadRejectionSummary struct {
	RowNumber int    `json:"rowNumber"`
	Reason    string `json:"reason"`
	Notes     string `json:"notes"`
}

func respondUploadResult(c *gin.Context, uploadID int, uploadType string, isSchoolAdmin bool, result services.UploadStoreResult) {
	status := "SUCCESS"
	rejectedCount := len(result.Rejected)
	if rejectedCount > 0 {
		status = "SUCCESS_WITH_REJECTIONS"
	}
	reportURL := ""
	if rejectedCount > 0 {
		reportURL = "/api/uploads/" + strconv.Itoa(uploadID) + "/rejected.xlsx"
	}
	response := uploadResponse{
		Status:            status,
		TotalRows:         result.Total,
		Inserted:          result.Inserted,
		Rejected:          rejectedCount,
		RejectedReportUrl: reportURL,
	}
	if rejectedCount > 0 && isSchoolAdmin && uploadType == "ACCEPTANCE" {
		response.RejectedExcelFilename = "rejected-siswa-perguruan-tinggi.xlsx"
		response.RejectedExcelURL = reportURL
	}
	if rejectedCount > 0 {
		response.Rejections = make([]uploadRejectionSummary, 0, rejectedCount)
		for _, rejection := range result.Rejected {
			response.Rejections = append(response.Rejections, uploadRejectionSummary{
				RowNumber: rejection.RowNumber,
				Reason:    rejection.Reason,
				Notes:     rejection.Notes,
			})
		}
	}
	c.JSON(http.StatusOK, response)
}

type deleteHistoryRequest struct {
	Password string `json:"password"`
}

func (h UploadHandler) DeleteHistory(c *gin.Context) {
	logging.Infof("upload", "delete_history", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role == "ADMIN" {
		utils.RespondForbidden(c)
		return
	}
	if user.Role != "ADMIN_MASTER" && user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}
	if user.Role == "SCHOOL_ADMIN" && user.SchoolID == nil {
		utils.RespondUnauthorized(c)
		return
	}

	uploadID, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || uploadID <= 0 {
		utils.RespondBadRequest(c, "Invalid upload id")
		return
	}

	var req deleteHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	password := strings.TrimSpace(req.Password)
	if password == "" {
		utils.RespondBadRequestMessage(c, "PASSWORD_REQUIRED")
		return
	}

	authService := services.AuthService{DB: h.DB}
	record, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("upload", "delete_history", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !utils.VerifyPassword(password, record.PasswordHash) {
		utils.RespondBadRequestMessage(c, "PASSWORD_SALAH")
		return
	}

	service := services.UploadService{DB: h.DB}
	upload, err := service.GetUploadByID(c, uploadID)
	if err != nil {
		utils.RespondBadRequest(c, "Upload not found")
		return
	}
	if upload.DeletedAt != nil {
		utils.RespondBadRequest(c, "Upload already deleted")
		return
	}
	if user.Role == "SCHOOL_ADMIN" && user.SchoolID != nil {
		if upload.SchoolID != *user.SchoolID {
			utils.RespondForbidden(c)
			return
		}
	}

	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		logging.Errorf("upload", "delete_history", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	defer func() { _ = tx.Rollback(c) }()

	var deletedPrestasi int64
	var deletedSiswaPT int64
	var deletedCohorts int64
	var deletedApplications int64

	if upload.Type == "ACHIEVEMENT" {
		tag, execErr := tx.Exec(c, `DELETE FROM prestasi_siswa WHERE import_batch_id = $1`, uploadID)
		if execErr != nil {
			logging.Errorf("upload", "delete_history", "err=%v", execErr)
			utils.RespondInternalServerError(c)
			return
		}
		deletedPrestasi = tag.RowsAffected()
	} else if upload.Type == "ACCEPTANCE" {
		tag, execErr := tx.Exec(c, `DELETE FROM siswa_perguruan_tinggi WHERE import_batch_id = $1`, uploadID)
		if execErr != nil {
			logging.Errorf("upload", "delete_history", "err=%v", execErr)
			utils.RespondInternalServerError(c)
			return
		}
		deletedSiswaPT = tag.RowsAffected()
	} else if upload.Type == "COHORTS" {
		tag, execErr := tx.Exec(c, `DELETE FROM student_cohorts WHERE import_batch_id = $1`, uploadID)
		if execErr != nil {
			logging.Errorf("upload", "delete_history", "err=%v", execErr)
			utils.RespondInternalServerError(c)
			return
		}
		deletedCohorts = tag.RowsAffected()
	} else if upload.Type == "APPLICATIONS" {
		tag, execErr := tx.Exec(c, `DELETE FROM student_higher_ed_applications WHERE import_batch_id = $1`, uploadID)
		if execErr != nil {
			logging.Errorf("upload", "delete_history", "err=%v", execErr)
			utils.RespondInternalServerError(c)
			return
		}
		deletedApplications = tag.RowsAffected()
	} else {
		utils.RespondBadRequest(c, "Unsupported module")
		return
	}

	tag, err := tx.Exec(c, `UPDATE uploads SET deleted_at = NOW(), deleted_by = $1, updated_at = NOW() WHERE id = $2`, user.ID, uploadID)
	if err != nil {
		logging.Errorf("upload", "delete_history", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	if err := tx.Commit(c); err != nil {
		logging.Errorf("upload", "delete_history", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"deleted_rows": gin.H{
			"prestasi_siswa":         deletedPrestasi,
			"siswa_perguruan_tinggi": deletedSiswaPT,
			"student_cohorts":        deletedCohorts,
			"student_higher_ed_applications": deletedApplications,
			"import_batches":         tag.RowsAffected(),
		},
	})
}

func respondWithUploadError(c *gin.Context, status int, code, message string, err error, details interface{}) {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	if err != nil {
		if status >= http.StatusInternalServerError {
			logging.Errorf("upload", "import_error", "status=%d code=%s message=%q err=%v", status, code, message, err)
		} else {
			logging.Warnf("upload", "import_error", "status=%d code=%s message=%q err=%v", status, code, message, err)
		}
	} else {
		if status >= http.StatusInternalServerError {
			logging.Errorf("upload", "import_error", "status=%d code=%s message=%q", status, code, message)
		} else {
			logging.Warnf("upload", "import_error", "status=%d code=%s message=%q", status, code, message)
		}
	}
	c.Header("Cache-Control", "no-store")
	if details == nil && err != nil {
		details = err.Error()
	}
	c.JSON(status, utils.APIResponse{
		Error:   code,
		Message: message,
		Details: details,
	})
}

func isPostgresUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func (h UploadHandler) DownloadRejectedReport(c *gin.Context) {
	logging.Infof("upload", "download_rejected_report", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}
	if user.SchoolID == nil {
		utils.RespondUnauthorized(c)
		return
	}

	uploadID, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || uploadID <= 0 {
		utils.RespondBadRequest(c, "Invalid upload id")
		return
	}

	service := services.UploadService{DB: h.DB}
	upload, err := service.GetUploadByID(c, uploadID)
	if err != nil {
		utils.RespondBadRequest(c, "Upload not found")
		return
	}
	if upload.SchoolID != *user.SchoolID {
		utils.RespondForbidden(c)
		return
	}

	rejections, err := service.ListRejections(c, uploadID)
	if err != nil {
		logging.Errorf("upload:rejected-report", "list_rejections", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	file := excelize.NewFile()
	sheet := "Rejected"
	if upload.Type == "ACCEPTANCE" {
		sheet = "REJECTED"
	}
	file.SetSheetName("Sheet1", sheet)
	var dataHeaders []string
	if upload.Type == "ACHIEVEMENT" {
		dataHeaders = parsers.AchievementTemplateHeaders
	} else {
		dataHeaders = parsers.AcceptanceTemplateHeaders
	}

	var headers []string
	if upload.Type == "ACCEPTANCE" {
		headers = append(headers, dataHeaders...)
		headers = append(headers, "row_number", "reason")
	} else {
		baseHeaders := []string{
			"Row_Number",
			"Reason",
			"Notes",
			"Dedupe_Key",
			"Search_Hint",
		}
		headers = append(baseHeaders, dataHeaders...)
	}
	for i, header := range headers {
		cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
		if cellErr != nil {
			logging.Errorf("upload:rejected-report", "write_header", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			logging.Errorf("upload:rejected-report", "write_header", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	for i, rejection := range rejections {
		rowValues := make([]interface{}, 0, len(headers))
		if upload.Type == "ACCEPTANCE" {
			for _, header := range dataHeaders {
				rowValues = append(rowValues, rejection.Data[header])
			}
			reason := rejection.Notes
			if strings.TrimSpace(reason) == "" {
				reason = rejection.Reason
			}
			rowValues = append(rowValues, rejection.RowNumber, reason)
		} else {
			rowValues = append(rowValues,
				rejection.RowNumber,
				rejection.Reason,
				rejection.Notes,
				rejection.DedupeKey,
				rejection.SearchHint,
			)
			for _, header := range dataHeaders {
				rowValues = append(rowValues, rejection.Data[header])
			}
		}
		cell, cellErr := excelize.CoordinatesToCellName(1, i+2)
		if cellErr != nil {
			logging.Errorf("upload:rejected-report", "write_row", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			logging.Errorf("upload:rejected-report", "write_row", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		logging.Errorf("upload:rejected-report", "write_file", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	filename := "rejected.xlsx"
	if upload.Type == "ACHIEVEMENT" {
		filename = "rejected-prestasi.xlsx"
	} else if upload.Type == "ACCEPTANCE" {
		filename = "rejected-siswa-perguruan-tinggi.xlsx"
	}
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}
