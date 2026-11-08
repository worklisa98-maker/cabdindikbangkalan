package handlers

import (
	"bytes"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type SchoolHandler struct {
	DB *db.DB
}

type mePasswordChangeRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type meSchoolMonitoringResponse struct {
	SchoolID           int     `json:"school_id"`
	SchoolName         string  `json:"school_name"`
	LastUpdate         *string `json:"last_update,omitempty"`
	LastUpdatePrestasi *string `json:"last_update_prestasi,omitempty"`
	LastUpdateSiswaPT  *string `json:"last_update_siswa_pt,omitempty"`
	Status             string  `json:"status"`
}

type meOpsMonitoringSchoolItem struct {
	SchoolID                int     `json:"school_id"`
	SchoolName              string  `json:"school_name"`
	Jenjang                 *string `json:"jenjang,omitempty"`
	StatusSekolah           *string `json:"status_sekolah,omitempty"`
	PrestasiUpdated         bool    `json:"prestasi_updated"`
	PrestasiLastUpdate      *string `json:"prestasi_last_update,omitempty"`
	SiswaUpdated            bool    `json:"siswa_updated"`
	SiswaLastUpdate         *string `json:"siswa_last_update,omitempty"`
	StatusLulusanFinalCount int     `json:"status_lulusan_final_count"`
	PendingValidasiCount    int     `json:"pending_validasi_count"`
}

type meOpsMonitoringSchoolsResponse struct {
	Items []meOpsMonitoringSchoolItem `json:"items"`
	Page  int                         `json:"page"`
	Limit int                         `json:"limit"`
	Total int                         `json:"total"`
}

type meOpsMonitoringSchoolRow struct {
	SchoolID                int
	SchoolName              string
	Jenjang                 sql.NullString
	StatusSekolah           sql.NullString
	LastUpdatePrestasi      sql.NullTime
	LastUpdateSiswaPT       sql.NullTime
	StatusLulusanFinalCount int
	PendingValidasiCount    int
}

func (h SchoolHandler) List(c *gin.Context) {
	logging.Infof("school", "list", "event=hit")
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

	service := services.SchoolService{DB: h.DB}
	result, err := service.List(c, user, services.SchoolFilters{
		SchoolID: requestedSchoolID,
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("school", "list", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]map[string]interface{}, 0, len(result))
	for _, school := range result {
		var lastUpdatePrestasi *string
		if school.LastUpdatePrestasi != nil {
			formatted := school.LastUpdatePrestasi.UTC().Format(time.RFC3339)
			lastUpdatePrestasi = &formatted
		}
		var lastUpdateSiswaPT *string
		if school.LastUpdateSiswaPT != nil {
			formatted := school.LastUpdateSiswaPT.UTC().Format(time.RFC3339)
			lastUpdateSiswaPT = &formatted
		}
		var supervisor interface{}
		if school.SupervisorUserID != nil && school.SupervisorUsername != nil && school.SupervisorName != nil {
			supervisor = map[string]interface{}{
				"id":       *school.SupervisorUserID,
				"username": *school.SupervisorUsername,
				"name":     *school.SupervisorName,
			}
		} else {
			supervisor = nil
		}
		payload = append(payload, map[string]interface{}{
			"id":                  school.ID,
			"npsnSekolah":         school.NPSNSekolah,
			"namaSekolah":         school.NamaSekolah,
			"bentukPendidikan":    school.BentukPendidikan,
			"statusSekolah":       school.StatusSekolah,
			"tmtAkreditasi":       school.TMTAkreditasi,
			"akreditasi":          school.Akreditasi,
			"jumlahTotalGuru":     school.JumlahTotalGuru,
			"jumlahGuruAsn":       school.JumlahGuruAsn,
			"jumlahGuruHonorer":   school.JumlahGuruHonorer,
			"jumlahTotalTendik":   school.JumlahTotalTendik,
			"jumlahTendikAsn":     school.JumlahTendikAsn,
			"jumlahTendikHonorer": school.JumlahTendikHonorer,
			"lastUpdatePrestasi":  lastUpdatePrestasi,
			"lastUpdateSiswaPt":   lastUpdateSiswaPT,
			"supervisor":          supervisor,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h SchoolHandler) MeSchools(c *gin.Context) {
	logging.Infof("school", "me_schools", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	scope, scopeErr := auth.GetMultiSchoolScope(c, h.DB, user)
	if scopeErr != nil {
		if scopeErr == auth.ErrForbidden {
			utils.RespondForbidden(c)
		} else {
			logging.Errorf("school", "me_schools", "err=%v", scopeErr)
			utils.RespondInternalServerError(c)
		}
		return
	}

	query := `
		SELECT id,
		       nama_sekolah,
		       npsn_sekolah,
		       bentuk_pendidikan
		FROM data_sekolah
	`
	args := []interface{}{}
	if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			utils.RespondData(c, http.StatusOK, []map[string]interface{}{})
			return
		}
		query += " WHERE id = ANY($1)"
		args = append(args, scope.SchoolIDs)
	}
	query += " ORDER BY nama_sekolah ASC"

	rows, err := h.DB.Pool.Query(c, query, args...)
	if err != nil {
		logging.Errorf("school", "me_schools", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()

	payload := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int
		var namaSekolah string
		var npsnSekolah *string
		var bentukPendidikan *string
		if scanErr := rows.Scan(&id, &namaSekolah, &npsnSekolah, &bentukPendidikan); scanErr != nil {
			logging.Errorf("school", "me_schools", "err=%v", scanErr)
			utils.RespondInternalServerError(c)
			return
		}
		payload = append(payload, map[string]interface{}{
			"id":               id,
			"namaSekolah":      namaSekolah,
			"npsnSekolah":      npsnSekolah,
			"bentukPendidikan": bentukPendidikan,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h SchoolHandler) ChangeMyPassword(c *gin.Context) {
	logging.Infof("school", "change_my_password", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != auth.RoleAdminMaster &&
		user.Role != auth.RoleAdminCabdin &&
		user.Role != auth.RolePengawas &&
		user.Role != auth.RoleGaspolSchoolUser {
		utils.RespondForbidden(c)
		return
	}

	var req mePasswordChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	oldPassword := strings.TrimSpace(req.OldPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if oldPassword == "" || newPassword == "" {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if len(newPassword) < 8 {
		utils.RespondBadRequestMessage(c, "PASSWORD_MIN_8")
		return
	}

	now := time.Now()
	if !settingsRateLimiter.Allow(user.ID, now) {
		utils.RespondTooManyRequests(c)
		return
	}

	authService := services.AuthService{DB: h.DB}
	userRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("school", "change_my_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !utils.VerifyPassword(oldPassword, userRecord.PasswordHash) {
		settingsRateLimiter.RecordFailure(user.ID, now)
		utils.RespondBadRequestMessage(c, "PASSWORD_LAMA_SALAH")
		return
	}
	settingsRateLimiter.Reset(user.ID)

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		logging.Errorf("school", "change_my_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	result, err := h.DB.Pool.Exec(c, `
		UPDATE users
		SET password_hash = $1,
		    token_version = token_version + 1,
		    password_updated_at = NOW(),
		    updated_at = NOW()
		WHERE id = $2
		  AND role IN ('ADMIN', 'ADMIN_MASTER', 'PENGAWAS', 'USER_SEKOLAH_GASPOL')
	`, hash, user.ID)
	if err != nil {
		logging.Errorf("school", "change_my_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if result.RowsAffected() == 0 {
		utils.RespondForbidden(c)
		return
	}

	adminService := services.AdminService{DB: h.DB}
	if err := adminService.LogAdminAction(c, user.ID, "CHANGE_PASSWORD_SELF"); err != nil {
		logging.Errorf("school", "change_my_password", "audit_err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{Message: "password updated"})
}

func (h SchoolHandler) MeSchoolMonitoring(c *gin.Context) {
	logging.Infof("school", "me_school_monitoring", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != auth.RoleAdminMaster && user.Role != auth.RoleAdminCabdin && user.Role != auth.RolePengawas {
		utils.RespondForbidden(c)
		return
	}

	scope, scopeErr := auth.GetMultiSchoolScope(c, h.DB, user)
	if scopeErr != nil {
		if scopeErr == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		logging.Errorf("school", "me_school_monitoring", "err=%v", scopeErr)
		utils.RespondInternalServerError(c)
		return
	}

	query := `
		SELECT id,
		       nama_sekolah,
		       last_update_prestasi,
		       last_update_siswa_pt,
		       CASE
		           WHEN last_update_prestasi IS NOT NULL OR last_update_siswa_pt IS NOT NULL THEN 'SUDAH_UPDATE'
		           ELSE 'BELUM_UPDATE'
		       END AS update_status,
		       CASE
		           WHEN last_update_prestasi IS NULL THEN last_update_siswa_pt
		           WHEN last_update_siswa_pt IS NULL THEN last_update_prestasi
		           ELSE GREATEST(last_update_prestasi, last_update_siswa_pt)
		       END AS last_update
		FROM data_sekolah
	`
	args := []interface{}{}
	if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			utils.RespondData(c, http.StatusOK, []meSchoolMonitoringResponse{})
			return
		}
		query += " WHERE id = ANY($1)"
		args = append(args, scope.SchoolIDs)
	}
	query += " ORDER BY nama_sekolah ASC"

	rows, err := h.DB.Pool.Query(c, query, args...)
	if err != nil {
		logging.Errorf("school", "me_school_monitoring", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()

	formatTimestamp := func(value sql.NullTime) *string {
		if !value.Valid {
			return nil
		}
		formatted := value.Time.UTC().Format(time.RFC3339)
		return &formatted
	}

	payload := make([]meSchoolMonitoringResponse, 0)
	for rows.Next() {
		var (
			schoolID           int
			schoolName         string
			lastUpdatePrestasi sql.NullTime
			lastUpdateSiswaPT  sql.NullTime
			updateStatus       string
			lastUpdate         sql.NullTime
		)
		if scanErr := rows.Scan(
			&schoolID,
			&schoolName,
			&lastUpdatePrestasi,
			&lastUpdateSiswaPT,
			&updateStatus,
			&lastUpdate,
		); scanErr != nil {
			logging.Errorf("school", "me_school_monitoring", "err=%v", scanErr)
			utils.RespondInternalServerError(c)
			return
		}
		payload = append(payload, meSchoolMonitoringResponse{
			SchoolID:           schoolID,
			SchoolName:         schoolName,
			LastUpdate:         formatTimestamp(lastUpdate),
			LastUpdatePrestasi: formatTimestamp(lastUpdatePrestasi),
			LastUpdateSiswaPT:  formatTimestamp(lastUpdateSiswaPT),
			Status:             updateStatus,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func parseOpsMonitoringRequestedSchoolID(c *gin.Context) (*int, bool) {
	raw := strings.TrimSpace(c.Query("school_id"))
	if raw == "" {
		return nil, true
	}
	parsed, convErr := strconv.Atoi(raw)
	if convErr != nil || parsed <= 0 {
		utils.RespondBadRequest(c, "Invalid school_id")
		return nil, false
	}
	return &parsed, true
}

func buildMeOpsMonitoringWhereClause(scope auth.MultiSchoolScope, requestedSchoolID *int, q string) (string, []interface{}, int) {
	whereParts := make([]string, 0, 3)
	args := make([]interface{}, 0, 5)
	argPos := 1
	if !scope.All {
		whereParts = append(whereParts, "ds.id = ANY($"+strconv.Itoa(argPos)+")")
		args = append(args, scope.SchoolIDs)
		argPos++
	}
	if requestedSchoolID != nil {
		whereParts = append(whereParts, "ds.id = $"+strconv.Itoa(argPos))
		args = append(args, *requestedSchoolID)
		argPos++
	}
	if q != "" {
		whereParts = append(whereParts, "ds.nama_sekolah ILIKE $"+strconv.Itoa(argPos))
		args = append(args, "%"+q+"%")
		argPos++
	}

	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = " WHERE " + strings.Join(whereParts, " AND ")
	}
	return whereClause, args, argPos
}

func buildMeOpsMonitoringDataQueries(whereClause string, paginationClause string) (string, string) {
	dataQuery := `
		WITH filtered_schools AS (
			SELECT ds.id,
			       ds.nama_sekolah,
			       ds.bentuk_pendidikan,
			       ds.status_sekolah,
			       ds.last_update_prestasi,
			       ds.last_update_siswa_pt
			FROM data_sekolah ds
			` + whereClause + `
		),
		target_schools AS (
			SELECT id,
			       nama_sekolah,
			       bentuk_pendidikan,
			       status_sekolah,
			       last_update_prestasi,
			       last_update_siswa_pt
			FROM filtered_schools
			ORDER BY nama_sekolah ASC` + paginationClause + `
		),
		validation_stats AS (
			SELECT c.school_id,
			       COUNT(c.id)::int AS total_xii,
			       COALESCE(SUM(CASE WHEN o.finalized_at IS NOT NULL THEN 1 ELSE 0 END), 0)::int AS final_count
			FROM class12_students c
			LEFT JOIN student_outcomes o
			  ON o.school_id = c.school_id
			 AND o.tahun_lulus = c.tahun_lulus
			 AND o.nisn = c.nisn
			 AND o.finalized_at IS NOT NULL
			WHERE c.school_id IN (SELECT id FROM target_schools)
			GROUP BY c.school_id
		)
		SELECT p.id,
		       p.nama_sekolah,
		       p.bentuk_pendidikan,
		       p.status_sekolah,
		       p.last_update_prestasi,
		       p.last_update_siswa_pt,
		       COALESCE(v.final_count, 0) AS status_lulusan_final_count,
		       GREATEST(COALESCE(v.total_xii, 0) - COALESCE(v.final_count, 0), 0) AS pending_validasi_count
		FROM target_schools p
		LEFT JOIN validation_stats v ON v.school_id = p.id
		ORDER BY p.nama_sekolah ASC
	`
	fallbackDataQuery := `
		WITH filtered_schools AS (
			SELECT ds.id,
			       ds.nama_sekolah,
			       ds.bentuk_pendidikan,
			       ds.status_sekolah,
			       ds.last_update_prestasi,
			       ds.last_update_siswa_pt
			FROM data_sekolah ds
			` + whereClause + `
		),
		target_schools AS (
			SELECT id,
			       nama_sekolah,
			       bentuk_pendidikan,
			       status_sekolah,
			       last_update_prestasi,
			       last_update_siswa_pt
			FROM filtered_schools
			ORDER BY nama_sekolah ASC` + paginationClause + `
		)
		SELECT p.id,
		       p.nama_sekolah,
		       p.bentuk_pendidikan,
		       p.status_sekolah,
		       p.last_update_prestasi,
		       p.last_update_siswa_pt,
		       0 AS status_lulusan_final_count,
		       0 AS pending_validasi_count
		FROM target_schools p
		ORDER BY p.nama_sekolah ASC
	`
	return dataQuery, fallbackDataQuery
}

func (h SchoolHandler) queryMeOpsMonitoringSchoolRows(c *gin.Context, whereClause string, args []interface{}, paginationClause string) ([]meOpsMonitoringSchoolRow, error) {
	dataQuery, fallbackDataQuery := buildMeOpsMonitoringDataQueries(whereClause, paginationClause)
	rows, err := h.DB.Pool.Query(c, dataQuery, args...)
	if err != nil {
		if isMissingRelation(err) {
			rows, err = h.DB.Pool.Query(c, fallbackDataQuery, args...)
		}
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	result := make([]meOpsMonitoringSchoolRow, 0)
	for rows.Next() {
		var item meOpsMonitoringSchoolRow
		if scanErr := rows.Scan(
			&item.SchoolID,
			&item.SchoolName,
			&item.Jenjang,
			&item.StatusSekolah,
			&item.LastUpdatePrestasi,
			&item.LastUpdateSiswaPT,
			&item.StatusLulusanFinalCount,
			&item.PendingValidasiCount,
		); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func formatOpsMonitoringUpdateStatusForExport(value sql.NullTime) string {
	if !value.Valid {
		return "Belum update"
	}
	return "Sudah update (" + value.Time.Format("02 Jan 2006 15:04") + ")"
}

func (h SchoolHandler) MeOpsMonitoringSchools(c *gin.Context) {
	logging.Infof("school", "me_ops_monitoring_schools", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != auth.RoleAdminMaster && user.Role != auth.RoleAdminCabdin && user.Role != auth.RolePengawas {
		utils.RespondForbidden(c)
		return
	}

	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid page")
			return
		}
		page = parsed
	}

	limit := 10
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid limit")
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}
	offset := (page - 1) * limit

	requestedSchoolID, ok := parseOpsMonitoringRequestedSchoolID(c)
	if !ok {
		return
	}

	scope, scopeErr := auth.GetMultiSchoolScope(c, h.DB, user)
	if scopeErr != nil {
		if scopeErr == auth.ErrForbidden {
			utils.RespondForbidden(c)
		} else {
			logging.Errorf("school", "me_ops_monitoring_schools", "err=%v", scopeErr)
			utils.RespondInternalServerError(c)
		}
		return
	}
	if requestedSchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *requestedSchoolID); err != nil {
			utils.RespondForbidden(c)
			return
		}
	}

	if !scope.All && len(scope.SchoolIDs) == 0 {
		utils.RespondData(c, http.StatusOK, meOpsMonitoringSchoolsResponse{
			Items: []meOpsMonitoringSchoolItem{},
			Page:  page,
			Limit: limit,
			Total: 0,
		})
		return
	}

	q := strings.TrimSpace(c.Query("q"))
	whereClause, args, argPos := buildMeOpsMonitoringWhereClause(scope, requestedSchoolID, q)

	var total int
	countQuery := `SELECT COUNT(1) FROM data_sekolah ds` + whereClause
	if err := h.DB.Pool.QueryRow(c, countQuery, args...).Scan(&total); err != nil {
		logging.Errorf("school", "me_ops_monitoring_schools", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	paginationClause := "\n\t\t\tLIMIT $" + strconv.Itoa(argPos) + " OFFSET $" + strconv.Itoa(argPos+1)
	dataArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := h.queryMeOpsMonitoringSchoolRows(c, whereClause, dataArgs, paginationClause)
	if err != nil {
		logging.Errorf("school", "me_ops_monitoring_schools", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	formatTimestamp := func(value sql.NullTime) *string {
		if !value.Valid {
			return nil
		}
		formatted := value.Time.UTC().Format(time.RFC3339)
		return &formatted
	}
	readNullableText := func(value sql.NullString) *string {
		if !value.Valid {
			return nil
		}
		trimmed := strings.TrimSpace(value.String)
		if trimmed == "" {
			return nil
		}
		return &trimmed
	}

	items := make([]meOpsMonitoringSchoolItem, 0)
	for _, row := range rows {
		items = append(items, meOpsMonitoringSchoolItem{
			SchoolID:                row.SchoolID,
			SchoolName:              row.SchoolName,
			Jenjang:                 readNullableText(row.Jenjang),
			StatusSekolah:           readNullableText(row.StatusSekolah),
			PrestasiUpdated:         row.LastUpdatePrestasi.Valid,
			PrestasiLastUpdate:      formatTimestamp(row.LastUpdatePrestasi),
			SiswaUpdated:            row.LastUpdateSiswaPT.Valid,
			SiswaLastUpdate:         formatTimestamp(row.LastUpdateSiswaPT),
			StatusLulusanFinalCount: row.StatusLulusanFinalCount,
			PendingValidasiCount:    row.PendingValidasiCount,
		})
	}

	utils.RespondData(c, http.StatusOK, meOpsMonitoringSchoolsResponse{
		Items: items,
		Page:  page,
		Limit: limit,
		Total: total,
	})
}

func (h SchoolHandler) MeOpsMonitoringSchoolsExportXLSX(c *gin.Context) {
	logging.Infof("school", "me_ops_monitoring_schools_export_xlsx", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != auth.RoleAdminMaster && user.Role != auth.RoleAdminCabdin && user.Role != auth.RolePengawas {
		utils.RespondForbidden(c)
		return
	}

	requestedSchoolID, ok := parseOpsMonitoringRequestedSchoolID(c)
	if !ok {
		return
	}

	scope, scopeErr := auth.GetMultiSchoolScope(c, h.DB, user)
	if scopeErr != nil {
		if scopeErr == auth.ErrForbidden {
			utils.RespondForbidden(c)
		} else {
			logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", scopeErr)
			utils.RespondInternalServerError(c)
		}
		return
	}
	if requestedSchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *requestedSchoolID); err != nil {
			utils.RespondForbidden(c)
			return
		}
	}

	q := strings.TrimSpace(c.Query("q"))
	whereClause, args, _ := buildMeOpsMonitoringWhereClause(scope, requestedSchoolID, q)
	rows, err := h.queryMeOpsMonitoringSchoolRows(c, whereClause, args, "")
	if err != nil {
		logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	file := excelize.NewFile()
	sheet := "Monitoring"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Nama Sekolah",
		"Jenjang",
		"Status Sekolah",
		"Update Data Prestasi",
		"Update Data Siswa PT / Rencana",
		"Status Lulusan (Final)",
		"Pending Validasi",
	}
	for i, header := range headers {
		cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
		if cellErr != nil {
			logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	headerStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	redRowStyle, err := file.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"FEE2E2"}, Pattern: 1},
	})
	if err != nil {
		logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	greenRowStyle, err := file.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"DCFCE7"}, Pattern: 1},
	})
	if err != nil {
		logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if err := file.SetCellStyle(sheet, "A1", "G1", headerStyle); err != nil {
		logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	readNullableText := func(value sql.NullString) string {
		if !value.Valid {
			return "-"
		}
		trimmed := strings.TrimSpace(value.String)
		if trimmed == "" {
			return "-"
		}
		return trimmed
	}

	for i, row := range rows {
		rowNum := i + 2
		rowValues := []interface{}{
			row.SchoolName,
			readNullableText(row.Jenjang),
			readNullableText(row.StatusSekolah),
			formatOpsMonitoringUpdateStatusForExport(row.LastUpdatePrestasi),
			formatOpsMonitoringUpdateStatusForExport(row.LastUpdateSiswaPT),
			row.StatusLulusanFinalCount,
			row.PendingValidasiCount,
		}
		cell, cellErr := excelize.CoordinatesToCellName(1, rowNum)
		if cellErr != nil {
			logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}

		isGreen := row.LastUpdatePrestasi.Valid &&
			row.LastUpdateSiswaPT.Valid &&
			row.StatusLulusanFinalCount > 0 &&
			row.PendingValidasiCount > 0
		rowStyleID := redRowStyle
		if isGreen {
			rowStyleID = greenRowStyle
		}
		if err := file.SetCellStyle(sheet, "A"+strconv.Itoa(rowNum), "G"+strconv.Itoa(rowNum), rowStyleID); err != nil {
			logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		logging.Errorf("school", "me_ops_monitoring_schools_export_xlsx", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\"monitoring-operasional-sekolah.xlsx\"")
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

type SchoolUpdate struct {
	NPSNSekolah         *string `json:"npsnSekolah"`
	NamaSekolah         *string `json:"namaSekolah"`
	BentukPendidikan    *string `json:"bentukPendidikan"`
	StatusSekolah       *string `json:"statusSekolah"`
	TMTAkreditasi       *string `json:"tmtAkreditasi"`
	Akreditasi          *string `json:"akreditasi"`
	JumlahGuruAsn       *int    `json:"jumlahGuruAsn"`
	JumlahGuruHonorer   *int    `json:"jumlahGuruHonorer"`
	JumlahTendikAsn     *int    `json:"jumlahTendikAsn"`
	JumlahTendikHonorer *int    `json:"jumlahTendikHonorer"`
}

type SettingsUpdate struct {
	School *SchoolUpdatePayload `json:"school"`
	User   *UserUpdatePayload   `json:"user"`
}

type SchoolUpdatePayload struct {
	NPSNSekolah         *string `json:"npsnSekolah"`
	NamaSekolah         *string `json:"namaSekolah"`
	BentukPendidikan    *string `json:"bentukPendidikan"`
	StatusSekolah       *string `json:"statusSekolah"`
	TMTAkreditasi       *string `json:"tmtAkreditasi"`
	Akreditasi          *string `json:"akreditasi"`
	JumlahGuruAsn       *int    `json:"jumlahGuruAsn"`
	JumlahGuruHonorer   *int    `json:"jumlahGuruHonorer"`
	JumlahTendikAsn     *int    `json:"jumlahTendikAsn"`
	JumlahTendikHonorer *int    `json:"jumlahTendikHonorer"`
}

type UserUpdatePayload struct {
	Username        *string `json:"username"`
	CurrentPassword *string `json:"current_password"`
	NewPassword     *string `json:"new_password"`
}

func (h SchoolHandler) Update(c *gin.Context) {
	logging.Infof("school", "update", "event=hit")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "Invalid school id")
		return
	}

	var payload SchoolUpdate
	if err := c.ShouldBindJSON(&payload); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if payload.NPSNSekolah == nil {
		utils.RespondBadRequestMessage(c, "NPSN_REQUIRED")
		return
	}
	normalizedNPSN := utils.NormalizeNPSN(*payload.NPSNSekolah)
	if !utils.IsValidNPSN(normalizedNPSN) {
		utils.RespondBadRequestMessage(c, "NPSN_INVALID")
		return
	}
	payload.NPSNSekolah = &normalizedNPSN

	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	service := services.SchoolService{DB: h.DB}
	school, err := service.Update(c, user, id, services.SchoolUpdate{
		NPSNSekolah:         payload.NPSNSekolah,
		NamaSekolah:         payload.NamaSekolah,
		BentukPendidikan:    payload.BentukPendidikan,
		StatusSekolah:       payload.StatusSekolah,
		TMTAkreditasi:       payload.TMTAkreditasi,
		Akreditasi:          payload.Akreditasi,
		JumlahGuruAsn:       payload.JumlahGuruAsn,
		JumlahGuruHonorer:   payload.JumlahGuruHonorer,
		JumlahTendikAsn:     payload.JumlahTendikAsn,
		JumlahTendikHonorer: payload.JumlahTendikHonorer,
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("school", "update", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, map[string]interface{}{
		"id":                  school.ID,
		"npsnSekolah":         school.NPSNSekolah,
		"namaSekolah":         school.NamaSekolah,
		"bentukPendidikan":    school.BentukPendidikan,
		"statusSekolah":       school.StatusSekolah,
		"tmtAkreditasi":       school.TMTAkreditasi,
		"akreditasi":          school.Akreditasi,
		"jumlahTotalGuru":     school.JumlahTotalGuru,
		"jumlahGuruAsn":       school.JumlahGuruAsn,
		"jumlahGuruHonorer":   school.JumlahGuruHonorer,
		"jumlahTotalTendik":   school.JumlahTotalTendik,
		"jumlahTendikAsn":     school.JumlahTendikAsn,
		"jumlahTendikHonorer": school.JumlahTendikHonorer,
	})
}

func (h SchoolHandler) Settings(c *gin.Context) {
	logging.Infof("school", "settings", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "SCHOOL_ADMIN" || user.SchoolID == nil {
		utils.RespondForbidden(c)
		return
	}

	service := services.SchoolService{DB: h.DB}
	result, err := service.List(c, user, services.SchoolFilters{
		SchoolID: user.SchoolID,
	})
	if err != nil || len(result) == 0 {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		if err != nil {
			logging.Errorf("school", "settings", "err=%v", err)
		} else {
			logging.Errorf("school", "settings", "err=empty_result")
		}
		utils.RespondInternalServerError(c)
		return
	}

	authService := services.AuthService{DB: h.DB}
	userRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("school", "settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	school := result[0]
	var lastUpdatePrestasi *string
	if school.LastUpdatePrestasi != nil {
		formatted := school.LastUpdatePrestasi.UTC().Format(time.RFC3339)
		lastUpdatePrestasi = &formatted
	}
	var lastUpdateSiswaPT *string
	if school.LastUpdateSiswaPT != nil {
		formatted := school.LastUpdateSiswaPT.UTC().Format(time.RFC3339)
		lastUpdateSiswaPT = &formatted
	}
	utils.RespondData(c, http.StatusOK, map[string]interface{}{
		"school": map[string]interface{}{
			"id":                  school.ID,
			"npsnSekolah":         school.NPSNSekolah,
			"namaSekolah":         school.NamaSekolah,
			"bentukPendidikan":    school.BentukPendidikan,
			"statusSekolah":       school.StatusSekolah,
			"tmtAkreditasi":       school.TMTAkreditasi,
			"akreditasi":          school.Akreditasi,
			"jumlahTotalGuru":     school.JumlahTotalGuru,
			"jumlahGuruAsn":       school.JumlahGuruAsn,
			"jumlahGuruHonorer":   school.JumlahGuruHonorer,
			"jumlahTotalTendik":   school.JumlahTotalTendik,
			"jumlahTendikAsn":     school.JumlahTendikAsn,
			"jumlahTendikHonorer": school.JumlahTendikHonorer,
			"lastUpdatePrestasi":  lastUpdatePrestasi,
			"lastUpdateSiswaPt":   lastUpdateSiswaPT,
		},
		"user": map[string]interface{}{
			"id":       userRecord.ID,
			"username": userRecord.Username,
		},
	})
}

func (h SchoolHandler) UpdateSettings(c *gin.Context) {
	logging.Infof("school", "update_settings", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "SCHOOL_ADMIN" || user.SchoolID == nil {
		utils.RespondForbidden(c)
		return
	}

	var payload SettingsUpdate
	if err := c.ShouldBindJSON(&payload); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	authService := services.AuthService{DB: h.DB}
	userRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("school", "update_settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	var schoolUpdate *services.SchoolUpdate
	if payload.School != nil {
		if payload.School.NPSNSekolah == nil {
			utils.RespondBadRequestMessage(c, "NPSN_REQUIRED")
			return
		}
		normalizedNPSN := utils.NormalizeNPSN(*payload.School.NPSNSekolah)
		if !utils.IsValidNPSN(normalizedNPSN) {
			utils.RespondBadRequestMessage(c, "NPSN_INVALID")
			return
		}
		payload.School.NPSNSekolah = &normalizedNPSN
		schoolUpdate = &services.SchoolUpdate{
			NPSNSekolah:         payload.School.NPSNSekolah,
			NamaSekolah:         payload.School.NamaSekolah,
			BentukPendidikan:    payload.School.BentukPendidikan,
			StatusSekolah:       payload.School.StatusSekolah,
			TMTAkreditasi:       payload.School.TMTAkreditasi,
			Akreditasi:          payload.School.Akreditasi,
			JumlahGuruAsn:       payload.School.JumlahGuruAsn,
			JumlahGuruHonorer:   payload.School.JumlahGuruHonorer,
			JumlahTendikAsn:     payload.School.JumlahTendikAsn,
			JumlahTendikHonorer: payload.School.JumlahTendikHonorer,
		}
	}

	var userUpdate *services.UserAccountUpdate
	var newUsername string
	var newPassword string
	usernameChanged := false
	passwordChanged := false
	if payload.User != nil {
		if payload.User.Username != nil {
			newUsername = strings.TrimSpace(*payload.User.Username)
			if newUsername == "" {
				utils.RespondBadRequestMessage(c, "USERNAME_REQUIRED")
				return
			}
			if newUsername != userRecord.Username {
				usernameChanged = true
			}
		}
		if payload.User.NewPassword != nil {
			newPassword = strings.TrimSpace(*payload.User.NewPassword)
			if newPassword != "" {
				passwordChanged = true
			}
		}
	}

	if usernameChanged || passwordChanged {
		currentPassword := ""
		if payload.User != nil && payload.User.CurrentPassword != nil {
			currentPassword = strings.TrimSpace(*payload.User.CurrentPassword)
		}
		if currentPassword == "" {
			utils.RespondBadRequestMessage(c, "PASSWORD_LAMA_REQUIRED")
			return
		}

		now := time.Now()
		if !settingsRateLimiter.Allow(user.ID, now) {
			utils.RespondTooManyRequests(c)
			return
		}

		if !utils.VerifyPassword(currentPassword, userRecord.PasswordHash) {
			settingsRateLimiter.RecordFailure(user.ID, now)
			utils.RespondBadRequestMessage(c, "PASSWORD_LAMA_SALAH")
			return
		}
		settingsRateLimiter.Reset(user.ID)
	}

	if usernameChanged {
		exists, err := authService.UsernameExists(c, newUsername, &user.ID)
		if err != nil {
			logging.Errorf("school", "update_settings", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		if exists {
			utils.RespondBadRequestMessage(c, "USERNAME_TAK_TERSEDIA")
			return
		}
		userUpdate = &services.UserAccountUpdate{UserID: user.ID, Username: &newUsername}
	}

	if passwordChanged {
		if len(newPassword) < 8 {
			utils.RespondBadRequestMessage(c, "PASSWORD_MIN_8")
			return
		}
		hash, err := utils.HashPassword(newPassword)
		if err != nil {
			logging.Errorf("school", "update_settings", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		if userUpdate == nil {
			userUpdate = &services.UserAccountUpdate{UserID: user.ID}
		}
		userUpdate.PasswordHash = &hash
	}

	settingsService := services.SettingsService{DB: h.DB}
	if err := settingsService.UpdateSettings(c, user, schoolUpdate, userUpdate); err != nil {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		logging.Errorf("school", "update_settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	schoolService := services.SchoolService{DB: h.DB}
	result, err := schoolService.List(c, user, services.SchoolFilters{SchoolID: user.SchoolID})
	if err != nil || len(result) == 0 {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		if err != nil {
			logging.Errorf("school", "update_settings", "err=%v", err)
		} else {
			logging.Errorf("school", "update_settings", "err=empty_result")
		}
		utils.RespondInternalServerError(c)
		return
	}

	userRecord, err = authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("school", "update_settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	school := result[0]
	utils.RespondData(c, http.StatusOK, map[string]interface{}{
		"school": map[string]interface{}{
			"id":                  school.ID,
			"npsnSekolah":         school.NPSNSekolah,
			"namaSekolah":         school.NamaSekolah,
			"bentukPendidikan":    school.BentukPendidikan,
			"statusSekolah":       school.StatusSekolah,
			"tmtAkreditasi":       school.TMTAkreditasi,
			"akreditasi":          school.Akreditasi,
			"jumlahTotalGuru":     school.JumlahTotalGuru,
			"jumlahGuruAsn":       school.JumlahGuruAsn,
			"jumlahGuruHonorer":   school.JumlahGuruHonorer,
			"jumlahTotalTendik":   school.JumlahTotalTendik,
			"jumlahTendikAsn":     school.JumlahTendikAsn,
			"jumlahTendikHonorer": school.JumlahTendikHonorer,
		},
		"user": map[string]interface{}{
			"id":       userRecord.ID,
			"username": userRecord.Username,
		},
	})
}
