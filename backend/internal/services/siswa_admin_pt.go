package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/parsers"
)

var adminPTWhitespace = regexp.MustCompile(`\s+`)

type AdminPTOfferInput struct {
	ID                 int64  `json:"id"`
	SchoolID           int    `json:"schoolId"`
	GraduationYear     int    `json:"graduationYear"`
	Path               string `json:"path"`
	StudentName        string `json:"studentName"`
	NISN               string `json:"nisn"`
	CampusName         string `json:"campusName"`
	StudyProgram       string `json:"studyProgram"`
	AnnouncementStatus string `json:"announcementStatus"`
}

type AdminPTOffer struct {
	ID                 int64      `json:"id"`
	SchoolID           int        `json:"schoolId"`
	SchoolName         string     `json:"schoolName"`
	GraduationYear     int        `json:"graduationYear"`
	Path               string     `json:"path"`
	StudentName        string     `json:"studentName"`
	NISN               string     `json:"nisn"`
	CampusName         string     `json:"campusName"`
	StudyProgram       string     `json:"studyProgram"`
	AnnouncementStatus string     `json:"announcementStatus"`
	SourceRole         string     `json:"sourceRole"`
	SourceUserID       *int       `json:"sourceUserId,omitempty"`
	ConflictStatus     string     `json:"conflictStatus"`
	SchoolConfirmedAt  *time.Time `json:"schoolConfirmedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type AdminPTImportResult struct {
	TotalRows        int                    `json:"totalRows"`
	Accepted         int                    `json:"accepted"`
	Rejected         int                    `json:"rejected"`
	ImportBatchID    *int                   `json:"importBatchId,omitempty"`
	RejectedReportID string                 `json:"rejectedReportId,omitempty"`
	RejectedFilename string                 `json:"rejectedFilename,omitempty"`
	Rejections       []SiswaImportRejection `json:"rejections"`
}

type PTOfficialCountInput struct {
	GraduationYear  int    `json:"graduationYear"`
	Path            string `json:"path"`
	OfficialCount   int    `json:"officialCount"`
	SourceNote      string `json:"sourceNote"`
	SourceUpdatedAt string `json:"sourceUpdatedAt"`
}

type PTOfficialCoverage struct {
	ID              int64     `json:"id"`
	GraduationYear  int       `json:"graduationYear"`
	Path            string    `json:"path"`
	OfficialCount   int       `json:"officialCount"`
	DetailCount     int       `json:"detailCount"`
	MissingCount    int       `json:"missingCount"`
	CoveragePercent float64   `json:"coveragePercent"`
	ExceedsOfficial bool      `json:"exceedsOfficial"`
	SourceNote      string    `json:"sourceNote"`
	SourceUpdatedAt time.Time `json:"sourceUpdatedAt"`
	UpdatedBy       *int      `json:"updatedBy,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func applyPTOfficialCoverageCalculation(item *PTOfficialCoverage) {
	item.MissingCount = item.OfficialCount - item.DetailCount
	if item.MissingCount < 0 {
		item.MissingCount = 0
	}
	item.ExceedsOfficial = item.DetailCount > item.OfficialCount
	if item.OfficialCount > 0 {
		item.CoveragePercent = float64(item.DetailCount) * 100 / float64(item.OfficialCount)
	}
}

func normalizeAdminPTStudentName(value string) (string, string) {
	display := adminPTWhitespace.ReplaceAllString(strings.TrimSpace(value), " ")
	return display, strings.ToLower(display)
}

func adminPTSourceRole(user auth.User) (string, error) {
	switch auth.NormalizeRole(user.Role) {
	case auth.RoleAdminCabdin:
		return "ADMIN", nil
	case auth.RoleAdminMaster:
		return "ADMIN_MASTER", nil
	default:
		return "", auth.ErrForbidden
	}
}

