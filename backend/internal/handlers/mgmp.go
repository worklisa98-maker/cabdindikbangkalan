package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/utils"
)

type MGMPHandler struct {
	DB                  *db.DB
	MasterEncryptionKey []byte
}

type mgmpMemberInput struct {
	ID             int64  `json:"id"`
	FullName       string `json:"fullName"`
	IdentityNumber string `json:"identityNumber"`
	SchoolID       *int   `json:"schoolId"`
	SchoolName     string `json:"schoolName"`
	Position       string `json:"position"`
	Email          string `json:"email"`
	WhatsApp       string `json:"whatsapp"`
	Status         string `json:"status"`
}

type mgmpRegistrationInput struct {
	OrganizationType string            `json:"organizationType"`
	Name             string            `json:"name"`
	Subject          string            `json:"subject"`
	Region           string            `json:"region"`
	ManagementPeriod string            `json:"managementPeriod"`
	Members          []mgmpMemberInput `json:"members"`
}

var mgmpWhatsAppPattern = regexp.MustCompile(`^\+?[0-9]{9,15}$`)

func normalizeMGMPPosition(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	v = strings.NewReplacer(" ", "_", "/", "_", "-", "_").Replace(v)
	if v == "SEKRETARIS" || v == "WAKIL" || v == "SEKRETARIS__WAKIL" {
		return "SEKRETARIS_WAKIL"
	}
	return v
}

func normalizeMemberStatus(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" {
		return "AKTIF"
	}
	return v
}

func validateMGMPRegistration(req *mgmpRegistrationInput) []string {
	problems := []string{}
	req.OrganizationType = strings.ToUpper(strings.TrimSpace(req.OrganizationType))
	if req.OrganizationType == "" {
		req.OrganizationType = "MGMP"
	}
	if req.OrganizationType != "MGMP" && req.OrganizationType != "JARKOM" {
		problems = append(problems, "Jenis organisasi harus MGMP atau JARKOM.")
	}
	req.Name, req.Subject, req.Region = strings.TrimSpace(req.Name), strings.TrimSpace(req.Subject), strings.TrimSpace(req.Region)
	if req.Name == "" || req.Subject == "" || req.Region == "" {
		problems = append(problems, fmt.Sprintf("Nama %s, bidang/jenis, dan wilayah wajib diisi.", req.OrganizationType))
	}
	if len(req.Members) < 2 {
		problems = append(problems, "Minimal dua anggota wajib diisi.")
	}
	emails := map[string]bool{}
	ketua, sekretaris := 0, 0
	validPositions := map[string]bool{"KETUA": true, "SEKRETARIS_WAKIL": true, "BENDAHARA": true, "ANGGOTA": true, "LAINNYA": true}
	for i := range req.Members {
		m := &req.Members[i]
		m.FullName, m.SchoolName = strings.TrimSpace(m.FullName), strings.TrimSpace(m.SchoolName)
		m.Email = strings.ToLower(strings.TrimSpace(m.Email))
		m.WhatsApp = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(m.WhatsApp))
		m.Position, m.Status = normalizeMGMPPosition(m.Position), normalizeMemberStatus(m.Status)
		row := i + 1
		if m.FullName == "" || m.SchoolName == "" || m.Email == "" || m.WhatsApp == "" {
			problems = append(problems, fmt.Sprintf("Baris %d: nama, sekolah, email, dan WhatsApp wajib diisi.", row))
		}
		if _, err := mail.ParseAddress(m.Email); err != nil {
			problems = append(problems, fmt.Sprintf("Baris %d: email tidak valid.", row))
		}
		if !mgmpWhatsAppPattern.MatchString(m.WhatsApp) {
			problems = append(problems, fmt.Sprintf("Baris %d: nomor WhatsApp tidak valid.", row))
		}
		if !validPositions[m.Position] {
			problems = append(problems, fmt.Sprintf("Baris %d: jabatan tidak valid.", row))
		}
		if m.Status != "AKTIF" && m.Status != "NONAKTIF" {
			problems = append(problems, fmt.Sprintf("Baris %d: status anggota tidak valid.", row))
		}
		if emails[m.Email] {
			problems = append(problems, fmt.Sprintf("Baris %d: email anggota duplikat.", row))
		}
		emails[m.Email] = true
		if m.Position == "KETUA" && m.Status == "AKTIF" {
			ketua++
		}
		if m.Position == "SEKRETARIS_WAKIL" && m.Status == "AKTIF" {
			sekretaris++
		}
	}
	if ketua != 1 || sekretaris != 1 {
		problems = append(problems, "Harus ada tepat satu Ketua dan satu Sekretaris/Wakil aktif.")
	}
	return problems
}

func generateMGMPTemporaryCode() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "TEMP-" + hex.EncodeToString(buffer), nil
}

func (h MGMPHandler) canonicalizeMGMPSchools(ctx context.Context, req *mgmpRegistrationInput) ([]string, error) {
	problems := make([]string, 0)
	for i := range req.Members {
		member := &req.Members[i]
		var id int
		var name string
		var err error
		if member.SchoolID != nil {
			err = h.DB.Pool.QueryRow(ctx, `SELECT id,nama_sekolah FROM data_sekolah WHERE id=$1 AND UPPER(TRIM(COALESCE(bentuk_pendidikan,'')))='SMK'`, *member.SchoolID).Scan(&id, &name)
		} else {
			err = h.DB.Pool.QueryRow(ctx, `SELECT id,nama_sekolah FROM data_sekolah WHERE LOWER(TRIM(nama_sekolah))=LOWER(TRIM($1)) AND UPPER(TRIM(COALESCE(bentuk_pendidikan,'')))='SMK' ORDER BY id LIMIT 1`, member.SchoolName).Scan(&id, &name)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			problems = append(problems, fmt.Sprintf("Sekolah pada anggota baris %d tidak ditemukan pada referensi SMK.", i+1))
			continue
		}
		if err != nil {
			logging.Errorf("mgmp", "public_register", "stage=validate_school row=%d school_id=%v err=%v", i+1, mgmpSchoolIDForLog(member.SchoolID), err)
			return nil, err
		}
		member.SchoolID = &id
		member.SchoolName = strings.TrimSpace(name)
	}
	return problems, nil
}

func isMGMPBusinessConstraint(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23502" || pgErr.Code == "23503" || pgErr.Code == "23505" || pgErr.Code == "23514"
}

func mgmpSchoolIDForLog(schoolID *int) any {
	if schoolID == nil {
		return nil
	}
	return *schoolID
}

