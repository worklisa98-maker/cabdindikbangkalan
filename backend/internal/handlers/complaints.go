package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const complaintMaxFile = 1 << 20

type ComplaintHandler struct {
	DB              *db.DB
	StorageDir      string
	TurnstileSecret string
	limiter         sync.Map
}

type complaintLimit struct {
	Count int
	Reset time.Time
}

func (h *ComplaintHandler) rateLimit(c *gin.Context, scope string, max int) bool {
	key := scope + ":" + c.ClientIP()
	now := time.Now()
	raw, _ := h.limiter.LoadOrStore(key, &complaintLimit{Reset: now.Add(time.Minute)})
	entry := raw.(*complaintLimit)
	if now.After(entry.Reset) {
		entry.Count, entry.Reset = 0, now.Add(time.Minute)
	}
	entry.Count++
	if entry.Count > max {
		c.Header("Retry-After", "60")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Terlalu banyak percobaan. Coba lagi nanti."})
		return false
	}
	return true
}

func randomToken(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}

func (h ComplaintHandler) Categories(c *gin.Context) {
	rows, err := h.DB.Pool.Query(c, `SELECT id,name,report_type FROM complaint_categories WHERE is_active ORDER BY report_type,name`)
	if err != nil {
		c.JSON(500, gin.H{"error": "Gagal memuat kategori"})
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int64
		var name, typ string
		_ = rows.Scan(&id, &name, &typ)
		out = append(out, gin.H{"id": id, "name": name, "reportType": typ})
	}
	c.JSON(200, gin.H{"data": out})
}

func (h ComplaintHandler) Schools(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	rows, err := h.DB.Pool.Query(c, `SELECT id,nama_sekolah,npsn_sekolah
		FROM data_sekolah
		WHERE $1='' OR nama_sekolah ILIKE '%'||$1||'%' OR npsn_sekolah ILIKE '%'||$1||'%'
		ORDER BY nama_sekolah LIMIT 30`, q)
	if err != nil {
		c.JSON(500, gin.H{"error": "Gagal memuat sekolah"})
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id int
		var name string
		var npsn *string
		_ = rows.Scan(&id, &name, &npsn)
		out = append(out, gin.H{"id": id, "name": name, "npsn": nullablePointerValue(npsn)})
	}
	c.JSON(200, gin.H{"data": out})
}

