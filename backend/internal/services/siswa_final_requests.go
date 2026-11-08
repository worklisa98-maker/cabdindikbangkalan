package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
)

var (
	ErrStudentFinalRequestValidation = errors.New("student final request validation")
	ErrStudentFinalDriveLink         = errors.New("student final drive link invalid")
	ErrStudentFinalRequestNotFound   = errors.New("student final request not found")
)

type StudentFinalRequestInput struct {
	StudentID   int64
	RequestType string
	Reason      string
	SchoolNote  string
	SupportLink string
}

type StudentFinalProcessInput struct {
	ID            int64
	Action        string
	ProcessorNote string
}

type StudentFinalRequestFilters struct {
	SchoolID    *int
	RequestType string
	Status      string
	Page        int
	PageSize    int
	Search      string
}

type StudentFinalRequestItem struct {
	ID            int64      `json:"id"`
	StudentID     int64      `json:"student_id"`
	SchoolID      int        `json:"school_id"`
	SchoolName    string     `json:"school_name"`
	NISN          string     `json:"nisn"`
	NamaSiswa     string     `json:"nama_siswa"`
	Rombel        string     `json:"rombel"`
	TahunLulus    int        `json:"tahun_lulus"`
	RequestType   string     `json:"request_type"`
	Reason        string     `json:"reason"`
	SchoolNote    string     `json:"school_note"`
	SupportLink   string     `json:"support_link"`
	Status        string     `json:"status"`
	SubmittedBy   string     `json:"submitted_by"`
	SubmittedAt   time.Time  `json:"submitted_at"`
	ProcessedBy   string     `json:"processed_by"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
	ProcessorNote string     `json:"processor_note"`
}

type StudentFinalAccessContext struct {
	CanApprove bool `json:"can_approve"`
	CanManage  bool `json:"can_manage"`
}

type DelegatedCabdinUser struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Delegated bool   `json:"delegated"`
}

var nonactiveReasons = map[string]struct{}{
	"Mengundurkan diri": {}, "Pindah sekolah": {}, "Cuti sekolah": {}, "Putus sekolah": {},
	"Meninggal dunia": {}, "Mutasi keluar wilayah Cabdin": {}, "Tidak melanjutkan sekolah": {}, "Lainnya": {},
}

var correctionReasons = map[string]struct{}{
	"Data ganda": {}, "Salah input": {}, "Salah import": {}, "Bukan siswa kelas akhir": {},
	"Salah NISN": {}, "Salah rombel": {}, "Salah tahun lulus": {}, "Salah sekolah": {}, "Lainnya": {},
}

func (s SiswaService) StudentFinalAccess(ctx context.Context, user auth.User) (StudentFinalAccessContext, error) {
	canApprove, err := s.CanApproveStudentFinalRequests(ctx, user)
	return StudentFinalAccessContext{CanApprove: canApprove, CanManage: auth.IsAdminMaster(user)}, err
}

func (s SiswaService) CanApproveStudentFinalRequests(ctx context.Context, user auth.User) (bool, error) {
	role := auth.NormalizeRole(user.Role)
	if role == auth.RoleAdminMaster {
		return true, nil
	}
	if role != auth.RoleAdminCabdin {
		return false, nil
	}
	if ok, _, err := tablesExist(ctx, s.DB, "student_final_request_approvers"); err != nil || !ok {
		return false, err
	}
	var delegated bool
	err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM student_final_request_approvers WHERE user_id = $1)`, user.ID).Scan(&delegated)
	return delegated, err
}