func (h MGMPHandler) PublicRegister(c *gin.Context) {
	var enabled bool
	if err := h.DB.Pool.QueryRow(c, `SELECT COALESCE((SELECT (value #>> '{}')::boolean FROM mgmp_settings WHERE key='registration_enabled'), true)`).Scan(&enabled); err != nil {
		logging.Errorf("mgmp", "public_register", "stage=load_settings err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !enabled {
		utils.RespondError(c, http.StatusServiceUnavailable, "REGISTRATION_DISABLED")
		return
	}
	var req mgmpRegistrationInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	if problems := validateMGMPRegistration(&req); len(problems) > 0 {
		utils.RespondBadRequest(c, problems)
		return
	}

	ctx := c.Request.Context()
	problems, err := h.canonicalizeMGMPSchools(ctx, &req)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if len(problems) > 0 {
		utils.RespondBadRequest(c, problems)
		return
	}
	temporaryCode, err := generateMGMPTemporaryCode()
	if err != nil {
		logging.Errorf("mgmp", "public_register", "stage=generate_registration_code err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	tx, err := h.DB.Pool.Begin(ctx)
	if err != nil {
		logging.Errorf("mgmp", "public_register", "stage=begin_tx err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO mgmp_registrations(registration_code,organization_type,name,subject,region,management_period) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')) RETURNING id`, temporaryCode, req.OrganizationType, req.Name, req.Subject, req.Region, strings.TrimSpace(req.ManagementPeriod)).Scan(&id)
	if err != nil {
		logging.Errorf("mgmp", "public_register", "stage=insert_registration err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	code := fmt.Sprintf("%s-%d-%04d", req.OrganizationType, time.Now().Year(), id)
	if _, err = tx.Exec(ctx, `UPDATE mgmp_registrations SET registration_code=$1 WHERE id=$2`, code, id); err != nil {
		logging.Errorf("mgmp", "public_register", "stage=generate_registration_code registration_id=%d err=%v", id, err)
		utils.RespondInternalServerError(c)
		return
	}
	for index, m := range req.Members {
		_, err = tx.Exec(ctx, `INSERT INTO mgmp_members(registration_id,full_name,identity_number,school_id,school_name,position,email,whatsapp,member_status) VALUES($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9)`, id, m.FullName, strings.TrimSpace(m.IdentityNumber), m.SchoolID, m.SchoolName, m.Position, m.Email, m.WhatsApp, m.Status)
		if err != nil {
			logging.Errorf("mgmp", "public_register", "stage=insert_member row=%d name=%q email=%q school_id=%v position=%q err=%v", index+1, m.FullName, m.Email, mgmpSchoolIDForLog(m.SchoolID), m.Position, err)
			if isMGMPBusinessConstraint(err) {
				utils.RespondBadRequestMessage(c, "Data anggota gagal disimpan. Periksa data anggota dan sekolah.")
			} else {
				utils.RespondInternalServerError(c)
			}
			return
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mgmp_history(registration_id,event_type,new_status,details) VALUES($1,'REGISTRATION_SUBMITTED','PENDING',jsonb_build_object('name',$2::text,'memberCount',$3::int))`, id, req.Name, len(req.Members)); err != nil {
		logging.Errorf("mgmp", "public_register", "stage=insert_history registration_id=%d err=%v", id, err)
		utils.RespondInternalServerError(c)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		logging.Errorf("mgmp", "public_register", "stage=commit registration_id=%d err=%v", id, err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusCreated, gin.H{"registrationId": code, "organizationType": req.OrganizationType, "name": req.Name, "status": "Menunggu Verifikasi"})
}

func (h MGMPHandler) PublicRegistrationStatus(c *gin.Context) {
	code := strings.ToUpper(strings.TrimSpace(c.Param("code")))
	var registrationID int64
	var groupID *int64
	var organizationType, name, subject, region, managementPeriod, status, note string
	var submittedAt time.Time
	err := h.DB.Pool.QueryRow(c, `SELECT id,group_id,organization_type,name,subject,region,COALESCE(management_period,''),status::text,CASE WHEN status='REVISION_REQUIRED' THEN COALESCE(revision_note,'') WHEN status='REJECTED' THEN COALESCE(rejection_reason,'') ELSE '' END,submitted_at FROM mgmp_registrations WHERE registration_code=$1`, code).Scan(&registrationID, &groupID, &organizationType, &name, &subject, &region, &managementPeriod, &status, &note, &submittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		utils.RespondError(c, http.StatusNotFound, "ID registrasi tidak ditemukan.")
		return
	}
	if err != nil {
		logging.Errorf("mgmp", "public_registration_status", "stage=load_registration err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	timeline := []gin.H{}
	historyRows, historyErr := h.DB.Pool.Query(c, `SELECT event_type,created_at,COALESCE(reason,''),COALESCE((details->>'accountsCreated')::int,0) FROM mgmp_history WHERE (registration_id=$1 OR ($2::bigint IS NOT NULL AND registration_id IS NULL AND group_id=$2)) AND event_type IN ('REGISTRATION_SUBMITTED','REGISTRATION_DELEGATED','REGISTRATION_REVISION_REQUIRED','REGISTRATION_REVISION_REQUESTED','REGISTRATION_RESUBMITTED','REGISTRATION_APPROVED','REGISTRATION_REJECTED','ACCOUNT_CREATED','ACCOUNTS_CREATED') ORDER BY created_at,id`, registrationID, groupID)
	if historyErr != nil {
		logging.Errorf("mgmp", "public_registration_status", "stage=load_timeline registration_id=%d err=%v", registrationID, historyErr)
		utils.RespondInternalServerError(c)
		return
	}
	defer historyRows.Close()
	for historyRows.Next() {
		var event, reason string
		var createdAt time.Time
		var accountsCreated int
		if err := historyRows.Scan(&event, &createdAt, &reason, &accountsCreated); err != nil {
			logging.Errorf("mgmp", "public_registration_status", "stage=scan_timeline registration_id=%d err=%v", registrationID, err)
			utils.RespondInternalServerError(c)
			return
		}
		if label, ok := publicMGMPHistoryLabel(event); ok {
			timeline = append(timeline, gin.H{"event": publicMGMPHistoryEvent(event), "label": label, "note": reason, "createdAt": createdAt})
		}
		if event == "REGISTRATION_APPROVED" && accountsCreated > 0 {
			timeline = append(timeline, gin.H{"event": "ACCOUNTS_CREATED", "label": "Akun Ketua dan Sekretaris/Wakil berhasil dibuat", "createdAt": createdAt})
		}
	}
	if err := historyRows.Err(); err != nil {
		logging.Errorf("mgmp", "public_registration_status", "stage=iterate_timeline registration_id=%d err=%v", registrationID, err)
		utils.RespondInternalServerError(c)
		return
	}

	credentials := []gin.H{}
	if status == "APPROVED" && groupID != nil {
		// TODO(email-activation): Public credential reveal is temporary for demo mode.
		// Remove or protect this lookup when email activation/reset password is available.
		credentialRows, credentialErr := h.DB.Pool.Query(c, `SELECT name,position,username,initial_password_ciphertext,initial_password_nonce FROM (SELECT COALESCE(m.full_name,u.nama_instansi,u.username) AS name,l.position,u.username,u.initial_password_ciphertext,u.initial_password_nonce,ROW_NUMBER() OVER (PARTITION BY l.position ORDER BY l.created_at DESC,l.id DESC) AS rank FROM mgmp_registrations r JOIN mgmp_groups g ON g.id=r.group_id JOIN mgmp_user_links l ON l.group_id=g.id AND l.is_active=TRUE JOIN users u ON u.id=l.user_id LEFT JOIN mgmp_members m ON m.id=l.member_id WHERE r.id=$1 AND r.group_id=$2 AND l.position IN ('KETUA','SEKRETARIS_WAKIL')) linked_accounts WHERE rank=1 ORDER BY CASE position WHEN 'KETUA' THEN 1 ELSE 2 END`, registrationID, *groupID)
		if credentialErr != nil {
			logging.Errorf("mgmp", "public_registration_status", "stage=load_credentials registration_id=%d err=%v", registrationID, credentialErr)
			utils.RespondInternalServerError(c)
			return
		}
		defer credentialRows.Close()
		for credentialRows.Next() {
			var credentialName, position, username string
			var cipher, nonce *string
			if err := credentialRows.Scan(&credentialName, &position, &username, &cipher, &nonce); err != nil {
				logging.Errorf("mgmp", "public_registration_status", "stage=scan_credentials registration_id=%d err=%v", registrationID, err)
				utils.RespondInternalServerError(c)
				return
			}
			if cipher == nil || nonce == nil {
				logging.Errorf("mgmp", "public_registration_status", "stage=decrypt_credentials registration_id=%d err=missing_ciphertext", registrationID)
				utils.RespondInternalServerError(c)
				return
			}
			password, err := utils.DecryptInitialPassword(h.MasterEncryptionKey, *cipher, *nonce)
			if err != nil {
				logging.Errorf("mgmp", "public_registration_status", "stage=decrypt_credentials registration_id=%d err=%v", registrationID, err)
				utils.RespondInternalServerError(c)
				return
			}
			credentials = append(credentials, gin.H{"name": credentialName, "position": position, "username": username, "password": password})
		}
		if err := credentialRows.Err(); err != nil {
			logging.Errorf("mgmp", "public_registration_status", "stage=iterate_credentials registration_id=%d err=%v", registrationID, err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	members := []gin.H{}
	if status == "REVISION_REQUIRED" {
		memberRows, memberErr := h.DB.Pool.Query(c, `SELECT id,full_name,COALESCE(identity_number,''),school_id,school_name,position,email,whatsapp,member_status FROM mgmp_members WHERE registration_id=$1 ORDER BY id`, registrationID)
		if memberErr != nil {
			logging.Errorf("mgmp", "public_registration_status", "stage=load_revision_members registration_id=%d err=%v", registrationID, memberErr)
			utils.RespondInternalServerError(c)
			return
		}
		defer memberRows.Close()
		for memberRows.Next() {
			var id int64
			var fullName, identityNumber, schoolName, position, email, whatsapp, memberStatus string
			var schoolID *int
			if memberErr = memberRows.Scan(&id, &fullName, &identityNumber, &schoolID, &schoolName, &position, &email, &whatsapp, &memberStatus); memberErr != nil {
				logging.Errorf("mgmp", "public_registration_status", "stage=scan_revision_members registration_id=%d err=%v", registrationID, memberErr)
				utils.RespondInternalServerError(c)
				return
			}
			members = append(members, gin.H{"id": id, "fullName": fullName, "identityNumber": identityNumber, "schoolId": schoolID, "schoolName": schoolName, "position": position, "email": email, "whatsapp": whatsapp, "status": memberStatus})
		}
		if memberErr = memberRows.Err(); memberErr != nil {
			logging.Errorf("mgmp", "public_registration_status", "stage=iterate_revision_members registration_id=%d err=%v", registrationID, memberErr)
			utils.RespondInternalServerError(c)
			return
		}
	}

	utils.RespondData(c, http.StatusOK, gin.H{"registrationId": code, "organizationType": organizationType, "name": name, "subject": subject, "region": region, "managementPeriod": managementPeriod, "status": status, "statusLabel": publicMGMPStatusLabel(status), "note": note, "submittedAt": submittedAt, "timeline": timeline, "credentials": credentials, "members": members})
}

func (h MGMPHandler) PublicResubmitRegistration(c *gin.Context) {
	code := strings.ToUpper(strings.TrimSpace(c.Param("code")))
	var req mgmpRegistrationInput
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "Data perbaikan tidak valid.")
		return
	}
	if problems := validateMGMPRegistration(&req); len(problems) > 0 {
		utils.RespondBadRequest(c, problems)
		return
	}
	ctx := c.Request.Context()
	problems, err := h.canonicalizeMGMPSchools(ctx, &req)
	if err != nil {
		logging.Errorf("mgmp", "registration_resubmit", "registration_code=%s stage=validate_schools err=%v", code, err)
		utils.RespondInternalServerError(c)
		return
	}
	if len(problems) > 0 {
		utils.RespondBadRequest(c, problems)
		return
	}
	tx, err := h.DB.Pool.Begin(ctx)
	if err != nil {
		logging.Errorf("mgmp", "registration_resubmit", "registration_code=%s stage=begin err=%v", code, err)
		utils.RespondInternalServerError(c)
		return
	}
	defer tx.Rollback(ctx)
	var registrationID int64
	var status, organizationType string
	var delegatedAdminID *int
	if err = tx.QueryRow(ctx, `SELECT id,status::text,organization_type,delegated_admin_id FROM mgmp_registrations WHERE registration_code=$1 FOR UPDATE`, code).Scan(&registrationID, &status, &organizationType, &delegatedAdminID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondNotFound(c)
		} else {
			logging.Errorf("mgmp", "registration_resubmit", "registration_code=%s stage=load err=%v", code, err)
			utils.RespondInternalServerError(c)
		}
		return
	}
	if status != "REVISION_REQUIRED" {
		utils.RespondConflictMessage(c, "Pendaftaran ini tidak sedang menunggu perbaikan dan tidak dapat dikirim ulang.")
		return
	}
	if req.OrganizationType != organizationType {
		utils.RespondBadRequestMessage(c, "Jenis organisasi tidak dapat diubah saat perbaikan.")
		return
	}
	var existingCount int
	if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM mgmp_members WHERE registration_id=$1`, registrationID).Scan(&existingCount); err != nil || existingCount != len(req.Members) {
		utils.RespondBadRequestMessage(c, "Daftar anggota harus mempertahankan anggota pendaftaran yang sama.")
		return
	}
	seenIDs := map[int64]bool{}
	for _, member := range req.Members {
		if member.ID <= 0 || seenIDs[member.ID] {
			utils.RespondBadRequestMessage(c, "Identitas anggota perbaikan tidak valid.")
			return
		}
		seenIDs[member.ID] = true
		tag, updateErr := tx.Exec(ctx, `UPDATE mgmp_members SET full_name=$1,identity_number=NULLIF($2,''),school_id=$3,school_name=$4,position=$5,email=$6,whatsapp=$7,member_status=$8,updated_at=NOW() WHERE id=$9 AND registration_id=$10`, member.FullName, strings.TrimSpace(member.IdentityNumber), member.SchoolID, member.SchoolName, member.Position, member.Email, member.WhatsApp, member.Status, member.ID, registrationID)
		if updateErr != nil || tag.RowsAffected() != 1 {
			logging.Errorf("mgmp", "registration_resubmit", "registration_id=%d stage=update_member member_id=%d err=%v", registrationID, member.ID, updateErr)
			utils.RespondBadRequestMessage(c, "Salah satu anggota tidak termasuk dalam pendaftaran ini.")
			return
		}
	}
	tag, err := tx.Exec(ctx, `UPDATE mgmp_registrations SET name=$1,subject=$2,region=$3,management_period=NULLIF($4,''),status='PENDING',updated_at=NOW() WHERE id=$5 AND status='REVISION_REQUIRED'`, req.Name, req.Subject, req.Region, strings.TrimSpace(req.ManagementPeriod), registrationID)
	if err != nil || tag.RowsAffected() != 1 {
		logging.Errorf("mgmp", "registration_resubmit", "registration_id=%d stage=update_registration err=%v", registrationID, err)
		utils.RespondConflictMessage(c, "Pendaftaran ini sudah diproses dan tidak dapat dikirim ulang.")
		return
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mgmp_history(registration_id,event_type,old_status,new_status,details) VALUES($1,'REGISTRATION_RESUBMITTED','REVISION_REQUIRED','PENDING',jsonb_build_object('delegatedAdminId',$2::int))`, registrationID, delegatedAdminID); err != nil {
		logging.Errorf("mgmp", "registration_resubmit", "registration_id=%d stage=insert_history err=%v", registrationID, err)
		utils.RespondInternalServerError(c)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		logging.Errorf("mgmp", "registration_resubmit", "registration_id=%d stage=commit err=%v", registrationID, err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"registrationId": code, "status": "PENDING", "delegatedAdminId": delegatedAdminID})
}

func publicMGMPStatusLabel(status string) string {
	labels := map[string]string{"PENDING": "Menunggu Verifikasi", "REVISION_REQUIRED": "Perlu Perbaikan", "APPROVED": "Disetujui", "REJECTED": "Ditolak"}
	return labels[status]
}

func publicMGMPHistoryEvent(event string) string {
	if event == "REGISTRATION_REVISION_REQUESTED" {
		return "REGISTRATION_REVISION_REQUIRED"
	}
	return event
}

func publicMGMPHistoryLabel(event string) (string, bool) {
	labels := map[string]string{
		"REGISTRATION_SUBMITTED":          "Pendaftaran dikirim",
		"REGISTRATION_DELEGATED":          "Pendaftaran diteruskan untuk proses verifikasi",
		"REGISTRATION_REVISION_REQUIRED":  "Permintaan perbaikan",
		"REGISTRATION_REVISION_REQUESTED": "Permintaan perbaikan",
		"REGISTRATION_RESUBMITTED":        "Pendaftaran diperbaiki dan dikirim ulang",
		"REGISTRATION_APPROVED":           "Pendaftaran disetujui",
		"REGISTRATION_REJECTED":           "Pendaftaran ditolak",
		"ACCOUNT_CREATED":                 "Akun organisasi dibuat",
		"ACCOUNTS_CREATED":                "Akun organisasi dibuat",
	}
	label, ok := labels[event]
	return label, ok
}

func (h MGMPHandler) PublicReferences(c *gin.Context) {
	rows, err := h.DB.Pool.Query(c, `SELECT id,nama_sekolah FROM data_sekolah WHERE UPPER(TRIM(COALESCE(bentuk_pendidikan,'')))='SMK' ORDER BY nama_sekolah`)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()
	schools := []gin.H{}
	for rows.Next() {
		var id int
		var name string
		if rows.Scan(&id, &name) == nil {
			schools = append(schools, gin.H{"id": id, "name": name})
		}
	}
	subjects := []string{"Bahasa Indonesia", "Bahasa Inggris", "Matematika", "Pendidikan Agama Islam", "Pendidikan Pancasila", "Informatika", "Projek Kreatif dan Kewirausahaan", "Dasar-Dasar Kejuruan", "Lainnya / belum terdaftar"}
	utils.RespondData(c, http.StatusOK, gin.H{"schools": schools, "subjects": subjects})
}

func (h MGMPHandler) DownloadMemberTemplate(c *gin.Context) {
	rows, err := h.DB.Pool.Query(c, `SELECT nama_sekolah FROM data_sekolah WHERE UPPER(TRIM(COALESCE(bentuk_pendidikan,'')))='SMK' ORDER BY nama_sekolah`)
	if err != nil {
		logging.Errorf("mgmp", "download_member_template", "stage=load_schools err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	schools := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			logging.Errorf("mgmp", "download_member_template", "stage=scan_school err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		schools = append(schools, strings.TrimSpace(name))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		logging.Errorf("mgmp", "download_member_template", "stage=iterate_schools err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	rows.Close()
	data, err := buildMGMPMemberTemplate(schools)
	if err != nil {
		logging.Errorf("mgmp", "download_member_template", "stage=build_workbook err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="template-daftar-anggota-mgmp-jarkom-smk.xlsx"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

func buildMGMPMemberTemplate(schools []string) ([]byte, error) {
	if len(schools) == 0 {
		return nil, errors.New("referensi sekolah SMK kosong")
	}
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", "Daftar Anggota"); err != nil {
		return nil, err
	}
	headers := []string{"Nama Lengkap", "NIP/NIK", "Sekolah Asal", "Jabatan", "Email", "Nomor WhatsApp", "Status Anggota"}
	if err := f.SetSheetRow("Daftar Anggota", "A1", &headers); err != nil {
		return nil, err
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"1E63D5"}, Pattern: 1}})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle("Daftar Anggota", "A1", "G1", style); err != nil {
		return nil, err
	}
	textStyle, err := f.NewStyle(&excelize.Style{NumFmt: 49})
	if err != nil {
		return nil, err
	}
	for _, ref := range []string{"B2:B2000", "F2:F2000"} {
		if err := f.SetCellStyle("Daftar Anggota", strings.Split(ref, ":")[0], strings.Split(ref, ":")[1], textStyle); err != nil {
			return nil, err
		}
	}
	if err := f.SetPanes("Daftar Anggota", &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}
	for col, width := range map[string]float64{"A": 26, "B": 20, "C": 34, "D": 24, "E": 28, "F": 20, "G": 18} {
		if err := f.SetColWidth("Daftar Anggota", col, col, width); err != nil {
			return nil, err
		}
	}
	addList := func(ref string, values []string) error {
		dv := excelize.NewDataValidation(true)
		dv.Sqref = ref
		errorStyle := "stop"
		dv.ErrorStyle = &errorStyle
		dv.ShowErrorMessage = true
		errorTitle := "Input tidak valid"
		dv.ErrorTitle = &errorTitle
		errorMessage := "Nilai harus dipilih dari daftar yang tersedia."
		dv.Error = &errorMessage
		if err := dv.SetDropList(values); err != nil {
			return err
		}
		return f.AddDataValidation("Daftar Anggota", dv)
	}
	if err := addList("D2:D2000", []string{"Ketua", "Sekretaris/Wakil", "Bendahara", "Anggota", "Lainnya"}); err != nil {
		return nil, err
	}
	if err := addList("G2:G2000", []string{"Aktif", "Nonaktif"}); err != nil {
		return nil, err
	}
	if _, err := f.NewSheet("Referensi Sekolah SMK"); err != nil {
		return nil, err
	}
	for index, name := range schools {
		if err := f.SetCellValue("Referensi Sekolah SMK", fmt.Sprintf("A%d", index+1), name); err != nil {
			return nil, err
		}
	}
	if err := f.SetDefinedName(&excelize.DefinedName{Name: "MGMP_SCHOOL_LIST", RefersTo: fmt.Sprintf("'Referensi Sekolah SMK'!$A$1:$A$%d", len(schools))}); err != nil {
		return nil, err
	}
	schoolValidation := excelize.NewDataValidation(false)
	schoolValidation.Sqref = "C2:C2000"
	errorStyle := "stop"
	schoolValidation.ErrorStyle = &errorStyle
	schoolValidation.ShowErrorMessage = true
	errorTitle := "Sekolah tidak valid"
	schoolValidation.ErrorTitle = &errorTitle
	errorMessage := "Sekolah Asal wajib dipilih dari referensi sekolah SMK."
	schoolValidation.Error = &errorMessage
	schoolValidation.SetSqrefDropList("MGMP_SCHOOL_LIST")
	if err := f.AddDataValidation("Daftar Anggota", schoolValidation); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mgmpCurrentUser(c *gin.Context) (auth.User, bool) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return auth.User{}, false
	}
	return u, true
}
func (h MGMPHandler) requireAdmin(c *gin.Context) (auth.User, bool) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return u, false
	}
	if !auth.IsAdmin(u) {
		utils.RespondForbidden(c)
		return u, false
	}
	return u, true
}
func (h MGMPHandler) scopedGroupID(ctx context.Context, user auth.User) (*int64, error) {
	if auth.NormalizeRole(user.Role) != auth.RoleMGMPUser {
		return nil, nil
	}
	var id int64
	err := h.DB.Pool.QueryRow(ctx, `SELECT l.group_id FROM mgmp_user_links l JOIN mgmp_groups g ON g.id=l.group_id WHERE l.user_id=$1 AND l.is_active=TRUE LIMIT 1`, user.ID).Scan(&id)
	return &id, err
}

func (h MGMPHandler) Workspace(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	gid, err := h.scopedGroupID(c, u)
	if err != nil || gid == nil {
		utils.RespondForbidden(c)
		return
	}
	year, _ := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(time.Now().Year())))
	month, _ := strconv.Atoi(c.DefaultQuery("month", strconv.Itoa(int(time.Now().Month()))))
	var data []byte
	err = h.DB.Pool.QueryRow(c, `SELECT jsonb_build_object('group',jsonb_build_object('id',g.id,'organizationType',g.organization_type,'name',g.name,'subject',g.subject,'region',g.region,'status',g.status),'summary',jsonb_build_object('monthCount',(SELECT COUNT(*) FROM mgmp_activities a WHERE a.group_id=g.id AND EXTRACT(YEAR FROM a.activity_date)=$2 AND EXTRACT(MONTH FROM a.activity_date)=$3),'yearCount',(SELECT COUNT(*) FROM mgmp_activities a WHERE a.group_id=g.id AND EXTRACT(YEAR FROM a.activity_date)=$2),'lastActivity',(SELECT MAX(activity_date) FROM mgmp_activities a WHERE a.group_id=g.id),'monthStatus',(SELECT CASE WHEN EXISTS(SELECT 1 FROM mgmp_activities a WHERE a.group_id=g.id AND EXTRACT(YEAR FROM a.activity_date)=$2 AND EXTRACT(MONTH FROM a.activity_date)=$3 AND a.attendance_url<>'' AND a.documentation_url<>'') THEN 'ADA_KEGIATAN' WHEN EXISTS(SELECT 1 FROM mgmp_activities a WHERE a.group_id=g.id AND EXTRACT(YEAR FROM a.activity_date)=$2 AND EXTRACT(MONTH FROM a.activity_date)=$3) THEN 'LAPORAN_BELUM_LENGKAP' WHEN EXISTS(SELECT 1 FROM mgmp_monthly_statuses s WHERE s.group_id=g.id AND s.year=$2 AND s.month=$3) THEN 'TIDAK_ADA_AGENDA' ELSE 'BELUM_MELAPOR' END)),'activities',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',a.id,'title',a.title,'activityDate',a.activity_date,'venue',a.venue,'description',a.description,'participantCount',a.participant_count,'activeMemberCount',a.active_member_count,'attendanceUrl',a.attendance_url,'documentationUrl',a.documentation_url,'notes',a.notes,'attendance',(SELECT COALESCE(jsonb_agg(jsonb_build_object('memberId',att.member_id,'fullName',att.member_name_snapshot,'schoolName',att.school_name_snapshot,'status',att.attendance_status,'absenceType',att.absence_type,'absenceNote',att.absence_note) ORDER BY att.member_name_snapshot),'[]'::jsonb) FROM mgmp_activity_attendance att WHERE att.activity_id=a.id),'createdBy',cu.nama_instansi,'createdByPosition',(SELECT position FROM mgmp_user_links WHERE user_id=a.created_by_user_id AND group_id=a.group_id LIMIT 1),'updatedBy',uu.nama_instansi,'updatedByPosition',(SELECT position FROM mgmp_user_links WHERE user_id=a.updated_by_user_id AND group_id=a.group_id LIMIT 1)) ORDER BY a.activity_date DESC),'[]'::jsonb) FROM mgmp_activities a JOIN users cu ON cu.id=a.created_by_user_id JOIN users uu ON uu.id=a.updated_by_user_id WHERE a.group_id=g.id AND EXTRACT(YEAR FROM a.activity_date)=$2 AND EXTRACT(MONTH FROM a.activity_date)=$3)) FROM mgmp_groups g WHERE g.id=$1`, *gid, year, month).Scan(&data)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	memberRows, memberErr := h.DB.Pool.Query(c, `SELECT id,full_name,school_name,position,email,whatsapp,member_status,COALESCE(deactivation_reason,''),COALESCE(deactivation_note,''),deactivated_at FROM mgmp_members WHERE group_id=$1 ORDER BY CASE member_status WHEN 'AKTIF' THEN 0 ELSE 1 END,position,full_name`, *gid)
	if memberErr == nil {
		defer memberRows.Close()
		members := []gin.H{}
		for memberRows.Next() {
			var id int64
			var fullName, schoolName, position, email, whatsapp, status, reason, note string
			var deactivatedAt *time.Time
			if memberRows.Scan(&id, &fullName, &schoolName, &position, &email, &whatsapp, &status, &reason, &note, &deactivatedAt) == nil {
				members = append(members, gin.H{"id": id, "fullName": fullName, "schoolName": schoolName, "position": position, "email": email, "whatsapp": whatsapp, "status": status, "deactivationReason": reason, "deactivationNote": note, "deactivatedAt": deactivatedAt})
			}
		}
		out["members"] = members
	}
	notificationRows, notificationErr := h.DB.Pool.Query(c, `SELECT n.id,m.full_name,n.event_type,n.reason,COALESCE(n.note,''),n.effective_date,n.created_at FROM mgmp_member_notifications n JOIN mgmp_members m ON m.id=n.member_id WHERE n.group_id=$1 AND NOT EXISTS(SELECT 1 FROM mgmp_member_notification_reads r WHERE r.notification_id=n.id AND r.user_id=$2) ORDER BY n.created_at`, *gid, u.ID)
	if notificationErr == nil {
		defer notificationRows.Close()
		notifications := []gin.H{}
		for notificationRows.Next() {
			var id int64
			var memberName, event, reason, note string
			var effectiveDate time.Time
			var createdAt time.Time
			if notificationRows.Scan(&id, &memberName, &event, &reason, &note, &effectiveDate, &createdAt) == nil {
				notifications = append(notifications, gin.H{"id": id, "memberName": memberName, "eventType": event, "reason": reason, "note": note, "effectiveDate": effectiveDate, "createdAt": createdAt})
			}
		}
		out["notifications"] = notifications
	}
	utils.RespondData(c, http.StatusOK, out)
}

type mgmpActivityInput struct {
	Title        string `json:"title"`
	ActivityDate string `json:"activityDate"`
	Venue        string `json:"venue"`
	Description  string `json:"description"`
	// Kept in the contract for legacy clients; the server always derives it
	// from PRESENT attendance records.
	ParticipantCount int    `json:"participantCount,omitempty"`
	AttendanceURL    string `json:"attendanceUrl"`
	DocumentationURL string `json:"documentationUrl"`
	Notes            string `json:"notes"`
	Attendance       []struct {
		MemberID    int64  `json:"memberId"`
		Status      string `json:"status"`
		AbsenceType string `json:"absenceType"`
		AbsenceNote string `json:"absenceNote"`
	} `json:"attendance"`
}

func validateActivity(req mgmpActivityInput) error {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Venue) == "" || strings.TrimSpace(req.Description) == "" {
		return errors.New("Data wajib kegiatan belum lengkap.")
	}
	if _, err := time.Parse("2006-01-02", req.ActivityDate); err != nil {
		return errors.New("Tanggal kegiatan tidak valid.")
	}
	for _, raw := range []string{req.AttendanceURL, req.DocumentationURL} {
		u, err := url.ParseRequestURI(strings.TrimSpace(raw))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return errors.New("Bukti absensi dan dokumentasi harus berupa URL http/https yang valid.")
		}
	}
	seen := map[int64]bool{}
	for _, item := range req.Attendance {
		item.Status = strings.ToUpper(strings.TrimSpace(item.Status))
		item.AbsenceType = strings.ToUpper(strings.TrimSpace(item.AbsenceType))
		if item.MemberID <= 0 || seen[item.MemberID] {
			return errors.New("Data kehadiran anggota tidak valid atau duplikat.")
		}
		seen[item.MemberID] = true
		if item.Status == "PRESENT" {
			if item.AbsenceType != "" || strings.TrimSpace(item.AbsenceNote) != "" {
				return errors.New("Anggota hadir tidak boleh memiliki alasan ketidakhadiran.")
			}
			continue
		}
		if item.Status != "ABSENT" || (item.AbsenceType != "EXCUSED" && item.AbsenceType != "UNEXCUSED") {
			return errors.New("Status kehadiran tidak valid.")
		}
		if item.AbsenceType == "EXCUSED" && strings.TrimSpace(item.AbsenceNote) == "" {
			return errors.New("Keterangan ketidakhadiran wajib diisi.")
		}
	}
	return nil
}

type mgmpAttendanceMember struct {
	ID         int64
	FullName   string
	SchoolName string
}

func attendanceCounts(req mgmpActivityInput) (present, excused, unexcused int) {
	for _, item := range req.Attendance {
		if strings.EqualFold(item.Status, "PRESENT") {
			present++
		} else if strings.EqualFold(item.AbsenceType, "EXCUSED") {
			excused++
		} else {
			unexcused++
		}
	}
	return
}

func (h MGMPHandler) SaveActivity(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	gid, err := h.scopedGroupID(c, u)
	if err != nil || gid == nil {
		logging.Errorf("mgmp", "save_activity", "stage=resolve_group user_id=%d err=%v", u.ID, err)
		utils.RespondForbidden(c)
		return
	}
	var status string
	if err = h.DB.Pool.QueryRow(c, `SELECT status::text FROM mgmp_groups WHERE id=$1`, *gid).Scan(&status); err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=resolve_group group_id=%d err=%v", *gid, err)
		utils.RespondInternalServerError(c)
		return
	}
	if status != "ACTIVE" {
		utils.RespondError(c, http.StatusConflict, "MGMP_INACTIVE")
		return
	}
	var req mgmpActivityInput
	if err = c.ShouldBindJSON(&req); err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=validate_members group_id=%d err=%v", *gid, err)
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	if err := validateActivity(req); err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=validate_members group_id=%d err=%v", *gid, err)
		utils.RespondBadRequestMessage(c, err.Error())
		return
	}
	if len(req.Attendance) == 0 {
		logging.Errorf("mgmp", "save_activity", "stage=validate_members group_id=%d err=%v", *gid, errors.New("attendance is empty"))
		utils.RespondBadRequestMessage(c, "Kehadiran anggota wajib diisi.")
		return
	}
	ctx := c.Request.Context()
	tx, err := h.DB.Pool.Begin(ctx)
	if err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=begin_tx group_id=%d err=%v", *gid, err)
		utils.RespondInternalServerError(c)
		return
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,full_name,school_name FROM mgmp_members WHERE group_id=$1 AND member_status='AKTIF' ORDER BY id FOR SHARE`, *gid)
	if err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=query_active_members group_id=%d err=%v", *gid, err)
		utils.RespondInternalServerError(c)
		return
	}
	active := map[int64]mgmpAttendanceMember{}
	for rows.Next() {
		var member mgmpAttendanceMember
		if err = rows.Scan(&member.ID, &member.FullName, &member.SchoolName); err != nil {
			rows.Close()
			logging.Errorf("mgmp", "save_activity", "stage=query_active_members group_id=%d err=%v", *gid, err)
			utils.RespondInternalServerError(c)
			return
		}
		active[member.ID] = member
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=query_active_members group_id=%d err=%v", *gid, err)
		utils.RespondInternalServerError(c)
		return
	}
	if len(active) == 0 || len(active) != len(req.Attendance) {
		logging.Errorf("mgmp", "save_activity", "stage=validate_members group_id=%d active_members=%d attendance_rows=%d err=%v", *gid, len(active), len(req.Attendance), errors.New("attendance coverage mismatch"))
		utils.RespondBadRequestMessage(c, "Kehadiran harus mencakup seluruh anggota aktif.")
		return
	}
	for _, item := range req.Attendance {
		if _, ok := active[item.MemberID]; !ok {
			logging.Errorf("mgmp", "save_activity", "stage=validate_members group_id=%d member_id=%d err=%v", *gid, item.MemberID, errors.New("member is inactive or belongs to another organization"))
			utils.RespondBadRequestMessage(c, "Kehadiran memuat anggota yang tidak aktif atau bukan anggota organisasi.")
			return
		}
	}
	present, excused, unexcused := attendanceCounts(req)
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO mgmp_activities(group_id,title,activity_date,venue,description,participant_count,active_member_count,attendance_url,documentation_url,notes,created_by_user_id,updated_by_user_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,''),$11,$11) RETURNING id`, *gid, strings.TrimSpace(req.Title), req.ActivityDate, strings.TrimSpace(req.Venue), strings.TrimSpace(req.Description), present, len(active), strings.TrimSpace(req.AttendanceURL), strings.TrimSpace(req.DocumentationURL), strings.TrimSpace(req.Notes), u.ID).Scan(&id)
	if err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=insert_activity group_id=%d err=%v", *gid, err)
		utils.RespondInternalServerError(c)
		return
	}
	for _, item := range req.Attendance {
		member := active[item.MemberID]
		status := strings.ToUpper(strings.TrimSpace(item.Status))
		absenceType := strings.ToUpper(strings.TrimSpace(item.AbsenceType))
		var absenceTypeValue, absenceNoteValue any
		if status == "ABSENT" {
			absenceTypeValue = absenceType
			if absenceType == "EXCUSED" {
				absenceNoteValue = strings.TrimSpace(item.AbsenceNote)
			}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO mgmp_activity_attendance(activity_id,member_id,member_name_snapshot,school_name_snapshot,attendance_status,absence_type,absence_note) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, member.ID, member.FullName, member.SchoolName, status, absenceTypeValue, absenceNoteValue); err != nil {
			logging.Errorf("mgmp", "save_activity", "stage=insert_attendance activity_id=%d member_id=%d attendance_status=%s absence_type=%v err=%v", id, member.ID, status, absenceTypeValue, err)
			utils.RespondInternalServerError(c)
			return
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM mgmp_monthly_statuses WHERE group_id=$1 AND year=EXTRACT(YEAR FROM $2::date) AND month=EXTRACT(MONTH FROM $2::date)`, *gid, req.ActivityDate); err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=update_monthly_status activity_id=%d group_id=%d err=%v", id, *gid, err)
		utils.RespondInternalServerError(c)
		return
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mgmp_history(group_id,activity_id,actor_user_id,event_type,details) VALUES($1,$2,$3,'ACTIVITY_CREATED',jsonb_build_object('title',$4::text,'activeMembers',$5::int,'present',$6::int,'excused',$7::int,'unexcused',$8::int))`, *gid, id, u.ID, strings.TrimSpace(req.Title), len(active), present, excused, unexcused); err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=insert_activity_history activity_id=%d group_id=%d err=%v", id, *gid, err)
		utils.RespondInternalServerError(c)
		return
	}
	if _, err = tx.Exec(ctx, `INSERT INTO mgmp_history(group_id,activity_id,actor_user_id,event_type,details) VALUES($1,$2,$3,'ATTENDANCE_CREATED',jsonb_build_object('present',$4::int,'excused',$5::int,'unexcused',$6::int))`, *gid, id, u.ID, present, excused, unexcused); err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=insert_attendance_history activity_id=%d group_id=%d err=%v", id, *gid, err)
		utils.RespondInternalServerError(c)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		logging.Errorf("mgmp", "save_activity", "stage=commit activity_id=%d group_id=%d err=%v", id, *gid, err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusCreated, gin.H{"id": id})
}

func (h MGMPHandler) UpdateActivity(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	gid, err := h.scopedGroupID(c, u)
	if err != nil || gid == nil {
		utils.RespondForbidden(c)
		return
	}
	var req mgmpActivityInput
	if c.ShouldBindJSON(&req) != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	if err := validateActivity(req); err != nil {
		utils.RespondBadRequestMessage(c, err.Error())
		return
	}
	if len(req.Attendance) == 0 {
		utils.RespondBadRequestMessage(c, "Kehadiran anggota wajib diisi.")
		return
	}
	ctx := c.Request.Context()
	tx, err := h.DB.Pool.Begin(ctx)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer tx.Rollback(ctx)
	present, excused, unexcused := attendanceCounts(req)
	tag, err := tx.Exec(ctx, `UPDATE mgmp_activities a SET title=$1,activity_date=$2,venue=$3,description=$4,participant_count=$5,active_member_count=$6,attendance_url=$7,documentation_url=$8,notes=NULLIF($9,''),updated_by_user_id=$10,updated_at=NOW() FROM mgmp_groups g WHERE a.id=$11 AND a.group_id=$12 AND g.id=a.group_id AND g.status='ACTIVE'`, req.Title, req.ActivityDate, req.Venue, req.Description, present, len(req.Attendance), req.AttendanceURL, req.DocumentationURL, req.Notes, u.ID, c.Param("id"), *gid)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		utils.RespondNotFound(c)
		return
	}
	for _, item := range req.Attendance {
		var name, school string
		if err = tx.QueryRow(ctx, `SELECT full_name,school_name FROM mgmp_members WHERE id=$1 AND group_id=$2`, item.MemberID, *gid).Scan(&name, &school); err != nil {
			utils.RespondBadRequestMessage(c, "Anggota pada data kehadiran tidak valid.")
			return
		}
		status, absenceType := strings.ToUpper(strings.TrimSpace(item.Status)), strings.ToUpper(strings.TrimSpace(item.AbsenceType))
		var absenceTypeValue, absenceNoteValue any
		if status == "ABSENT" {
			absenceTypeValue = absenceType
			if absenceType == "EXCUSED" {
				absenceNoteValue = strings.TrimSpace(item.AbsenceNote)
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO mgmp_activity_attendance(activity_id,member_id,member_name_snapshot,school_name_snapshot,attendance_status,absence_type,absence_note) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(activity_id,member_id) DO UPDATE SET attendance_status=EXCLUDED.attendance_status,absence_type=EXCLUDED.absence_type,absence_note=EXCLUDED.absence_note,updated_at=NOW()`, c.Param("id"), item.MemberID, name, school, status, absenceTypeValue, absenceNoteValue)
		if err != nil {
			utils.RespondInternalServerError(c)
			return
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM mgmp_activity_attendance WHERE activity_id=$1 AND NOT (member_id=ANY($2::bigint[]))`, c.Param("id"), attendanceMemberIDs(req))
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO mgmp_history(group_id,activity_id,actor_user_id,event_type,details) VALUES($1,$2,$3,'ACTIVITY_UPDATED',jsonb_build_object('title',$4))`, *gid, c.Param("id"), u.ID, req.Title)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO mgmp_history(group_id,activity_id,actor_user_id,event_type,details) VALUES($1,$2,$3,'ATTENDANCE_REVISED',jsonb_build_object('present',$4::int,'excused',$5::int,'unexcused',$6::int))`, *gid, c.Param("id"), u.ID, present, excused, unexcused)
	}
	if err != nil || tx.Commit(ctx) != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"updated": true})
}

func attendanceMemberIDs(req mgmpActivityInput) []int64 {
	ids := make([]int64, 0, len(req.Attendance))
	for _, item := range req.Attendance {
		ids = append(ids, item.MemberID)
	}
	return ids
}

func validateAdditionalMember(member *mgmpMemberInput) []string {
	problems := []string{}
	member.FullName = strings.TrimSpace(member.FullName)
	member.IdentityNumber = strings.TrimSpace(member.IdentityNumber)
	member.SchoolName = strings.TrimSpace(member.SchoolName)
	member.Email = strings.ToLower(strings.TrimSpace(member.Email))
	member.WhatsApp = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(member.WhatsApp))
	member.Position = normalizeMGMPPosition(member.Position)
	member.Status = "AKTIF"
	if member.FullName == "" || member.SchoolName == "" || member.Email == "" || member.WhatsApp == "" {
		problems = append(problems, "Nama, sekolah, email, dan WhatsApp wajib diisi")
	}
	if _, err := mail.ParseAddress(member.Email); err != nil {
		problems = append(problems, "Email tidak valid")
	}
	if !mgmpWhatsAppPattern.MatchString(member.WhatsApp) {
		problems = append(problems, "Nomor WhatsApp tidak valid")
	}
	if !map[string]bool{"KETUA": true, "SEKRETARIS_WAKIL": true, "BENDAHARA": true, "ANGGOTA": true, "LAINNYA": true}[member.Position] {
		problems = append(problems, "Jabatan tidak valid")
	}
	return problems
}