func (s SiswaService) ImportAdminPTOffers(ctx context.Context, user auth.User, filename string, rows []parsers.AdminPTOfferRow) (AdminPTImportResult, error) {
	result := AdminPTImportResult{TotalRows: len(rows), Rejections: make([]SiswaImportRejection, 0)}
	sourceRole, err := adminPTSourceRole(user)
	if err != nil {
		return result, err
	}
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return result, err
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	schoolByKey, err := loadAdminPTSchools(ctx, tx, scope)
	if err != nil {
		return result, err
	}
	campusValues := make([]string, 0, len(rows))
	for _, row := range rows {
		campusValues = append(campusValues, row.NamaPT)
	}
	campusByKey, err := fetchRefKampusByKeys(ctx, tx, collectRefKampusKeys(campusValues...))
	if err != nil {
		return result, err
	}

	var uploadID *int
	var reportSchoolID int
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"Tahun_Lulus": row.TahunLulus, "Jalur": row.Jalur, "Nama_Siswa": row.NamaSiswa,
					"NISN": row.NISN, "Nama_Sekolah": row.NamaSekolah, "Nama_PT": row.NamaPT,
					"Program_Studi":     row.ProgramStudi,
					"Status_Pengumuman": row.Status,
				},
			})
		}
		schoolName := normalizeAdminPTLookupKey(row.NamaSekolah)
		if schoolName == "" {
			reject("Nama sekolah wajib diisi.")
			continue
		}
		school, ok := schoolByKey[schoolName]
		if !ok {
			reject("Nama sekolah tidak dikenali atau berada di luar cakupan kewenangan admin.")
			continue
		}
		if reportSchoolID == 0 {
			reportSchoolID = school.ID
		}
		year, ok := parseYearStrict(row.TahunLulus)
		if !ok || year <= 0 {
			reject("Tahun lulus wajib diisi dan harus berupa tahun yang valid.")
			continue
		}
		if uploadID == nil {
			var id int
			err = tx.QueryRow(ctx, `
				INSERT INTO uploads (school_id, user_id, type, year, storage_key, original_filename, status)
				VALUES ($1,$2,'ACCEPTANCE',$3,$4,$5,'PROCESSING') RETURNING id
			`, school.ID, user.ID, year, "admin-pt/"+strings.TrimSpace(filename), strings.TrimSpace(filename)).Scan(&id)
			if err != nil {
				return result, err
			}
			uploadID = &id
			result.ImportBatchID = &id
		}
		studentName, normalizedName := normalizeAdminPTStudentName(row.NamaSiswa)
		if studentName == "" {
			reject("Nama siswa wajib diisi.")
			continue
		}
		nisn := strings.TrimSpace(row.NISN)
		if nisn != "" && !isNumericOnly(nisn) {
			reject("NISN harus berupa angka apabila diisi.")
			continue
		}
		path, ok := normalizeJalurPenerimaan(row.Jalur)
		if !ok {
			reject("Jalur penerimaan tidak valid.")
			continue
		}
		campus, ok := resolveRefKampusStrict(row.NamaPT, campusByKey)
		if !ok {
			reject(strictRefKampusRejectReason)
			continue
		}
		program := adminPTWhitespace.ReplaceAllString(strings.TrimSpace(row.ProgramStudi), " ")
		if program == "" {
			reject("Program studi/jurusan wajib diisi.")
			continue
		}
		announcementStatus, validStatus := normalizeAcceptanceAnnouncementStatus(row.Status)
		if !validStatus || announcementStatus == "" {
			reject("Status_Pengumuman wajib diisi dengan DITERIMA atau DITOLAK.")
			continue
		}
		input := AdminPTOfferInput{
			SchoolID: school.ID, GraduationYear: year, Path: path, StudentName: studentName,
			NISN: nisn, CampusName: normalizeRefKampusName(campus.Nama), StudyProgram: program,
			AnnouncementStatus: announcementStatus,
		}
		_, insertErr := s.createAdminPTOfferTx(ctx, tx, user, sourceRole, input, campus.ID, uploadID)
		if insertErr != nil {
			if duplicate := new(adminPTDuplicateError); errors.As(insertErr, &duplicate) {
				reject(duplicate.Error())
				continue
			}
			return result, insertErr
		}
		_ = normalizedName
		result.Accepted++
	}
	if uploadID != nil {
		uploadRejections := make([]UploadRejection, 0, len(result.Rejections))
		for _, rejection := range result.Rejections {
			uploadRejections = append(uploadRejections, UploadRejection{RowNumber: rejection.RowNumber, Reason: rejection.Reason, Data: rejection.Data})
		}
		if err = insertRejectionsTx(ctx, tx, *uploadID, uploadRejections); err != nil {
			return result, err
		}
		status := "SUCCESS"
		_, err = tx.Exec(ctx, `UPDATE uploads SET status=$1, updated_at=NOW() WHERE id=$2`, status, *uploadID)
		if err != nil {
			return result, err
		}
	}
	if len(result.Rejections) > 0 && reportSchoolID > 0 {
		reportID, reportErr := s.insertRejectedReportTx(ctx, tx, reportSchoolID, "ADMIN_PT_OFFERS",
			"rejected-admin-pengumuman-pt.xlsx", parsers.AdminPTOfferTemplateHeaders, result.Rejections)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-admin-pengumuman-pt.xlsx"
	}
	result.Rejected = len(result.Rejections)
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

