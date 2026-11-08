package handlers

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type AdminHandler struct {
	DB                  *db.DB
	MasterEncryptionKey []byte
}

type adminUserRequest struct {
	Username     string  `json:"username"`
	Password     string  `json:"password"`
	Role         string  `json:"role"`
	SchoolID     *int    `json:"schoolId"`
	NamaInstansi *string `json:"namaInstansi"`
	NPSNInstansi *string `json:"npsnInstansi"`
}

type cabdinAdminResponse struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Username  string   `json:"username"`
	Levels    []string `json:"levels"`
	CreatedAt string   `json:"createdAt"`
}

type cabdinAdminCreateRequest struct {
	Name     string   `json:"name"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Levels   []string `json:"levels"`
}

type cabdinAdminLevelsRequest struct {
	Levels []string `json:"levels"`
}

const (
	reauthWindow      = 5 * time.Minute
	reauthMaxFailures = 10
)

var reauthFailureTracker = struct {
	mu       sync.Mutex
	failures map[int][]time.Time
}{
	failures: map[int][]time.Time{},
}

func isReauthBlocked(adminID int, now time.Time) bool {
	reauthFailureTracker.mu.Lock()
	defer reauthFailureTracker.mu.Unlock()

	failures := reauthFailureTracker.failures[adminID]
	if len(failures) == 0 {
		return false
	}

	cutoff := now.Add(-reauthWindow)
	pruned := failures[:0]
	for _, ts := range failures {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}
	reauthFailureTracker.failures[adminID] = pruned
	return len(pruned) >= reauthMaxFailures
}

func recordReauthFailure(adminID int, now time.Time) {
	reauthFailureTracker.mu.Lock()
	defer reauthFailureTracker.mu.Unlock()

	reauthFailureTracker.failures[adminID] = append(reauthFailureTracker.failures[adminID], now)
}

func (h AdminHandler) CreateUser(c *gin.Context) {
	logging.Infof("admin", "create_user", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "ADMIN_MASTER" {
		utils.RespondForbidden(c)
		return
	}

	var req adminUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	if req.Username == "" || req.Password == "" || req.Role == "" {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if req.SchoolID == nil {
		if req.NPSNInstansi == nil {
			utils.RespondBadRequestMessage(c, "NPSN_REQUIRED")
			return
		}
		normalizedNPSN := utils.NormalizeNPSN(*req.NPSNInstansi)
		if !utils.IsValidNPSN(normalizedNPSN) {
			utils.RespondBadRequestMessage(c, "NPSN_INVALID")
			return
		}
		req.NPSNInstansi = &normalizedNPSN
	}
	if user.Role == "ADMIN" {
		if req.SchoolID == nil {
			utils.RespondForbidden(c)
			return
		}
		if !ensureAdminSchoolLevel(c, h.DB, user, *req.SchoolID) {
			return
		}
	}

	adminService := services.AdminService{DB: h.DB}

	var initialPasswordCiphertext *string
	var initialPasswordNonce *string
	var initialPasswordEncrypted *string
	if req.Role == "SCHOOL_ADMIN" {
		support, supErr := adminService.InitialPasswordSupport(c)
		if supErr != nil {
			logging.Errorf("admin", "create_user", "err=%v", supErr)
			utils.RespondInternalServerError(c)
			return
		}
		if support.V2 {
			ciphertext, nonce, encErr := utils.EncryptInitialPassword(h.MasterEncryptionKey, req.Password)
			if encErr != nil {
				logging.Errorf("admin", "create_user", "err=%v", encErr)
				utils.RespondInternalServerError(c)
				return
			}
			initialPasswordCiphertext = &ciphertext
			initialPasswordNonce = &nonce
		} else if support.V1 {
			encrypted, encErr := utils.EncryptString(h.MasterEncryptionKey, req.Password)
			if encErr != nil {
				logging.Errorf("admin", "create_user", "err=%v", encErr)
				utils.RespondInternalServerError(c)
				return
			}
			initialPasswordEncrypted = &encrypted
		}
	}

	record, err := adminService.CreateSchoolUser(c, services.AdminUserRequest{
		Username:                  req.Username,
		Password:                  req.Password,
		Role:                      req.Role,
		SchoolID:                  req.SchoolID,
		NamaInstansi:              req.NamaInstansi,
		NPSNInstansi:              req.NPSNInstansi,
		InitialPasswordCiphertext: initialPasswordCiphertext,
		InitialPasswordNonce:      initialPasswordNonce,
		InitialPasswordEncrypted:  initialPasswordEncrypted,
	})
	if err != nil {
		switch err.Error() {
		case "invalid payload", "only SCHOOL_ADMIN is supported", "school not found", "school info is required":
			utils.RespondBadRequest(c, err.Error())
		default:
			logging.Errorf("admin", "create_user", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, userResponse{
		ID:           record.ID,
		Username:     record.Username,
		Role:         record.Role,
		SchoolID:     record.SchoolID,
		NPSNInstansi: derefString(record.NPSNInstansi),
		NamaInstansi: derefString(record.NamaInstansi),
	})
}

func (h AdminHandler) ListCabdinAdmins(c *gin.Context) {
	logging.Infof("admin", "list_cabdin_admins", "event=hit")
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
	records, err := service.ListCabdinAdmins(c)
	if err != nil {
		logging.Errorf("admin", "list_cabdin_admins", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	payload := make([]cabdinAdminResponse, 0, len(records))
	for _, record := range records {
		payload = append(payload, cabdinAdminResponse{
			ID:        record.ID,
			Name:      record.Name,
			Username:  record.Username,
			Levels:    record.Levels,
			CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdminHandler) CreateCabdinAdmin(c *gin.Context) {
	logging.Infof("admin", "create_cabdin_admin", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	var req cabdinAdminCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	service := services.AdminService{DB: h.DB}
	record, err := service.CreateCabdinAdmin(c, services.CabdinAdminCreate{
		Name:     req.Name,
		Username: req.Username,
		Password: req.Password,
		Levels:   req.Levels,
	})
	if err != nil {
		switch err.Error() {
		case "INVALID_PAYLOAD", "PASSWORD_MIN_8", "LEVELS_REQUIRED", "LEVEL_INVALID", "USERNAME_EXISTS":
			utils.RespondBadRequestMessage(c, err.Error())
		default:
			logging.Errorf("admin", "create_cabdin_admin", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, cabdinAdminResponse{
		ID:        record.ID,
		Name:      record.Name,
		Username:  record.Username,
		Levels:    record.Levels,
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func (h AdminHandler) UpdateCabdinAdminLevels(c *gin.Context) {
	logging.Infof("admin", "update_cabdin_admin_levels", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		utils.RespondBadRequest(c, "Invalid user id")
		return
	}

	var req cabdinAdminLevelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	service := services.AdminService{DB: h.DB}
	record, err := service.UpdateCabdinAdminLevels(c, userID, req.Levels)
	if err != nil {
		switch err.Error() {
		case "INVALID_USER_ID", "LEVELS_REQUIRED", "LEVEL_INVALID", "USER_NOT_FOUND", "USER_NOT_CABDIN":
			utils.RespondBadRequestMessage(c, err.Error())
		default:
			logging.Errorf("admin", "update_cabdin_admin_levels", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, cabdinAdminResponse{
		ID:        record.ID,
		Name:      record.Name,
		Username:  record.Username,
		Levels:    record.Levels,
		CreatedAt: record.CreatedAt.UTC().Format(time.RFC3339),
	})
}

type schoolAdminResponse struct {
	ID                 int     `json:"id"`
	Username           string  `json:"username"`
	SchoolID           *int    `json:"schoolId"`
	NamaSekolah        *string `json:"namaSekolah,omitempty"`
	NamaInstansi       *string `json:"namaInstansi,omitempty"`
	NPSNInstansi       *string `json:"npsnInstansi,omitempty"`
	LastSeenAt         *string `json:"lastSeenAt,omitempty"`
	IsOnline           bool    `json:"isOnline"`
	HasInitialPassword bool    `json:"hasInitialPassword"`
	LastUpdatePrestasi *string `json:"lastUpdatePrestasi,omitempty"`
	LastUpdateSiswaPT  *string `json:"lastUpdateSiswaPt,omitempty"`
}

func (h AdminHandler) ListSchoolAdmins(c *gin.Context) {
	logging.Infof("admin", "list_school_admins", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	now := time.Now().UTC()
	var schoolID *int
	if raw := strings.TrimSpace(c.Query("schoolId")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid schoolId")
			return
		}
		schoolID = &parsed
	}

	page := 1
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed < 1 {
			utils.RespondBadRequest(c, "Invalid page")
			return
		}
		page = parsed
	}
	pageSize := 10
	if raw := strings.TrimSpace(c.Query("pageSize")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed < 1 {
			utils.RespondBadRequest(c, "Invalid pageSize")
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		pageSize = parsed
	}
	offset := (page - 1) * pageSize

	filters := services.SchoolAdminFilters{
		SortBy:   strings.TrimSpace(c.Query("sortBy")),
		Order:    strings.TrimSpace(c.Query("order")),
		Query:    strings.TrimSpace(c.Query("q")),
		SchoolID: schoolID,
		Limit:    pageSize,
		Offset:   offset,
	}
	if user.Role == "ADMIN" {
		filters.AllowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if filters.AllowedLevels == nil {
			filters.AllowedLevels = []string{}
		}
	}
	service := services.AdminService{DB: h.DB}
	records, err := service.ListSchoolAdmins(c, now, filters)
	if err != nil {
		logging.Errorf("admin", "list_school_admins", "err=%v sort_by=%s order=%s q=%s school_id=%v", err, filters.SortBy, filters.Order, filters.Query, filters.SchoolID)
		logging.Errorf("admin", "list_school_admins", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	totalCount, err := service.CountSchoolAdmins(c, filters)
	if err != nil {
		logging.Errorf("admin", "count_school_admins", "err=%v sort_by=%s order=%s q=%s school_id=%v", err, filters.SortBy, filters.Order, filters.Query, filters.SchoolID)
		logging.Errorf("admin", "count_school_admins", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	var payload []schoolAdminResponse
	for _, record := range records {
		namaSekolah := record.NamaSekolah
		if namaSekolah == nil || strings.TrimSpace(*namaSekolah) == "" {
			fallback := "-"
			namaSekolah = &fallback
		}
		var lastSeen *string
		if record.LastSeenAt != nil {
			formatted := record.LastSeenAt.Format(time.RFC3339)
			lastSeen = &formatted
		}
		var lastUpdatePrestasi *string
		if record.LastUpdatePrestasi != nil {
			formatted := record.LastUpdatePrestasi.UTC().Format(time.RFC3339)
			lastUpdatePrestasi = &formatted
		}
		var lastUpdateSiswaPT *string
		if record.LastUpdateSiswaPT != nil {
			formatted := record.LastUpdateSiswaPT.UTC().Format(time.RFC3339)
			lastUpdateSiswaPT = &formatted
		}
		payload = append(payload, schoolAdminResponse{
			ID:                 record.ID,
			Username:           record.Username,
			SchoolID:           record.SchoolID,
			NamaSekolah:        namaSekolah,
			NamaInstansi:       record.NamaInstansi,
			NPSNInstansi:       record.NPSNInstansi,
			LastSeenAt:         lastSeen,
			IsOnline:           record.IsOnline,
			HasInitialPassword: record.HasInitialPassword,
			LastUpdatePrestasi: lastUpdatePrestasi,
			LastUpdateSiswaPT:  lastUpdateSiswaPT,
		})
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"data": payload,
		"meta": gin.H{
			"total_count": totalCount,
			"total":       totalCount,
		},
	})
}

type schoolAdminUpdateRequest struct {
	Username     string  `json:"username"`
	Password     *string `json:"password"`
	SchoolID     *int    `json:"schoolId"`
	NamaInstansi *string `json:"namaInstansi"`
	NPSNInstansi *string `json:"npsnInstansi"`
}

func (h AdminHandler) UpdateSchoolAdmin(c *gin.Context) {
	logging.Infof("admin", "update_school_admin", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "Invalid user id")
		return
	}
	if user.Role == "ADMIN" {
		var targetSchoolID sql.NullInt32
		if err := h.DB.Pool.QueryRow(c, `SELECT school_id FROM users WHERE id = $1 AND role = 'SCHOOL_ADMIN'`, userID).Scan(&targetSchoolID); err != nil {
			utils.RespondBadRequest(c, "Invalid user id")
			return
		}
		if targetSchoolID.Valid {
			if !ensureAdminSchoolLevel(c, h.DB, user, int(targetSchoolID.Int32)) {
				return
			}
		}
	}

	var req schoolAdminUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if req.Password != nil {
		utils.RespondBadRequest(c, "Password updates are not allowed on this endpoint")
		return
	}
	if user.Role == "ADMIN" {
		var existingSchoolID sql.NullInt32
		if err := h.DB.Pool.QueryRow(c, `SELECT school_id FROM users WHERE id = $1 AND role = 'SCHOOL_ADMIN'`, userID).Scan(&existingSchoolID); err != nil {
			utils.RespondBadRequest(c, "Invalid user id")
			return
		}
		if existingSchoolID.Valid {
			if !ensureAdminSchoolLevel(c, h.DB, user, int(existingSchoolID.Int32)) {
				return
			}
		}
		if req.SchoolID != nil {
			if !ensureAdminSchoolLevel(c, h.DB, user, *req.SchoolID) {
				return
			}
		}
	}

	service := services.AdminService{DB: h.DB}
	record, err := service.UpdateSchoolAdmin(c, userID, services.SchoolAdminUpdate{
		Username:     req.Username,
		SchoolID:     req.SchoolID,
		NamaInstansi: req.NamaInstansi,
		NPSNInstansi: req.NPSNInstansi,
	})
	if err != nil {
		utils.RespondBadRequestMessage(c, err.Error())
		return
	}

	var lastSeen *string
	isOnline := false
	if record.LastSeenAt != nil {
		formatted := record.LastSeenAt.Format(time.RFC3339)
		lastSeen = &formatted
		isOnline = services.IsUserOnline(record.LastSeenAt, time.Now().UTC())
	}

	utils.RespondData(c, http.StatusOK, schoolAdminResponse{
		ID:                 record.ID,
		Username:           record.Username,
		SchoolID:           record.SchoolID,
		NamaSekolah:        record.NamaSekolah,
		NamaInstansi:       record.NamaInstansi,
		NPSNInstansi:       record.NPSNInstansi,
		LastSeenAt:         lastSeen,
		IsOnline:           isOnline,
		HasInitialPassword: record.HasInitialPassword,
	})
}

type schoolAdminPasswordResetRequest struct {
	AdminPassword string `json:"admin_password"`
	NewPassword   string `json:"new_password"`
}

func (h AdminHandler) UpdateSchoolAdminPassword(c *gin.Context) {
	logging.Infof("admin", "update_school_admin_password", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "Invalid user id")
		return
	}
	if user.Role == "ADMIN" {
		var targetSchoolID sql.NullInt32
		if err := h.DB.Pool.QueryRow(c, `SELECT school_id FROM users WHERE id = $1 AND role = 'SCHOOL_ADMIN'`, userID).Scan(&targetSchoolID); err != nil {
			utils.RespondBadRequest(c, "Invalid user id")
			return
		}
		if targetSchoolID.Valid {
			if !ensureAdminSchoolLevel(c, h.DB, user, int(targetSchoolID.Int32)) {
				return
			}
		}
	}

	var req schoolAdminPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	adminPassword := strings.TrimSpace(req.AdminPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if adminPassword == "" || newPassword == "" {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if len(newPassword) < 8 {
		utils.RespondBadRequestMessage(c, "PASSWORD_MIN_8")
		return
	}

	authService := services.AuthService{DB: h.DB}
	adminRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("admin", "update_school_admin_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !utils.VerifyPassword(adminPassword, adminRecord.PasswordHash) {
		utils.RespondBadRequestMessage(c, "ADMIN_PASSWORD_SALAH")
		return
	}

	service := services.AdminService{DB: h.DB}
	if err := service.UpdateSchoolAdminPassword(c, userID, newPassword, h.MasterEncryptionKey); err != nil {
		switch err.Error() {
		case "password too short", "invalid payload", "school admin not found":
			utils.RespondBadRequest(c, err.Error())
		default:
			logging.Errorf("admin", "update_school_admin_password", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{Message: "password updated"})
}

type schoolListResponse struct {
	ID          int     `json:"id"`
	NPSN        *string `json:"npsnSekolah,omitempty"`
	NamaSekolah string  `json:"namaSekolah"`
}

type schoolSuggestResponse struct {
	ID          int     `json:"id"`
	NPSN        *string `json:"npsn"`
	NamaSekolah string  `json:"nama_sekolah"`
}

type monitoringSchoolResponse struct {
	SchoolID         int    `json:"school_id"`
	NamaSekolah      string `json:"nama_sekolah"`
	BentukPendidikan string `json:"bentuk_pendidikan"`
	Akreditasi       string `json:"akreditasi"`
	GuruAsn          int    `json:"guru_asn"`
	GuruHonorer      int    `json:"guru_honorer"`
	TendikAsn        int    `json:"tendik_asn"`
	TendikHonorer    int    `json:"tendik_honorer"`
}

func (h AdminHandler) ListSchools(c *gin.Context) {
	logging.Infof("admin", "list_schools", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	service := services.AdminService{DB: h.DB}
	var allowedLevels []string
	if user.Role == "ADMIN" {
		allowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if allowedLevels == nil {
			allowedLevels = []string{}
		}
	}
	records, err := service.ListSchools(c, allowedLevels)
	if err != nil {
		logging.Errorf("admin", "list_schools", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	payload := make([]schoolListResponse, 0, len(records))
	for _, record := range records {
		payload = append(payload, schoolListResponse{
			ID:          record.ID,
			NPSN:        record.NPSN,
			NamaSekolah: record.NamaSekolah,
		})
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdminHandler) SuggestSchools(c *gin.Context) {
	logging.Infof("admin", "suggest_schools", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		utils.RespondData(c, http.StatusOK, []schoolSuggestResponse{})
		return
	}

	limit := 10
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, convErr := strconv.Atoi(raw); convErr == nil {
			limit = parsed
		}
	}

	service := services.AdminService{DB: h.DB}
	var allowedLevels []string
	if user.Role == "ADMIN" {
		allowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if allowedLevels == nil {
			allowedLevels = []string{}
		}
	}
	records, err := service.SuggestSchools(c, query, limit, allowedLevels)
	if err != nil {
		logging.Errorf("admin", "suggest_schools", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	payload := make([]schoolSuggestResponse, 0, len(records))
	for _, record := range records {
		payload = append(payload, schoolSuggestResponse{
			ID:          record.ID,
			NPSN:        record.NPSN,
			NamaSekolah: record.NamaSekolah,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdminHandler) ListMonitoringSchools(c *gin.Context) {
	logging.Infof("admin", "list_monitoring_schools", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	var schoolID *int
	if raw := strings.TrimSpace(c.Query("schoolId")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid schoolId")
			return
		}
		schoolID = &id
	}

	parseOptionalInt := func(key string) (*int, error) {
		raw := strings.TrimSpace(c.Query(key))
		if raw == "" {
			return nil, nil
		}
		value, convErr := strconv.Atoi(raw)
		if convErr != nil {
			return nil, convErr
		}
		return &value, nil
	}

	limit := 10
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, convErr := strconv.Atoi(raw); convErr == nil && parsed > 0 {
			limit = parsed
		}
	}
	offset := 0
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		if parsed, convErr := strconv.Atoi(raw); convErr == nil && parsed >= 0 {
			offset = parsed
		}
	}

	guruAsnMin, err := parseOptionalInt("guruAsnMin")
	if err != nil {
		utils.RespondBadRequest(c, "Invalid guruAsnMin")
		return
	}
	guruAsnMax, err := parseOptionalInt("guruAsnMax")
	if err != nil {
		utils.RespondBadRequest(c, "Invalid guruAsnMax")
		return
	}
	tendikAsnMin, err := parseOptionalInt("tendikAsnMin")
	if err != nil {
		utils.RespondBadRequest(c, "Invalid tendikAsnMin")
		return
	}
	tendikAsnMax, err := parseOptionalInt("tendikAsnMax")
	if err != nil {
		utils.RespondBadRequest(c, "Invalid tendikAsnMax")
		return
	}

	filters := services.MonitoringSchoolFilters{
		Jenjang:      strings.TrimSpace(c.Query("jenjang")),
		Status:       strings.TrimSpace(c.Query("status")),
		Akreditasi:   strings.TrimSpace(c.Query("akreditasi")),
		SchoolID:     schoolID,
		Query:        strings.TrimSpace(c.Query("q")),
		SortBy:       strings.TrimSpace(c.Query("sortBy")),
		Order:        strings.TrimSpace(c.Query("order")),
		Limit:        limit,
		Offset:       offset,
		GuruAsnMin:   guruAsnMin,
		GuruAsnMax:   guruAsnMax,
		TendikAsnMin: tendikAsnMin,
		TendikAsnMax: tendikAsnMax,
	}
	if user.Role == "ADMIN" {
		filters.AllowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if filters.AllowedLevels == nil {
			filters.AllowedLevels = []string{}
		}
	}

	service := services.AdminService{DB: h.DB}
	rows, err := service.ListMonitoringSchools(c, filters)
	if err != nil {
		logging.Errorf("admin", "list_monitoring_schools", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	payload := make([]monitoringSchoolResponse, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, monitoringSchoolResponse{
			SchoolID:         row.SchoolID,
			NamaSekolah:      row.NamaSekolah,
			BentukPendidikan: row.BentukPendidikan,
			Akreditasi:       row.Akreditasi,
			GuruAsn:          row.GuruAsn,
			GuruHonorer:      row.GuruHonorer,
			TendikAsn:        row.TendikAsn,
			TendikHonorer:    row.TendikHonorer,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AdminHandler) ExportMonitoringSchools(c *gin.Context) {
	logging.Infof("admin", "export_monitoring_schools", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	var schoolID *int
	if raw := strings.TrimSpace(c.Query("schoolId")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid schoolId")
			return
		}
		schoolID = &id
	}

	parseOptionalInt := func(key string) (*int, error) {
		raw := strings.TrimSpace(c.Query(key))
		if raw == "" {
			return nil, nil
		}
		value, convErr := strconv.Atoi(raw)
		if convErr != nil {
			return nil, convErr
		}
		return &value, nil
	}

	guruAsnMin, err := parseOptionalInt("guruAsnMin")
	if err != nil {
		utils.RespondBadRequest(c, "Invalid guruAsnMin")
		return
	}
	guruAsnMax, err := parseOptionalInt("guruAsnMax")
	if err != nil {
		utils.RespondBadRequest(c, "Invalid guruAsnMax")
		return
	}
	tendikAsnMin, err := parseOptionalInt("tendikAsnMin")
	if err != nil {
		utils.RespondBadRequest(c, "Invalid tendikAsnMin")
		return
	}
	tendikAsnMax, err := parseOptionalInt("tendikAsnMax")
	if err != nil {
		utils.RespondBadRequest(c, "Invalid tendikAsnMax")
		return
	}

	filters := services.MonitoringSchoolFilters{
		Jenjang:      strings.TrimSpace(c.Query("jenjang")),
		Status:       strings.TrimSpace(c.Query("status")),
		Akreditasi:   strings.TrimSpace(c.Query("akreditasi")),
		SchoolID:     schoolID,
		Query:        strings.TrimSpace(c.Query("q")),
		SortBy:       strings.TrimSpace(c.Query("sortBy")),
		Order:        strings.TrimSpace(c.Query("order")),
		GuruAsnMin:   guruAsnMin,
		GuruAsnMax:   guruAsnMax,
		TendikAsnMin: tendikAsnMin,
		TendikAsnMax: tendikAsnMax,
	}
	if user.Role == "ADMIN" {
		filters.AllowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if filters.AllowedLevels == nil {
			filters.AllowedLevels = []string{}
		}
	}

	service := services.AdminService{DB: h.DB}
	rows, err := service.ListMonitoringSchools(c, filters)
	if err != nil {
		logging.Errorf("admin", "export_monitoring_schools", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	file := excelize.NewFile()
	sheet := "Monitoring_Sekolah"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Nama Sekolah",
		"Bentuk Sekolah",
		"Akreditasi Sekolah",
		"Guru (ASN/Yayasan)",
		"Guru Honorer",
		"Tendik (ASN/Yayasan)",
		"Tendik Honorer",
	}

	for i, header := range headers {
		cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
		if cellErr != nil {
			logging.Errorf("admin", "export_monitoring_schools", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			logging.Errorf("admin", "export_monitoring_schools", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	for i, row := range rows {
		rowValues := []interface{}{
			row.NamaSekolah,
			row.BentukPendidikan,
			row.Akreditasi,
			row.GuruAsn,
			row.GuruHonorer,
			row.TendikAsn,
			row.TendikHonorer,
		}
		cell, cellErr := excelize.CoordinatesToCellName(1, i+2)
		if cellErr != nil {
			logging.Errorf("admin", "export_monitoring_schools", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			logging.Errorf("admin", "export_monitoring_schools", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		logging.Errorf("admin", "export_monitoring_schools", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\"monitoring-data-sekolah.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

type schoolAdminCreateRequest struct {
	SchoolID int    `json:"school_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h AdminHandler) CreateSchoolAdmin(c *gin.Context) {
	logging.Infof("admin", "create_school_admin", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	var req schoolAdminCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if req.SchoolID == 0 || req.Username == "" || req.Password == "" {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if user.Role == "ADMIN" {
		if !ensureAdminSchoolLevel(c, h.DB, user, req.SchoolID) {
			return
		}
	}

	service := services.AdminService{DB: h.DB}
	support, supErr := service.InitialPasswordSupport(c)
	if supErr != nil {
		logging.Errorf("admin", "create_school_admin", "err=%v", supErr)
		utils.RespondInternalServerError(c)
		return
	}

	var initialPasswordCiphertext *string
	var initialPasswordNonce *string
	var initialPasswordEncrypted *string
	if support.V2 {
		ciphertext, nonce, encErr := utils.EncryptInitialPassword(h.MasterEncryptionKey, req.Password)
		if encErr != nil {
			logging.Errorf("admin", "create_school_admin", "err=%v", encErr)
			utils.RespondInternalServerError(c)
			return
		}
		initialPasswordCiphertext = &ciphertext
		initialPasswordNonce = &nonce
	} else if support.V1 {
		encrypted, encErr := utils.EncryptString(h.MasterEncryptionKey, req.Password)
		if encErr != nil {
			logging.Errorf("admin", "create_school_admin", "err=%v", encErr)
			utils.RespondInternalServerError(c)
			return
		}
		initialPasswordEncrypted = &encrypted
	}

	record, err := service.CreateSchoolAdmin(c, req.SchoolID, req.Username, req.Password, initialPasswordCiphertext, initialPasswordNonce, initialPasswordEncrypted)
	if err != nil {
		switch err.Error() {
		case "password too short", "username already exists", "school not found", "invalid payload":
			utils.RespondBadRequest(c, err.Error())
		default:
			logging.Errorf("admin", "create_school_admin", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, userResponse{
		ID:           record.ID,
		Username:     record.Username,
		Role:         record.Role,
		SchoolID:     record.SchoolID,
		NPSNInstansi: derefString(record.NPSNInstansi),
		NamaInstansi: derefString(record.NamaInstansi),
	})
}

type bulkGenerateRequest struct {
	Jenjang string `json:"jenjang"`
	Status  string `json:"status"`
}

type generatedSchoolAdminResponse struct {
	SchoolID    int    `json:"schoolId"`
	NamaSekolah string `json:"namaSekolah"`
	Username    string `json:"username"`
	Reason      string `json:"reason,omitempty"`
}

type skippedSchoolAdminResponse struct {
	SchoolID    int    `json:"schoolId"`
	NamaSekolah string `json:"namaSekolah"`
	Reason      string `json:"reason"`
}

type rejectedSchoolAdminResponse struct {
	SchoolID    int    `json:"schoolId"`
	NamaSekolah string `json:"namaSekolah"`
	Reason      string `json:"reason"`
}

type bulkGenerateSummaryResponse struct {
	CreatedCount   int                            `json:"createdCount"`
	GeneratedCount int                            `json:"generatedCount"`
	SkippedCount   int                            `json:"skippedCount"`
	RejectedCount  int                            `json:"rejectedCount"`
	Generated      []generatedSchoolAdminResponse `json:"generated"`
	Skipped        []skippedSchoolAdminResponse   `json:"skipped"`
	Rejected       []rejectedSchoolAdminResponse  `json:"rejected"`
}

func (h AdminHandler) GenerateSchoolAdmins(c *gin.Context) {
	logging.Infof("admin", "generate_school_admins", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	var req bulkGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if user.Role == "ADMIN" {
		if !ensureAdminJenjangAllowed(c, user, req.Jenjang) {
			return
		}
	}

	download := strings.TrimSpace(c.Query("download")) == "1" || strings.EqualFold(strings.TrimSpace(c.Query("download")), "true")

	service := services.AdminService{DB: h.DB}
	if _, supErr := service.InitialPasswordSupport(c); supErr != nil {
		logging.Errorf("admin", "generate_school_admins", "err=%v", supErr)
		utils.RespondInternalServerError(c)
		return
	}
	summary, err := service.GenerateSchoolAdmins(c, services.BulkGenerateFilters{
		Jenjang: req.Jenjang,
		Status:  req.Status,
	}, h.MasterEncryptionKey)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	action := "GENERATE_SCHOOL_ADMIN"
	if err := service.LogAdminAction(c, user.ID, action); err != nil {
		logging.Errorf("admin", "generate_school_admins", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	// ===============================
	// MODE DOWNLOAD (EXCEL)
	// ===============================
	if download {
		normalizedJenjang := strings.ToUpper(strings.TrimSpace(req.Jenjang))
		normalizedStatus := strings.ToUpper(strings.TrimSpace(req.Status))
		xlsx, xerr := buildGeneratedSchoolAdminsXLSX(summary, normalizedJenjang, normalizedStatus)
		if xerr != nil {
			logging.Errorf("admin", "generate_school_admins", "err=%v", xerr)
			utils.RespondInternalServerError(c)
			return
		}

		c.Header("Cache-Control", "no-store")
		c.Header("Content-Disposition", "attachment; filename=\"akun-admin-sekolah.xlsx\"")
		c.Data(
			http.StatusOK,
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			xlsx,
		)
		return
	}

	generated := make([]generatedSchoolAdminResponse, 0, len(summary.Generated))
	for _, item := range summary.Generated {
		generated = append(generated, generatedSchoolAdminResponse{
			SchoolID:    item.SchoolID,
			NamaSekolah: item.NamaSekolah,
			Username:    item.Username,
			Reason:      item.Reason,
		})
	}
	skipped := make([]skippedSchoolAdminResponse, 0, len(summary.Skipped))
	for _, item := range summary.Skipped {
		skipped = append(skipped, skippedSchoolAdminResponse{
			SchoolID:    item.SchoolID,
			NamaSekolah: item.NamaSekolah,
			Reason:      item.Reason,
		})
	}
	rejected := make([]rejectedSchoolAdminResponse, 0, len(summary.Rejected))
	for _, item := range summary.Rejected {
		rejected = append(rejected, rejectedSchoolAdminResponse{
			SchoolID:    item.SchoolID,
			NamaSekolah: item.NamaSekolah,
			Reason:      item.Reason,
		})
	}

	utils.RespondData(c, http.StatusOK, bulkGenerateSummaryResponse{
		CreatedCount:   len(summary.Generated),
		GeneratedCount: len(summary.Generated),
		SkippedCount:   len(summary.Skipped),
		RejectedCount:  len(summary.Rejected),
		Generated:      generated,
		Skipped:        skipped,
		Rejected:       rejected,
	})
}

type reauthRequest struct {
	AdminPassword string `json:"adminPassword"`
}

type revealSchoolAdminPasswordResponse struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	NPSN        string `json:"npsn"`
	NamaSekolah string `json:"namaSekolah"`
}

func (h AdminHandler) RevealSchoolAdminPassword(c *gin.Context) {
	logging.Infof("admin", "reveal_school_admin_password", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	targetUserID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "Invalid user id")
		return
	}

	var req reauthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	adminPassword := strings.TrimSpace(req.AdminPassword)
	if adminPassword == "" {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	now := time.Now().UTC()
	if isReauthBlocked(user.ID, now) {
		c.JSON(http.StatusTooManyRequests, utils.APIResponse{Message: "RATE_LIMIT"})
		return
	}

	authService := services.AuthService{DB: h.DB}
	adminRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("admin", "reveal_school_admin_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !utils.VerifyPassword(adminPassword, adminRecord.PasswordHash) {
		recordReauthFailure(user.ID, now)
		utils.RespondBadRequestMessage(c, "ADMIN_PASSWORD_SALAH")
		return
	}

	service := services.AdminService{DB: h.DB}
	cred, err := service.GetSchoolAdminCredential(c, targetUserID)
	if err != nil {
		utils.RespondBadRequest(c, "school admin not found")
		return
	}
	if user.Role == "ADMIN" {
		if cred.SchoolID == nil {
			utils.RespondForbidden(c)
			return
		}
		if !ensureAdminSchoolLevel(c, h.DB, user, *cred.SchoolID) {
			return
		}
	}

	var password string
	if cred.InitialPasswordCiphertext != nil && cred.InitialPasswordNonce != nil {
		decrypted, decErr := utils.DecryptInitialPassword(h.MasterEncryptionKey, *cred.InitialPasswordCiphertext, *cred.InitialPasswordNonce)
		if decErr != nil {
			logging.Errorf("admin", "reveal_school_admin_password", "err=%v", decErr)
			utils.RespondInternalServerError(c)
			return
		}
		password = decrypted
	} else if cred.InitialPasswordEncrypted != nil {
		decrypted, decErr := utils.DecryptString(h.MasterEncryptionKey, *cred.InitialPasswordEncrypted)
		if decErr != nil {
			logging.Errorf("admin", "reveal_school_admin_password", "err=%v", decErr)
			utils.RespondInternalServerError(c)
			return
		}
		password = decrypted
	} else {
		utils.RespondBadRequestMessage(c, "INITIAL_PASSWORD_NOT_AVAILABLE")
		return
	}

	if err := service.LogAdminActionWithDetails(c, user.ID, "REVEAL_PASSWORD", &targetUserID, nil); err != nil {
		logging.Errorf("admin", "reveal_school_admin_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, revealSchoolAdminPasswordResponse{
		Username:    cred.Username,
		Password:    password,
		NPSN:        derefString(cred.NPSN),
		NamaSekolah: derefString(cred.NamaSekolah),
	})
}

type exportSchoolAdminCredentialsRequest struct {
	AdminPassword string `json:"adminPassword"`
	Format        string `json:"format"`
}

func (h AdminHandler) ExportSchoolAdminCredentials(c *gin.Context) {
	logging.Infof("admin:export_school_admins", "export_credentials", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}
	logging.Infof("admin:export_school_admins", "export_credentials", "actor_id=%d", user.ID)

	var req exportSchoolAdminCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.Warnf("admin:export_school_admins", "bad_payload", "actor_id=%d err=%v", user.ID, err)
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	adminPassword := strings.TrimSpace(req.AdminPassword)
	if adminPassword == "" {
		logging.Warnf("admin:export_school_admins", "admin_password_required", "actor_id=%d", user.ID)
		utils.RespondBadRequestMessage(c, "ADMIN_PASSWORD_REQUIRED")
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "csv"
	}
	if format != "csv" {
		logging.Warnf("admin:export_school_admins", "invalid_format", "actor_id=%d format=%s", user.ID, format)
		utils.RespondBadRequest(c, "Invalid format")
		return
	}

	now := time.Now().UTC()
	if isReauthBlocked(user.ID, now) {
		logging.Warnf("admin:export_school_admins", "rate_limited", "actor_id=%d", user.ID)
		c.JSON(http.StatusTooManyRequests, utils.APIResponse{Error: "TOO_MANY_REQUESTS", Message: "RATE_LIMIT"})
		return
	}

	authService := services.AuthService{DB: h.DB}
	adminRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("admin:export_school_admins", "admin_lookup_failed", "actor_id=%d err=%v", user.ID, err)
		utils.RespondInternalServerError(c)
		return
	}
	if !utils.VerifyPassword(adminPassword, adminRecord.PasswordHash) {
		recordReauthFailure(user.ID, now)
		logging.Warnf("admin:export_school_admins", "reauth_failed", "actor_id=%d", user.ID)
		c.JSON(http.StatusUnauthorized, utils.APIResponse{Error: "UNAUTHORIZED", Message: "Password salah"})
		return
	}

	service := services.AdminService{DB: h.DB}
	var allowedLevels []string
	if user.Role == "ADMIN" {
		allowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if allowedLevels == nil {
			allowedLevels = []string{}
		}
	}
	records, err := service.ListSchoolAdminCredentials(c, allowedLevels)
	if err != nil {
		logging.Errorf("admin:export_school_admins", "list_credentials_failed", "actor_id=%d err=%v", user.ID, err)
		utils.RespondInternalServerError(c)
		return
	}

	if len(h.MasterEncryptionKey) == 0 {
		logging.Errorf("admin:export_school_admins", "misconfig_key_missing", "actor_id=%d", user.ID)
		utils.RespondInternalServerError(c)
		return
	}

	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{"username", "password", "npsn", "nama_sekolah"}); err != nil {
		logging.Errorf("admin:export_school_admins", "csv_header_write_failed", "actor_id=%d err=%v", user.ID, err)
		utils.RespondInternalServerError(c)
		return
	}
	decryptErrorCount := 0
	for _, record := range records {
		password := ""
		if record.InitialPasswordCiphertext != nil && record.InitialPasswordNonce != nil {
			decrypted, decErr := utils.DecryptInitialPassword(h.MasterEncryptionKey, *record.InitialPasswordCiphertext, *record.InitialPasswordNonce)
			if decErr != nil {
				decryptErrorCount++
				logging.Errorf("admin:export_school_admins", "decrypt_failed", "actor_id=%d count=%d err=%v", user.ID, decryptErrorCount, decErr)
				utils.RespondInternalServerError(c)
				return
			}
			password = decrypted
		} else if record.InitialPasswordEncrypted != nil {
			decrypted, decErr := utils.DecryptString(h.MasterEncryptionKey, *record.InitialPasswordEncrypted)
			if decErr != nil {
				decryptErrorCount++
				logging.Errorf("admin:export_school_admins", "decrypt_failed", "actor_id=%d count=%d err=%v", user.ID, decryptErrorCount, decErr)
				utils.RespondInternalServerError(c)
				return
			}
			password = decrypted
		}

		npsn := derefString(record.NPSN)
		namaSekolah := derefString(record.NamaSekolah)
		if err := writer.Write([]string{record.Username, password, npsn, namaSekolah}); err != nil {
			logging.Errorf("admin:export_school_admins", "csv_row_write_failed", "actor_id=%d err=%v", user.ID, err)
			utils.RespondInternalServerError(c)
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		logging.Errorf("admin:export_school_admins", "csv_flush_failed", "actor_id=%d err=%v", user.ID, err)
		utils.RespondInternalServerError(c)
		return
	}

	count := len(records)
	if err := service.LogAdminActionWithDetails(c, user.ID, "EXPORT_CREDENTIALS", nil, &count); err != nil {
		logging.Warnf("admin:export_school_admins", "audit_log_failed", "actor_id=%d err=%v", user.ID, err)
		/* disuruh ai
		utils.RespondInternalServerError(c)
		return */
	}

	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "attachment; filename=\"akun-admin-sekolah.csv\"")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buffer.Bytes())
	logging.Infof("admin:export_school_admins", "export_credentials", "result=success actor_id=%d count=%d", user.ID, count)
}

type resetDataRequest struct {
	SchoolID int `json:"school_id"`
	Delete   struct {
		Achievement          bool `json:"achievement"`
		SiswaPerguruanTinggi bool `json:"siswa_perguruan_tinggi"`
	} `json:"delete"`
	Password string `json:"password"`
}

type resetDataResponse struct {
	Achievement          int64   `json:"achievement"`
	SiswaPerguruanTinggi int64   `json:"siswa_perguruan_tinggi"`
	LastUpdatePrestasi   *string `json:"last_update_prestasi"`
	LastUpdateSiswaPT    *string `json:"last_update_siswa_pt"`
}

func (h AdminHandler) ResetSchoolData(c *gin.Context) {
	logging.Infof("admin", "reset_school_data", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) {
		utils.RespondForbidden(c)
		return
	}

	var req resetDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	if req.SchoolID <= 0 {
		utils.RespondBadRequest(c, "Invalid school_id")
		return
	}
	if !req.Delete.Achievement && !req.Delete.SiswaPerguruanTinggi {
		utils.RespondBadRequest(c, "At least one delete option is required")
		return
	}
	if user.Role == "ADMIN" {
		if !ensureAdminSchoolLevel(c, h.DB, user, req.SchoolID) {
			return
		}
	}

	password := strings.TrimSpace(req.Password)
	if password == "" {
		utils.RespondBadRequest(c, "Password is required")
		return
	}

	authService := services.AuthService{DB: h.DB}
	adminRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("admin", "reset_school_data", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !utils.VerifyPassword(password, adminRecord.PasswordHash) {
		utils.RespondBadRequestMessage(c, "password_invalid")
		return
	}

	var schoolExists bool
	if err := h.DB.Pool.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM data_sekolah WHERE id = $1)`, req.SchoolID).Scan(&schoolExists); err != nil {
		logging.Errorf("admin", "reset_school_data", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !schoolExists {
		utils.RespondBadRequest(c, "school not found")
		return
	}

	service := services.AdminService{DB: h.DB}
	result, err := service.ResetSchoolData(c, req.SchoolID, services.ResetDataOptions{
		Achievement:          req.Delete.Achievement,
		SiswaPerguruanTinggi: req.Delete.SiswaPerguruanTinggi,
	})
	if err != nil {
		logging.Errorf("admin", "reset_school_data", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	var lastUpdatePrestasi *string
	if result.LastUpdatePrestasi != nil {
		formatted := result.LastUpdatePrestasi.UTC().Format(time.RFC3339)
		lastUpdatePrestasi = &formatted
	}
	var lastUpdateSiswaPT *string
	if result.LastUpdateSiswaPT != nil {
		formatted := result.LastUpdateSiswaPT.UTC().Format(time.RFC3339)
		lastUpdateSiswaPT = &formatted
	}

	utils.RespondData(c, http.StatusOK, resetDataResponse{
		Achievement:          result.AchievementDeleted,
		SiswaPerguruanTinggi: result.SiswaPerguruanTinggiDeleted,
		LastUpdatePrestasi:   lastUpdatePrestasi,
		LastUpdateSiswaPT:    lastUpdateSiswaPT,
	})
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (h AdminHandler) ChangePassword(c *gin.Context) {
	logging.Infof("admin", "change_password", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	oldPassword := strings.TrimSpace(req.OldPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if oldPassword == "" || newPassword == "" {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if len(newPassword) < 8 {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if newPassword == oldPassword {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	now := time.Now()
	if !settingsRateLimiter.Allow(user.ID, now) {
		utils.RespondTooManyRequests(c)
		return
	}

	authService := services.AuthService{DB: h.DB}
	adminRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("admin", "change_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !utils.VerifyPassword(oldPassword, adminRecord.PasswordHash) {
		settingsRateLimiter.RecordFailure(user.ID, now)
		c.JSON(http.StatusBadRequest, gin.H{"code": "OLD_PASSWORD_INVALID"})
		return
	}
	settingsRateLimiter.Reset(user.ID)

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		logging.Errorf("admin", "change_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	result, err := h.DB.Pool.Exec(c, `
        UPDATE users
        SET password_hash = $1,
            token_version = token_version + 1,
            password_updated_at = NOW(),
            updated_at = NOW()
        WHERE id = $2 AND role IN ('ADMIN', 'ADMIN_MASTER')
    `, hash, user.ID)
	if err != nil {
		logging.Errorf("admin", "change_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if result.RowsAffected() == 0 {
		utils.RespondForbidden(c)
		return
	}

	service := services.AdminService{DB: h.DB}
	if err := service.LogAdminAction(c, user.ID, "CHANGE_PASSWORD"); err != nil {
		logging.Errorf("admin", "change_password", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{Message: "password updated"})
}

func ensureAdminSchoolLevel(c *gin.Context, database *db.DB, user auth.User, schoolID int) bool {
	if user.Role != "ADMIN" {
		return true
	}
	allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
	if len(allowed) == 0 {
		utils.RespondForbidden(c)
		return false
	}
	var jenjang string
	if err := database.Pool.QueryRow(c, `
        SELECT COALESCE(UPPER(TRIM(bentuk_pendidikan)), '')
        FROM data_sekolah
        WHERE id = $1
    `, schoolID).Scan(&jenjang); err != nil {
		utils.RespondBadRequest(c, "school not found")
		return false
	}
	for _, level := range allowed {
		if level == jenjang {
			return true
		}
	}
	utils.RespondForbidden(c)
	return false
}

func ensureAdminJenjangAllowed(c *gin.Context, user auth.User, jenjang string) bool {
	if user.Role != "ADMIN" {
		return true
	}
	allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
	if len(allowed) == 0 {
		utils.RespondForbidden(c)
		return false
	}
	normalized := strings.ToUpper(strings.TrimSpace(jenjang))
	for _, level := range allowed {
		if level == normalized {
			return true
		}
	}
	utils.RespondForbidden(c)
	return false
}

// helper pembuat excel
func buildGeneratedSchoolAdminsXLSX(summary services.BulkGenerateSummary, jenjang, status string) ([]byte, error) {
	file := excelize.NewFile()
	sheet := "Akun_Admin_Sekolah"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{"Nama Sekolah", "NPSN", "Username", "Password", "Jenjang", "Status", "Result", "Reason"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := file.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}

	rows := make([][]interface{}, 0, len(summary.Generated)+len(summary.Skipped)+len(summary.Rejected))
	appendRow := func(namaSekolah string, npsn *string, username, password, result, reason string) {
		npsnValue := ""
		if npsn != nil {
			npsnValue = *npsn
		}
		rows = append(rows, []interface{}{
			namaSekolah,
			npsnValue,
			username,
			password,
			jenjang,
			status,
			result,
			reason,
		})
	}

	for _, it := range summary.Generated {
		appendRow(it.NamaSekolah, it.NPSN, it.Username, it.Password, "CREATED", it.Reason)
	}
	for _, it := range summary.Skipped {
		appendRow(it.NamaSekolah, it.NPSN, "", "", "SKIPPED", it.Reason)
	}
	for _, it := range summary.Rejected {
		appendRow(it.NamaSekolah, it.NPSN, "", "", "REJECTED", it.Reason)
	}

	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := file.SetSheetRow(sheet, cell, &row); err != nil {
			return nil, err
		}
	}

	_ = file.SetColWidth(sheet, "A", "A", 35)
	_ = file.SetColWidth(sheet, "B", "B", 18)
	_ = file.SetColWidth(sheet, "C", "C", 28)
	_ = file.SetColWidth(sheet, "D", "D", 22)
	_ = file.SetColWidth(sheet, "E", "F", 12)
	_ = file.SetColWidth(sheet, "G", "G", 12)
	_ = file.SetColWidth(sheet, "H", "H", 30)

	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