func (h MGMPHandler) canonicalizeAdditionalMember(ctx context.Context, member *mgmpMemberInput) error {
	var id int
	var name string
	var err error
	if member.SchoolID != nil {
		err = h.DB.Pool.QueryRow(ctx, `SELECT id,nama_sekolah FROM data_sekolah WHERE id=$1 AND UPPER(TRIM(COALESCE(bentuk_pendidikan,'')))='SMK'`, *member.SchoolID).Scan(&id, &name)
	} else {
		err = h.DB.Pool.QueryRow(ctx, `SELECT id,nama_sekolah FROM data_sekolah WHERE LOWER(TRIM(nama_sekolah))=LOWER(TRIM($1)) AND UPPER(TRIM(COALESCE(bentuk_pendidikan,'')))='SMK' ORDER BY id LIMIT 1`, member.SchoolName).Scan(&id, &name)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("Sekolah tidak ditemukan pada referensi SMK")
	}
	if err != nil {
		return err
	}
	member.SchoolID, member.SchoolName = &id, strings.TrimSpace(name)
	return nil
}

func (h MGMPHandler) insertAdditionalMember(ctx context.Context, tx pgx.Tx, gid int64, actorID int, member mgmpMemberInput, event string) (int64, error) {
	var duplicate bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM mgmp_members WHERE group_id=$1 AND (LOWER(email)=LOWER($2) OR (NULLIF(TRIM($3),'') IS NOT NULL AND identity_number=$3)))`, gid, member.Email, member.IdentityNumber).Scan(&duplicate)
	if err != nil {
		return 0, err
	}
	if duplicate {
		return 0, errors.New("Email atau NIP/NIK sudah pernah terdaftar; gunakan Aktifkan Kembali Anggota")
	}
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO mgmp_members(group_id,full_name,identity_number,school_id,school_name,position,email,whatsapp,member_status) VALUES($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,'AKTIF') RETURNING id`, gid, member.FullName, member.IdentityNumber, member.SchoolID, member.SchoolName, member.Position, member.Email, member.WhatsApp).Scan(&id)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO mgmp_history(group_id,actor_user_id,event_type,details) VALUES($1,$2,$3,jsonb_build_object('memberId',$4::bigint,'memberName',$5::text))`, gid, actorID, event, id, member.FullName)
	}
	return id, err
}

func (h MGMPHandler) AddMember(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	gid, err := h.scopedGroupID(c, u)
	if err != nil || gid == nil {
		utils.RespondForbidden(c)
		return
	}
	var member mgmpMemberInput
	if c.ShouldBindJSON(&member) != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	if problems := validateAdditionalMember(&member); len(problems) > 0 {
		utils.RespondBadRequest(c, problems)
		return
	}
	if err := h.canonicalizeAdditionalMember(c, &member); err != nil {
		utils.RespondBadRequestMessage(c, err.Error())
		return
	}
	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer tx.Rollback(c)
	id, err := h.insertAdditionalMember(c, tx, *gid, u.ID, member, "MEMBER_ADDED")
	if err != nil {
		utils.RespondConflictMessage(c, err.Error())
		return
	}
	if tx.Commit(c) != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusCreated, gin.H{"id": id})
}

func (h MGMPHandler) ImportMembers(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	gid, err := h.scopedGroupID(c, u)
	if err != nil || gid == nil {
		utils.RespondForbidden(c)
		return
	}
	var req struct {
		Members []mgmpMemberInput `json:"members"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Members) == 0 || len(req.Members) > 2000 {
		utils.RespondBadRequestMessage(c, "Data import kosong atau melebihi 2.000 baris.")
		return
	}
	results := make([]map[string]any, 0, len(req.Members))
	success := 0
	for index, original := range req.Members {
		member := original
		reasons := validateAdditionalMember(&member)
		if len(reasons) == 0 {
			if err := h.canonicalizeAdditionalMember(c, &member); err != nil {
				reasons = append(reasons, err.Error())
			}
		}
		if len(reasons) == 0 {
			tx, beginErr := h.DB.Pool.Begin(c)
			if beginErr != nil {
				utils.RespondInternalServerError(c)
				return
			}
			_, insertErr := h.insertAdditionalMember(c, tx, *gid, u.ID, member, "MEMBER_IMPORTED")
			if insertErr == nil {
				insertErr = tx.Commit(c)
			} else {
				_ = tx.Rollback(c)
			}
			if insertErr != nil {
				reasons = append(reasons, insertErr.Error())
			} else {
				success++
			}
		}
		status := "BERHASIL"
		reason := "-"
		if len(reasons) > 0 {
			status, reason = "DITOLAK", strings.Join(reasons, "; ")
		}
		results = append(results, map[string]any{"row": index + 2, "fullName": original.FullName, "identityNumber": original.IdentityNumber, "schoolName": original.SchoolName, "position": original.Position, "email": original.Email, "whatsapp": original.WhatsApp, "status": original.Status, "importStatus": status, "reason": reason})
	}
	raw, _ := json.Marshal(results)
	var importID int64
	err = h.DB.Pool.QueryRow(c, `INSERT INTO mgmp_member_imports(group_id,actor_user_id,total_rows,success_rows,rejected_rows,results) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, *gid, u.ID, len(req.Members), success, len(req.Members)-success, raw).Scan(&importID)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"importId": importID, "total": len(req.Members), "success": success, "rejected": len(req.Members) - success, "results": results})
}

func (h MGMPHandler) DownloadMemberImportResult(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	gid, err := h.scopedGroupID(c, u)
	if err != nil || gid == nil {
		utils.RespondForbidden(c)
		return
	}
	var raw []byte
	if err := h.DB.Pool.QueryRow(c, `SELECT results FROM mgmp_member_imports WHERE id=$1 AND group_id=$2`, c.Param("id"), *gid).Scan(&raw); err != nil {
		utils.RespondNotFound(c)
		return
	}
	var results []map[string]any
	if json.Unmarshal(raw, &results) != nil {
		utils.RespondInternalServerError(c)
		return
	}
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	_ = f.SetSheetName("Sheet1", "Hasil Import")
	headers := []string{"Nama Lengkap", "NIP/NIK", "Sekolah Asal", "Jabatan", "Email", "Nomor WhatsApp", "Status Anggota", "Status Import", "Alasan Penolakan"}
	_ = f.SetSheetRow("Hasil Import", "A1", &headers)
	for index, item := range results {
		row := []any{item["fullName"], item["identityNumber"], item["schoolName"], item["position"], item["email"], item["whatsapp"], item["status"], item["importStatus"], item["reason"]}
		_ = f.SetSheetRow("Hasil Import", fmt.Sprintf("A%d", index+2), &row)
	}
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"1E63D5"}, Pattern: 1}})
	_ = f.SetCellStyle("Hasil Import", "A1", "I1", style)
	_ = f.SetColWidth("Hasil Import", "A", "I", 22)
	buffer, err := f.WriteToBuffer()
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="hasil-import-anggota-%s.xlsx"`, c.Param("id")))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h MGMPHandler) AcknowledgeMemberNotification(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	gid, err := h.scopedGroupID(c, u)
	if err != nil || gid == nil {
		utils.RespondForbidden(c)
		return
	}
	tag, err := h.DB.Pool.Exec(c, `INSERT INTO mgmp_member_notification_reads(notification_id,user_id) SELECT id,$1 FROM mgmp_member_notifications WHERE id=$2 AND group_id=$3 ON CONFLICT DO NOTHING`, u.ID, c.Param("id"), *gid)
	if err != nil || tag.RowsAffected() == 0 {
		utils.RespondNotFound(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"acknowledged": true})
}

