package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const clarificationExpiredReason = "Informasi/bukti tidak dilengkapi sampai batas waktu yang ditentukan."

const complaintAssigneesQuery = `SELECT id,
		COALESCE(NULLIF(TRIM(nama_instansi), ''), username) AS display_name,
		COALESCE(NULLIF(TRIM(nama_instansi), ''), '') AS institution
	FROM users
	WHERE UPPER(TRIM(role::text)) IN ('ADMIN', 'ADMIN_CABDIN')
	ORDER BY display_name, username`

const complaintAssigneeExistsQuery = `SELECT EXISTS(
	SELECT 1
	FROM users
	WHERE id=$1
	  AND UPPER(TRIM(role::text)) IN ('ADMIN', 'ADMIN_CABDIN')
)`

type complaintActionRequest struct {
	Action     string `json:"action"`
	Message    string `json:"message"`
	AssignedTo *int   `json:"assignedTo"`
	Public     bool   `json:"public"`
}

type complaintEvidenceUpload struct {
	name, mime, sum string
	data            []byte
}

func readComplaintEvidence(files []*multipart.FileHeader) ([]complaintEvidenceUpload, bool, error) {
	pending := make([]complaintEvidenceUpload, 0, len(files))
	hasImage := false
	for _, fh := range files {
		file, err := fh.Open()
		if err != nil {
			return nil, false, err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, complaintMaxFile+1))
		closeErr := file.Close()
		mime, valid := validComplaintFile(fh, data)
		if readErr != nil || closeErr != nil || !valid {
			return nil, false, errors.New("invalid complaint evidence")
		}
		digest := sha256.Sum256(data)
		pending = append(pending, complaintEvidenceUpload{
			name: filepath.Base(fh.Filename), mime: mime,
			sum: hex.EncodeToString(digest[:]), data: data,
		})
		hasImage = hasImage || strings.HasPrefix(mime, "image/")
	}
	return pending, hasImage, nil
}

func complaintActionAllowed(role, status, action string, assignedToActor bool) bool {
	master := auth.NormalizeRole(role) == auth.RoleAdminMaster
	if !master && !assignedToActor {
		return false
	}
	closed := status == "COMPLETED" || status == "REJECTED" || status == "OUT_OF_SCOPE"
	switch action {
	case "VERIFY_SELF", "ASSIGN":
		return master && (status == "RECEIVED" || status == "VERIFICATION" || status == "REOPENED")
	case "REASSIGN", "TAKEOVER":
		return master && !closed
	case "REOPEN":
		return master && closed
	case "OUT_OF_SCOPE":
		return master && !closed
	case "NEEDS_INFO":
		return !closed
	case "ACCEPT_CLARIFICATION":
		return status == "CLARIFICATION_SUBMITTED"
	case "PROGRESS":
		return status == "FOLLOW_UP" || status == "RESPONDED"
	case "RESPOND":
		return status == "FOLLOW_UP" || status == "CLARIFICATION_SUBMITTED"
	case "COMPLETE":
		return status == "RESPONDED"
	case "REJECT":
		return !closed
	default:
		return false
	}
}

func (h ComplaintHandler) expireClarifications(c *gin.Context) {
	rows, err := h.DB.Pool.Query(c, `UPDATE complaints
		SET status='REJECTED', closed_reason=$1, clarification_deadline=NULL, updated_at=NOW()
		WHERE status='NEEDS_INFO' AND clarification_deadline < NOW()
		RETURNING id`, clarificationExpiredReason)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			_, _ = h.DB.Pool.Exec(c, `INSERT INTO complaint_timeline
				(complaint_id,action,old_status,new_status,message,is_internal)
				VALUES($1,'CLARIFICATION_EXPIRED','NEEDS_INFO','REJECTED',$2,FALSE)`, id, clarificationExpiredReason)
			_, _ = h.DB.Pool.Exec(c, `INSERT INTO complaint_audit_log
				(complaint_id,action,old_value,new_value,note,is_public)
				VALUES($1,'CLARIFICATION_EXPIRED','{"status":"NEEDS_INFO"}','{"status":"REJECTED"}',$2,TRUE)`,
				id, clarificationExpiredReason)
		}
	}
}

