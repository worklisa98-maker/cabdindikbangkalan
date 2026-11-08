package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"cabdin-bangkalan-backend/internal/auth"
	"github.com/gin-gonic/gin"
)

const complaintPopupMaxFile = 5 << 20

func complaintPopupMaster(c *gin.Context) (auth.User, bool) {
	u, err := auth.GetCurrentUser(c)
	if err != nil || auth.NormalizeRole(u.Role) != auth.RoleAdminMaster {
		c.JSON(http.StatusForbidden, gin.H{"error": "Settings informasi popup hanya dapat dikelola Admin Master."})
		return auth.User{}, false
	}
	return u, true
}

func (h ComplaintHandler) PopupAdminList(c *gin.Context) {
	if _, ok := complaintPopupMaster(c); !ok {
		return
	}
	rows, err := h.DB.Pool.Query(c, `SELECT id,title,alt_text,mime_type,size_bytes,is_active,created_at
		FROM complaint_popup_images ORDER BY is_active DESC,created_at DESC,id DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Daftar gambar popup tidak dapat dimuat."})
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, size int64
		var title, alt, mime string
		var active bool
		var created any
		if rows.Scan(&id, &title, &alt, &mime, &size, &active, &created) == nil {
			out = append(out, gin.H{"id": id, "title": title, "altText": alt, "mimeType": mime,
				"sizeBytes": size, "isActive": active, "createdAt": created,
				"imageUrl": "/api/admin/complaints-popup/" + strconv.FormatInt(id, 10) + "/image"})
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h ComplaintHandler) PopupAdminUpload(c *gin.Context) {
	u, ok := complaintPopupMaster(c)
	if !ok {
		return
	}
	title, alt := strings.TrimSpace(c.PostForm("title")), strings.TrimSpace(c.PostForm("altText"))
	active := c.PostForm("isActive") == "true"
	file, header, err := c.Request.FormFile("image")
	if err != nil || title == "" || alt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Judul, teks alternatif, dan gambar wajib diisi."})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, complaintPopupMaxFile+1))
	if err != nil || len(data) == 0 || len(data) > complaintPopupMaxFile {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gambar maksimal 5 MB."})
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	mime := http.DetectContentType(data)
	valid := (ext == ".jpg" || ext == ".jpeg") && mime == "image/jpeg" ||
		ext == ".png" && mime == "image/png" || ext == ".webp" && mime == "image/webp"
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gambar harus berupa JPG, PNG, atau WebP yang valid."})
		return
	}
	token, _ := randomToken(24)
	key := filepath.ToSlash(filepath.Join("pengaduan-masyarakat", "informasi", token+ext))
	full := filepath.Join(h.StorageDir, filepath.FromSlash(key))
	if err = os.MkdirAll(filepath.Dir(full), 0750); err != nil || os.WriteFile(full, data, 0640) != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Storage gambar popup belum siap."})
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(full)
		}
	}()
	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		c.JSON(500, gin.H{"error": "Gambar belum dapat disimpan."})
		return
	}
	defer tx.Rollback(c)
	if active {
		_, err = tx.Exec(c, `UPDATE complaint_popup_images SET is_active=FALSE WHERE is_active=TRUE`)
	}
	if err == nil {
		_, err = tx.Exec(c, `INSERT INTO complaint_popup_images
			(title,alt_text,storage_key,mime_type,size_bytes,is_active,created_by)
			VALUES($1,$2,$3,$4,$5,$6,$7)`, title, alt, key, mime, len(data), active, u.ID)
	}
	if err != nil || tx.Commit(c) != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Data gambar popup gagal disimpan."})
		return
	}
	committed = true
	c.JSON(http.StatusCreated, gin.H{"message": "Gambar informasi berhasil disimpan."})
}

func (h ComplaintHandler) PopupAdminActivate(c *gin.Context) {
	u, ok := complaintPopupMaster(c)
	if !ok {
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Active bool `json:"active"`
	}
	if id <= 0 || c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Permintaan tidak valid."})
		return
	}
	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		c.JSON(500, gin.H{"error": "Status popup belum dapat diubah."})
		return
	}
	defer tx.Rollback(c)
	if req.Active {
		_, err = tx.Exec(c, `UPDATE complaint_popup_images SET is_active=FALSE WHERE is_active=TRUE`)
	}
	var changed int64
	if err == nil {
		tag, updateErr := tx.Exec(c, `UPDATE complaint_popup_images SET is_active=$1 WHERE id=$2`, req.Active, id)
		err, changed = updateErr, tag.RowsAffected()
	}
	if err != nil || changed != 1 || tx.Commit(c) != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gambar popup tidak ditemukan."})
		return
	}
	_, _ = h.DB.Pool.Exec(c, `INSERT INTO admin_audit_log(admin_id,action,count)
		VALUES($1,$2,1)`, u.ID,
		"COMPLAINT_POPUP_STATUS:"+strconv.FormatInt(id, 10)+":"+strconv.FormatBool(req.Active))
	c.JSON(http.StatusOK, gin.H{"message": "Status popup berhasil diperbarui."})
}

func (h ComplaintHandler) PopupAdminImage(c *gin.Context) {
	if _, ok := complaintPopupMaster(c); !ok {
		return
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var key, mime string
	if h.DB.Pool.QueryRow(c, `SELECT storage_key,mime_type FROM complaint_popup_images WHERE id=$1`, id).
		Scan(&key, &mime) != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", mime)
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(filepath.Join(h.StorageDir, filepath.FromSlash(key)))
}
