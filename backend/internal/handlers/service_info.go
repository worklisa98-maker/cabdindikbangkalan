package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

const serviceInfoUploadDir = "/data/uploads/informasi-pelayanan-cabdin"
const maxServiceInfoFileSize = 20 * 1024 * 1024

type ServiceInfoHandler struct {
	DB *db.DB
}

func (h ServiceInfoHandler) PublicList(c *gin.Context) {
	service := services.SiswaService{DB: h.DB}
	items, err := service.ListServiceInfo(c, false)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"items": items})
}

func (h ServiceInfoHandler) AdminList(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := services.SiswaService{DB: h.DB}
	canManage, err := service.CanManageServiceInfo(c, user)
	if err != nil || !canManage {
		if err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondForbidden(c)
		return
	}
	items, err := service.ListServiceInfo(c, true)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"items": items, "can_manage_delegates": auth.IsAdminMaster(user)})
}

func (h ServiceInfoHandler) Upsert(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := services.SiswaService{DB: h.DB}
	canManage, err := service.CanManageServiceInfo(c, user)
	if err != nil || !canManage {
		if err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondForbidden(c)
		return
	}
	input, err := parseServiceInfoForm(c)
	if err != nil {
		utils.RespondBadRequestMessage(c, err.Error())
		return
	}
	item, err := service.UpsertServiceInfo(c, user, input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrServiceInfoForbidden), errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		case errors.Is(err, services.ErrServiceInfoValidation):
			utils.RespondBadRequest(c, "Judul dan kategori wajib diisi.")
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, item)
}

func (h ServiceInfoHandler) PublicFile(c *gin.Context) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		utils.RespondNotFound(c)
		return
	}
	var key, name, contentType string
	var active, archived bool
	err = h.DB.Pool.QueryRow(c, `
		SELECT COALESCE(file_storage_key, ''), COALESCE(file_name, ''), COALESCE(content_type, ''), is_active, is_archived
		FROM cabdin_service_info_items WHERE id = $1
	`, id).Scan(&key, &name, &contentType, &active, &archived)
	if err != nil || key == "" || !active || archived {
		utils.RespondNotFound(c)
		return
	}
	path := filepath.Join(serviceInfoUploadDir, filepath.Clean(key))
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(serviceInfoUploadDir)) {
		utils.RespondNotFound(c)
		return
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(name))
	}
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	if strings.TrimSpace(c.Query("download")) != "" {
		c.Header("Content-Disposition", `attachment; filename="`+sanitizeServiceInfoFilename(name)+`"`)
	}
	c.File(path)
}

type serviceInfoDelegatesRequest struct {
	UserIDs []int `json:"user_ids"`
}

func (h ServiceInfoHandler) ListDelegates(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := services.SiswaService{DB: h.DB}
	items, err := service.ListServiceInfoDelegates(c, user)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"items": items})
}

func (h ServiceInfoHandler) SetDelegates(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req serviceInfoDelegatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	service := services.SiswaService{DB: h.DB}
	if err := service.SetServiceInfoDelegates(c, user, req.UserIDs); err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"ok": true})
}

func parseServiceInfoForm(c *gin.Context) (services.ServiceInfoUpsertInput, error) {
	var input services.ServiceInfoUpsertInput
	if raw := strings.TrimSpace(c.PostForm("id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return input, errors.New("ID tidak valid.")
		}
		input.ID = id
	}
	input.Title = c.PostForm("title")
	input.Category = c.PostForm("category")
	input.SortOrder, _ = strconv.Atoi(strings.TrimSpace(c.PostForm("sort_order")))
	input.IsBanner = parseBoolForm(c.PostForm("is_banner"))
	input.IsActive = true
	if raw := strings.TrimSpace(c.PostForm("is_active")); raw != "" {
		input.IsActive = parseBoolForm(raw)
	}
	input.IsArchived = parseBoolForm(c.PostForm("is_archived"))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return input, nil
		}
		return input, errors.New("File upload tidak valid.")
	}
	defer file.Close()
	if header.Size > maxServiceInfoFileSize {
		return input, errors.New("Ukuran file maksimal 20 MB.")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxServiceInfoFileSize+1))
	if err != nil || int64(len(data)) > maxServiceInfoFileSize {
		return input, errors.New("Gagal membaca file upload.")
	}
	if err := os.MkdirAll(serviceInfoUploadDir, 0o755); err != nil {
		return input, errors.New("Persistent storage /data belum siap.")
	}
	name := sanitizeServiceInfoFilename(header.Filename)
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return input, errors.New("Gagal membuat nama file.")
	}
	key := hex.EncodeToString(random) + "_" + name
	path := filepath.Join(serviceInfoUploadDir, key)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return input, errors.New("Gagal menyimpan file ke persistent storage.")
	}
	input.FileKey = key
	input.FileName = name
	input.ContentType = header.Header.Get("Content-Type")
	input.FileSize = int64(len(data))
	return input, nil
}

func parseBoolForm(raw string) bool {
	value := strings.ToLower(strings.TrimSpace(raw))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func sanitizeServiceInfoFilename(name string) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, filepath.Base(name))
	if clean == "" || clean == "." {
		return "file"
	}
	return clean
}