func validComplaintFile(header *multipart.FileHeader, data []byte) (string, bool) {
	if len(data) == 0 || len(data) > complaintMaxFile {
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	mime := http.DetectContentType(data)
	ok := map[string][]string{".jpg": {"image/jpeg"}, ".jpeg": {"image/jpeg"}, ".png": {"image/png"}, ".webp": {"image/webp"}, ".pdf": {"application/pdf"}}[ext]
	for _, allowed := range ok {
		if strings.HasPrefix(mime, allowed) {
			return allowed, true
		}
	}
	return "", false
}

func (h *ComplaintHandler) SubmitPublic(c *gin.Context) { h.submit(c, nil) }
func (h *ComplaintHandler) SubmitAuthenticated(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	h.submit(c, &u)
}

func (h *ComplaintHandler) submit(c *gin.Context, user *auth.User) {
	if !h.rateLimit(c, "submit", 5) {
		return
	}
	if user == nil && h.TurnstileSecret != "" {
		token := strings.TrimSpace(c.PostForm("cf-turnstile-response"))
		resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{"secret": {h.TurnstileSecret}, "response": {token}, "remoteip": {c.ClientIP()}})
		if err != nil {
			c.JSON(503, gin.H{"error": "Layanan verifikasi keamanan tidak tersedia"})
			return
		}
		var verified struct {
			Success bool `json:"success"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 32<<10)).Decode(&verified)
		resp.Body.Close()
		if err != nil || !verified.Success {
			c.JSON(400, gin.H{"error": "Verifikasi keamanan gagal"})
			return
		}
	}
	form, err := c.MultipartForm()
	if err != nil && err != http.ErrNotMultipart {
		c.JSON(400, gin.H{"error": "Form tidak valid"})
		return
	}
	files := []*multipart.FileHeader{}
	if form != nil {
		files = form.File["attachments"]
	}
	input, validationErrors := readComplaintSubmission(c.PostForm, user != nil)
	if c.PostForm("truthConfirmed") != "true" {
		validationErrors["truthConfirmed"] = "Pernyataan kebenaran wajib dicentang."
	}
	validateComplaintAttachmentCount(len(files), validationErrors)
	if input.CategoryID > 0 {
		var categoryValid bool
		err = h.DB.Pool.QueryRow(c, `SELECT EXISTS(
			SELECT 1 FROM complaint_categories
			WHERE id=$1 AND report_type=$2 AND is_active
		)`, input.CategoryID, input.ReportType).Scan(&categoryValid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memvalidasi kategori."})
			return
		}
		if !categoryValid {
			validationErrors["categoryId"] = "Kategori tidak valid untuk jenis laporan ini."
		}
	}
	var schoolName string
	if input.TargetType == "SCHOOL" && input.SchoolID > 0 {
		err = h.DB.Pool.QueryRow(c, `SELECT nama_sekolah FROM data_sekolah WHERE id=$1`, input.SchoolID).Scan(&schoolName)
		if err == pgx.ErrNoRows {
			validationErrors["schoolId"] = "Pilih sekolah dari daftar yang tersedia."
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memvalidasi sekolah."})
			return
		}
	}
	if len(validationErrors) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  firstComplaintValidationError(validationErrors),
			"fields": validationErrors,
		})
		return
	}
	type pending struct {
		name, mime string
		data       []byte
		sum        string
	}
	uploads := []pending{}
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			c.JSON(400, gin.H{"error": "Lampiran tidak dapat dibaca"})
			return
		}
		data, err := io.ReadAll(io.LimitReader(f, complaintMaxFile+1))
		f.Close()
		mime, ok := validComplaintFile(fh, data)
		if err != nil || !ok {
			c.JSON(400, gin.H{"error": "Lampiran harus JPG, PNG, WebP, atau PDF dan maksimal 1 MB"})
			return
		}
		sum := sha256.Sum256(data)
		uploads = append(uploads, pending{filepath.Base(fh.Filename), mime, data, hex.EncodeToString(sum[:])})
	}
	reportRand, err := randomToken(7)
	if err != nil {
		c.JSON(500, gin.H{"error": "Gagal membuat ID"})
		return
	}
	pinRand, err := randomToken(6)
	if err != nil {
		c.JSON(500, gin.H{"error": "Gagal membuat PIN"})
		return
	}
	reportID := fmt.Sprintf("%s-%d-%s", map[string]string{"PENGADUAN": "PGD", "KRITIK": "KRT", "SARAN": "SRN"}[input.ReportType], time.Now().Year(), reportRand)
	pinHash, _ := bcrypt.GenerateFromPassword([]byte(pinRand), bcrypt.DefaultCost)
	var userID any
	var username, role, institution, name string
	if user != nil {
		userID = user.ID
		role = user.Role
		institution = user.NamaInstansi
		if err = h.DB.Pool.QueryRow(c, `SELECT username FROM users WHERE id=$1`, user.ID).Scan(&username); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Identitas akun tidak dapat dimuat."})
			return
		}
		name = username
	} else {
		userID = nil
		name = input.ReporterName
	}
	if input.Confidentiality == "ANONYMOUS" {
		name = ""
		input.WhatsApp = ""
		input.Email = ""
	}
	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		c.JSON(500, gin.H{"error": "Gagal menyimpan laporan"})
		return
	}
	defer tx.Rollback(c)
	var id int64
	err = tx.QueryRow(c, `INSERT INTO complaints(report_id,pin_hash,report_type,category_id,title,target_type,school_id,target_name,event_date,event_location,related_parties,details,impact,expected_resolution,reason,expected_benefit,confidentiality,reporter_user_id,reporter_name,reporter_username,reporter_role,reporter_institution,reporter_whatsapp,reporter_email,truth_confirmed)
VALUES($1,$2,$3,NULLIF($4,0),$5,$6,NULLIF($7,0),$8,NULLIF($9,'')::date,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,true) RETURNING id`,
		reportID, string(pinHash), input.ReportType, input.CategoryID, input.Title, input.TargetType,
		input.SchoolID, nullableString(input.OtherTargetName), input.EventDate, nullableString(input.EventLocation),
		nullableString(input.RelatedParties), input.Details, nullableString(input.Impact),
		nullableString(input.ExpectedResolution), nullableString(input.Reason), nullableString(input.ExpectedBenefit),
		input.Confidentiality, userID, nullableString(name), nullableString(username), nullableString(role),
		nullableString(institution), nullableString(input.WhatsApp), nullableString(input.Email)).Scan(&id)
	if err != nil {
		c.JSON(500, gin.H{"error": "Gagal menyimpan laporan"})
		return
	}
	dir := filepath.Join(h.StorageDir, "pengaduan-kritik-saran", "attachments", reportID)
	if err = os.MkdirAll(dir, 0700); err != nil {
		c.JSON(500, gin.H{"error": "Storage lampiran belum siap"})
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(dir)
		}
	}()
	for _, up := range uploads {
		rnd, _ := randomToken(20)
		key := filepath.ToSlash(filepath.Join("pengaduan-kritik-saran", "attachments", reportID, rnd+filepath.Ext(up.name)))
		full := filepath.Join(h.StorageDir, filepath.FromSlash(key))
		if err = os.WriteFile(full, up.data, 0600); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan lampiran."})
			return
		}
		_, err = tx.Exec(c, `INSERT INTO complaint_attachments(complaint_id,storage_key,original_name,mime_type,size_bytes,sha256) VALUES($1,$2,$3,$4,$5,$6)`, id, key, up.name, up.mime, len(up.data), up.sum)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data lampiran."})
			return
		}
	}
	_, _ = tx.Exec(c, `INSERT INTO complaint_timeline(complaint_id,new_status,message) VALUES($1,'RECEIVED','Laporan diterima')`, id)
	_, _ = tx.Exec(c, `INSERT INTO complaint_audit_log(complaint_id,action,new_value,is_public)
		VALUES($1,'COMPLAINT_RECEIVED',jsonb_build_object('status','RECEIVED'),TRUE)`, id)
	if err = tx.Commit(c); err != nil {
		c.JSON(500, gin.H{"error": "Gagal menyimpan laporan"})
		return
	}
	committed = true
	c.JSON(201, gin.H{"data": gin.H{"reportId": reportID, "pin": pinRand, "reportType": input.ReportType, "title": input.Title, "receivedAt": time.Now(), "status": "RECEIVED"}})
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullablePointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (h *ComplaintHandler) Track(c *gin.Context) {
	if !h.rateLimit(c, "track", 10) {
		return
	}
	h.expireClarifications(c)
	var req struct {
		ReportID string `json:"reportId"`
		PIN      string `json:"pin"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "Data tidak valid"})
		return
	}
	var id int64
	var hash, typ, title, status, targetType string
	var targetName, schoolName, officialResponse, closedReason *string
	var deadline *time.Time
	var created time.Time
	err := h.DB.Pool.QueryRow(c, `SELECT c.id,c.pin_hash,c.report_type,c.title,c.status,c.created_at,
		c.target_type,c.target_name,s.nama_sekolah,c.official_response,c.closed_reason,c.clarification_deadline
		FROM complaints c LEFT JOIN data_sekolah s ON s.id=c.school_id WHERE c.report_id=$1`,
		strings.ToUpper(strings.TrimSpace(req.ReportID))).Scan(&id, &hash, &typ, &title, &status, &created,
		&targetType, &targetName, &schoolName, &officialResponse, &closedReason, &deadline)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.PIN)) != nil {
		time.Sleep(150 * time.Millisecond)
		c.JSON(401, gin.H{"error": "ID Laporan atau PIN tidak cocok"})
		return
	}
	rows, _ := h.DB.Pool.Query(c, `SELECT new_status,message,created_at,action FROM complaint_timeline
		WHERE complaint_id=$1 AND is_internal=false ORDER BY created_at,id`, id)
	defer rows.Close()
	timeline := []gin.H{}
	for rows.Next() {
		var s, action string
		var message *string
		var at time.Time
		_ = rows.Scan(&s, &message, &at, &action)
		timeline = append(timeline, gin.H{"status": s, "message": message, "createdAt": at, "action": action})
	}
	var clarification gin.H
	var clarificationID, round int64
	var requestText string
	var requestedAt, deadlineAt time.Time
	var responseText *string
	var respondedAt *time.Time
	err = h.DB.Pool.QueryRow(c, `SELECT id,round_number,request_text,requested_at,deadline_at,response_text,responded_at
		FROM complaint_clarifications WHERE complaint_id=$1 ORDER BY round_number DESC LIMIT 1`, id).
		Scan(&clarificationID, &round, &requestText, &requestedAt, &deadlineAt, &responseText, &respondedAt)
	if err == nil {
		clarification = gin.H{"id": clarificationID, "round": round, "request": requestText,
			"requestedAt": requestedAt, "deadlineAt": deadlineAt, "response": responseText,
			"respondedAt": respondedAt, "canRespond": status == "NEEDS_INFO" && respondedAt == nil && deadlineAt.After(time.Now())}
	}
	attachments := []gin.H{}
	attachmentRows, attachmentErr := h.DB.Pool.Query(c, `SELECT id,original_name,mime_type,size_bytes,created_at
		FROM complaint_attachments WHERE complaint_id=$1 AND public_to_reporter=TRUE ORDER BY created_at,id`, id)
	if attachmentErr == nil {
		defer attachmentRows.Close()
		for attachmentRows.Next() {
			var aid, size int64
			var name, mime string
			var at time.Time
			if attachmentRows.Scan(&aid, &name, &mime, &size, &at) == nil {
				attachments = append(attachments, gin.H{"id": aid, "name": name, "mimeType": mime,
					"sizeBytes": size, "createdAt": at})
			}
		}
	}
	var hasHiddenFollowUpEvidence bool
	_ = h.DB.Pool.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM complaint_attachments
		WHERE complaint_id=$1 AND kind='FOLLOW_UP' AND public_to_reporter=FALSE)`, id).
		Scan(&hasHiddenFollowUpEvidence)
	c.JSON(200, gin.H{"data": gin.H{"reportId": strings.ToUpper(strings.TrimSpace(req.ReportID)),
		"reportType": typ, "title": title, "status": status, "createdAt": created,
		"targetType": targetType, "targetName": complaintTargetDisplay(targetType, schoolName, targetName),
		"officialResponse": officialResponse, "closedReason": closedReason,
		"clarificationDeadline": deadline, "clarification": clarification,
		"attachments": attachments, "hasHiddenFollowUpEvidence": hasHiddenFollowUpEvidence,
		"timeline": timeline}})
}

func (h ComplaintHandler) Mine(c *gin.Context) {
	u, _ := auth.GetCurrentUser(c)
	rows, err := h.DB.Pool.Query(c, `SELECT report_id,report_type,title,status,created_at FROM complaints WHERE reporter_user_id=$1 ORDER BY created_at DESC`, u.ID)
	if err != nil {
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var rid, typ, title, status string
		var at time.Time
		_ = rows.Scan(&rid, &typ, &title, &status, &at)
		out = append(out, gin.H{"reportId": rid, "reportType": typ, "title": title, "status": status, "createdAt": at})
	}
	c.JSON(200, gin.H{"data": out})
}

func (h ComplaintHandler) AdminList(c *gin.Context) {
	h.expireClarifications(c)
	u, _ := auth.GetCurrentUser(c)
	role := auth.NormalizeRole(u.Role)
	if role != auth.RoleAdminMaster && role != auth.RoleAdminCabdin {
		c.JSON(403, gin.H{"error": "Forbidden"})
		return
	}
	where := "TRUE"
	args := []any{}
	if role == auth.RoleAdminCabdin {
		where = "assigned_to=$1"
		args = append(args, u.ID)
	}
	rows, err := h.DB.Pool.Query(c, `SELECT c.report_id,c.report_type,c.title,c.status,c.confidentiality,
		c.reporter_name,c.assigned_to,c.created_at,c.target_type,c.target_name,s.nama_sekolah,
		u.username,c.clarification_deadline
		FROM complaints c
		LEFT JOIN data_sekolah s ON s.id=c.school_id
		LEFT JOIN users u ON u.id=c.assigned_to
		WHERE `+strings.ReplaceAll(where, "assigned_to", "c.assigned_to")+`
		ORDER BY c.created_at DESC`, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var rid, typ, title, status, conf, targetType string
		var reporter, otherTargetName, schoolName, assigneeName *string
		var assigned *int
		var at time.Time
		var deadline *time.Time
		_ = rows.Scan(&rid, &typ, &title, &status, &conf, &reporter, &assigned, &at, &targetType,
			&otherTargetName, &schoolName, &assigneeName, &deadline)
		display := "Anonim"
		if reporter != nil {
			display = *reporter
		}
		if role == auth.RoleAdminCabdin && conf == "CONFIDENTIAL" {
			display = "Identitas dirahasiakan"
		}
		out = append(out, gin.H{
			"reportId": rid, "reportType": typ, "title": title, "status": status,
			"confidentiality": conf, "reporter": display, "assignedTo": assigned,
			"createdAt": at, "targetType": targetType,
			"targetName":   complaintTargetDisplay(targetType, schoolName, otherTargetName),
			"assigneeName": assigneeName, "clarificationDeadline": deadline,
			"needsAction": status == "RECEIVED" || status == "REOPENED" || status == "CLARIFICATION_SUBMITTED",
		})
	}
	c.JSON(200, gin.H{"data": out})
}

func (h ComplaintHandler) AdminDetail(c *gin.Context) {
	u, err := complaintAdminUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}
	where := "c.report_id=$1"
	args := []any{c.Param("id")}
	if auth.NormalizeRole(u.Role) == auth.RoleAdminCabdin {
		where += " AND c.assigned_to=$2"
		args = append(args, u.ID)
	}
	var (
		id                                                        int64
		categoryID                                                *int64
		reportID, reportType, title, targetType, details          string
		confidentiality, status                                   string
		categoryName                                              *string
		schoolID, assignedTo                                      *int
		schoolName, otherTargetName, eventLocation, related       *string
		impact, expectedResolution, reason, expectedBenefit       *string
		reporterName, reporterUsername, reporterRole, institution *string
		whatsapp, email                                           *string
		officialResponse, closedReason, assigneeName              *string
		officialResponseAt, clarificationDeadline                 *time.Time
		eventDate                                                 *time.Time
		createdAt, updatedAt                                      time.Time
	)
	err = h.DB.Pool.QueryRow(c, `SELECT c.id,c.report_id,c.report_type,c.category_id,cc.name,c.title,
		c.target_type,c.school_id,s.nama_sekolah,c.target_name,c.event_date,c.event_location,
		c.related_parties,c.details,c.impact,c.expected_resolution,c.reason,c.expected_benefit,
		c.confidentiality,c.reporter_name,c.reporter_username,c.reporter_role,c.reporter_institution,
		c.reporter_whatsapp,c.reporter_email,c.assigned_to,c.status,c.created_at,c.updated_at,
		c.official_response,c.official_response_at,c.closed_reason,c.clarification_deadline,u.username
		FROM complaints c
		LEFT JOIN complaint_categories cc ON cc.id=c.category_id
		LEFT JOIN data_sekolah s ON s.id=c.school_id
		LEFT JOIN users u ON u.id=c.assigned_to
		WHERE `+where, args...).Scan(
		&id, &reportID, &reportType, &categoryID, &categoryName, &title, &targetType,
		&schoolID, &schoolName, &otherTargetName, &eventDate, &eventLocation, &related,
		&details, &impact, &expectedResolution, &reason, &expectedBenefit, &confidentiality,
		&reporterName, &reporterUsername, &reporterRole, &institution, &whatsapp, &email,
		&assignedTo, &status, &createdAt, &updatedAt, &officialResponse, &officialResponseAt,
		&closedReason, &clarificationDeadline, &assigneeName,
	)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Laporan tidak ditemukan."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat detail laporan."})
		return
	}
	reporterDisplay := "Anonim"
	if reporterName != nil && strings.TrimSpace(*reporterName) != "" {
		reporterDisplay = *reporterName
	}
	hideIdentity := auth.NormalizeRole(u.Role) == auth.RoleAdminCabdin && confidentiality == "CONFIDENTIAL"
	if hideIdentity {
		reporterDisplay = "Identitas dirahasiakan"
		whatsapp, email = nil, nil
	}
	rows, err := h.DB.Pool.Query(c, `SELECT id,original_name,mime_type,size_bytes,created_at,kind,public_to_reporter
		FROM complaint_attachments WHERE complaint_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memuat lampiran laporan."})
		return
	}
	defer rows.Close()
	attachments := []gin.H{}
	for rows.Next() {
		var attachmentID, size int64
		var originalName, mimeType, kind string
		var publicToReporter bool
		var at time.Time
		if rows.Scan(&attachmentID, &originalName, &mimeType, &size, &at, &kind, &publicToReporter) == nil {
			attachments = append(attachments, gin.H{
				"id": attachmentID, "name": originalName, "mimeType": mimeType,
				"sizeBytes": size, "createdAt": at, "kind": kind,
				"publicToReporter": publicToReporter,
				"url":              fmt.Sprintf("/api/admin/complaints/%s/attachments/%d", reportID, attachmentID),
			})
		}
	}
	timeline := []gin.H{}
	timelineRows, timelineErr := h.DB.Pool.Query(c, `SELECT action,old_status,new_status,message,
		is_internal,actor_role,created_at FROM complaint_timeline
		WHERE complaint_id=$1 ORDER BY created_at,id`, id)
	if timelineErr == nil {
		defer timelineRows.Close()
		for timelineRows.Next() {
			var action string
			var oldStatus, newStatus, message, actorRole *string
			var internal bool
			var at time.Time
			if timelineRows.Scan(&action, &oldStatus, &newStatus, &message, &internal, &actorRole, &at) == nil {
				timeline = append(timeline, gin.H{"action": action, "oldStatus": oldStatus,
					"status": newStatus, "message": message, "internal": internal,
					"actorRole": actorRole, "createdAt": at})
			}
		}
	}
	clarifications := []gin.H{}
	clarificationRows, clarificationErr := h.DB.Pool.Query(c, `SELECT id,round_number,request_text,
		requested_at,deadline_at,response_text,responded_at,review_result,review_note,reviewed_at
		FROM complaint_clarifications WHERE complaint_id=$1 ORDER BY round_number`, id)
	if clarificationErr == nil {
		defer clarificationRows.Close()
		for clarificationRows.Next() {
			var cid int64
			var round int
			var request string
			var requestedAt, deadlineAt time.Time
			var response, result, note *string
			var respondedAt, reviewedAt *time.Time
			if clarificationRows.Scan(&cid, &round, &request, &requestedAt, &deadlineAt,
				&response, &respondedAt, &result, &note, &reviewedAt) == nil {
				clarifications = append(clarifications, gin.H{"id": cid, "round": round,
					"request": request, "requestedAt": requestedAt, "deadlineAt": deadlineAt,
					"response": response, "respondedAt": respondedAt, "reviewResult": result,
					"reviewNote": note, "reviewedAt": reviewedAt})
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"reportId": reportID, "reportType": reportType, "categoryId": categoryID,
		"category": categoryName, "title": title, "targetType": targetType,
		"targetName": complaintTargetDisplay(targetType, schoolName, otherTargetName),
		"schoolId":   schoolID, "schoolName": schoolName, "otherTargetName": otherTargetName,
		"eventDate": eventDate, "eventLocation": eventLocation, "relatedParties": related,
		"details": details, "impact": impact, "expectedResolution": expectedResolution,
		"reason": reason, "expectedBenefit": expectedBenefit,
		"confidentiality": confidentiality, "reporter": reporterDisplay,
		"reporterUsername": reporterUsername, "reporterRole": reporterRole,
		"reporterInstitution": institution, "reporterWhatsapp": whatsapp,
		"reporterEmail": email, "assignedTo": assignedTo, "status": status,
		"assigneeName": assigneeName, "officialResponse": officialResponse,
		"officialResponseAt": officialResponseAt, "closedReason": closedReason,
		"clarificationDeadline": clarificationDeadline, "clarifications": clarifications,
		"timeline": timeline, "attachments": attachments, "createdAt": createdAt, "updatedAt": updatedAt,
	}})
}