type adminPTSchool struct {
	ID   int
	Name string
}

func loadAdminPTSchools(ctx context.Context, tx pgx.Tx, scope auth.MultiSchoolScope) (map[string]adminPTSchool, error) {
	query := `SELECT id, nama_sekolah FROM data_sekolah`
	args := []any{}
	if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return map[string]adminPTSchool{}, nil
		}
		query += ` WHERE id = ANY($1)`
		args = append(args, scope.SchoolIDs)
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]adminPTSchool{}
	for rows.Next() {
		var item adminPTSchool
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		result[normalizeAdminPTLookupKey(item.Name)] = item
	}
	return result, rows.Err()
}

func normalizeAdminPTLookupKey(value string) string {
	return strings.ToLower(adminPTWhitespace.ReplaceAllString(strings.TrimSpace(value), " "))
}

type adminPTDuplicateError struct{ message string }

func (e *adminPTDuplicateError) Error() string { return e.message }

func adminPTExistingMessage(studentName string, year int, path, schoolName, sourceRole, campus, program string, createdAt time.Time) string {
	action := "Entri Admin tidak disimpan."
	if sourceRole == "ADMIN" || sourceRole == "ADMIN_MASTER" {
		action = "Entri Admin tidak disimpan; gunakan fitur edit pada entri yang sudah ada."
	}
	return fmt.Sprintf(
		"Data %s dari %s tahun lulus %d untuk jalur %s sudah diinput oleh %s pada %s. Existing: %s - %s. %s",
		studentName,
		schoolName,
		year,
		path,
		adminPTSourceLabel(sourceRole),
		createdAt.Format("02 Jan 2006 15:04"),
		campus,
		program,
		action,
	)
}