func (s SiswaService) ListStudentFinalDelegates(ctx context.Context, user auth.User) ([]DelegatedCabdinUser, error) {
	if !auth.IsAdminMaster(user) {
		return []DelegatedCabdinUser{}, auth.ErrForbidden
	}
	if ok, _, err := tablesExist(ctx, s.DB, "student_final_request_approvers"); err != nil || !ok {
		return []DelegatedCabdinUser{}, err
	}
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT u.id, u.username, COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS name,
		       d.user_id IS NOT NULL AS delegated
		FROM users u
		LEFT JOIN student_final_request_approvers d ON d.user_id = u.id
		WHERE UPPER(TRIM(u.role::text)) IN ('ADMIN', 'ADMIN_CABDIN')
		ORDER BY name, u.username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DelegatedCabdinUser{}
	for rows.Next() {
		var item DelegatedCabdinUser
		if err := rows.Scan(&item.ID, &item.Username, &item.Name, &item.Delegated); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s SiswaService) SetStudentFinalDelegates(ctx context.Context, user auth.User, userIDs []int) error {
	if !auth.IsAdminMaster(user) {
		return auth.ErrForbidden
	}
	if ok, _, err := tablesExist(ctx, s.DB, "student_final_request_approvers"); err != nil || !ok {
		return err
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM student_final_request_approvers`); err != nil {
		return err
	}
	for _, id := range userIDs {
		if id <= 0 {
			continue
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO student_final_request_approvers (user_id, created_by)
			SELECT id, $2 FROM users WHERE id = $1 AND UPPER(TRIM(role::text)) IN ('ADMIN', 'ADMIN_CABDIN')
			ON CONFLICT (user_id) DO NOTHING
		`, id, user.ID)
		if err != nil {
			return err
		}
		_ = tag
	}
	return tx.Commit(ctx)
}

func (s SiswaService) SubmitStudentFinalRequest(ctx context.Context, user auth.User, input StudentFinalRequestInput) (StudentFinalRequestItem, error) {
	if !auth.IsSchoolAdmin(user) || user.SchoolID == nil {
		return StudentFinalRequestItem{}, auth.ErrForbidden
	}
	reqType := normalizeFinalRequestType(input.RequestType)
	reason := strings.TrimSpace(input.Reason)
	note := strings.TrimSpace(input.SchoolNote)
	link := strings.TrimSpace(input.SupportLink)
	if input.StudentID <= 0 || reqType == "" || reason == "" || note == "" {
		return StudentFinalRequestItem{}, ErrStudentFinalRequestValidation
	}
	if reqType == "NONAKTIF" {
		if _, ok := nonactiveReasons[reason]; !ok {
			return StudentFinalRequestItem{}, ErrStudentFinalRequestValidation
		}
		if !isGoogleDriveFileLink(link) {
			return StudentFinalRequestItem{}, ErrStudentFinalDriveLink
		}
	} else if _, ok := correctionReasons[reason]; !ok {
		return StudentFinalRequestItem{}, ErrStudentFinalRequestValidation
	}

	var schoolID int
	err := s.DB.Pool.QueryRow(ctx, `SELECT school_id FROM class12_students WHERE id = $1`, input.StudentID).Scan(&schoolID)
	if errors.Is(err, pgx.ErrNoRows) {
		return StudentFinalRequestItem{}, ErrStudentFinalRequestNotFound
	}
	if err != nil {
		return StudentFinalRequestItem{}, err
	}
	if schoolID != *user.SchoolID {
		return StudentFinalRequestItem{}, auth.ErrForbidden
	}

	var id int64
	err = s.DB.Pool.QueryRow(ctx, `
		WITH updated AS (
			UPDATE student_final_requests
			SET reason = $3,
			    school_note = $4,
			    support_link = NULLIF(TRIM($5), ''),
			    status = 'PENDING',
			    submitted_by = $6,
			    submitted_at = NOW(),
			    processed_by = NULL,
			    processed_at = NULL,
			    processor_note = NULL,
			    updated_at = NOW()
			WHERE student_id = $1 AND request_type = $2 AND status = 'REJECTED'
			RETURNING id
		), inserted AS (
			INSERT INTO student_final_requests (student_id, school_id, request_type, reason, school_note, support_link, submitted_by)
			SELECT $1, $7, $2, $3, $4, NULLIF(TRIM($5), ''), $6
			WHERE NOT EXISTS (SELECT 1 FROM updated)
			ON CONFLICT (student_id, request_type) WHERE status = 'PENDING' DO UPDATE SET
				reason = EXCLUDED.reason,
				school_note = EXCLUDED.school_note,
				support_link = EXCLUDED.support_link,
				submitted_by = EXCLUDED.submitted_by,
				submitted_at = NOW(),
				updated_at = NOW()
			RETURNING id
		)
		SELECT id FROM updated UNION ALL SELECT id FROM inserted LIMIT 1
	`, input.StudentID, reqType, reason, note, link, user.ID, schoolID).Scan(&id)
	if err != nil {
		return StudentFinalRequestItem{}, err
	}
	return s.GetStudentFinalRequest(ctx, id)
}

