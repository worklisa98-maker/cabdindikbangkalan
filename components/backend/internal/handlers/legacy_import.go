package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type LegacyImportHandler struct {
	DB *db.DB
}

type legacyImportLevelResponse struct {
	Level     string `json:"level"`
	Enabled   bool   `json:"enabled"`
	Override  bool   `json:"override,omitempty"`
	Locked    bool   `json:"locked,omitempty"`
	Source    string `json:"source,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type legacyImportSettingsResponse struct {
	GlobalEnabled   bool                        `json:"globalEnabled"`
	AutoDisableYear int                         `json:"autoDisableYear"`
	ForceEnabled    bool                        `json:"forceEnabled"`
	MaxYear         int                         `json:"maxYear"`
	Levels          []legacyImportLevelResponse `json:"levels,omitempty"`
}

type legacyImportSettingsUpdateRequest struct {
	GlobalEnabled   *bool `json:"globalEnabled"`
	AutoDisableYear *int  `json:"autoDisableYear"`
	ForceEnabled    *bool `json:"forceEnabled"`
	MaxYear         *int  `json:"maxYear"`
}

type legacyImportLevelUpdateRequest struct {
	Level    string `json:"level"`
	Enabled  *bool  `json:"enabled"`
	Override *bool  `json:"override"`
}

type legacyImportLevelsRequest struct {
	Levels []legacyImportLevelUpdateRequest `json:"levels"`
}

func (h LegacyImportHandler) GetMasterSettings(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	service := services.LegacyImportService{DB: h.DB}
	settings, err := service.GetSettings(c)
	if err != nil {
		logging.Errorf("legacy_import", "get_settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	levelsMap, err := service.ListLevels(c, "MASTER", nil)
	if err != nil {
		logging.Errorf("legacy_import", "list_master_levels", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	levels := make([]legacyImportLevelResponse, 0, len(services.LegacyImportLevels()))
	for _, level := range services.LegacyImportLevels() {
		normalized := strings.ToUpper(strings.TrimSpace(level))
		item := legacyImportLevelResponse{Level: normalized}
		if record, ok := levelsMap[normalized]; ok {
			item.Enabled = record.Enabled
			item.Override = true
			if !record.UpdatedAt.IsZero() {
				item.UpdatedAt = record.UpdatedAt.UTC().Format(time.RFC3339)
			}
		} else {
			item.Enabled = true
			item.Override = false
		}
		levels = append(levels, item)
	}

	utils.RespondData(c, http.StatusOK, legacyImportSettingsResponse{
		GlobalEnabled:   settings.GlobalEnabled,
		AutoDisableYear: settings.AutoDisableYear,
		ForceEnabled:    settings.ForceEnabled,
		MaxYear:         settings.MaxYear,
		Levels:          levels,
	})
}

func (h LegacyImportHandler) UpdateMasterSettings(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	var req legacyImportSettingsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}

	service := services.LegacyImportService{DB: h.DB}
	settings, err := service.UpdateSettings(c, services.LegacyImportSettingsUpdate{
		GlobalEnabled:   req.GlobalEnabled,
		AutoDisableYear: req.AutoDisableYear,
		ForceEnabled:    req.ForceEnabled,
		MaxYear:         req.MaxYear,
	}, user.ID)
	if err != nil {
		switch err.Error() {
		case "AUTO_DISABLE_YEAR_INVALID", "MAX_YEAR_INVALID":
			utils.RespondBadRequestMessage(c, err.Error())
		default:
			logging.Errorf("legacy_import", "update_settings", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, legacyImportSettingsResponse{
		GlobalEnabled:   settings.GlobalEnabled,
		AutoDisableYear: settings.AutoDisableYear,
		ForceEnabled:    settings.ForceEnabled,
		MaxYear:         settings.MaxYear,
	})
}

func (h LegacyImportHandler) UpdateMasterLevels(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}

	var req legacyImportLevelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if len(req.Levels) == 0 {
		utils.RespondBadRequestMessage(c, "LEVELS_REQUIRED")
		return
	}

	service := services.LegacyImportService{DB: h.DB}
	updates := make(map[string]bool)
	deletions := make([]string, 0)

	for _, levelReq := range req.Levels {
		level := strings.TrimSpace(levelReq.Level)
		override := true
		if levelReq.Override != nil {
			override = *levelReq.Override
		}
		if !override {
			deletions = append(deletions, level)
			continue
		}
		if levelReq.Enabled == nil {
			utils.RespondBadRequestMessage(c, "LEVEL_ENABLED_REQUIRED")
			return
		}
		updates[level] = *levelReq.Enabled
	}

	if len(updates) > 0 {
		if err := service.UpsertLevelSettings(c, "MASTER", nil, updates, user.ID); err != nil {
			switch err.Error() {
			case "LEVEL_REQUIRED", "LEVEL_INVALID":
				utils.RespondBadRequestMessage(c, err.Error())
			default:
				logging.Errorf("legacy_import", "update_master_levels", "err=%v", err)
				utils.RespondInternalServerError(c)
			}
			return
		}
	}

	for _, level := range deletions {
		if err := service.DeleteLevel(c, "MASTER", nil, level); err != nil {
			switch err.Error() {
			case "LEVEL_REQUIRED", "LEVEL_INVALID":
				utils.RespondBadRequestMessage(c, err.Error())
			default:
				logging.Errorf("legacy_import", "delete_master_level", "err=%v", err)
				utils.RespondInternalServerError(c)
			}
			return
		}
	}

	utils.RespondData(c, http.StatusOK, gin.H{"status": "ok"})
}

func (h LegacyImportHandler) GetCabdinLevels(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != auth.RoleAdminCabdin {
		utils.RespondForbidden(c)
		return
	}

	service := services.LegacyImportService{DB: h.DB}
	settings, err := service.GetSettings(c)
	if err != nil {
		logging.Errorf("legacy_import", "get_settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	masterLevels, err := service.ListLevels(c, "MASTER", nil)
	if err != nil {
		logging.Errorf("legacy_import", "list_master_levels", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	cabdinLevels, err := service.ListLevels(c, "CABDIN", &user.ID)
	if err != nil {
		logging.Errorf("legacy_import", "list_cabdin_levels", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
	levels := make([]legacyImportLevelResponse, 0)
	if allowed == nil {
		allowed = []string{}
	}

	for _, level := range allowed {
		normalized := strings.ToUpper(strings.TrimSpace(level))
		item := legacyImportLevelResponse{Level: normalized}
		now := time.Now()

		enabled := true
		source := "DEFAULT"
		locked := false

		if !settings.GlobalEnabled {
			enabled = false
			source = "GLOBAL_DISABLED"
			locked = true
		} else if now.Year() >= settings.AutoDisableYear && !settings.ForceEnabled {
			enabled = false
			source = "AUTO_DISABLED"
			locked = true
		} else if master, ok := masterLevels[normalized]; ok {
			enabled = master.Enabled
			source = "MASTER"
			locked = true
			if !master.UpdatedAt.IsZero() {
				item.UpdatedAt = master.UpdatedAt.UTC().Format(time.RFC3339)
			}
		} else if cabdin, ok := cabdinLevels[normalized]; ok {
			enabled = cabdin.Enabled
			source = "CABDIN"
			locked = false
			if !cabdin.UpdatedAt.IsZero() {
				item.UpdatedAt = cabdin.UpdatedAt.UTC().Format(time.RFC3339)
			}
		}

		item.Enabled = enabled
		item.Locked = locked
		item.Source = source
		levels = append(levels, item)
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"settings": legacyImportSettingsResponse{
			GlobalEnabled:   settings.GlobalEnabled,
			AutoDisableYear: settings.AutoDisableYear,
			ForceEnabled:    settings.ForceEnabled,
			MaxYear:         settings.MaxYear,
		},
		"levels": levels,
	})
}

func (h LegacyImportHandler) UpdateCabdinLevels(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != auth.RoleAdminCabdin {
		utils.RespondForbidden(c)
		return
	}

	var req legacyImportLevelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if len(req.Levels) == 0 {
		utils.RespondBadRequestMessage(c, "LEVELS_REQUIRED")
		return
	}

	allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
	if allowed == nil {
		allowed = []string{}
	}
	allowedSet := map[string]struct{}{}
	for _, level := range allowed {
		allowedSet[strings.ToUpper(strings.TrimSpace(level))] = struct{}{}
	}

	service := services.LegacyImportService{DB: h.DB}
	masterLevels, err := service.ListLevels(c, "MASTER", nil)
	if err != nil {
		logging.Errorf("legacy_import", "list_master_levels", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	updates := make(map[string]bool)
	for _, levelReq := range req.Levels {
		level := strings.ToUpper(strings.TrimSpace(levelReq.Level))
		if _, ok := allowedSet[level]; !ok {
			utils.RespondBadRequestMessage(c, "LEVEL_NOT_ALLOWED")
			return
		}
		if _, locked := masterLevels[level]; locked {
			c.JSON(http.StatusForbidden, utils.APIResponse{Error: "FORBIDDEN", Message: "MASTER_OVERRIDE_ACTIVE"})
			return
		}
		if levelReq.Enabled == nil {
			utils.RespondBadRequestMessage(c, "LEVEL_ENABLED_REQUIRED")
			return
		}
		updates[level] = *levelReq.Enabled
	}

	if err := service.UpsertLevelSettings(c, "CABDIN", &user.ID, updates, user.ID); err != nil {
		switch err.Error() {
		case "LEVEL_REQUIRED", "LEVEL_INVALID":
			utils.RespondBadRequestMessage(c, err.Error())
		default:
			logging.Errorf("legacy_import", "update_cabdin_levels", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{"status": "ok"})
}

func (h LegacyImportHandler) GetStatus(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	schoolID, ok := h.resolveSchoolScope(c, user)
	if !ok {
		return
	}

	level, err := h.fetchSchoolLevel(c, schoolID)
	if err != nil {
		utils.RespondBadRequest(c, "Invalid school level")
		return
	}

	var cabdinUserID *int
	if user.Role == auth.RoleAdminCabdin {
		cabdinUserID = &user.ID
	}

	service := services.LegacyImportService{DB: h.DB}
	enabled, settings, source, err := service.EffectiveLegacyEnabled(c, level, cabdinUserID, time.Now())
	if err != nil {
		logging.Errorf("legacy_import", "get_status", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"level":               level,
		"enabled":             enabled,
		"source":              source,
		"globalEnabled":       settings.GlobalEnabled,
		"autoDisableYear":     settings.AutoDisableYear,
		"forceEnabled":        settings.ForceEnabled,
		"legacyImportMaxYear": settings.MaxYear,
	})
}

func (h LegacyImportHandler) DownloadTemplate(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	schoolID, ok := h.resolveSchoolScope(c, user)
	if !ok {
		return
	}

	level, err := h.fetchSchoolLevel(c, schoolID)
	if err != nil {
		utils.RespondBadRequest(c, "Invalid school level")
		return
	}

	var cabdinUserID *int
	if user.Role == auth.RoleAdminCabdin {
		cabdinUserID = &user.ID
	}

	service := services.LegacyImportService{DB: h.DB}
	enabled, settings, source, err := service.EffectiveLegacyEnabled(c, level, cabdinUserID, time.Now())
	if err != nil {
		logging.Errorf("legacy_import", "download_template", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !enabled {
		c.JSON(http.StatusForbidden, utils.APIResponse{Error: "FORBIDDEN", Message: "LEGACY_IMPORT_DISABLED", Details: gin.H{"source": source, "auto_disable_year": settings.AutoDisableYear}})
		return
	}

	templateType := strings.ToLower(strings.TrimSpace(c.Param("type")))
	var (
		filename    string
		contentType string
		content     []byte
		buildErr    error
	)

	switch templateType {
	case "higher-ed", "diterima-perguruan-tinggi", "pt":
		filename = "template-lulusan-diterima-pt.xlsx"
		contentType = xlsxContentType
		content, buildErr = buildLegacyHigherEdTemplate()
	case "work", "diterima-dunia-kerja":
		filename = "template-lulusan-diterima-kerja.xlsx"
		contentType = xlsxContentType
		content, buildErr = buildLegacyFinalStatusTemplate("Pendataan_Dunia_Kerja")
	case "entrepreneur", "berwirausaha":
		filename = "template-lulusan-berwirausaha.xlsx"
		contentType = xlsxContentType
		content, buildErr = buildLegacyFinalStatusTemplate("Pendataan_Berwirausaha")
	case "undefined", "belum-melanjutkan":
		filename = "template-lulusan-belum-melanjutkan.xlsx"
		contentType = xlsxContentType
		content, buildErr = buildLegacyFinalStatusTemplate("Pendataan_Belum_Melanjutkan")
	default:
		utils.RespondBadRequest(c, "Unknown template type")
		return
	}

	if buildErr != nil {
		logging.Errorf("legacy_import", "download_template", "err=%v", buildErr)
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, contentType, content)
}

func (h LegacyImportHandler) resolveSchoolScope(c *gin.Context, user auth.User) (int, bool) {
	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid school_id")
			return 0, false
		}
		requestedSchoolID = &parsed
	}

	scope, err := auth.GetSchoolScope(user, requestedSchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
		} else {
			utils.RespondUnauthorized(c)
		}
		return 0, false
	}
	if scope == nil {
		utils.RespondBadRequest(c, "school_id wajib diisi.")
		return 0, false
	}
	return *scope, true
}

func (h LegacyImportHandler) fetchSchoolLevel(c *gin.Context, schoolID int) (string, error) {
	var level string
	if err := h.DB.Pool.QueryRow(c, `
		SELECT COALESCE(UPPER(TRIM(bentuk_pendidikan)), '')
		FROM data_sekolah
		WHERE id = $1
	`, schoolID).Scan(&level); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("LEVEL_NOT_FOUND")
		}
		return "", err
	}
	if strings.TrimSpace(level) == "" {
		return "", errors.New("LEVEL_NOT_SET")
	}
	return level, nil
}