func (h ComplaintHandler) ComplaintAssignees(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil || auth.NormalizeRole(u.Role) != auth.RoleAdminMaster {
		c.JSON(http.StatusForbidden, gin.H{"error": "Hanya Admin Master yang dapat melihat daftar petugas."})
		return
	}
	rows, err := h.DB.Pool.Query(c, complaintAssigneesQuery)
	if err != nil {
		logging.Errorf(
			"complaints",
			"assignees_query",
			"user_id=%d role=%s err=%v",
			u.ID,
			auth.NormalizeRole(u.Role),
			err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Daftar petugas tidak dapat dimuat."})
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int
		var displayName, institution string
		if err := rows.Scan(&id, &displayName, &institution); err != nil {
			logging.Errorf(
				"complaints",
				"assignees_scan",
				"user_id=%d role=%s err=%v",
				u.ID,
				auth.NormalizeRole(u.Role),
				err,
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Daftar petugas tidak dapat dimuat."})
			return
		}
		out = append(out, gin.H{"id": id, "name": displayName, "institution": institution})
	}
	if err := rows.Err(); err != nil {
		logging.Errorf(
			"complaints",
			"assignees_rows",
			"user_id=%d role=%s err=%v",
			u.ID,
			auth.NormalizeRole(u.Role),
			err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Daftar petugas tidak dapat dimuat."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h ComplaintHandler) ComplaintAction(c *gin.Context) {
	u, err := complaintAdminUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak."})
		return
	}
	var req complaintActionRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data tindakan tidak valid."})
		return
	}
	req.Action = strings.ToUpper(strings.TrimSpace(req.Action))
	req.Message = strings.TrimSpace(req.Message)
	role := auth.NormalizeRole(u.Role)
	master := role == auth.RoleAdminMaster

	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tindakan belum dapat diproses."})
		return
	}
	defer tx.Rollback(c)

	var id int64
	var old string
	var assigned *int
	var official *string
	err = tx.QueryRow(c, `SELECT id,status,assigned_to,official_response
		FROM complaints WHERE report_id=$1 FOR UPDATE`, c.Param("id")).Scan(&id, &old, &assigned, &official)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Laporan tidak ditemukan."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Laporan tidak dapat diproses."})
		return
	}
	if !master && (assigned == nil || *assigned != u.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Laporan ini tidak ditugaskan kepada Anda."})
		return
	}
	assignedToActor := assigned != nil && *assigned == u.ID
	if !complaintActionAllowed(role, old, req.Action, assignedToActor) {
		c.JSON(http.StatusConflict, gin.H{"error": "Tindakan tidak diizinkan pada status laporan saat ini."})
		return
	}

	next, action, public := old, req.Action, req.Public
	var nextAssigned any = assigned
	var deadline any
	requireMessage := false
	switch req.Action {
	case "VERIFY_SELF":
		if !master || (old != "RECEIVED" && old != "VERIFICATION" && old != "REOPENED") {
			goto invalid
		}
		next, nextAssigned, public = "FOLLOW_UP", u.ID, true
	case "ASSIGN", "REASSIGN":
		if !master || req.AssignedTo == nil || *req.AssignedTo <= 0 {
			goto invalid
		}
		requireMessage = true
		var valid bool
		err = tx.QueryRow(c, complaintAssigneeExistsQuery, *req.AssignedTo).Scan(&valid)
		if err != nil {
			logging.Errorf(
				"complaints",
				"assignee_validation_query",
				"user_id=%d target_user_id=%d action=%s err=%v",
				u.ID,
				*req.AssignedTo,
				req.Action,
				err,
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Penerima penugasan belum dapat diverifikasi."})
			return
		}
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Admin Cabdin tujuan tidak valid."})
			return
		}
		if req.Action == "ASSIGN" && old != "RECEIVED" && old != "VERIFICATION" && old != "REOPENED" {
			goto invalid
		}
		next, nextAssigned, public = "FOLLOW_UP", *req.AssignedTo, false
	case "TAKEOVER":
		if !master {
			goto invalid
		}
		requireMessage = true
		next, nextAssigned, public = "FOLLOW_UP", u.ID, false
	case "NEEDS_INFO":
		if old == "COMPLETED" || old == "REJECTED" || old == "OUT_OF_SCOPE" {
			goto invalid
		}
		requireMessage, public = true, true
		next = "NEEDS_INFO"
		deadline = time.Now().UTC().AddDate(0, 0, 7)
		var round int
		_ = tx.QueryRow(c, `SELECT COALESCE(MAX(round_number),0)+1
			FROM complaint_clarifications WHERE complaint_id=$1`, id).Scan(&round)
		_, err = tx.Exec(c, `INSERT INTO complaint_clarifications
			(complaint_id,round_number,request_text,requested_by,deadline_at)
			VALUES($1,$2,$3,$4,$5)`, id, round, req.Message, u.ID, deadline)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Permintaan klarifikasi gagal disimpan."})
			return
		}
	case "ACCEPT_CLARIFICATION":
		if old != "CLARIFICATION_SUBMITTED" {
			goto invalid
		}
		next, public = "FOLLOW_UP", true
		_, err = tx.Exec(c, `UPDATE complaint_clarifications SET
			review_result='ACCEPTED',reviewed_by=$1,reviewed_at=NOW(),review_note=$2
			WHERE id=(SELECT id FROM complaint_clarifications WHERE complaint_id=$3 ORDER BY round_number DESC LIMIT 1)
			AND responded_at IS NOT NULL`, u.ID, nullableString(req.Message), id)
	case "PROGRESS":
		if old != "FOLLOW_UP" && old != "RESPONDED" {
			goto invalid
		}
		requireMessage, next = true, old
	case "RESPOND":
		c.JSON(http.StatusConflict, gin.H{"error": "Gunakan formulir tanggapan resmi dan bukti tindak lanjut."})
		return
	case "COMPLETE":
		if old != "RESPONDED" || official == nil || strings.TrimSpace(*official) == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "Tanggapan resmi dan minimal satu bukti foto tindak lanjut wajib dilengkapi sebelum laporan diselesaikan."})
			return
		}
		var hasImage bool
		err = tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM complaint_attachments
			WHERE complaint_id=$1 AND kind='FOLLOW_UP'
			AND mime_type IN ('image/jpeg','image/png','image/webp'))`, id).Scan(&hasImage)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Kelengkapan laporan belum dapat diverifikasi."})
			return
		}
		if !hasImage {
			c.JSON(http.StatusConflict, gin.H{"error": "Tanggapan resmi dan minimal satu bukti foto tindak lanjut wajib dilengkapi sebelum laporan diselesaikan."})
			return
		}
		next, public = "COMPLETED", true
	case "REJECT":
		if old == "COMPLETED" || old == "OUT_OF_SCOPE" {
			goto invalid
		}
		requireMessage, next, public = true, "REJECTED", true
	case "OUT_OF_SCOPE":
		if !master || old == "COMPLETED" {
			goto invalid
		}
		requireMessage, next, public = true, "OUT_OF_SCOPE", true
	case "REOPEN":
		if !master || (old != "COMPLETED" && old != "REJECTED" && old != "OUT_OF_SCOPE") {
			goto invalid
		}
		requireMessage, next, public = true, "REOPENED", true
	default:
		goto invalid
	}
	if requireMessage && req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Catatan atau alasan wajib diisi."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tindakan gagal disimpan."})
		return
	}

	if next == "REJECTED" || next == "OUT_OF_SCOPE" {
		_, err = tx.Exec(c, `UPDATE complaints SET status=$1,assigned_to=$2,closed_reason=$3,
			clarification_deadline=NULL,updated_at=NOW() WHERE id=$4`, next, nextAssigned, req.Message, id)
	} else if next == "NEEDS_INFO" {
		_, err = tx.Exec(c, `UPDATE complaints SET status=$1,assigned_to=$2,
			clarification_deadline=$3,updated_at=NOW() WHERE id=$4`, next, nextAssigned, deadline, id)
	} else {
		_, err = tx.Exec(c, `UPDATE complaints SET status=$1,assigned_to=$2,
			clarification_deadline=NULL,updated_at=NOW() WHERE id=$3`, next, nextAssigned, id)
	}
	if err != nil {
		logging.Errorf(
			"complaints",
			"action_update",
			"user_id=%d report_id=%s action=%s err=%v",
			u.ID,
			c.Param("id"),
			req.Action,
			err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Status laporan gagal diperbarui."})
		return
	}
	_, err = tx.Exec(c, `INSERT INTO complaint_timeline
		(complaint_id,actor_user_id,actor_role,action,old_status,new_status,message,is_internal)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, u.ID, role, action, old, next,
		nullableString(req.Message), !public)
	if err != nil {
		logging.Errorf(
			"complaints",
			"action_timeline_insert",
			"user_id=%d report_id=%s action=%s err=%v",
			u.ID,
			c.Param("id"),
			req.Action,
			err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tindakan gagal disimpan."})
		return
	}
	_, err = tx.Exec(c, `INSERT INTO complaint_audit_log
		(complaint_id,actor_user_id,actor_role,action,old_value,new_value,note,is_public)
		VALUES($1,$2,$3,$4,jsonb_build_object('status',$5::text,'assignedTo',$6::integer),
		jsonb_build_object('status',$7::text,'assignedTo',$8::integer),$9,$10)`,
		id, u.ID, role, action, old, assigned, next, nextAssigned, nullableString(req.Message), public)
	if err != nil {
		logging.Errorf(
			"complaints",
			"action_audit_insert",
			"user_id=%d report_id=%s action=%s err=%v",
			u.ID,
			c.Param("id"),
			req.Action,
			err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tindakan gagal disimpan."})
		return
	}
	if err = tx.Commit(c); err != nil {
		logging.Errorf(
			"complaints",
			"action_commit",
			"user_id=%d report_id=%s action=%s err=%v",
			u.ID,
			c.Param("id"),
			req.Action,
			err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tindakan gagal disimpan."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Tindakan berhasil disimpan.", "data": gin.H{"status": next}})
	return

invalid:
	c.JSON(http.StatusConflict, gin.H{"error": "Tindakan tidak diizinkan pada status laporan saat ini."})
}

func (h ComplaintHandler) SubmitClarification(c *gin.Context) {
	if !h.rateLimit(c, "clarification", 8) {
		return
	}
	reportID := strings.ToUpper(strings.TrimSpace(c.PostForm("reportId")))
	pin := strings.TrimSpace(c.PostForm("pin"))
	text := strings.TrimSpace(c.PostForm("clarification"))
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Form klarifikasi tidak valid."})
		return
	}
	files := form.File["attachments"]
	if text == "" || len(files) < 1 || len(files) > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Klarifikasi tertulis dan minimal satu bukti (maksimal tiga) wajib dikirim."})
		return
	}
	var complaintID int64
	var hash, status string
	if h.DB.Pool.QueryRow(c, `SELECT id,pin_hash,status FROM complaints WHERE report_id=$1`,
		reportID).Scan(&complaintID, &hash, &status) != nil || !verifyComplaintPIN(hash, pin) {
		time.Sleep(150 * time.Millisecond)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "ID Laporan atau PIN tidak cocok."})
		return
	}
	if status != "NEEDS_INFO" {
		c.JSON(http.StatusConflict, gin.H{"error": "Tidak ada permintaan klarifikasi yang aktif."})
		return
	}
	type upload struct {
		name, mime, sum string
		data            []byte
	}
	pending := []upload{}
	for _, fh := range files {
		f, openErr := fh.Open()
		if openErr != nil {
			c.JSON(400, gin.H{"error": "Bukti tidak dapat dibaca."})
			return
		}
		data, readErr := io.ReadAll(io.LimitReader(f, complaintMaxFile+1))
		f.Close()
		mime, ok := validComplaintFile(fh, data)
		if readErr != nil || !ok {
			c.JSON(400, gin.H{"error": "Bukti harus JPG, PNG, WebP, atau PDF dan maksimal 1 MB."})
			return
		}
		sum := sha256.Sum256(data)
		pending = append(pending, upload{filepath.Base(fh.Filename), mime, hex.EncodeToString(sum[:]), data})
	}
	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		c.JSON(500, gin.H{"error": "Klarifikasi belum dapat disimpan."})
		return
	}
	defer tx.Rollback(c)
	var clarificationID int64
	err = tx.QueryRow(c, `UPDATE complaint_clarifications SET response_text=$1,responded_at=NOW()
		WHERE id=(SELECT id FROM complaint_clarifications WHERE complaint_id=$2 AND responded_at IS NULL
		AND deadline_at>=NOW() ORDER BY round_number DESC LIMIT 1) RETURNING id`, text, complaintID).Scan(&clarificationID)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Permintaan klarifikasi sudah dijawab atau melewati tenggat."})
		return
	}
	dir := filepath.Join(h.StorageDir, "pengaduan-kritik-saran", "clarifications", reportID)
	if os.MkdirAll(dir, 0700) != nil {
		c.JSON(500, gin.H{"error": "Storage klarifikasi belum siap."})
		return
	}
	written := []string{}
	defer func() {
		if err != nil {
			for _, name := range written {
				_ = os.Remove(name)
			}
		}
	}()
	for _, up := range pending {
		token, _ := randomToken(20)
		key := filepath.ToSlash(filepath.Join("pengaduan-kritik-saran", "clarifications", reportID, token+filepath.Ext(up.name)))
		full := filepath.Join(h.StorageDir, filepath.FromSlash(key))
		if err = os.WriteFile(full, up.data, 0600); err != nil {
			break
		}
		written = append(written, full)
		_, err = tx.Exec(c, `INSERT INTO complaint_attachments
			(complaint_id,storage_key,original_name,mime_type,size_bytes,sha256,public_to_reporter,kind,clarification_id)
			VALUES($1,$2,$3,$4,$5,$6,TRUE,'CLARIFICATION',$7)`,
			complaintID, key, up.name, up.mime, len(up.data), up.sum, clarificationID)
		if err != nil {
			break
		}
	}
	if err == nil {
		_, err = tx.Exec(c, `UPDATE complaints SET status='CLARIFICATION_SUBMITTED',
			clarification_deadline=NULL,updated_at=NOW() WHERE id=$1 AND status='NEEDS_INFO'`, complaintID)
	}
	if err == nil {
		_, err = tx.Exec(c, `INSERT INTO complaint_timeline
			(complaint_id,action,old_status,new_status,message,is_internal)
			VALUES($1,'CLARIFICATION_SUBMITTED','NEEDS_INFO','CLARIFICATION_SUBMITTED',
			'Klarifikasi dan bukti tambahan telah dikirim pelapor.',FALSE)`, complaintID)
	}
	if err == nil {
		_, err = tx.Exec(c, `INSERT INTO complaint_audit_log
			(complaint_id,action,old_value,new_value,note,is_public)
			VALUES($1,'CLARIFICATION_SUBMITTED','{"status":"NEEDS_INFO"}',
			'{"status":"CLARIFICATION_SUBMITTED"}',$2,TRUE)`, complaintID, text)
	}
	if err != nil || tx.Commit(c) != nil {
		c.JSON(500, gin.H{"error": "Klarifikasi gagal disimpan."})
		return
	}
	err = nil
	c.JSON(http.StatusCreated, gin.H{"message": "Klarifikasi berhasil dikirim dan menunggu verifikasi petugas."})
}