func (h MGMPHandler) SetNoAgenda(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	gid, err := h.scopedGroupID(c, u)
	if err != nil || gid == nil {
		utils.RespondForbidden(c)
		return
	}
	var groupStatus string
	if err := h.DB.Pool.QueryRow(c, `SELECT status::text FROM mgmp_groups WHERE id=$1`, *gid).Scan(&groupStatus); err != nil || groupStatus != "ACTIVE" {
		utils.RespondError(c, http.StatusConflict, "MGMP_INACTIVE")
		return
	}
	var req struct {
		Year   int    `json:"year"`
		Month  int    `json:"month"`
		Reason string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Year < 2000 || req.Month < 1 || req.Month > 12 || strings.TrimSpace(req.Reason) == "" {
		utils.RespondBadRequestMessage(c, "Tahun, bulan, dan alasan wajib diisi.")
		return
	}
	var exists bool
	_ = h.DB.Pool.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM mgmp_activities WHERE group_id=$1 AND EXTRACT(YEAR FROM activity_date)=$2 AND EXTRACT(MONTH FROM activity_date)=$3)`, *gid, req.Year, req.Month).Scan(&exists)
	if exists {
		utils.RespondConflictMessage(c, "Bulan ini sudah memiliki kegiatan.")
		return
	}
	_, err = h.DB.Pool.Exec(c, `INSERT INTO mgmp_monthly_statuses(group_id,year,month,no_agenda_reason,created_by_user_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(group_id,year,month) DO UPDATE SET no_agenda_reason=EXCLUDED.no_agenda_reason,updated_at=NOW()`, *gid, req.Year, req.Month, strings.TrimSpace(req.Reason), u.ID)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	_, _ = h.DB.Pool.Exec(c, `INSERT INTO mgmp_history(group_id,actor_user_id,event_type,new_status,reason) VALUES($1,$2,'NO_AGENDA_DECLARED','TIDAK_ADA_AGENDA',$3)`, *gid, u.ID, req.Reason)
	utils.RespondData(c, http.StatusOK, gin.H{"status": "TIDAK_ADA_AGENDA"})
}

func (h MGMPHandler) ListRegistrations(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	where := "TRUE"
	args := []any{}
	if !auth.IsAdminMaster(u) {
		where = "r.delegated_admin_id=$1"
		args = append(args, u.ID)
	}
	rows, err := h.DB.Pool.Query(c, `SELECT r.id,r.registration_code,r.organization_type,r.name,r.subject,r.region,COALESCE(r.management_period,''),r.status::text,r.delegated_admin_id,COALESCE(u.nama_instansi,u.username,''),r.submitted_at,(SELECT COUNT(*) FROM mgmp_members m WHERE m.registration_id=r.id) FROM mgmp_registrations r LEFT JOIN users u ON u.id=r.delegated_admin_id WHERE `+where+` ORDER BY r.submitted_at DESC`, args...)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var code, organizationType, name, subject, region, period, status, delegateName string
		var delegateID *int
		var submitted time.Time
		var count int
		if rows.Scan(&id, &code, &organizationType, &name, &subject, &region, &period, &status, &delegateID, &delegateName, &submitted, &count) == nil {
			items = append(items, gin.H{"id": id, "registrationId": code, "organizationType": organizationType, "name": name, "subject": subject, "region": region, "period": period, "status": status, "delegatedAdminId": delegateID, "delegatedAdminName": delegateName, "submittedAt": submitted, "memberCount": count})
		}
	}
	utils.RespondData(c, http.StatusOK, items)
}

func (h MGMPHandler) RegistrationDetail(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	var data []byte
	err := h.DB.Pool.QueryRow(c, `SELECT jsonb_build_object('id',r.id,'registrationId',r.registration_code,'organizationType',r.organization_type,'name',r.name,'subject',r.subject,'region',r.region,'period',r.management_period,'status',r.status,'delegatedAdminId',r.delegated_admin_id,'revisionNote',r.revision_note,'rejectionReason',r.rejection_reason,'potentialGroups',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',g.id,'organizationType',g.organization_type,'name',g.name,'subject',g.subject,'region',g.region,'status',g.status)),'[]'::jsonb) FROM mgmp_groups g WHERE g.organization_type=r.organization_type AND (LOWER(TRIM(g.name))=LOWER(TRIM(r.name)) OR LOWER(TRIM(g.subject))=LOWER(TRIM(r.subject)))),'members',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',m.id,'fullName',m.full_name,'schoolName',m.school_name,'position',m.position,'email',m.email,'whatsapp',m.whatsapp,'status',m.member_status) ORDER BY m.id),'[]'::jsonb) FROM mgmp_members m WHERE m.registration_id=r.id)) FROM mgmp_registrations r WHERE r.id=$1 AND ($2::boolean OR r.delegated_admin_id=$3)`, c.Param("id"), auth.IsAdminMaster(u), u.ID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		utils.RespondNotFound(c)
		return
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	var out any
	_ = json.Unmarshal(data, &out)
	utils.RespondData(c, http.StatusOK, out)
}

func (h MGMPHandler) ListDelegates(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	if !auth.IsAdminMaster(u) {
		utils.RespondForbidden(c)
		return
	}
	rows, err := h.DB.Pool.Query(c, `SELECT id,username,COALESCE(NULLIF(TRIM(nama_instansi),''),username) FROM users WHERE role::text IN ('ADMIN','ADMIN_CABDIN') ORDER BY 3`)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int
		var username, name string
		if rows.Scan(&id, &username, &name) == nil {
			items = append(items, gin.H{"id": id, "username": username, "name": name})
		}
	}
	utils.RespondData(c, http.StatusOK, items)
}

func (h MGMPHandler) DelegateRegistration(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	if !auth.IsAdminMaster(u) {
		utils.RespondForbidden(c)
		return
	}
	var req struct {
		AdminID int `json:"adminId"`
	}
	if c.ShouldBindJSON(&req) != nil || req.AdminID <= 0 {
		utils.RespondBadRequest(c, "Admin wajib dipilih.")
		return
	}
	tag, err := h.DB.Pool.Exec(c, `UPDATE mgmp_registrations SET delegated_admin_id=$1,updated_at=NOW() WHERE id=$2 AND status IN ('PENDING','REVISION_REQUIRED')`, req.AdminID, c.Param("id"))
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		utils.RespondNotFound(c)
		return
	}
	_, _ = h.DB.Pool.Exec(c, `INSERT INTO mgmp_history(registration_id,actor_user_id,event_type,details) VALUES($1,$2,'REGISTRATION_DELEGATED',jsonb_build_object('adminId',$3))`, c.Param("id"), u.ID, req.AdminID)
	utils.RespondData(c, http.StatusOK, gin.H{"delegated": true})
}

func slugMGMP(value string) string {
	v := utils.SlugifySchoolName(value)
	v = strings.ReplaceAll(v, "-", "")
	if len(v) > 16 {
		v = v[:16]
	}
	if v == "" {
		v = "mgmp"
	}
	return v
}
func (h MGMPHandler) uniqueUsername(ctx context.Context, tx pgx.Tx, base string) (string, error) {
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s%d", base, i+1)
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username=$1)`, candidate).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", errors.New("USERNAME_LIMIT")
}