func (h ComplaintHandler) AdminAttachment(c *gin.Context) {
	u, err := complaintAdminUser(c)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}
	attachmentID, parseErr := strconv.ParseInt(c.Param("attachmentId"), 10, 64)
	if parseErr != nil || attachmentID <= 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lampiran tidak ditemukan."})
		return
	}
	where := "c.report_id=$1 AND a.id=$2"
	args := []any{c.Param("id"), attachmentID}
	if auth.NormalizeRole(u.Role) == auth.RoleAdminCabdin {
		where += " AND c.assigned_to=$3"
		args = append(args, u.ID)
	}
	var key, name, mimeType string
	err = h.DB.Pool.QueryRow(c, `SELECT a.storage_key,a.original_name,a.mime_type
		FROM complaint_attachments a JOIN complaints c ON c.id=a.complaint_id
		WHERE `+where, args...).Scan(&key, &name, &mimeType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lampiran tidak ditemukan."})
		return
	}
	c.Header("Content-Type", mimeType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.FileAttachment(filepath.Join(h.StorageDir, filepath.FromSlash(key)), filepath.Base(name))
}

func complaintAdminUser(c *gin.Context) (auth.User, error) {
	u, err := auth.GetCurrentUser(c)
	if err != nil || !auth.IsAdmin(u) {
		return auth.User{}, auth.ErrForbidden
	}
	return u, nil
}