func (s SiswaService) createAdminPTOfferTx(ctx context.Context, tx pgx.Tx, user auth.User, sourceRole string, input AdminPTOfferInput, campusID int, uploadID *int) (AdminPTOffer, error) {
	studentName, normalizedName := normalizeAdminPTStudentName(input.StudentName)
	// Serialize writes for the same student/year/path so concurrent manual and
	// import requests cannot create competing Admin records.
	identityLock := fmt.Sprintf("%d|%d|%s|%s", input.SchoolID, input.GraduationYear, normalizedName, strings.ToUpper(strings.TrimSpace(input.Path)))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, identityLock); err != nil {
		return AdminPTOffer{}, err
	}
	var existingSource, existingSchool, existingCampus, existingProgram string
	var existingCreatedAt time.Time
	existingErr := tx.QueryRow(ctx, `
		SELECT offer.source_role, school.nama_sekolah, offer.kampus, offer.prodi, offer.created_at
		FROM outcome_pt_offers offer
		JOIN data_sekolah school ON school.id=offer.school_id
		WHERE offer.school_id=$1
		  AND offer.tahun_lulus=$2
		  AND offer.normalized_student_name=$3
		  AND UPPER(TRIM(offer.jalur))=UPPER(TRIM($4))
		  AND UPPER(TRIM(offer.kampus))=UPPER(TRIM($5))
		  AND UPPER(TRIM(offer.prodi))=UPPER(TRIM($6))
		  AND offer.deleted_at IS NULL
		ORDER BY offer.created_at ASC, offer.id ASC
		FOR UPDATE OF offer
		LIMIT 1
	`, input.SchoolID, input.GraduationYear, normalizedName, input.Path, input.CampusName, input.StudyProgram).Scan(
		&existingSource, &existingSchool, &existingCampus, &existingProgram, &existingCreatedAt,
	)
	if existingErr == nil {
		return AdminPTOffer{}, &adminPTDuplicateError{message: adminPTExistingMessage(
			studentName, input.GraduationYear, input.Path, existingSchool, existingSource,
			existingCampus, existingProgram, existingCreatedAt,
		)}
	}
	if !errors.Is(existingErr, pgx.ErrNoRows) {
		return AdminPTOffer{}, existingErr
	}
	var nisn any
	if strings.TrimSpace(input.NISN) != "" {
		nisn = strings.TrimSpace(input.NISN)
	}
	var item AdminPTOffer
	err := tx.QueryRow(ctx, `
		INSERT INTO outcome_pt_offers (
			school_id,tahun_lulus,nisn,student_name,normalized_student_name,jalur,kampus,prodi,
			tahun_diterima,announcement_status,source,source_role,source_user_id,ref_kampus_id,import_batch_id
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$2,$9,'ADMIN_ENTRY',$10,$11,$12,$13)
		ON CONFLICT DO NOTHING
		RETURNING id,school_id,tahun_lulus,jalur,student_name,COALESCE(nisn,''),kampus,prodi,
		          announcement_status,source_role,source_user_id,conflict_status,school_confirmed_at,created_at,updated_at
	`, input.SchoolID, input.GraduationYear, nisn, studentName, normalizedName, input.Path,
		input.CampusName, input.StudyProgram, input.AnnouncementStatus, sourceRole, user.ID, campusID, uploadID).Scan(
		&item.ID, &item.SchoolID, &item.GraduationYear, &item.Path, &item.StudentName, &item.NISN,
		&item.CampusName, &item.StudyProgram, &item.AnnouncementStatus, &item.SourceRole, &item.SourceUserID,
		&item.ConflictStatus, &item.SchoolConfirmedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminPTOffer{}, &adminPTDuplicateError{message: fmt.Sprintf(
			"Data %s tahun lulus %d untuk jalur %s sudah ada. Entri Admin tidak disimpan.",
			studentName, input.GraduationYear, input.Path)}
	}
	if err != nil {
		return AdminPTOffer{}, err
	}
	if err := s.markAdminPTConflictsTx(ctx, tx, item.ID, input.SchoolID, input.GraduationYear, normalizedName, input.Path, input.NISN); err != nil {
		return AdminPTOffer{}, err
	}
	if err := s.reconcilePTIdentityTx(ctx, tx, input.SchoolID, input.GraduationYear, normalizedName); err != nil {
		return AdminPTOffer{}, err
	}
	after, _ := json.Marshal(item)
	_, err = tx.Exec(ctx, `INSERT INTO pt_acceptance_offer_audit (offer_id,action,actor_user_id,after_json) VALUES ($1,'CREATE',$2,$3)`, item.ID, user.ID, after)
	return item, err
}

func adminPTSourceLabel(source string) string {
	switch strings.ToUpper(strings.TrimSpace(source)) {
	case "ADMIN_MASTER":
		return "Admin Master"
	case "ADMIN":
		return "Admin Cabdin"
	case "SCHOOL_ADMIN":
		return "sekolah"
	default:
		return "sistem"
	}
}