func verifyComplaintPIN(hash, pin string) bool {
	return bcryptCompare([]byte(hash), []byte(pin))
}

// Kept behind a tiny function to make authentication behavior independently testable.
var bcryptCompare = func(hash, password []byte) bool {
	return bcrypt.CompareHashAndPassword(hash, password) == nil
}

func (h ComplaintHandler) PublicAttachment(c *gin.Context) {
	reportID := strings.ToUpper(strings.TrimSpace(c.GetHeader("X-Complaint-ID")))
	pin := strings.TrimSpace(c.GetHeader("X-Complaint-PIN"))
	attachmentID, _ := strconv.ParseInt(c.Param("attachmentId"), 10, 64)
	var hash, key, name, mime string
	err := h.DB.Pool.QueryRow(c, `SELECT c.pin_hash,a.storage_key,a.original_name,a.mime_type
		FROM complaint_attachments a JOIN complaints c ON c.id=a.complaint_id
		WHERE c.report_id=$1 AND a.id=$2 AND a.public_to_reporter=TRUE`, reportID, attachmentID).
		Scan(&hash, &key, &name, &mime)
	if err != nil || !verifyComplaintPIN(hash, pin) {
		time.Sleep(100 * time.Millisecond)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Lampiran tidak dapat diakses."})
		return
	}
	c.Header("Content-Type", mime)
	c.Header("X-Content-Type-Options", "nosniff")
	c.FileAttachment(filepath.Join(h.StorageDir, filepath.FromSlash(key)), filepath.Base(name))
}