func canDecideMGMPRegistration(action, status string) bool {
	action, status = strings.ToUpper(strings.TrimSpace(action)), strings.ToUpper(strings.TrimSpace(status))
	return (action == "REJECT" && (status == "PENDING" || status == "REVISION_REQUIRED")) ||
		((action == "APPROVE" || action == "REVISION") && status == "PENDING")
}

func (h MGMPHandler) DecideRegistration(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	var req struct {
		Action          string `json:"action"`
		Reason          string `json:"reason"`
		ExistingGroupID *int64 `json:"existingGroupId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	req.Action = strings.ToUpper(strings.TrimSpace(req.Action))
	if req.Action != "APPROVE" && req.Action != "REVISION" && req.Action != "REJECT" {
		utils.RespondBadRequest(c, "Aksi tidak valid.")
		return
	}
	if req.Action != "APPROVE" && strings.TrimSpace(req.Reason) == "" {
		utils.RespondBadRequestMessage(c, "Alasan/keterangan wajib diisi.")
		return
	}
	if auth.IsAdminMaster(u) {
		utils.RespondForbidden(c)
		return
	}
	ctx := c.Request.Context()
	tx, err := h.DB.Pool.Begin(ctx)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer tx.Rollback(ctx)
	var rid int64
	var organizationType, name, subject, region, period, status string
	var delegated *int
	err = tx.QueryRow(ctx, `SELECT id,organization_type,name,subject,region,COALESCE(management_period,''),status::text,delegated_admin_id FROM mgmp_registrations WHERE id=$1 FOR UPDATE`, c.Param("id")).Scan(&rid, &organizationType, &name, &subject, &region, &period, &status, &delegated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondNotFound(c)
		} else {
			logging.Errorf("mgmp", "registration_"+strings.ToLower(req.Action), "registration_id=%s actor_id=%d stage=load err=%v", c.Param("id"), u.ID, err)
			utils.RespondInternalServerError(c)
		}
		return
	}
	if delegated == nil || *delegated != u.ID {
		utils.RespondForbidden(c)
		return
	}
	if !canDecideMGMPRegistration(req.Action, status) {
		utils.RespondConflictMessage(c, "Pendaftaran ini sudah diproses dan tidak dapat menjalankan keputusan tersebut.")
		return
	}
	if req.Action == "REVISION" || req.Action == "REJECT" {
		next := "REVISION_REQUIRED"
		event := "REGISTRATION_REVISION_REQUESTED"
		updateSQL := `UPDATE mgmp_registrations SET status='REVISION_REQUIRED',revision_note=$1,updated_at=NOW() WHERE id=$2 AND status='PENDING'`
		if req.Action == "REJECT" {
			next = "REJECTED"
			event = "REGISTRATION_REJECTED"
			updateSQL = `UPDATE mgmp_registrations SET status='REJECTED',rejection_reason=$1,updated_at=NOW() WHERE id=$2 AND status IN ('PENDING','REVISION_REQUIRED')`
		}
		var tag pgconn.CommandTag
		tag, err = tx.Exec(ctx, updateSQL, strings.TrimSpace(req.Reason), rid)
		if err == nil && tag.RowsAffected() != 1 {
			err = errors.New("invalid registration transition")
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO mgmp_history(registration_id,actor_user_id,event_type,old_status,new_status,reason) VALUES($1,$2,$3,$4,$5,$6)`, rid, u.ID, event, status, next, req.Reason)
		}
		if err != nil {
			logging.Errorf("mgmp", "registration_"+strings.ToLower(req.Action), "registration_id=%d actor_id=%d stage=update_and_history err=%v", rid, u.ID, err)
			utils.RespondInternalServerError(c)
			return
		}
		if err = tx.Commit(ctx); err != nil {
			logging.Errorf("mgmp", "registration_"+strings.ToLower(req.Action), "registration_id=%d actor_id=%d stage=commit err=%v", rid, u.ID, err)
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, gin.H{"status": next})
		return
	}
	var gid int64
	merging := req.ExistingGroupID != nil
	if req.ExistingGroupID != nil {
		gid = *req.ExistingGroupID
		var exists bool
		_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM mgmp_groups WHERE id=$1 AND organization_type=$4 AND ($2::boolean OR delegated_admin_id=$3))`, gid, auth.IsAdminMaster(u), u.ID, organizationType).Scan(&exists)
		if !exists {
			utils.RespondBadRequestMessage(c, "Organisasi existing dengan jenis yang sama tidak ditemukan.")
			return
		}
	} else {
		err = tx.QueryRow(ctx, `INSERT INTO mgmp_groups(organization_type,name,subject,region,management_period,status,delegated_admin_id,activated_at) VALUES($1,$2,$3,$4,NULLIF($5,''),'ACTIVE',$6,NOW()) RETURNING id`, organizationType, name, subject, region, period, delegated).Scan(&gid)
		if err != nil {
			utils.RespondConflictMessage(c, "Organisasi terindikasi duplikat. Gunakan opsi gabungkan ke organisasi existing.")
			return
		}
	}
	memberUpdateQuery := `UPDATE mgmp_members SET group_id=$1 WHERE registration_id=$2`
	if merging {
		memberUpdateQuery = `UPDATE mgmp_members incoming SET group_id=$1 WHERE incoming.registration_id=$2 AND NOT EXISTS (SELECT 1 FROM mgmp_members existing WHERE existing.group_id=$1 AND LOWER(existing.email)=LOWER(incoming.email))`
	}
	_, err = tx.Exec(ctx, memberUpdateQuery, gid, rid)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	rows, err := tx.Query(ctx, `SELECT COALESCE((SELECT existing.id FROM mgmp_members existing WHERE existing.group_id=$2 AND LOWER(existing.email)=LOWER(incoming.email) LIMIT 1),incoming.id),incoming.full_name,incoming.position,incoming.email FROM mgmp_members incoming WHERE incoming.registration_id=$1 AND incoming.position IN ('KETUA','SEKRETARIS_WAKIL') AND incoming.member_status='AKTIF' ORDER BY incoming.position`, rid, gid)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	type leader struct {
		id                    int64
		name, position, email string
	}
	leaders := []leader{}
	for rows.Next() {
		var l leader
		_ = rows.Scan(&l.id, &l.name, &l.position, &l.email)
		leaders = append(leaders, l)
	}
	rows.Close()
	if len(leaders) != 2 {
		utils.RespondBadRequestMessage(c, "Ketua dan Sekretaris/Wakil aktif belum lengkap.")
		return
	}
	credentials := []gin.H{}
	for _, l := range leaders {
		base := slugMGMP(name)
		if l.position == "KETUA" {
			base += ".ketua"
		} else {
			base += ".sekretaris"
		}
		username, e := h.uniqueUsername(ctx, tx, base)
		if e != nil {
			utils.RespondInternalServerError(c)
			return
		}
		password, e := utils.GenerateStrongPassword(10)
		if e != nil {
			utils.RespondInternalServerError(c)
			return
		}
		hash, e := utils.HashPassword(password)
		if e != nil {
			utils.RespondInternalServerError(c)
			return
		}
		cipher, nonce, e := utils.EncryptInitialPassword(h.MasterEncryptionKey, password)
		if e != nil {
			utils.RespondInternalServerError(c)
			return
		}
		var uid int
		e = tx.QueryRow(ctx, `INSERT INTO users(username,password_hash,role,nama_instansi,initial_password_ciphertext,initial_password_nonce,initial_password_set_at) VALUES($1,$2,'USER_MGMP',$3,$4,$5,NOW()) RETURNING id`, username, hash, l.name, cipher, nonce).Scan(&uid)
		if e != nil {
			utils.RespondInternalServerError(c)
			return
		}
		_, e = tx.Exec(ctx, `INSERT INTO mgmp_user_links(group_id,user_id,member_id,position) VALUES($1,$2,$3,$4)`, gid, uid, l.id, l.position)
		if e != nil {
			utils.RespondInternalServerError(c)
			return
		}
		credentials = append(credentials, gin.H{"userId": uid, "name": l.name, "position": l.position, "username": username, "password": password})
	}
	var approvalTag pgconn.CommandTag
	approvalTag, err = tx.Exec(ctx, `UPDATE mgmp_registrations SET status='APPROVED',group_id=$1,updated_at=NOW() WHERE id=$2 AND status='PENDING'`, gid, rid)
	if err == nil && approvalTag.RowsAffected() != 1 {
		err = errors.New("invalid registration transition")
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO mgmp_history(group_id,registration_id,actor_user_id,event_type,old_status,new_status,details) VALUES($1,$2,$3,'REGISTRATION_APPROVED',$4,'APPROVED',jsonb_build_object('accountsCreated',2))`, gid, rid, u.ID, status)
	}
	if err != nil {
		logging.Errorf("mgmp", "registration_approve", "registration_id=%d actor_id=%d stage=update_and_history err=%v", rid, u.ID, err)
		utils.RespondInternalServerError(c)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		logging.Errorf("mgmp", "registration_approve", "registration_id=%d actor_id=%d stage=commit err=%v", rid, u.ID, err)
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"status": "APPROVED", "groupId": gid, "credentials": credentials})
}