func (s SiswaService) markAdminPTConflictsTx(ctx context.Context, tx pgx.Tx, offerID int64, schoolID, year int, normalizedName, path, nisn string) error {
	var samePathDifferent, multiPath bool
	err := tx.QueryRow(ctx, `
		SELECT
		  EXISTS(SELECT 1 FROM outcome_pt_offers WHERE id<>$1 AND school_id=$2 AND tahun_lulus=$3
		    AND normalized_student_name=$4 AND UPPER(TRIM(jalur))=UPPER(TRIM($5)) AND announcement_status='DITERIMA' AND deleted_at IS NULL),
		  EXISTS(SELECT 1 FROM outcome_pt_offers WHERE id<>$1 AND school_id=$2 AND tahun_lulus=$3
		    AND normalized_student_name=$4 AND UPPER(TRIM(jalur))<>UPPER(TRIM($5)) AND announcement_status='DITERIMA' AND deleted_at IS NULL)
	`, offerID, schoolID, year, normalizedName, path).Scan(&samePathDifferent, &multiPath)
	if err != nil {
		return err
	}
	status := "NONE"
	if samePathDifferent {
		status = "SAME_PATH_DIFFERENT_DESTINATION"
	} else if multiPath {
		status = "MULTI_OFFER"
	}
	if strings.TrimSpace(nisn) != "" {
		var nisnConflict bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM outcome_pt_offers WHERE id<>$1 AND school_id=$2 AND tahun_lulus=$3 AND normalized_student_name=$4 AND nisn IS NOT NULL AND nisn<>$5 AND deleted_at IS NULL)`,
			offerID, schoolID, year, normalizedName, strings.TrimSpace(nisn)).Scan(&nisnConflict)
		if err != nil {
			return err
		}
		if nisnConflict {
			status = "NISN_CONFLICT"
		}
		var crossOutcome bool
		err = tx.QueryRow(ctx, `
			SELECT
			  EXISTS(SELECT 1 FROM outcome_work_details WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3)
			  OR EXISTS(SELECT 1 FROM outcome_business_details WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3)
		`, schoolID, year, strings.TrimSpace(nisn)).Scan(&crossOutcome)
		if err != nil {
			return err
		}
		if crossOutcome {
			status = "CROSS_OUTCOME"
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE outcome_pt_offers
		SET conflict_status=CASE WHEN announcement_status='DITERIMA' THEN $1 ELSE 'NONE' END,
		    conflict_detail=NULL,updated_at=NOW()
		WHERE school_id=$2 AND tahun_lulus=$3 AND normalized_student_name=$4 AND deleted_at IS NULL
	`, status, schoolID, year, normalizedName)
	return err
}

