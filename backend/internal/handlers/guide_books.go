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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

const maxGuideBookImageSize = 5 * 1024 * 1024

var guideBookImageFilePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type GuideBookHandler struct {
	DB              *db.DB
	StorageLocalDir string
}

type guideBookSlidePayload struct {
	ID          int64  `json:"id"`
	LayoutType  string `json:"layoutType"`
	TextHTML    string `json:"textHtml"`
	Image1URL   string `json:"image1Url"`
	Image2URL   string `json:"image2Url"`
	ColumnRatio string `json:"columnRatio"`
	SortOrder   int    `json:"sortOrder"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type guideBookPayload struct {
	ID           int64                   `json:"id"`
	Role         string                  `json:"role"`
	Title        string                  `json:"title"`
	ContentHTML  string                  `json:"contentHtml"`
	SortOrder    int                     `json:"sortOrder"`
	PlacementKey string                  `json:"placementKey"`
	IsPublic     bool                    `json:"isPublic"`
	Slides       []guideBookSlidePayload `json:"slides"`
	IsActive     *bool                   `json:"isActive,omitempty"`
	CreatedAt    string                  `json:"createdAt,omitempty"`
	UpdatedAt    string                  `json:"updatedAt"`
}

type guideBookSlideUpsertRequest struct {
	LayoutType  string `json:"layoutType"`
	TextHTML    string `json:"textHtml"`
	Image1URL   string `json:"image1Url"`
	Image2URL   string `json:"image2Url"`
	ColumnRatio string `json:"columnRatio"`
	SortOrder   int    `json:"sortOrder"`
}

type guideBookUpsertRequest struct {
	Role         string                        `json:"role"`
	Title        string                        `json:"title"`
	ContentHTML  string                        `json:"contentHtml"`
	SortOrder    int                           `json:"sortOrder"`
	IsActive     bool                          `json:"isActive"`
	PlacementKey string                        `json:"placementKey"`
	IsPublic     bool                          `json:"isPublic"`
	Slides       []guideBookSlideUpsertRequest `json:"slides"`
}

func (h GuideBookHandler) List(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	effectiveRole, ok := services.GuideBookEffectiveRole(user.Role)
	if !ok {
		utils.RespondForbidden(c)
		return
	}

	service := services.GuideBookService{DB: h.DB}
	items, err := service.ListVisible(c, effectiveRole)
	if err != nil {
		if errors.Is(err, services.ErrGuideBookValidation) {
			utils.RespondBadRequest(c, "Invalid role")
			return
		}
		logging.Errorf("guide_books", "guide_books.list", "role=%q effective_role=%q err=%v", user.Role, effectiveRole, err)
		utils.RespondInternalServerError(c)
		return
	}

	payload := make([]guideBookPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, mapGuideBookPayload(item, false, false))
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GuideBookHandler) GetActiveByPlacement(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	placementKey := strings.TrimSpace(c.Param("placement"))
	service := services.GuideBookService{DB: h.DB}
	item, err := service.GetActiveByPlacement(c, placementKey, false)
	if err != nil {
		if errors.Is(err, services.ErrGuideBookValidation) {
			utils.RespondBadRequest(c, "Invalid placement")
			return
		}
		logging.Errorf("guide_books", "guide_books.get_active_by_placement", "role=%q placement=%q public_only=false err=%v", user.Role, placementKey, err)
		utils.RespondInternalServerError(c)
		return
	}

	if item == nil {
		utils.RespondData(c, http.StatusOK, nil)
		return
	}

	utils.RespondData(c, http.StatusOK, mapGuideBookPayload(*item, false, false))
}

func (h GuideBookHandler) GetPublicActiveByPlacement(c *gin.Context) {
	placementKey := strings.TrimSpace(c.Param("placement"))
	service := services.GuideBookService{DB: h.DB}
	item, err := service.GetActiveByPlacement(c, placementKey, true)
	if err != nil {
		if errors.Is(err, services.ErrGuideBookValidation) {
			utils.RespondBadRequest(c, "Invalid placement")
			return
		}
		logging.Errorf("guide_books", "guide_books.get_public_active_by_placement", "placement=%q public_only=true err=%v", placementKey, err)
		utils.RespondInternalServerError(c)
		return
	}

	if item == nil {
		utils.RespondData(c, http.StatusOK, nil)
		return
	}

	utils.RespondData(c, http.StatusOK, mapGuideBookPayload(*item, false, true))
}

func (h GuideBookHandler) ListAdmin(c *gin.Context) {
	user, ok := h.requireAdminMaster(c)
	if !ok {
		return
	}

	includeInactive := false
	rawInclude := strings.TrimSpace(c.Query("includeInactive"))
	if rawInclude == "" {
		rawInclude = strings.TrimSpace(c.Query("include_inactive"))
	}
	if rawInclude != "" {
		parsed, err := strconv.ParseBool(rawInclude)
		if err != nil {
			utils.RespondBadRequest(c, "Invalid includeInactive")
			return
		}
		includeInactive = parsed
	}

	role := strings.TrimSpace(c.Query("role"))
	service := services.GuideBookService{DB: h.DB}
	items, err := service.ListAdmin(c, services.GuideBookAdminFilters{
		Role:            role,
		IncludeInactive: includeInactive,
	})
	if err != nil {
		if errors.Is(err, services.ErrGuideBookValidation) {
			utils.RespondBadRequest(c, "Invalid role")
			return
		}
		logging.Errorf("guide_books", "guide_books.list_admin", "role=%q filter_role=%q include_inactive=%t err=%v", user.Role, role, includeInactive, err)
		utils.RespondInternalServerError(c)
		return
	}

	payload := make([]guideBookPayload, 0, len(items))
	for _, item := range items {
		payload = append(payload, mapGuideBookPayload(item, true, false))
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GuideBookHandler) CreateAdmin(c *gin.Context) {
	_, ok := h.requireAdminMaster(c)
	if !ok {
		return
	}

	var req guideBookUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid")
		return
	}

	service := services.GuideBookService{DB: h.DB}
	item, err := service.Create(c, mapGuideBookUpsertRequest(req))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrGuideBookValidation):
			utils.RespondBadRequest(c, "title/role/placement/slides tidak valid")
		case errors.Is(err, services.ErrGuideBookConflict):
			utils.RespondConflictMessage(c, "Sudah ada guide book aktif pada placement ini.")
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusCreated, mapGuideBookPayload(item, true, false))
}

func (h GuideBookHandler) UpdateAdmin(c *gin.Context) {
	_, ok := h.requireAdminMaster(c)
	if !ok {
		return
	}

	id, parseOK := parseGuideBookIDParam(c)
	if !parseOK {
		return
	}

	var req guideBookUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid")
		return
	}

	service := services.GuideBookService{DB: h.DB}
	item, err := service.Update(c, id, mapGuideBookUpsertRequest(req))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrGuideBookValidation):
			utils.RespondBadRequest(c, "title/role/placement/slides tidak valid")
		case errors.Is(err, services.ErrGuideBookConflict):
			utils.RespondConflictMessage(c, "Sudah ada guide book aktif pada placement ini.")
		case errors.Is(err, services.ErrGuideBookNotFound):
			utils.RespondNotFound(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, mapGuideBookPayload(item, true, false))
}

func (h GuideBookHandler) DeleteAdmin(c *gin.Context) {
	_, ok := h.requireAdminMaster(c)
	if !ok {
		return
	}

	id, parseOK := parseGuideBookIDParam(c)
	if !parseOK {
		return
	}

	service := services.GuideBookService{DB: h.DB}
	if err := service.Delete(c, id); err != nil {
		switch {
		case errors.Is(err, services.ErrGuideBookValidation):
			utils.RespondBadRequest(c, "Invalid id")
		case errors.Is(err, services.ErrGuideBookNotFound):
			utils.RespondNotFound(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{"deleted": true, "id": id})
}

func (h GuideBookHandler) UploadImageAdmin(c *gin.Context) {
	_, ok := h.requireAdminMaster(c)
	if !ok {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.RespondBadRequest(c, "Field file wajib diisi")
		return
	}
	if fileHeader.Size <= 0 {
		utils.RespondBadRequest(c, "File kosong")
		return
	}
	if fileHeader.Size > maxGuideBookImageSize {
		utils.RespondBadRequest(c, "Ukuran file maksimal 5MB")
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer src.Close()

	data, err := io.ReadAll(io.LimitReader(src, maxGuideBookImageSize+1))
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if len(data) == 0 {
		utils.RespondBadRequest(c, "File kosong")
		return
	}
	if len(data) > maxGuideBookImageSize {
		utils.RespondBadRequest(c, "Ukuran file maksimal 5MB")
		return
	}

	detectedMIME := http.DetectContentType(data)
	ext, ok := guideBookImageExtensionByMIME(detectedMIME)
	if !ok {
		utils.RespondBadRequest(c, "Mime file tidak didukung (png/jpg/webp)")
		return
	}

	targetDir := filepath.Join(h.storageBaseDir(), "guide-books")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	fileName, err := generateGuideBookImageFileName(ext)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	targetPath := filepath.Join(targetDir, fileName)
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	service := services.GuideBookService{DB: h.DB}
	if err := service.RecordMedia(c, services.GuideBookMediaInput{
		FileName: fileName,
		MIME:     detectedMIME,
		Size:     len(data),
	}); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	url := services.GuideBookStoredImageURL(fileName)
	utils.RespondData(c, http.StatusOK, gin.H{
		"url":      url,
		"fileName": fileName,
		"mime":     detectedMIME,
		"size":     len(data),
	})
}

func (h GuideBookHandler) ServeImage(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !services.IsGuideBookViewerRole(user.Role) {
		utils.RespondForbidden(c)
		return
	}

	h.serveImageFile(c, false)
}

func (h GuideBookHandler) ServePublicImage(c *gin.Context) {
	fileName, ok := parseGuideBookImageFileParam(c)
	if !ok {
		return
	}

	service := services.GuideBookService{DB: h.DB}
	allowed, err := service.CanServePublicImage(c, fileName)
	if err != nil {
		if errors.Is(err, services.ErrGuideBookValidation) {
			utils.RespondBadRequest(c, "Invalid file")
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	if !allowed {
		utils.RespondNotFound(c)
		return
	}

	h.serveImageFile(c, true)
}

func (h GuideBookHandler) serveImageFile(c *gin.Context, isPublic bool) {
	fileName, ok := parseGuideBookImageFileParam(c)
	if !ok {
		return
	}

	fullPath := filepath.Join(h.storageBaseDir(), "guide-books", fileName)
	stat, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			utils.RespondNotFound(c)
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	if stat.IsDir() {
		utils.RespondNotFound(c)
		return
	}

	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(fileName))); strings.TrimSpace(contentType) != "" {
		c.Header("Content-Type", contentType)
	}
	if isPublic {
		c.Header("Cache-Control", "public, max-age=300")
	} else {
		c.Header("Cache-Control", "private, no-store")
	}
	c.File(fullPath)
}

func (h GuideBookHandler) requireAdminMaster(c *gin.Context) (auth.User, bool) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return auth.User{}, false
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return auth.User{}, false
	}
	return user, true
}

func (h GuideBookHandler) storageBaseDir() string {
	base := strings.TrimSpace(h.StorageLocalDir)
	if base == "" {
		return "./uploads"
	}
	return base
}

func parseGuideBookIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		utils.RespondBadRequest(c, "Invalid id")
		return 0, false
	}
	return id, true
}

func parseGuideBookImageFileParam(c *gin.Context) (string, bool) {
	fileName := strings.TrimSpace(c.Param("file"))
	if !guideBookImageFilePattern.MatchString(fileName) || strings.Contains(fileName, "..") || filepath.Base(fileName) != fileName {
		utils.RespondBadRequest(c, "Invalid file")
		return "", false
	}
	return fileName, true
}

func guideBookImageExtensionByMIME(mimeType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return ".png", true
	case "image/jpeg":
		return ".jpg", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

func generateGuideBookImageFileName(ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + ext, nil
}

func mapGuideBookUpsertRequest(req guideBookUpsertRequest) services.GuideBookUpsertInput {
	slides := make([]services.GuideBookSlideUpsertInput, 0, len(req.Slides))
	for _, slide := range req.Slides {
		slides = append(slides, services.GuideBookSlideUpsertInput{
			LayoutType:  slide.LayoutType,
			TextHTML:    slide.TextHTML,
			Image1URL:   slide.Image1URL,
			Image2URL:   slide.Image2URL,
			ColumnRatio: slide.ColumnRatio,
			SortOrder:   slide.SortOrder,
		})
	}
	return services.GuideBookUpsertInput{
		Role:         req.Role,
		Title:        req.Title,
		ContentHTML:  req.ContentHTML,
		SortOrder:    req.SortOrder,
		IsActive:     req.IsActive,
		PlacementKey: req.PlacementKey,
		IsPublic:     req.IsPublic,
		Slides:       slides,
	}
}

func mapGuideBookPayload(item services.GuideBook, includeAdminFields bool, publicMode bool) guideBookPayload {
	contentHTML := item.ContentHTML
	if publicMode {
		contentHTML = strings.ReplaceAll(
			contentHTML,
			"/api/guide-books/images/",
			"/api/public/guide-books/images/",
		)
	}

	slides := make([]guideBookSlidePayload, 0, len(item.Slides))
	for _, slide := range item.Slides {
		image1URL := slide.Image1URL
		image2URL := slide.Image2URL
		if publicMode {
			image1URL = services.GuideBookPublicImageURL(image1URL)
			image2URL = services.GuideBookPublicImageURL(image2URL)
		}
		slides = append(slides, guideBookSlidePayload{
			ID:          slide.ID,
			LayoutType:  slide.LayoutType,
			TextHTML:    slide.TextHTML,
			Image1URL:   image1URL,
			Image2URL:   image2URL,
			ColumnRatio: slide.ColumnRatio,
			SortOrder:   slide.SortOrder,
			CreatedAt:   slide.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   slide.UpdatedAt.Format(time.RFC3339),
		})
	}

	payload := guideBookPayload{
		ID:           item.ID,
		Role:         item.Role,
		Title:        item.Title,
		ContentHTML:  contentHTML,
		SortOrder:    item.SortOrder,
		PlacementKey: item.PlacementKey,
		IsPublic:     item.IsPublic,
		Slides:       slides,
		UpdatedAt:    item.UpdatedAt.Format(time.RFC3339),
	}
	if includeAdminFields {
		isActive := item.IsActive
		payload.IsActive = &isActive
		payload.CreatedAt = item.CreatedAt.Format(time.RFC3339)
	}
	return payload
}