func (h ComplaintHandler) RespondWithEvidence(c *gin.Context) {
	u, err := complaintAdminUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak."})
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Form tanggapan dan bukti tidak valid."})
		return
	}
	response := strings.TrimSpace(c.PostForm("response"))
	files := form.File["attachments"]
	if response == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tanggapan resmi wajib diisi."})
		return
	}
	if len(files) < 1 || len(files) > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih satu sampai tiga file bukti tindak lanjut."})
		return
	}
	pending, hasImage, err := readComplaintEvidence(files)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bukti harus JPG, PNG, WebP, atau PDF dan maksimal 1 MB."})
		return
	}
	if !hasImage {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal satu bukti harus berupa foto atau screenshot JPG, PNG, atau WebP."})
		return
	}
	message := strings.TrimSpace(c.PostForm("message"))
	public := c.PostForm("public") != "false"
	role := auth.NormalizeRole(u.Role)
	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tanggapan belum dapat diproses."})
		return
	}
	defer tx.Rollback(c)

	var complaintID int64
	var old string
	var assigned *int
	err = tx.QueryRow(c, `SELECT id,status,assigned_to FROM complaints WHERE report_id=$1 FOR UPDATE`,
		c.Param("id")).Scan(&complaintID, &old, &assigned)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Laporan tidak ditemukan."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Laporan belum dapat diproses."})
		return
	}
	if role == auth.RoleAdminCabdin && (assigned == nil || *assigned != u.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Laporan ini tidak ditugaskan kepada Anda."})
		return
	}
	if old != "FOLLOW_UP" && old != "CLARIFICATION_SUBMITTED" {
		c.JSON(http.StatusConflict, gin.H{"error": "Tanggapan resmi tidak diizinkan pada status laporan saat ini."})
		return
	}
	var existingCount int
	if err = tx.QueryRow(c, `SELECT COUNT(*) FROM complaint_attachments
		WHERE complaint_id=$1 AND kind='FOLLOW_UP'`, complaintID).Scan(&existingCount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bukti belum dapat diverifikasi."})
		return
	}
	if existingCount+len(pending) > 3 {
		c.JSON(http.StatusConflict, gin.H{"error": "Maksimal tiga bukti tindak lanjut secara total per laporan."})
		return
	}

	dir := filepath.Join(h.StorageDir, "pengaduan-kritik-saran", "follow-up", c.Param("id"))
	if os.MkdirAll(dir, 0700) != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Storage bukti belum siap."})
		return
	}
	written := []string{}
	committed := false
	defer func() {
		if !committed {
			for _, file := range written {
				_ = os.Remove(file)
			}
		}
	}()
	attachmentIDs := make([]int64, 0, len(pending))
	visibilities := make([]bool, 0, len(pending))
	for _, up := range pending {
		token, tokenErr := randomToken(20)
		if tokenErr != nil {
			err = tokenErr
			break
		}
		key := filepath.ToSlash(filepath.Join("pengaduan-kritik-saran", "follow-up", c.Param("id"), token+filepath.Ext(up.name)))
		full := filepath.Join(h.StorageDir, filepath.FromSlash(key))
		if err = os.WriteFile(full, up.data, 0600); err != nil {
			break
		}
		written = append(written, full)
		var attachmentID int64
		err = tx.QueryRow(c, `INSERT INTO complaint_attachments
			(complaint_id,storage_key,original_name,mime_type,size_bytes,sha256,public_to_reporter,kind,uploaded_by)
			VALUES($1,$2,$3,$4,$5,$6,$7,'FOLLOW_UP',$8) RETURNING id`, complaintID, key,
			up.name, up.mime, len(up.data), up.sum, public, u.ID).Scan(&attachmentID)
		if err != nil {
			break
		}
		attachmentIDs = append(attachmentIDs, attachmentID)
		visibilities = append(visibilities, public)
	}
	if err == nil {
		_, err = tx.Exec(c, `UPDATE complaints SET official_response=$1,official_response_at=NOW(),
			status='RESPONDED',clarification_deadline=NULL,updated_at=NOW() WHERE id=$2`, response, complaintID)
	}
	if err == nil {
		timelineMessage := "Tanggapan resmi diberikan dan bukti tindak lanjut telah dilampirkan."
		_, err = tx.Exec(c, `INSERT INTO complaint_timeline
			(complaint_id,actor_user_id,actor_role,action,old_status,new_status,message,is_internal,attachment_id)
			VALUES($1,$2,$3,'RESPOND',$4,'RESPONDED',$5,FALSE,$6)`, complaintID, u.ID, role,
			old, timelineMessage, attachmentIDs[len(attachmentIDs)-1])
	}
	if err == nil {
		auditOld, _ := json.Marshal(gin.H{"status": old})
		auditNew, _ := json.Marshal(gin.H{"status": "RESPONDED", "attachmentIds": attachmentIDs,
			"publicToReporter": visibilities})
		_, err = tx.Exec(c, `INSERT INTO complaint_audit_log
			(complaint_id,actor_user_id,actor_role,action,old_value,new_value,note,is_public,attachment_id)
			VALUES($1,$2,$3,'RESPOND',$4::jsonb,$5::jsonb,$6,TRUE,$7)`, complaintID, u.ID, role,
			auditOld, auditNew, nullableString(message), attachmentIDs[len(attachmentIDs)-1])
	}
	if err != nil {
		logging.Errorf("complaints", "respond_with_evidence", "user_id=%d report_id=%s err=%v", u.ID, c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tanggapan dan bukti gagal disimpan."})
		return
	}
	if err = tx.Commit(c); err != nil {
		logging.Errorf("complaints", "respond_commit", "user_id=%d report_id=%s err=%v", u.ID, c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tanggapan dan bukti gagal disimpan."})
		return
	}
	committed = true
	c.JSON(http.StatusCreated, gin.H{"message": "Tanggapan resmi dan bukti tindak lanjut berhasil disimpan.",
		"data": gin.H{"status": "RESPONDED"}})
}

func (h ComplaintHandler) UploadComplaintEvidence(c *gin.Context) {
	u, err := complaintAdminUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak."})
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Form bukti tidak valid."})
		return
	}
	files := form.File["attachments"]
	if len(files) < 1 || len(files) > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih satu sampai tiga file bukti."})
		return
	}
	message := strings.TrimSpace(c.PostForm("message"))
	public := c.PostForm("public") == "true"
	pending, requestHasImage, err := readComplaintEvidence(files)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bukti harus JPG, PNG, WebP, atau PDF dan maksimal 1 MB."})
		return
	}
	role := auth.NormalizeRole(u.Role)
	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		c.JSON(500, gin.H{"error": "Bukti belum dapat disimpan."})
		return
	}
	defer tx.Rollback(c)
	var complaintID int64
	var assigned *int
	var status string
	err = tx.QueryRow(c, `SELECT id,assigned_to,status FROM complaints WHERE report_id=$1 FOR UPDATE`,
		c.Param("id")).Scan(&complaintID, &assigned, &status)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Laporan tidak ditemukan."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bukti belum dapat disimpan."})
		return
	}
	if role == auth.RoleAdminCabdin && (assigned == nil || *assigned != u.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Laporan ini tidak ditugaskan kepada Anda."})
		return
	}
	if status != "RESPONDED" {
		c.JSON(http.StatusConflict, gin.H{"error": "Bukti tambahan hanya dapat diunggah setelah tanggapan resmi diberikan."})
		return
	}
	var existingCount int
	var existingHasImage bool
	err = tx.QueryRow(c, `SELECT COUNT(*), COALESCE(BOOL_OR(mime_type IN ('image/jpeg','image/png','image/webp')),FALSE)
		FROM complaint_attachments WHERE complaint_id=$1 AND kind='FOLLOW_UP'`, complaintID).
		Scan(&existingCount, &existingHasImage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bukti belum dapat diverifikasi."})
		return
	}
	if existingCount+len(pending) > 3 {
		c.JSON(http.StatusConflict, gin.H{"error": "Maksimal tiga bukti tindak lanjut secara total per laporan."})
		return
	}
	if !existingHasImage && !requestHasImage {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal satu bukti harus berupa foto atau screenshot JPG, PNG, atau WebP."})
		return
	}
	dir := filepath.Join(h.StorageDir, "pengaduan-kritik-saran", "follow-up", c.Param("id"))
	if os.MkdirAll(dir, 0700) != nil {
		c.JSON(500, gin.H{"error": "Storage bukti belum siap."})
		return
	}
	written := []string{}
	committed := false
	defer func() {
		if !committed {
			for _, file := range written {
				_ = os.Remove(file)
			}
		}
	}()
	attachmentIDs := make([]int64, 0, len(pending))
	visibilities := make([]bool, 0, len(pending))
	for _, up := range pending {
		token, tokenErr := randomToken(20)
		if tokenErr != nil {
			err = tokenErr
			break
		}
		key := filepath.ToSlash(filepath.Join("pengaduan-kritik-saran", "follow-up", c.Param("id"), token+filepath.Ext(up.name)))
		full := filepath.Join(h.StorageDir, filepath.FromSlash(key))
		if err = os.WriteFile(full, up.data, 0600); err != nil {
			break
		}
		written = append(written, full)
		var attachmentID int64
		err = tx.QueryRow(c, `INSERT INTO complaint_attachments
			(complaint_id,storage_key,original_name,mime_type,size_bytes,sha256,public_to_reporter,kind,uploaded_by)
			VALUES($1,$2,$3,$4,$5,$6,$7,'FOLLOW_UP',$8) RETURNING id`,
			complaintID, key, up.name, up.mime, len(up.data), up.sum, public, u.ID).Scan(&attachmentID)
		if err != nil {
			break
		}
		attachmentIDs = append(attachmentIDs, attachmentID)
		visibilities = append(visibilities, public)
	}
	if err == nil {
		_, err = tx.Exec(c, `INSERT INTO complaint_timeline
			(complaint_id,actor_user_id,actor_role,action,old_status,new_status,message,is_internal,attachment_id)
			VALUES($1,$2,$3,'FOLLOW_UP_EVIDENCE',$4,$4,$5,$6,$7)`,
			complaintID, u.ID, role, status, nullableString(message), !public, attachmentIDs[len(attachmentIDs)-1])
	}
	if err == nil {
		auditNew, _ := json.Marshal(gin.H{"attachmentIds": attachmentIDs, "publicToReporter": visibilities})
		_, err = tx.Exec(c, `INSERT INTO complaint_audit_log
			(complaint_id,actor_user_id,actor_role,action,new_value,note,is_public,attachment_id)
			VALUES($1,$2,$3,'FOLLOW_UP_EVIDENCE',$4::jsonb,$5,$6,$7)`,
			complaintID, u.ID, role, auditNew, nullableString(message), public, attachmentIDs[len(attachmentIDs)-1])
	}
	if err != nil || tx.Commit(c) != nil {
		c.JSON(500, gin.H{"error": "Bukti tindak lanjut gagal disimpan."})
		return
	}
	committed = true
	c.JSON(http.StatusCreated, gin.H{"message": "Bukti tindak lanjut berhasil disimpan."})
}