func (h MGMPHandler) Monitoring(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	year, _ := strconv.Atoi(c.DefaultQuery("year", strconv.Itoa(time.Now().Year())))
	month, _ := strconv.Atoi(c.DefaultQuery("month", strconv.Itoa(int(time.Now().Month()))))
	organizationType := strings.ToUpper(strings.TrimSpace(c.Query("organizationType")))
	if organizationType != "" && organizationType != "MGMP" && organizationType != "JARKOM" {
		utils.RespondBadRequestMessage(c, "Filter jenis organisasi tidak valid.")
		return
	}
	where := "TRUE"
	args := []any{year, month}
	if organizationType != "" {
		args = append(args, organizationType)
		where += fmt.Sprintf(" AND g.organization_type=$%d", len(args))
	}
	if !auth.IsAdminMaster(u) {
		args = append(args, u.ID)
		where += fmt.Sprintf(" AND g.delegated_admin_id=$%d", len(args))
	}
	rows, err := h.DB.Pool.Query(c, `SELECT g.id,g.organization_type,g.name,g.subject,g.region,g.status::text,COUNT(a.id) FILTER(WHERE EXTRACT(YEAR FROM a.activity_date)=$1 AND EXTRACT(MONTH FROM a.activity_date)=$2),COUNT(a.id) FILTER(WHERE EXTRACT(YEAR FROM a.activity_date)=$1),MAX(a.activity_date),CASE WHEN COUNT(a.id) FILTER(WHERE EXTRACT(YEAR FROM a.activity_date)=$1 AND EXTRACT(MONTH FROM a.activity_date)=$2 AND a.attendance_url<>'' AND a.documentation_url<>'')>0 THEN 'ADA_KEGIATAN' WHEN COUNT(a.id) FILTER(WHERE EXTRACT(YEAR FROM a.activity_date)=$1 AND EXTRACT(MONTH FROM a.activity_date)=$2)>0 THEN 'LAPORAN_BELUM_LENGKAP' WHEN EXISTS(SELECT 1 FROM mgmp_monthly_statuses ms WHERE ms.group_id=g.id AND ms.year=$1 AND ms.month=$2) THEN 'TIDAK_ADA_AGENDA' ELSE 'BELUM_MELAPOR' END FROM mgmp_groups g LEFT JOIN mgmp_activities a ON a.group_id=g.id WHERE `+where+` GROUP BY g.id ORDER BY g.organization_type,g.name`, args...)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	kpi := map[string]int{"total": 0, "active": 0, "noAgenda": 0, "incomplete": 0, "missing": 0, "inactive": 0}
	for rows.Next() {
		var id int64
		var itemType, name, subject, region, gstatus, mstatus string
		var monthCount, yearCount int
		var last *time.Time
		if rows.Scan(&id, &itemType, &name, &subject, &region, &gstatus, &monthCount, &yearCount, &last, &mstatus) == nil {
			kpi["total"]++
			if gstatus == "ACTIVE" {
				kpi["active"]++
			} else if gstatus == "INACTIVE" {
				kpi["inactive"]++
			}
			switch mstatus {
			case "TIDAK_ADA_AGENDA":
				kpi["noAgenda"]++
			case "LAPORAN_BELUM_LENGKAP":
				kpi["incomplete"]++
			case "BELUM_MELAPOR":
				kpi["missing"]++
			}
			missingStreak := 0
			if mstatus == "BELUM_MELAPOR" {
				for offset := 0; offset < 24; offset++ {
					selected := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -offset, 0)
					var reported bool
					_ = h.DB.Pool.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM mgmp_activities WHERE group_id=$1 AND EXTRACT(YEAR FROM activity_date)=$2 AND EXTRACT(MONTH FROM activity_date)=$3) OR EXISTS(SELECT 1 FROM mgmp_monthly_statuses WHERE group_id=$1 AND year=$2 AND month=$3)`, id, selected.Year(), int(selected.Month())).Scan(&reported)
					if reported {
						break
					}
					missingStreak++
				}
			}
			items = append(items, gin.H{"id": id, "organizationType": itemType, "name": name, "subject": subject, "region": region, "groupStatus": gstatus, "monthStatus": mstatus, "monthCount": monthCount, "yearCount": yearCount, "lastActivity": last, "missingStreak": missingStreak})
		}
	}
	warningThreshold := 3
	_ = h.DB.Pool.QueryRow(c, `SELECT COALESCE((SELECT (value #>> '{}')::int FROM mgmp_settings WHERE key='missing_report_warning_months'),3)`).Scan(&warningThreshold)
	utils.RespondData(c, http.StatusOK, gin.H{"kpi": kpi, "items": items, "year": year, "month": month, "warningThreshold": warningThreshold})
}

func (h MGMPHandler) GroupDetail(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	var data []byte
	err := h.DB.Pool.QueryRow(c, `SELECT jsonb_build_object('id',g.id,'organizationType',g.organization_type,'name',g.name,'subject',g.subject,'region',g.region,'period',g.management_period,'status',g.status,'deactivationReason',g.deactivation_reason,'accounts',(SELECT COALESCE(jsonb_agg(jsonb_build_object('userId',u.id,'name',m.full_name,'position',l.position,'username',u.username,'isActive',l.is_active)),'[]'::jsonb) FROM mgmp_user_links l JOIN users u ON u.id=l.user_id LEFT JOIN mgmp_members m ON m.id=l.member_id WHERE l.group_id=g.id),'members',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',m.id,'fullName',m.full_name,'schoolName',m.school_name,'position',m.position,'email',m.email,'whatsapp',m.whatsapp,'status',m.member_status,'deactivationReason',m.deactivation_reason,'deactivationNote',m.deactivation_note,'deactivatedAt',m.deactivated_at,'attendanceStats',jsonb_build_object('activities',(SELECT COUNT(*) FROM mgmp_activity_attendance s WHERE s.member_id=m.id),'present',(SELECT COUNT(*) FROM mgmp_activity_attendance s WHERE s.member_id=m.id AND s.attendance_status='PRESENT'),'excused',(SELECT COUNT(*) FROM mgmp_activity_attendance s WHERE s.member_id=m.id AND s.absence_type='EXCUSED'),'unexcused',(SELECT COUNT(*) FROM mgmp_activity_attendance s WHERE s.member_id=m.id AND s.absence_type='UNEXCUSED'))) ORDER BY m.position,m.full_name),'[]'::jsonb) FROM mgmp_members m WHERE m.group_id=g.id),'activities',(SELECT COALESCE(jsonb_agg(jsonb_build_object('id',a.id,'title',a.title,'activityDate',a.activity_date,'venue',a.venue,'participantCount',a.participant_count,'activeMemberCount',a.active_member_count,'attendanceUrl',a.attendance_url,'documentationUrl',a.documentation_url,'attendance',(SELECT COALESCE(jsonb_agg(jsonb_build_object('memberId',att.member_id,'fullName',att.member_name_snapshot,'schoolName',att.school_name_snapshot,'status',att.attendance_status,'absenceType',att.absence_type,'absenceNote',att.absence_note) ORDER BY att.member_name_snapshot),'[]'::jsonb) FROM mgmp_activity_attendance att WHERE att.activity_id=a.id)) ORDER BY a.activity_date DESC),'[]'::jsonb) FROM mgmp_activities a WHERE a.group_id=g.id)) FROM mgmp_groups g WHERE g.id=$1 AND ($2::boolean OR g.delegated_admin_id=$3)`, c.Param("id"), auth.IsAdminMaster(u), u.ID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		utils.RespondNotFound(c)
		return
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	historyRows, historyErr := h.DB.Pool.Query(c, `SELECT h.event_type,COALESCE(u.nama_instansi,u.username,'Publik'),h.reason,h.created_at FROM mgmp_history h LEFT JOIN users u ON u.id=h.actor_user_id WHERE h.group_id=$1 ORDER BY h.created_at DESC LIMIT 50`, c.Param("id"))
	if historyErr == nil {
		defer historyRows.Close()
		history := []gin.H{}
		for historyRows.Next() {
			var event, actor string
			var reason *string
			var created time.Time
			if historyRows.Scan(&event, &actor, &reason, &created) == nil {
				history = append(history, gin.H{"eventType": event, "actor": actor, "reason": reason, "createdAt": created})
			}
		}
		out["history"] = history
	}
	utils.RespondData(c, http.StatusOK, out)
}

func (h MGMPHandler) RevealAccounts(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	rows, err := h.DB.Pool.Query(c, `SELECT u.id,u.username,COALESCE(m.full_name,u.nama_instansi),l.position,u.initial_password_ciphertext,u.initial_password_nonce FROM mgmp_user_links l JOIN mgmp_groups g ON g.id=l.group_id JOIN users u ON u.id=l.user_id LEFT JOIN mgmp_members m ON m.id=l.member_id WHERE l.group_id=$1 AND ($2::boolean OR g.delegated_admin_id=$3) ORDER BY l.position`, c.Param("id"), auth.IsAdminMaster(u), u.ID)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var uid int
		var username, name, position, cipher, nonce string
		if rows.Scan(&uid, &username, &name, &position, &cipher, &nonce) == nil {
			password, e := utils.DecryptInitialPassword(h.MasterEncryptionKey, cipher, nonce)
			if e != nil {
				password = "(tidak dapat dibuka)"
			}
			items = append(items, gin.H{"userId": uid, "username": username, "name": name, "position": position, "password": password})
		}
	}
	utils.RespondData(c, http.StatusOK, items)
}

func (h MGMPHandler) ResetAccount(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	var allowed bool
	_ = h.DB.Pool.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM mgmp_user_links l JOIN mgmp_groups g ON g.id=l.group_id WHERE l.group_id=$1 AND l.user_id=$2 AND ($3::boolean OR g.delegated_admin_id=$4))`, c.Param("id"), c.Param("userId"), auth.IsAdminMaster(u), u.ID).Scan(&allowed)
	if !allowed {
		utils.RespondForbidden(c)
		return
	}
	password, err := utils.GenerateStrongPassword(10)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	hash, _ := utils.HashPassword(password)
	cipher, nonce, _ := utils.EncryptInitialPassword(h.MasterEncryptionKey, password)
	_, err = h.DB.Pool.Exec(c, `UPDATE users SET password_hash=$1,initial_password_ciphertext=$2,initial_password_nonce=$3,initial_password_set_at=NOW(),token_version=token_version+1,updated_at=NOW() WHERE id=$4`, hash, cipher, nonce, c.Param("userId"))
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	_, _ = h.DB.Pool.Exec(c, `INSERT INTO mgmp_history(group_id,actor_user_id,event_type,details) VALUES($1,$2,'ACCOUNT_PASSWORD_RESET',jsonb_build_object('userId',$3::int))`, c.Param("id"), u.ID, c.Param("userId"))
	utils.RespondData(c, http.StatusOK, gin.H{"password": password})
}

func (h MGMPHandler) SetMemberStatus(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	var req struct {
		Action        string `json:"action"`
		Reason        string `json:"reason"`
		Note          string `json:"note"`
		EffectiveDate string `json:"effectiveDate"`
	}
	if c.ShouldBindJSON(&req) != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	req.Action = strings.ToUpper(strings.TrimSpace(req.Action))
	if req.Action != "DEACTIVATE" && req.Action != "REMOVE" && req.Action != "REACTIVATE" {
		utils.RespondBadRequestMessage(c, "Aksi anggota tidak valid.")
		return
	}
	if req.Action != "REACTIVATE" {
		if strings.TrimSpace(req.Reason) == "" || strings.TrimSpace(req.Note) == "" {
			utils.RespondBadRequestMessage(c, "Alasan dan keterangan wajib diisi.")
			return
		}
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			utils.RespondBadRequestMessage(c, "Tanggal efektif tidak valid.")
			return
		}
	}
	ctx := c.Request.Context()
	tx, err := h.DB.Pool.Begin(ctx)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer tx.Rollback(ctx)
	var gid int64
	var name, oldStatus, position string
	err = tx.QueryRow(ctx, `SELECT m.group_id,m.full_name,m.member_status,m.position FROM mgmp_members m JOIN mgmp_groups g ON g.id=m.group_id WHERE m.id=$1 AND ($2::boolean OR g.delegated_admin_id=$3) FOR UPDATE`, c.Param("memberId"), auth.IsAdminMaster(u), u.ID).Scan(&gid, &name, &oldStatus, &position)
	if errors.Is(err, pgx.ErrNoRows) {
		utils.RespondNotFound(c)
		return
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if req.Action != "REACTIVATE" && (position == "KETUA" || position == "SEKRETARIS_WAKIL") {
		var activeManagers int
		_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM mgmp_user_links WHERE group_id=$1 AND is_active=TRUE`, gid).Scan(&activeManagers)
		if activeManagers <= 1 {
			utils.RespondConflictMessage(c, "Pengurus terakhir tidak dapat dinonaktifkan. Tunjuk pengganti aktif terlebih dahulu.")
			return
		}
	}
	if req.Action == "REACTIVATE" {
		if oldStatus == "AKTIF" {
			utils.RespondConflictMessage(c, "Anggota sudah aktif.")
			return
		}
		_, err = tx.Exec(ctx, `UPDATE mgmp_members SET member_status='AKTIF',deactivation_reason=NULL,deactivation_note=NULL,deactivated_at=NULL,deactivated_by_user_id=NULL,updated_at=NOW() WHERE id=$1`, c.Param("memberId"))
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO mgmp_history(group_id,actor_user_id,event_type,old_status,new_status,details) VALUES($1,$2,'MEMBER_REACTIVATED',$3,'AKTIF',jsonb_build_object('memberId',$4::bigint,'memberName',$5::text))`, gid, u.ID, oldStatus, c.Param("memberId"), name)
		}
	} else {
		newStatus, event := "NONAKTIF", "MEMBER_DEACTIVATED"
		if req.Action == "REMOVE" {
			newStatus = "DIKELUARKAN"
		}
		_, err = tx.Exec(ctx, `UPDATE mgmp_members SET member_status=$1,deactivation_reason=$2,deactivation_note=$3,deactivated_at=$4::date,deactivated_by_user_id=$5,updated_at=NOW() WHERE id=$6`, newStatus, strings.TrimSpace(req.Reason), strings.TrimSpace(req.Note), req.EffectiveDate, u.ID, c.Param("memberId"))
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE mgmp_user_links SET is_active=FALSE,deactivated_at=NOW() WHERE member_id=$1 AND is_active=TRUE`, c.Param("memberId"))
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO mgmp_member_notifications(group_id,member_id,event_type,reason,note,effective_date,actor_user_id) VALUES($1,$2,$3,$4,$5,$6::date,$7)`, gid, c.Param("memberId"), event, strings.TrimSpace(req.Reason), strings.TrimSpace(req.Note), req.EffectiveDate, u.ID)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO mgmp_history(group_id,actor_user_id,event_type,old_status,new_status,reason,details) VALUES($1,$2,$3,$4,$5,$6,jsonb_build_object('memberId',$7::bigint,'memberName',$8::text,'note',$9::text,'effectiveDate',$10::text))`, gid, u.ID, event, oldStatus, newStatus, strings.TrimSpace(req.Reason), c.Param("memberId"), name, strings.TrimSpace(req.Note), req.EffectiveDate)
		}
	}
	if err != nil || tx.Commit(ctx) != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"updated": true})
}