func complaintTargetDisplay(targetType string, schoolName, otherTargetName *string) string {
	switch targetType {
	case "SCHOOL":
		if schoolName != nil && strings.TrimSpace(*schoolName) != "" {
			return *schoolName
		}
		if otherTargetName != nil && strings.TrimSpace(*otherTargetName) != "" {
			return *otherTargetName
		}
		return "Satuan Pendidikan"
	case "OTHER":
		if otherTargetName != nil && strings.TrimSpace(*otherTargetName) != "" {
			return *otherTargetName
		}
		return "Lainnya"
	case "CABDIN":
		return "Cabang Dinas Pendidikan Wilayah Kabupaten Bangkalan"
	case "SIPRES":
		return "SIPRES/aplikasi digital"
	default:
		return targetType
	}
}

func (h ComplaintHandler) UpdateStatus(c *gin.Context) {
	u, _ := auth.GetCurrentUser(c)
	role := auth.NormalizeRole(u.Role)
	if role != auth.RoleAdminMaster && role != auth.RoleAdminCabdin {
		c.JSON(403, gin.H{"error": "Forbidden"})
		return
	}
	var req struct {
		Status, Message string
		Internal        bool
		AssignedTo      *int
	}
	if c.ShouldBindJSON(&req) != nil {
		return
	}
	rid := c.Param("id")
	tx, _ := h.DB.Pool.Begin(c)
	defer tx.Rollback(c)
	var id int64
	var old string
	var assigned *int
	if tx.QueryRow(c, `SELECT id,status,assigned_to FROM complaints WHERE report_id=$1 FOR UPDATE`, rid).Scan(&id, &old, &assigned) != nil {
		c.JSON(404, gin.H{"error": "Tidak ditemukan"})
		return
	}
	if role == auth.RoleAdminCabdin && (assigned == nil || *assigned != u.ID) {
		c.JSON(403, gin.H{"error": "Forbidden"})
		return
	}
	if req.Status == "RESPONDED" || req.Status == "COMPLETED" {
		c.JSON(http.StatusConflict, gin.H{"error": "Gunakan tindakan resmi pengaduan agar tanggapan dan bukti dapat diverifikasi."})
		return
	}
	_, err := tx.Exec(c, `UPDATE complaints SET status=$1,assigned_to=COALESCE($2,assigned_to),updated_at=NOW() WHERE id=$3`, req.Status, req.AssignedTo, id)
	if err != nil {
		return
	}
	_, _ = tx.Exec(c, `INSERT INTO complaint_timeline(complaint_id,actor_user_id,old_status,new_status,message,is_internal) VALUES($1,$2,$3,$4,$5,$6)`, id, u.ID, old, req.Status, req.Message, req.Internal)
	_, _ = tx.Exec(c, `INSERT INTO complaint_audit_log(complaint_id,actor_user_id,action,old_value,new_value) VALUES($1,$2,'STATUS_UPDATE',jsonb_build_object('status',$3),jsonb_build_object('status',$4))`, id, u.ID, old, req.Status)
	_ = tx.Commit(c)
	c.JSON(200, gin.H{"message": "Laporan diperbarui"})
}

func (h ComplaintHandler) ActivePopup(c *gin.Context) {
	var title, alt, key string
	err := h.DB.Pool.QueryRow(c, `SELECT title,alt_text,storage_key FROM complaint_popup_images WHERE is_active LIMIT 1`).Scan(&title, &alt, &key)
	if err == pgx.ErrNoRows {
		c.JSON(200, gin.H{"data": nil})
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"title": title, "altText": alt, "imageUrl": "/api/public/complaints/popup-image"}})
}
func (h ComplaintHandler) PopupImage(c *gin.Context) {
	var key, mime string
	if h.DB.Pool.QueryRow(c, `SELECT storage_key,mime_type FROM complaint_popup_images WHERE is_active LIMIT 1`).Scan(&key, &mime) != nil {
		c.Status(404)
		return
	}
	c.Header("Content-Type", mime)
	c.Header("Cache-Control", "private, max-age=300")
	c.File(filepath.Join(h.StorageDir, filepath.FromSlash(key)))
}