func (s SiswaService) CreateAdminPTOffer(ctx context.Context, user auth.User, input AdminPTOfferInput) (AdminPTOffer, error) {
	sourceRole, err := adminPTSourceRole(user)
	if err != nil {
		return AdminPTOffer{}, err
	}
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil || auth.EnsureSchoolInScope(scope, input.SchoolID) != nil {
		return AdminPTOffer{}, auth.ErrForbidden
	}
	if input.GraduationYear <= 0 {
		return AdminPTOffer{}, errors.New("INVALID_YEAR")
	}
	input.Path, _ = normalizeJalurPenerimaan(input.Path)
	input.StudentName, _ = normalizeAdminPTStudentName(input.StudentName)
	if input.Path == "" || input.StudentName == "" || strings.TrimSpace(input.CampusName) == "" || strings.TrimSpace(input.StudyProgram) == "" {
		return AdminPTOffer{}, errors.New("INVALID_PAYLOAD")
	}
	if input.NISN != "" && !isNumericOnly(input.NISN) {
		return AdminPTOffer{}, errors.New("INVALID_NISN")
	}
	input.AnnouncementStatus, _ = normalizeAcceptanceAnnouncementStatus(input.AnnouncementStatus)
	if input.AnnouncementStatus == "" {
		return AdminPTOffer{}, errors.New("INVALID_ANNOUNCEMENT_STATUS")
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return AdminPTOffer{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	campuses, err := fetchRefKampusByKeys(ctx, tx, collectRefKampusKeys(input.CampusName))
	if err != nil {
		return AdminPTOffer{}, err
	}
	campus, ok := resolveRefKampusStrict(input.CampusName, campuses)
	if !ok {
		return AdminPTOffer{}, errors.New("UNKNOWN_CAMPUS")
	}
	input.CampusName = normalizeRefKampusName(campus.Nama)
	item, err := s.createAdminPTOfferTx(ctx, tx, user, sourceRole, input, campus.ID, nil)
	if err != nil {
		return AdminPTOffer{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AdminPTOffer{}, err
	}
	return item, nil
}

func (s SiswaService) ListAdminPTOffers(ctx context.Context, user auth.User, year int, limit int) ([]AdminPTOffer, error) {
	if _, err := adminPTSourceRole(user); err != nil {
		return nil, err
	}
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := `WHERE offer.source_role IN ('ADMIN','ADMIN_MASTER') AND offer.deleted_at IS NULL`
	args := []any{}
	index := 1
	if year > 0 {
		where += fmt.Sprintf(" AND offer.tahun_lulus=$%d", index)
		args = append(args, year)
		index++
	}
	if !scope.All {
		where += fmt.Sprintf(" AND offer.school_id=ANY($%d)", index)
		args = append(args, scope.SchoolIDs)
		index++
	}
	args = append(args, limit)
	rows, err := s.DB.Pool.Query(ctx, fmt.Sprintf(`
		SELECT offer.id,offer.school_id,school.nama_sekolah,offer.tahun_lulus,offer.jalur,
		  offer.student_name,COALESCE(offer.nisn,''),offer.kampus,offer.prodi,offer.announcement_status,offer.source_role,
		  offer.source_user_id,offer.conflict_status,offer.school_confirmed_at,offer.created_at,offer.updated_at
		FROM outcome_pt_offers offer JOIN data_sekolah school ON school.id=offer.school_id
		%s ORDER BY offer.updated_at DESC LIMIT $%d
	`, where, index), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]AdminPTOffer, 0)
	for rows.Next() {
		var item AdminPTOffer
		if err := rows.Scan(&item.ID, &item.SchoolID, &item.SchoolName, &item.GraduationYear, &item.Path,
			&item.StudentName, &item.NISN, &item.CampusName, &item.StudyProgram, &item.AnnouncementStatus, &item.SourceRole,
			&item.SourceUserID, &item.ConflictStatus, &item.SchoolConfirmedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s SiswaService) UpdateAdminPTOffer(ctx context.Context, user auth.User, offerID int64, input AdminPTOfferInput) (AdminPTOffer, error) {
	if _, err := adminPTSourceRole(user); err != nil {
		return AdminPTOffer{}, err
	}
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil || auth.EnsureSchoolInScope(scope, input.SchoolID) != nil {
		return AdminPTOffer{}, auth.ErrForbidden
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return AdminPTOffer{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var before AdminPTOffer
	err = tx.QueryRow(ctx, `
		SELECT id,school_id,tahun_lulus,jalur,student_name,COALESCE(nisn,''),kampus,prodi,
		  announcement_status,source_role,source_user_id,conflict_status,school_confirmed_at,created_at,updated_at
		FROM outcome_pt_offers WHERE id=$1 AND source_role IN ('ADMIN','ADMIN_MASTER') AND deleted_at IS NULL
	`, offerID).Scan(&before.ID, &before.SchoolID, &before.GraduationYear, &before.Path, &before.StudentName,
		&before.NISN, &before.CampusName, &before.StudyProgram, &before.AnnouncementStatus, &before.SourceRole, &before.SourceUserID,
		&before.ConflictStatus, &before.SchoolConfirmedAt, &before.CreatedAt, &before.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminPTOffer{}, auth.ErrForbidden
	}
	if err != nil {
		return AdminPTOffer{}, err
	}
	if auth.EnsureSchoolInScope(scope, before.SchoolID) != nil {
		return AdminPTOffer{}, auth.ErrForbidden
	}
	role := auth.NormalizeRole(user.Role)
	if role == auth.RoleAdminCabdin &&
		(before.SourceRole != "ADMIN" || before.SourceUserID == nil || *before.SourceUserID != user.ID) {
		return AdminPTOffer{}, auth.ErrForbidden
	}
	if before.SchoolConfirmedAt != nil {
		return AdminPTOffer{}, errors.New("Data sudah dikonfirmasi sekolah dan tidak dapat diedit langsung. Gunakan alur koreksi/validasi.")
	}
	var reconciliationProtected bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
		  SELECT 1 FROM pt_offer_reconciliations
		  WHERE offer_id=$1 AND reconciliation_status IN ('AUTO_FINAL','EXISTING_FINAL')
		)
	`, offerID).Scan(&reconciliationProtected); err != nil {
		return AdminPTOffer{}, err
	}
	if reconciliationProtected {
		return AdminPTOffer{}, errors.New("Data sudah terikat keputusan final dan tidak dapat diedit langsung.")
	}
	if strings.TrimSpace(before.NISN) != "" {
		var finalized bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
			  SELECT 1 FROM student_outcomes
			  WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3 AND finalized_at IS NOT NULL
			)
		`, before.SchoolID, before.GraduationYear, before.NISN).Scan(&finalized); err != nil {
			return AdminPTOffer{}, err
		}
		if finalized {
			return AdminPTOffer{}, errors.New("Data sudah digunakan dalam status final dan tidak dapat diedit langsung.")
		}
	}
	path, ok := normalizeJalurPenerimaan(input.Path)
	name, normalizedName := normalizeAdminPTStudentName(input.StudentName)
	if !ok || name == "" || input.GraduationYear <= 0 || strings.TrimSpace(input.StudyProgram) == "" {
		return AdminPTOffer{}, errors.New("INVALID_PAYLOAD")
	}
	if input.NISN != "" && !isNumericOnly(input.NISN) {
		return AdminPTOffer{}, errors.New("INVALID_NISN")
	}
	input.AnnouncementStatus, _ = normalizeAcceptanceAnnouncementStatus(input.AnnouncementStatus)
	if input.AnnouncementStatus == "" {
		return AdminPTOffer{}, errors.New("INVALID_ANNOUNCEMENT_STATUS")
	}
	identityLock := fmt.Sprintf("%d|%d|%s|%s", input.SchoolID, input.GraduationYear, normalizedName, strings.ToUpper(path))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, identityLock); err != nil {
		return AdminPTOffer{}, err
	}
	campuses, err := fetchRefKampusByKeys(ctx, tx, collectRefKampusKeys(input.CampusName))
	if err != nil {
		return AdminPTOffer{}, err
	}
	campus, ok := resolveRefKampusStrict(input.CampusName, campuses)
	if !ok {
		return AdminPTOffer{}, errors.New("UNKNOWN_CAMPUS")
	}
	var nisn any
	if strings.TrimSpace(input.NISN) != "" {
		nisn = strings.TrimSpace(input.NISN)
	}
	var item AdminPTOffer
	err = tx.QueryRow(ctx, `
		UPDATE outcome_pt_offers SET school_id=$2,tahun_lulus=$3,jalur=$4,student_name=$5,
		  normalized_student_name=$6,nisn=$7,kampus=$8,prodi=$9,ref_kampus_id=$10,
		  announcement_status=$11,conflict_status='NONE',conflict_detail=NULL,updated_at=NOW()
		WHERE id=$1
		RETURNING id,school_id,tahun_lulus,jalur,student_name,COALESCE(nisn,''),kampus,prodi,
		  announcement_status,source_role,source_user_id,conflict_status,school_confirmed_at,created_at,updated_at
	`, offerID, input.SchoolID, input.GraduationYear, path, name, normalizedName, nisn,
		normalizeRefKampusName(campus.Nama), strings.TrimSpace(input.StudyProgram), campus.ID, input.AnnouncementStatus).
		Scan(&item.ID, &item.SchoolID, &item.GraduationYear, &item.Path, &item.StudentName, &item.NISN,
			&item.CampusName, &item.StudyProgram, &item.AnnouncementStatus, &item.SourceRole, &item.SourceUserID, &item.ConflictStatus,
			&item.SchoolConfirmedAt, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return AdminPTOffer{}, err
	}
	if err := s.markAdminPTConflictsTx(ctx, tx, item.ID, item.SchoolID, item.GraduationYear, normalizedName, path, item.NISN); err != nil {
		return AdminPTOffer{}, err
	}
	oldNormalizedName := strings.ToLower(strings.Join(strings.Fields(before.StudentName), " "))
	if err := s.reconcilePTIdentityTx(ctx, tx, item.SchoolID, item.GraduationYear, normalizedName); err != nil {
		return AdminPTOffer{}, err
	}
	if before.SchoolID != item.SchoolID || before.GraduationYear != item.GraduationYear || oldNormalizedName != normalizedName {
		if err := s.reconcilePTIdentityTx(ctx, tx, before.SchoolID, before.GraduationYear, oldNormalizedName); err != nil {
			return AdminPTOffer{}, err
		}
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(item)
	if _, err = tx.Exec(ctx, `INSERT INTO pt_acceptance_offer_audit(offer_id,action,actor_user_id,before_json,after_json) VALUES($1,'UPDATE',$2,$3,$4)`, offerID, user.ID, beforeJSON, afterJSON); err != nil {
		return AdminPTOffer{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AdminPTOffer{}, err
	}
	return item, nil
}

func (s SiswaService) UpsertPTOfficialCount(ctx context.Context, user auth.User, input PTOfficialCountInput) (PTOfficialCoverage, error) {
	if _, err := adminPTSourceRole(user); err != nil {
		return PTOfficialCoverage{}, err
	}
	path, ok := normalizeJalurPenerimaan(input.Path)
	if !ok || input.GraduationYear <= 0 || input.OfficialCount < 0 {
		return PTOfficialCoverage{}, errors.New("INVALID_PAYLOAD")
	}
	date := time.Now()
	if strings.TrimSpace(input.SourceUpdatedAt) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(input.SourceUpdatedAt))
		if err != nil {
			return PTOfficialCoverage{}, errors.New("INVALID_DATE")
		}
		date = parsed
	}
	_, err := s.DB.Pool.Exec(ctx, `
		INSERT INTO pt_official_acceptance_counts (tahun_lulus,jalur,official_count,source_note,source_updated_at,created_by,updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$6)
		ON CONFLICT (tahun_lulus,jalur) DO UPDATE SET official_count=EXCLUDED.official_count,
		  source_note=EXCLUDED.source_note,source_updated_at=EXCLUDED.source_updated_at,
		  updated_by=EXCLUDED.updated_by,updated_at=NOW()
	`, input.GraduationYear, path, input.OfficialCount, strings.TrimSpace(input.SourceNote), date, user.ID)
	if err != nil {
		return PTOfficialCoverage{}, err
	}
	items, err := s.ListPTOfficialCoverage(ctx, user, input.GraduationYear)
	if err != nil {
		return PTOfficialCoverage{}, err
	}
	for _, item := range items {
		if item.Path == path {
			return item, nil
		}
	}
	return PTOfficialCoverage{}, pgx.ErrNoRows
}

func (s SiswaService) ListPTOfficialCoverage(ctx context.Context, user auth.User, year int) ([]PTOfficialCoverage, error) {
	if auth.NormalizeRole(user.Role) == "" {
		return nil, auth.ErrForbidden
	}
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return nil, err
	}
	where := ""
	args := []any{year}
	if !scope.All {
		where = " AND offer.school_id = ANY($2)"
		args = append(args, scope.SchoolIDs)
	}
	rows, err := s.DB.Pool.Query(ctx, fmt.Sprintf(`
		SELECT c.id,c.tahun_lulus,c.jalur,c.official_count,
		  COUNT(DISTINCT (
		    offer.school_id::text || '|' || offer.tahun_lulus::text || '|' ||
		    COALESCE(NULLIF(TRIM(offer.nisn),''),
		      'NAME:' || LOWER(REGEXP_REPLACE(TRIM(COALESCE(offer.student_name,'')),'\s+',' ','g'))) ||
		    '|' || UPPER(TRIM(offer.jalur))
		  )) FILTER (WHERE offer.announcement_status='DITERIMA' AND offer.deleted_at IS NULL)::int AS detail_count,
		  c.source_note,c.source_updated_at,c.updated_by,c.updated_at
		FROM pt_official_acceptance_counts c
		LEFT JOIN outcome_pt_offers offer ON offer.tahun_lulus=c.tahun_lulus AND UPPER(TRIM(offer.jalur))=UPPER(TRIM(c.jalur)) %s
		WHERE ($1=0 OR c.tahun_lulus=$1)
		GROUP BY c.id ORDER BY c.tahun_lulus DESC,c.jalur
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]PTOfficialCoverage, 0)
	for rows.Next() {
		var item PTOfficialCoverage
		if err := rows.Scan(&item.ID, &item.GraduationYear, &item.Path, &item.OfficialCount, &item.DetailCount, &item.SourceNote, &item.SourceUpdatedAt, &item.UpdatedBy, &item.UpdatedAt); err != nil {
			return nil, err
		}
		applyPTOfficialCoverageCalculation(&item)
		result = append(result, item)
	}
	return result, rows.Err()
}

func atoiOrZero(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}