func (h MGMPHandler) SetGroupStatus(c *gin.Context) {
	u, ok := h.requireAdmin(c)
	if !ok {
		return
	}
	var req struct {
		Action        string `json:"action"`
		Reason        string `json:"reason"`
		EffectiveDate string `json:"effectiveDate"`
	}
	if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Reason) == "" {
		utils.RespondBadRequestMessage(c, "Alasan wajib diisi.")
		return
	}
	req.Action = strings.ToUpper(strings.TrimSpace(req.Action))
	if req.Action != "DEACTIVATE" && req.Action != "REACTIVATE" {
		utils.RespondBadRequest(c, "Aksi tidak valid.")
		return
	}
	next := "ACTIVE"
	event := "GROUP_REACTIVATED"
	if req.Action == "DEACTIVATE" {
		next = "INACTIVE"
		event = "GROUP_DEACTIVATED"
		if _, err := time.Parse("2006-01-02", req.EffectiveDate); err != nil {
			utils.RespondBadRequestMessage(c, "Tanggal efektif wajib dan harus valid.")
			return
		}
	}
	var old string
	err := h.DB.Pool.QueryRow(c, `SELECT status::text FROM mgmp_groups WHERE id=$1 AND ($2::boolean OR delegated_admin_id=$3)`, c.Param("id"), auth.IsAdminMaster(u), u.ID).Scan(&old)
	if err != nil {
		utils.RespondNotFound(c)
		return
	}
	if req.Action == "DEACTIVATE" {
		_, err = h.DB.Pool.Exec(c, `UPDATE mgmp_groups SET status='INACTIVE',deactivated_at=$1::date,deactivation_reason=$2,updated_at=NOW() WHERE id=$3`, req.EffectiveDate, req.Reason, c.Param("id"))
	} else {
		_, err = h.DB.Pool.Exec(c, `UPDATE mgmp_groups SET status='ACTIVE',activated_at=NOW(),deactivated_at=NULL,deactivation_reason=NULL,updated_at=NOW() WHERE id=$1`, c.Param("id"))
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	_, _ = h.DB.Pool.Exec(c, `INSERT INTO mgmp_history(group_id,actor_user_id,event_type,old_status,new_status,reason) VALUES($1,$2,$3,$4,$5,$6)`, c.Param("id"), u.ID, event, old, next, req.Reason)
	utils.RespondData(c, http.StatusOK, gin.H{"status": next})
}