func (s SiswaService) ProcessStudentFinalRequest(ctx context.Context, user auth.User, input StudentFinalProcessInput) (StudentFinalRequestItem, error) {
	canApprove, err := s.CanApproveStudentFinalRequests(ctx, user)
	if err != nil || !canApprove {
		if err != nil {
			return StudentFinalRequestItem{}, err
		}
		return StudentFinalRequestItem{}, auth.ErrForbidden
	}
	action := strings.ToUpper(strings.TrimSpace(input.Action))
	status := ""
	if action == "APPROVE" || action == "APPROVED" {
		status = "APPROVED"
	}
	if action == "REJECT" || action == "REJECTED" {
		status = "REJECTED"
	}
	if input.ID <= 0 || status == "" {
		return StudentFinalRequestItem{}, ErrStudentFinalRequestValidation
	}
	tag, err := s.DB.Pool.Exec(ctx, `
		UPDATE student_final_requests
		SET status = $1, processed_by = $2, processed_at = NOW(), processor_note = NULLIF(TRIM($3), ''), updated_at = NOW()
		WHERE id = $4 AND status = 'PENDING'
	`, status, user.ID, input.ProcessorNote, input.ID)
	if err != nil {
		return StudentFinalRequestItem{}, err
	}
	if tag.RowsAffected() == 0 {
		return StudentFinalRequestItem{}, ErrStudentFinalRequestNotFound
	}
	return s.GetStudentFinalRequest(ctx, input.ID)
}