func (h MGMPHandler) History(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	where := "TRUE"
	args := []any{}
	if auth.NormalizeRole(u.Role) == auth.RoleMGMPUser {
		gid, err := h.scopedGroupID(c, u)
		if err != nil || gid == nil {
			utils.RespondForbidden(c)
			return
		}
		where = "h.group_id=$1"
		args = append(args, *gid)
	} else if auth.IsAdminMaster(u) {
	} else if auth.IsAdmin(u) {
		where = "EXISTS(SELECT 1 FROM mgmp_groups g WHERE g.id=h.group_id AND g.delegated_admin_id=$1) OR EXISTS(SELECT 1 FROM mgmp_registrations r WHERE r.id=h.registration_id AND r.delegated_admin_id=$1)"
		args = append(args, u.ID)
	} else {
		utils.RespondForbidden(c)
		return
	}
	rows, err := h.DB.Pool.Query(c, `SELECT h.id,h.event_type,COALESCE(g.name,r.name,''),COALESCE(u.nama_instansi,u.username,'Publik'),h.old_status,h.new_status,h.reason,h.details,h.created_at FROM mgmp_history h LEFT JOIN mgmp_groups g ON g.id=h.group_id LEFT JOIN mgmp_registrations r ON r.id=h.registration_id LEFT JOIN users u ON u.id=h.actor_user_id WHERE `+where+` ORDER BY h.created_at DESC LIMIT 500`, args...)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var event, name, actor string
		var oldStatus, newStatus, reason *string
		var details []byte
		var created time.Time
		if rows.Scan(&id, &event, &name, &actor, &oldStatus, &newStatus, &reason, &details, &created) == nil {
			var d any
			_ = json.Unmarshal(details, &d)
			items = append(items, gin.H{"id": id, "eventType": event, "mgmpName": name, "actor": actor, "oldStatus": oldStatus, "newStatus": newStatus, "reason": reason, "details": d, "createdAt": created})
		}
	}
	utils.RespondData(c, http.StatusOK, items)
}

func (h MGMPHandler) Settings(c *gin.Context) {
	u, ok := mgmpCurrentUser(c)
	if !ok {
		return
	}
	if !auth.IsAdminMaster(u) {
		utils.RespondForbidden(c)
		return
	}
	if c.Request.Method == http.MethodGet {
		rows, err := h.DB.Pool.Query(c, `SELECT key,value FROM mgmp_settings ORDER BY key`)
		if err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		defer rows.Close()
		out := map[string]any{}
		for rows.Next() {
			var key string
			var raw []byte
			if rows.Scan(&key, &raw) == nil {
				var value any
				_ = json.Unmarshal(raw, &value)
				out[key] = value
			}
		}
		utils.RespondData(c, http.StatusOK, out)
		return
	}
	var req map[string]any
	if c.ShouldBindJSON(&req) != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer tx.Rollback(c)
	for key, value := range req {
		if key == "email" || strings.Contains(strings.ToLower(key), "smtp") {
			continue
		}
		raw, _ := json.Marshal(value)
		_, err = tx.Exec(c, `INSERT INTO mgmp_settings(key,value,updated_by_user_id,updated_at) VALUES($1,$2,$3,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_by_user_id=EXCLUDED.updated_by_user_id,updated_at=NOW()`, key, raw, u.ID)
		if err != nil {
			utils.RespondInternalServerError(c)
			return
		}
	}
	if tx.Commit(c) != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"updated": true})
}