func (s SiswaService) ListStudentFinalRequests(ctx context.Context, user auth.User, filters StudentFinalRequestFilters) ([]StudentFinalRequestItem, int, error) {
	if strings.EqualFold(filters.Status, "PENDING") {
		canApprove, err := s.CanApproveStudentFinalRequests(ctx, user)
		if err != nil || !canApprove {
			if err != nil {
				return nil, 0, err
			}
			return nil, 0, auth.ErrForbidden
		}
	}
	scopeUser := user
	scopeUser.Role = normalizeAdminRole(scopeUser.Role)
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, scopeUser)
	if err != nil {
		return nil, 0, err
	}
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return nil, 0, auth.ErrForbidden
		}
	}
	if !scope.All && len(scope.SchoolIDs) == 0 {
		return []StudentFinalRequestItem{}, 0, nil
	}
	reqType := normalizeFinalRequestType(filters.RequestType)
	status := strings.ToUpper(strings.TrimSpace(filters.Status))

	base := `
		FROM student_final_requests r
		JOIN class12_students c ON c.id = r.student_id
		JOIN data_sekolah ds ON ds.id = r.school_id
		LEFT JOIN users submitter ON submitter.id = r.submitted_by
		LEFT JOIN users processor ON processor.id = r.processed_by
	`
	where := ""
	args := []interface{}{}
	idx := 1
	add := func(cond string, val interface{}) {
		if where == "" {
			where = " WHERE " + cond
		} else {
			where += " AND " + cond
		}
		args = append(args, val)
		idx++
	}
	if !scope.All {
		add(fmt.Sprintf("r.school_id = ANY($%d)", idx), scope.SchoolIDs)
	}
	if filters.SchoolID != nil {
		add(fmt.Sprintf("r.school_id = $%d", idx), *filters.SchoolID)
	}
	if reqType != "" {
		add(fmt.Sprintf("r.request_type = $%d", idx), reqType)
	}
	if status != "" && status != "ALL" {
		add(fmt.Sprintf("r.status = $%d", idx), status)
	}
	if term := strings.ToLower(strings.TrimSpace(filters.Search)); term != "" {
		cond := fmt.Sprintf("(LOWER(c.nisn) LIKE $%d OR LOWER(c.nama_siswa) LIKE $%d OR LOWER(ds.nama_sekolah) LIKE $%d)", idx, idx+1, idx+2)
		if where == "" {
			where = " WHERE " + cond
		} else {
			where += " AND " + cond
		}
		like := "%" + term + "%"
		args = append(args, like, like, like)
		idx += 3
	}
	var total int
	if err := s.DB.Pool.QueryRow(ctx, "SELECT COUNT(*) "+base+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizePage(filters.Page, filters.PageSize)
	query := `
		SELECT r.id, r.student_id, r.school_id, ds.nama_sekolah, COALESCE(c.nisn, ''), COALESCE(c.nama_siswa, ''),
		       COALESCE(c.rombel, ''), COALESCE(c.tahun_lulus, 0), r.request_type, r.reason, r.school_note,
		       COALESCE(r.support_link, ''), r.status, COALESCE(submitter.username, ''), r.submitted_at,
		       COALESCE(processor.username, ''), r.processed_at, COALESCE(r.processor_note, '')
	` + base + where + fmt.Sprintf(" ORDER BY r.submitted_at DESC, r.id DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []StudentFinalRequestItem{}
	for rows.Next() {
		item, scanErr := scanStudentFinalRequest(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s SiswaService) GetStudentFinalRequest(ctx context.Context, id int64) (StudentFinalRequestItem, error) {
	row := s.DB.Pool.QueryRow(ctx, `
		SELECT r.id, r.student_id, r.school_id, ds.nama_sekolah, COALESCE(c.nisn, ''), COALESCE(c.nama_siswa, ''),
		       COALESCE(c.rombel, ''), COALESCE(c.tahun_lulus, 0), r.request_type, r.reason, r.school_note,
		       COALESCE(r.support_link, ''), r.status, COALESCE(submitter.username, ''), r.submitted_at,
		       COALESCE(processor.username, ''), r.processed_at, COALESCE(r.processor_note, '')
		FROM student_final_requests r
		JOIN class12_students c ON c.id = r.student_id
		JOIN data_sekolah ds ON ds.id = r.school_id
		LEFT JOIN users submitter ON submitter.id = r.submitted_by
		LEFT JOIN users processor ON processor.id = r.processed_by
		WHERE r.id = $1
	`, id)
	item, err := scanStudentFinalRequest(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return StudentFinalRequestItem{}, ErrStudentFinalRequestNotFound
	}
	return item, err
}

type studentFinalScanner interface {
	Scan(dest ...interface{}) error
}

func scanStudentFinalRequest(row studentFinalScanner) (StudentFinalRequestItem, error) {
	var item StudentFinalRequestItem
	err := row.Scan(&item.ID, &item.StudentID, &item.SchoolID, &item.SchoolName, &item.NISN, &item.NamaSiswa, &item.Rombel, &item.TahunLulus, &item.RequestType, &item.Reason, &item.SchoolNote, &item.SupportLink, &item.Status, &item.SubmittedBy, &item.SubmittedAt, &item.ProcessedBy, &item.ProcessedAt, &item.ProcessorNote)
	return item, err
}

func normalizeFinalRequestType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "NONAKTIF":
		return "NONAKTIF"
	case "KOREKSI":
		return "KOREKSI"
	default:
		return ""
	}
}

func isGoogleDriveFileLink(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Host)
	path := parsed.EscapedPath()
	if host == "drive.google.com" && strings.HasPrefix(path, "/drive/folders/") {
		return false
	}
	if host == "drive.google.com" && strings.HasPrefix(path, "/file/d/") {
		return true
	}
	if (host == "docs.google.com") && (strings.HasPrefix(path, "/document/d/") || strings.HasPrefix(path, "/spreadsheets/d/") || strings.HasPrefix(path, "/presentation/d/")) {
		return true
	}
	return false
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
