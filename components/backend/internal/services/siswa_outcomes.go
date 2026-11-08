package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
)

var (
	ErrInvalidOutcome     = errors.New("invalid outcome payload")
	ErrOutcomeYearLocked  = errors.New("outcome year locked")
	ErrOutcomeFinalLocked = errors.New("outcome final locked")
)

type SiswaOutcomeListFilters struct {
	SchoolID *int
	Status   string
	Jalur    string
	Jenjang  string
	Page     int
	PageSize int
	Search   string
	Year     int
}

type SiswaOutcomeListItem struct {
	ID              int64      `json:"id"`
	SchoolID        int        `json:"school_id"`
	SchoolName      string     `json:"school_name"`
	SchoolLevel     string     `json:"school_level"`
	NISN            string     `json:"nisn"`
	NamaSiswa       string     `json:"nama_siswa"`
	Rombel          string     `json:"rombel"`
	TahunLulus      int        `json:"tahun_lulus"`
	FinalStatus     string     `json:"final_status"`
	FinalPath       string     `json:"final_path"`
	Detail          string     `json:"detail"`
	PTKampus        string     `json:"pt_kampus,omitempty"`
	PTProdi         string     `json:"pt_prodi,omitempty"`
	PTJalur         string     `json:"pt_jalur,omitempty"`
	PTTahunDiterima *int       `json:"pt_tahun_diterima,omitempty"`
	FinalizedAt     *time.Time `json:"finalized_at,omitempty"`
}

type OutcomePTDetail struct {
	Kampus        string `json:"kampus"`
	Prodi         string `json:"prodi"`
	Jalur         string `json:"jalur"`
	TahunDiterima *int   `json:"tahun_diterima,omitempty"`
}

type OutcomeWorkDetail struct {
	Perusahaan string `json:"perusahaan"`
	Jabatan    string `json:"jabatan"`
	TahunMulai *int   `json:"tahun_mulai,omitempty"`
}

type OutcomeBusinessDetail struct {
	Bidang     string `json:"bidang"`
	NamaUsaha  string `json:"nama_usaha"`
	TahunMulai *int   `json:"tahun_mulai,omitempty"`
}

type SiswaOutcomeDetail struct {
	FinalStatus    string                 `json:"final_status"`
	FinalPath      string                 `json:"final_path"`
	FinalizedAt    *time.Time             `json:"finalized_at,omitempty"`
	PtDetail       *OutcomePTDetail       `json:"pt_detail,omitempty"`
	WorkDetail     *OutcomeWorkDetail     `json:"work_detail,omitempty"`
	BusinessDetail *OutcomeBusinessDetail `json:"business_detail,omitempty"`
}

type SiswaOutcomePayload struct {
	FinalStatus    string
	FinalPath      string
	PtDetail       *OutcomePTDetail
	WorkDetail     *OutcomeWorkDetail
	BusinessDetail *OutcomeBusinessDetail
	Reason         string
}

type SiswaOutcomeApplyResult struct {
	AutoPlanUpdated bool
	Warning         string
}

type outcomeAuditSnapshot struct {
	Outcome *SiswaOutcomeDetail `json:"outcome,omitempty"`
	Plan    *SiswaPlanRecord    `json:"plan,omitempty"`
}

func normalizeFinalStatus(raw string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(raw))
	if normalized == "" {
		return "", ErrInvalidOutcome
	}
	switch normalized {
	case "PT", "KERJA", "WIRAUSAHA", "BELUM":
		return normalized, nil
	default:
		return "", ErrInvalidOutcome
	}
}

func (s SiswaService) ListOutcomes(ctx context.Context, user auth.User, filters SiswaOutcomeListFilters) ([]SiswaOutcomeListItem, int, error) {
	statusFilter := strings.ToUpper(strings.TrimSpace(filters.Status))
	if statusFilter == "" {
		statusFilter = "ALL"
	}
	if statusFilter != "ALL" {
		normalized, err := normalizeFinalStatus(statusFilter)
		if err != nil {
			return nil, 0, ErrInvalidOutcome
		}
		statusFilter = normalized
	}

	scope, err := auth.GetSchoolScope(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}
	if scope == nil && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
		return nil, 0, auth.ErrForbidden
	}
	ok, missing, err := tablesExist(
		ctx,
		s.DB,
		"class12_students",
		"student_outcomes",
		"data_sekolah",
		"outcome_pt_details",
		"outcome_work_details",
		"outcome_business_details",
	)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaOutcomeListItem{}, 0, nil
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(ds.nama_sekolah, '') AS school_name,
               COALESCE(ds.bentuk_pendidikan, '') AS school_level,
               c.nisn,
               c.nama_siswa,
               c.rombel,
               c.tahun_lulus,
               o.final_status,
               COALESCE(o.final_path::text, '') AS final_path,
               o.finalized_at,
               COALESCE(pt.kampus, '') AS pt_kampus,
               COALESCE(pt.prodi, '') AS pt_prodi,
               COALESCE(pt.jalur::text, '') AS pt_jalur,
               pt.tahun_diterima AS pt_tahun_diterima,
               COALESCE(wk.perusahaan, '') AS kerja_perusahaan,
               COALESCE(wb.bidang_usaha, '') AS usaha_bidang
        FROM class12_students c
        JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN outcome_pt_details pt
          ON pt.school_id = o.school_id
         AND pt.tahun_lulus = o.tahun_lulus
         AND pt.nisn = o.nisn
        LEFT JOIN outcome_work_details wk
          ON wk.school_id = o.school_id
         AND wk.tahun_lulus = o.tahun_lulus
         AND wk.nisn = o.nisn
        LEFT JOIN outcome_business_details wb
          ON wb.school_id = o.school_id
         AND wb.tahun_lulus = o.tahun_lulus
         AND wb.nisn = o.nisn
        WHERE o.finalized_at IS NOT NULL
    `

	args := []interface{}{}
	idx := 1
	addClause := func(condition string, value interface{}) {
		query += " AND " + condition
		args = append(args, value)
		idx++
	}

	if statusFilter != "ALL" {
		addClause(fmt.Sprintf("o.final_status = $%d", idx), statusFilter)
	}
	if scope != nil {
		addClause(fmt.Sprintf("c.school_id = $%d", idx), *scope)
	}
	allowed := []string{}
	if user.Role == "ADMIN" {
		allowed = auth.NormalizeAllowedLevels(user.AllowedLevels)
	}
	jenjangFilter := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
	if jenjangFilter == "ALL" {
		jenjangFilter = ""
	}
	if jenjangFilter != "" {
		if user.Role == "ADMIN" {
			if len(allowed) == 0 || !containsString(allowed, jenjangFilter) {
				query += " AND 1=0"
			} else {
				addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx), jenjangFilter)
			}
		} else {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx), jenjangFilter)
		}
	} else if user.Role == "ADMIN" {
		if len(allowed) == 0 {
			query += " AND 1=0"
		} else {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx), allowed)
		}
	}
	if filters.Year != 0 {
		addClause(fmt.Sprintf("c.tahun_lulus = $%d", idx), filters.Year)
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		query += fmt.Sprintf(" AND (LOWER(COALESCE(c.nisn, '')) LIKE $%d OR LOWER(COALESCE(c.nama_siswa, '')) LIKE $%d)", idx, idx+1)
		args = append(args, search, search)
		idx += 2
	}
	if strings.TrimSpace(filters.Jalur) != "" {
		rawJalur := strings.TrimSpace(filters.Jalur)
		if strings.EqualFold(rawJalur, "ALL") {
			rawJalur = ""
		}
		if rawJalur != "" {
			jalurFilter := ""
			switch strings.ToUpper(rawJalur) {
			case "SNBP":
				jalurFilter = "SNBP"
			case "SNBT":
				jalurFilter = "SNBT"
			case "SNBM", "SNBM (MANDIRI)":
				jalurFilter = "SNBM (Mandiri)"
			case "KEDINASAN":
				jalurFilter = "Kedinasan"
			case "LAINNYA":
				jalurFilter = "Lainnya"
			default:
				return nil, 0, ErrInvalidOutcome
			}
			if statusFilter != "PT" {
				if statusFilter != "ALL" {
					query += " AND 1=0"
				} else {
					addClause(fmt.Sprintf("o.final_status = $%d", idx), "PT")
				}
			}
			jalurCase := `
                CASE
                  WHEN LOWER(COALESCE(pt.jalur::text, '')) LIKE '%snbp%' THEN 'SNBP'
                  WHEN LOWER(COALESCE(pt.jalur::text, '')) LIKE '%snbt%' THEN 'SNBT'
                  WHEN LOWER(COALESCE(pt.jalur::text, '')) LIKE '%mandiri%' OR LOWER(COALESCE(pt.jalur::text, '')) LIKE '%snbm%' THEN 'SNBM (Mandiri)'
                  WHEN LOWER(COALESCE(pt.jalur::text, '')) LIKE '%kedinas%' THEN 'Kedinasan'
                  ELSE 'Lainnya'
                END
            `
			addClause(fmt.Sprintf("(%s) = $%d", jalurCase, idx), jalurFilter)
		}
	}

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS filtered"
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filters.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filters.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	query += " ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaOutcomeListItem, 0)
	for rows.Next() {
		var item SiswaOutcomeListItem
		var ptKampus, ptProdi, ptJalur, kerjaPerusahaan, usahaBidang string
		var ptTahunDiterima *int
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.SchoolName,
			&item.SchoolLevel,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.FinalStatus,
			&item.FinalPath,
			&item.FinalizedAt,
			&ptKampus,
			&ptProdi,
			&ptJalur,
			&ptTahunDiterima,
			&kerjaPerusahaan,
			&usahaBidang,
		); err != nil {
			return nil, 0, err
		}
		item.PTKampus = ptKampus
		item.PTProdi = ptProdi
		item.PTJalur = ptJalur
		item.PTTahunDiterima = ptTahunDiterima
		switch item.FinalStatus {
		case "PT":
			if ptKampus != "" || ptProdi != "" {
				item.Detail = strings.TrimSpace(strings.TrimSpace(ptKampus) + " - " + strings.TrimSpace(ptProdi))
			}
		case "KERJA":
			item.Detail = strings.TrimSpace(kerjaPerusahaan)
		case "WIRAUSAHA":
			item.Detail = strings.TrimSpace(usahaBidang)
		default:
			item.Detail = ""
		}
		result = append(result, item)
	}

	return result, total, nil
}

func (s SiswaService) GetOutcomeDetail(ctx context.Context, user auth.User, studentID int64) (SiswaPlanCohort, *SiswaOutcomeDetail, error) {
	var cohort SiswaPlanCohort
	var bentuk string
	ok, missing, err := tablesExist(
		ctx,
		s.DB,
		"class12_students",
		"student_outcomes",
		"data_sekolah",
	)
	if err != nil {
		return SiswaPlanCohort{}, nil, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "detail", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return SiswaPlanCohort{}, nil, ErrSiswaNotFound
	}
	err = s.DB.Pool.QueryRow(ctx, `
        SELECT c.id,
               c.school_id,
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel, ''),
               COALESCE(c.tahun_lulus, 0),
               COALESCE(ds.bentuk_pendidikan, '')
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        WHERE c.id = $1
    `, studentID).Scan(
		&cohort.ID,
		&cohort.SchoolID,
		&cohort.NISN,
		&cohort.NamaSiswa,
		&cohort.Rombel,
		&cohort.TahunLulus,
		&bentuk,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiswaPlanCohort{}, nil, ErrSiswaNotFound
		}
		return SiswaPlanCohort{}, nil, err
	}

	if user.Role == "SCHOOL_ADMIN" {
		if user.SchoolID == nil || *user.SchoolID != cohort.SchoolID {
			return SiswaPlanCohort{}, nil, auth.ErrForbidden
		}
	} else if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return SiswaPlanCohort{}, nil, auth.ErrForbidden
		}
		normalized := strings.ToUpper(strings.TrimSpace(bentuk))
		if normalized == "" {
			return SiswaPlanCohort{}, nil, auth.ErrForbidden
		}
		allowedSet := map[string]struct{}{}
		for _, level := range allowed {
			allowedSet[level] = struct{}{}
		}
		if _, ok := allowedSet[normalized]; !ok {
			return SiswaPlanCohort{}, nil, auth.ErrForbidden
		}
	}

	var detail SiswaOutcomeDetail
	var finalPath *string
	var finalizedAt *time.Time
	err = s.DB.Pool.QueryRow(ctx, `
        SELECT final_status, final_path, finalized_at
        FROM student_outcomes
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, cohort.SchoolID, cohort.TahunLulus, cohort.NISN).Scan(&detail.FinalStatus, &finalPath, &finalizedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return cohort, nil, nil
		}
		return cohort, nil, err
	}
	if finalPath != nil {
		detail.FinalPath = *finalPath
	}
	detail.FinalizedAt = finalizedAt

	switch detail.FinalStatus {
	case "PT":
		var pt OutcomePTDetail
		var tahunDiterima *int
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT COALESCE(kampus, ''), COALESCE(prodi, ''), COALESCE(jalur::text, ''), tahun_diterima
            FROM outcome_pt_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, cohort.SchoolID, cohort.TahunLulus, cohort.NISN).Scan(&pt.Kampus, &pt.Prodi, &pt.Jalur, &tahunDiterima); err == nil {
			pt.TahunDiterima = tahunDiterima
			detail.PtDetail = &pt
		}
	case "KERJA":
		var wk OutcomeWorkDetail
		var tahunMulai *int
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT COALESCE(perusahaan, ''), COALESCE(jabatan, ''), tahun_mulai
            FROM outcome_work_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, cohort.SchoolID, cohort.TahunLulus, cohort.NISN).Scan(&wk.Perusahaan, &wk.Jabatan, &tahunMulai); err == nil {
			wk.TahunMulai = tahunMulai
			detail.WorkDetail = &wk
		}
	case "WIRAUSAHA":
		var wb OutcomeBusinessDetail
		var tahunMulai *int
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT COALESCE(bidang_usaha, ''), COALESCE(nama_usaha, ''), tahun_mulai
            FROM outcome_business_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, cohort.SchoolID, cohort.TahunLulus, cohort.NISN).Scan(&wb.Bidang, &wb.NamaUsaha, &tahunMulai); err == nil {
			wb.TahunMulai = tahunMulai
			detail.BusinessDetail = &wb
		}
	}

	return cohort, &detail, nil
}

func (s SiswaService) UpsertOutcome(ctx context.Context, user auth.User, studentID int64, payload SiswaOutcomePayload) (SiswaOutcomeApplyResult, error) {
	if user.Role != "SCHOOL_ADMIN" {
		return SiswaOutcomeApplyResult{}, auth.ErrForbidden
	}
	if user.SchoolID == nil {
		return SiswaOutcomeApplyResult{}, auth.ErrUnauthorized
	}
	ok, missing, err := tablesExist(
		ctx,
		s.DB,
		"class12_students",
		"student_outcomes",
		"student_outcome_audit",
		"student_plans",
		"outcome_pt_details",
		"outcome_work_details",
		"outcome_business_details",
	)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "upsert", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return SiswaOutcomeApplyResult{}, ErrSiswaNotFound
	}

	var schoolID int
	var nisn string
	var tahunLulus int
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT school_id, nisn, tahun_lulus
        FROM class12_students
        WHERE id = $1
    `, studentID).Scan(&schoolID, &nisn, &tahunLulus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiswaOutcomeApplyResult{}, ErrSiswaNotFound
		}
		return SiswaOutcomeApplyResult{}, err
	}
	if schoolID != *user.SchoolID {
		return SiswaOutcomeApplyResult{}, auth.ErrForbidden
	}
	currentYear := time.Now().Year()
	if tahunLulus != currentYear {
		return SiswaOutcomeApplyResult{}, ErrOutcomeYearLocked
	}

	var existingFinalizedAt *time.Time
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT finalized_at
        FROM student_outcomes
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn).Scan(&existingFinalizedAt); err == nil {
		if existingFinalizedAt != nil {
			return SiswaOutcomeApplyResult{}, ErrOutcomeFinalLocked
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return SiswaOutcomeApplyResult{}, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, user.ID, "MANUAL", false)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return SiswaOutcomeApplyResult{}, err
	}
	return result, nil
}

func (s SiswaService) ExportOutcomeTemplateRows(ctx context.Context, user auth.User, filters SiswaPlanListFilters, limit int) ([]SiswaPlanCohort, int, error) {
	scope, err := auth.GetSchoolScope(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}
	if scope == nil {
		return nil, 0, auth.ErrForbidden
	}
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_outcomes", "data_sekolah")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "export", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaPlanCohort{}, 0, nil
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel, ''),
               COALESCE(c.tahun_lulus, 0)
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
         AND o.finalized_at IS NOT NULL
        WHERE c.school_id = $1
          AND o.id IS NULL
    `

	args := []interface{}{*scope}
	idx := 2
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return []SiswaPlanCohort{}, 0, nil
		}
		query += fmt.Sprintf(" AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx)
		args = append(args, allowed)
		idx++
	}
	if filters.Year != 0 {
		query += fmt.Sprintf(" AND c.tahun_lulus = $%d", idx)
		args = append(args, filters.Year)
		idx++
	}

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS filtered"
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit > 0 && total > limit {
		return nil, total, ErrExportLimitExceeded
	}

	query += " ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaPlanCohort, 0)
	for rows.Next() {
		var item SiswaPlanCohort
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, item)
	}
	return result, total, nil
}

func (s SiswaService) ImportOutcomes(ctx context.Context, user auth.User, schoolID int, rows []parsers.SiswaOutcomeRow) (SiswaImportResult, error) {
	result := SiswaImportResult{
		TotalRows:  len(rows),
		Rejections: make([]SiswaImportRejection, 0),
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	nisnSet := map[string]struct{}{}
	for _, row := range rows {
		nisn := strings.TrimSpace(row.NISN)
		if nisn != "" {
			nisnSet[nisn] = struct{}{}
		}
	}
	nisns := make([]string, 0, len(nisnSet))
	for nisn := range nisnSet {
		nisns = append(nisns, nisn)
	}

	classMap, err := fetchClass12Map(ctx, tx, schoolID, nisns)
	if err != nil {
		return result, err
	}

	existingFinal := map[string]struct{}{}
	if len(nisns) > 0 {
		rows, err := tx.Query(ctx, `
            SELECT nisn, tahun_lulus
            FROM student_outcomes
            WHERE school_id = $1
              AND nisn = ANY($2)
              AND finalized_at IS NOT NULL
        `, schoolID, nisns)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var nisn string
			var tahun int
			if scanErr := rows.Scan(&nisn, &tahun); scanErr == nil {
				existingFinal[buildClass12Key(nisn, tahun)] = struct{}{}
			}
		}
		rows.Close()
	}

	seen := map[string]struct{}{}
	for _, row := range rows {
		reject := func(reason string) {
			result.Rejections = append(result.Rejections, SiswaImportRejection{
				RowNumber: row.RowNumber,
				Reason:    reason,
				Data: map[string]string{
					"NISN":                  strings.TrimSpace(row.NISN),
					"Nama_Siswa":            strings.TrimSpace(row.NamaSiswa),
					"Rombel":                strings.TrimSpace(row.Rombel),
					"Tahun_Lulus":           strings.TrimSpace(row.TahunLulus),
					"Final_Status":          strings.TrimSpace(row.FinalStatus),
					"Final_Path":            strings.TrimSpace(row.FinalPath),
					"PT_Kampus":             strings.TrimSpace(row.PTKampus),
					"PT_Prodi":              strings.TrimSpace(row.PTProdi),
					"PT_Tahun_Diterima":     strings.TrimSpace(row.PTTahunDiterima),
					"Kerja_Perusahaan":      strings.TrimSpace(row.KerjaPerusahaan),
					"Kerja_Jabatan":         strings.TrimSpace(row.KerjaJabatan),
					"Kerja_Tahun_Mulai":     strings.TrimSpace(row.KerjaTahunMulai),
					"Wirausaha_Bidang":      strings.TrimSpace(row.WirausahaBidang),
					"Wirausaha_Nama":        strings.TrimSpace(row.WirausahaNama),
					"Wirausaha_Tahun_Mulai": strings.TrimSpace(row.WirausahaTahun),
				},
			})
		}

		nisn := strings.TrimSpace(row.NISN)
		tahunLulus, yearOK := parseYearStrict(row.TahunLulus)
		if nisn == "" || !yearOK || tahunLulus <= 0 {
			reject("NISN dan Tahun Lulus wajib diisi.")
			continue
		}
		if !isNumericOnly(nisn) {
			reject("NISN harus berupa angka.")
			continue
		}

		finalStatusRaw := strings.TrimSpace(row.FinalStatus)
		if finalStatusRaw == "" {
			result.Accepted++
			continue
		}

		key := buildClass12Key(nisn, tahunLulus)
		if _, exists := seen[key]; exists {
			reject("Data duplikat pada file (NISN + Tahun Lulus).")
			continue
		}
		seen[key] = struct{}{}

		if _, ok := classMap[key]; !ok {
			reject("Data siswa tidak ditemukan.")
			continue
		}
		if _, ok := existingFinal[key]; ok {
			reject("Data sudah final dan terkunci.")
			continue
		}

		status, err := normalizeFinalStatus(finalStatusRaw)
		if err != nil {
			reject("Final status tidak valid.")
			continue
		}

		payload := SiswaOutcomePayload{FinalStatus: status}
		if status == "PT" {
			finalPath, ok := normalizeJalurPenerimaan(row.FinalPath)
			if !ok {
				reject("Final path tidak valid.")
				continue
			}
			payload.FinalPath = finalPath
			kampus := strings.TrimSpace(row.PTKampus)
			prodi := strings.TrimSpace(row.PTProdi)
			if kampus == "" || prodi == "" {
				reject("Kampus dan prodi wajib diisi untuk PT.")
				continue
			}
			var tahunDiterima *int
			if strings.TrimSpace(row.PTTahunDiterima) != "" {
				parsed, ok := parseYearStrict(row.PTTahunDiterima)
				if !ok || parsed <= 0 {
					reject("Tahun diterima tidak valid.")
					continue
				}
				tahunDiterima = &parsed
			}
			payload.PtDetail = &OutcomePTDetail{
				Kampus:        kampus,
				Prodi:         prodi,
				Jalur:         finalPath,
				TahunDiterima: tahunDiterima,
			}
		} else if status == "KERJA" {
			perusahaan := strings.TrimSpace(row.KerjaPerusahaan)
			if perusahaan == "" {
				reject("Perusahaan wajib diisi untuk status kerja.")
				continue
			}
			var tahunMulai *int
			if strings.TrimSpace(row.KerjaTahunMulai) != "" {
				parsed, ok := parseYearStrict(row.KerjaTahunMulai)
				if !ok || parsed <= 0 {
					reject("Tahun mulai kerja tidak valid.")
					continue
				}
				tahunMulai = &parsed
			}
			payload.WorkDetail = &OutcomeWorkDetail{
				Perusahaan: perusahaan,
				Jabatan:    strings.TrimSpace(row.KerjaJabatan),
				TahunMulai: tahunMulai,
			}
		} else if status == "WIRAUSAHA" {
			bidang := strings.TrimSpace(row.WirausahaBidang)
			if bidang == "" {
				reject("Bidang usaha wajib diisi untuk wirausaha.")
				continue
			}
			var tahunMulai *int
			if strings.TrimSpace(row.WirausahaTahun) != "" {
				parsed, ok := parseYearStrict(row.WirausahaTahun)
				if !ok || parsed <= 0 {
					reject("Tahun mulai usaha tidak valid.")
					continue
				}
				tahunMulai = &parsed
			}
			payload.BusinessDetail = &OutcomeBusinessDetail{
				Bidang:     bidang,
				NamaUsaha:  strings.TrimSpace(row.WirausahaNama),
				TahunMulai: tahunMulai,
			}
		}

		if _, err := s.applyOutcomeTx(ctx, tx, schoolID, nisn, tahunLulus, payload, user.ID, "IMPORT", true); err != nil {
			switch {
			case errors.Is(err, ErrInvalidOutcome):
				reject("Data status akhir tidak valid.")
			case errors.Is(err, ErrSiswaNotFound):
				reject("Data siswa tidak ditemukan.")
			case errors.Is(err, ErrOutcomeFinalLocked):
				reject("Data sudah final dan terkunci.")
			default:
				return result, err
			}
			continue
		}

		result.Accepted++
	}

	if len(result.Rejections) > 0 {
		reportID, reportErr := s.insertRejectedReportTx(
			ctx,
			tx,
			schoolID,
			"OUTCOMES",
			"rejected-siswa-outcomes.xlsx",
			parsers.SiswaOutcomeTemplateHeaders,
			result.Rejections,
		)
		if reportErr != nil {
			return result, reportErr
		}
		result.RejectedReportID = reportID
		result.RejectedFilename = "rejected-siswa-outcomes.xlsx"
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.Rejected = len(result.Rejections)
	return result, nil
}

type class12Record struct {
	NISN       string
	TahunLulus int
}

func buildClass12Key(nisn string, tahunLulus int) string {
	return strings.TrimSpace(nisn) + "|" + strconv.Itoa(tahunLulus)
}

func fetchClass12Map(ctx context.Context, tx pgx.Tx, schoolID int, nisns []string) (map[string]class12Record, error) {
	result := map[string]class12Record{}
	if len(nisns) == 0 {
		return result, nil
	}

	rows, err := tx.Query(ctx, `
        SELECT nisn, tahun_lulus
        FROM class12_students
        WHERE school_id = $1
          AND nisn = ANY($2)
    `, schoolID, nisns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var record class12Record
		if scanErr := rows.Scan(&record.NISN, &record.TahunLulus); scanErr != nil {
			return nil, scanErr
		}
		result[buildClass12Key(record.NISN, record.TahunLulus)] = record
	}
	return result, nil
}

func (s SiswaService) applyOutcomeTx(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	nisn string,
	tahunLulus int,
	payload SiswaOutcomePayload,
	changedBy int,
	source string,
	blockIfFinal bool,
) (SiswaOutcomeApplyResult, error) {
	normalizedStatus, err := normalizeFinalStatus(payload.FinalStatus)
	if err != nil {
		return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
	}

	var finalPath *string
	if normalizedStatus == "PT" {
		normalizedPath, ok := normalizeJalurPenerimaan(payload.FinalPath)
		if !ok {
			return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
		}
		finalPath = &normalizedPath
		if payload.PtDetail == nil || strings.TrimSpace(payload.PtDetail.Kampus) == "" || strings.TrimSpace(payload.PtDetail.Prodi) == "" {
			return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
		}
		payload.PtDetail.Jalur = normalizedPath
	} else if normalizedStatus == "KERJA" {
		if payload.WorkDetail == nil || strings.TrimSpace(payload.WorkDetail.Perusahaan) == "" {
			return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
		}
	} else if normalizedStatus == "WIRAUSAHA" {
		if payload.BusinessDetail == nil || strings.TrimSpace(payload.BusinessDetail.Bidang) == "" {
			return SiswaOutcomeApplyResult{}, ErrInvalidOutcome
		}
	}

	var existingFinalizedAt *time.Time
	_ = tx.QueryRow(ctx, `
        SELECT finalized_at
        FROM student_outcomes
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn).Scan(&existingFinalizedAt)
	if blockIfFinal && existingFinalizedAt != nil {
		return SiswaOutcomeApplyResult{}, ErrOutcomeFinalLocked
	}

	beforeSnapshot, err := s.fetchOutcomeSnapshot(ctx, tx, schoolID, nisn, tahunLulus, true)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}

	finalizedAt := time.Now()
	if _, err := tx.Exec(ctx, `
        INSERT INTO student_outcomes (
            school_id, tahun_lulus, nisn, final_status, final_path, finalized_at, finalized_by, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
        ON CONFLICT (school_id, tahun_lulus, nisn)
        DO UPDATE SET
            final_status = EXCLUDED.final_status,
            final_path = EXCLUDED.final_path,
            finalized_at = EXCLUDED.finalized_at,
            finalized_by = EXCLUDED.finalized_by,
            updated_at = NOW()
    `, schoolID, tahunLulus, nisn, normalizedStatus, finalPath, finalizedAt, changedBy); err != nil {
		return SiswaOutcomeApplyResult{}, err
	}

	if normalizedStatus == "PT" {
		pt := payload.PtDetail
		if _, err := tx.Exec(ctx, `
            INSERT INTO outcome_pt_details (
                school_id, tahun_lulus, nisn, kampus, prodi, jalur, tahun_diterima, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
            ON CONFLICT (school_id, tahun_lulus, nisn)
            DO UPDATE SET
                kampus = EXCLUDED.kampus,
                prodi = EXCLUDED.prodi,
                jalur = EXCLUDED.jalur,
                tahun_diterima = EXCLUDED.tahun_diterima,
                updated_at = NOW()
        `, schoolID, tahunLulus, nisn,
			strings.TrimSpace(pt.Kampus),
			strings.TrimSpace(pt.Prodi),
			strings.TrimSpace(pt.Jalur),
			pt.TahunDiterima,
		); err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_work_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_business_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
	} else if normalizedStatus == "KERJA" {
		wk := payload.WorkDetail
		if _, err := tx.Exec(ctx, `
            INSERT INTO outcome_work_details (
                school_id, tahun_lulus, nisn, perusahaan, jabatan, tahun_mulai, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, NOW())
            ON CONFLICT (school_id, tahun_lulus, nisn)
            DO UPDATE SET
                perusahaan = EXCLUDED.perusahaan,
                jabatan = EXCLUDED.jabatan,
                tahun_mulai = EXCLUDED.tahun_mulai,
                updated_at = NOW()
        `, schoolID, tahunLulus, nisn,
			strings.TrimSpace(wk.Perusahaan),
			strings.TrimSpace(wk.Jabatan),
			wk.TahunMulai,
		); err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_pt_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_business_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
	} else if normalizedStatus == "WIRAUSAHA" {
		wb := payload.BusinessDetail
		if _, err := tx.Exec(ctx, `
            INSERT INTO outcome_business_details (
                school_id, tahun_lulus, nisn, bidang_usaha, nama_usaha, tahun_mulai, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, NOW())
            ON CONFLICT (school_id, tahun_lulus, nisn)
            DO UPDATE SET
                bidang_usaha = EXCLUDED.bidang_usaha,
                nama_usaha = EXCLUDED.nama_usaha,
                tahun_mulai = EXCLUDED.tahun_mulai,
                updated_at = NOW()
        `, schoolID, tahunLulus, nisn,
			strings.TrimSpace(wb.Bidang),
			strings.TrimSpace(wb.NamaUsaha),
			wb.TahunMulai,
		); err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_pt_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_work_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
	} else {
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_pt_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_work_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
		_, _ = tx.Exec(ctx, `DELETE FROM outcome_business_details WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3`, schoolID, tahunLulus, nisn)
	}

	result := SiswaOutcomeApplyResult{}
	if normalizedStatus == "PT" && finalPath != nil {
		updated, err := s.autoSyncPlanFromFinalTx(ctx, tx, schoolID, nisn, tahunLulus, *finalPath, payload.PtDetail)
		if err != nil {
			return SiswaOutcomeApplyResult{}, err
		}
		if updated {
			result.AutoPlanUpdated = true
			result.Warning = "Rencana diperbarui otomatis karena hasil akhir berbeda."
		}
	}

	afterSnapshot, err := s.fetchOutcomeSnapshot(ctx, tx, schoolID, nisn, tahunLulus, result.AutoPlanUpdated)
	if err != nil {
		return SiswaOutcomeApplyResult{}, err
	}

	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = source
	}
	if result.AutoPlanUpdated {
		reason = reason + "_AUTO_SYNC_PLAN"
	}

	beforeJSON, _ := json.Marshal(beforeSnapshot)
	afterJSON, _ := json.Marshal(afterSnapshot)
	_, _ = tx.Exec(ctx, `
        INSERT INTO student_outcome_audit (
            school_id, tahun_lulus, nisn, changed_by, before_json, after_json, reason
        ) VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, schoolID, tahunLulus, nisn, changedBy, beforeJSON, afterJSON, reason)

	return result, nil
}

func (s SiswaService) autoSyncPlanFromFinalTx(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	nisn string,
	tahunLulus int,
	finalPath string,
	ptDetail *OutcomePTDetail,
) (bool, error) {
	if ptDetail == nil {
		return false, nil
	}

	var planJalur []string
	var planChoicesRaw []byte
	var planSource string
	err := tx.QueryRow(ctx, `
        SELECT plan_jalur, plan_choices, plan_source
        FROM student_plans
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn).Scan(&planJalur, &planChoicesRaw, &planSource)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}

	updated := false
	normalizedJalur, _ := normalizePlanJalurList(planJalur)
	if !containsPlanJalur(normalizedJalur, finalPath) {
		normalizedJalur = append(normalizedJalur, finalPath)
		normalizedJalur, _ = normalizePlanJalurList(normalizedJalur)
		updated = true
	}
	if strings.EqualFold(finalPath, "SNBP") && containsPlanJalur(normalizedJalur, "SNBT") {
		nextJalur := make([]string, 0, len(normalizedJalur))
		for _, jalur := range normalizedJalur {
			if !strings.EqualFold(jalur, "SNBT") {
				nextJalur = append(nextJalur, jalur)
			}
		}
		normalizedJalur = nextJalur
		updated = true
	}

	choices := decodePlanChoices(planChoicesRaw)
	finalChoice := SiswaPlanChoice{Kampus: strings.TrimSpace(ptDetail.Kampus), Prodi: strings.TrimSpace(ptDetail.Prodi)}
	if finalChoice.Kampus != "" && finalChoice.Prodi != "" {
		exists := false
		for _, choice := range choices {
			if strings.EqualFold(choice.Kampus, finalChoice.Kampus) && strings.EqualFold(choice.Prodi, finalChoice.Prodi) {
				exists = true
				break
			}
		}
		if !exists {
			if len(choices) < 3 {
				choices = append(choices, finalChoice)
			} else if len(choices) >= 3 {
				choices[len(choices)-1] = finalChoice
			}
			updated = true
		}
	}

	if !updated && !errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}

	choicesJSON, err := json.Marshal(choices)
	if err != nil {
		return false, err
	}

	if _, err := tx.Exec(ctx, `
        INSERT INTO student_plans (
            school_id, tahun_lulus, nisn, plan_jalur, plan_choices, plan_source, updated_at
        ) VALUES ($1, $2, $3, $4, $5, 'AUTO_FROM_FINAL', NOW())
        ON CONFLICT (school_id, tahun_lulus, nisn)
        DO UPDATE SET
            plan_jalur = EXCLUDED.plan_jalur,
            plan_choices = EXCLUDED.plan_choices,
            plan_source = 'AUTO_FROM_FINAL',
            updated_at = NOW()
    `, schoolID, tahunLulus, nisn, normalizedJalur, choicesJSON); err != nil {
		return false, err
	}

	return true, nil
}

func (s SiswaService) fetchOutcomeSnapshot(ctx context.Context, tx pgx.Tx, schoolID int, nisn string, tahunLulus int, includePlan bool) (*outcomeAuditSnapshot, error) {
	var snapshot outcomeAuditSnapshot

	var detail SiswaOutcomeDetail
	var finalPath *string
	var finalizedAt *time.Time
	err := tx.QueryRow(ctx, `
        SELECT final_status, final_path, finalized_at
        FROM student_outcomes
        WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
    `, schoolID, tahunLulus, nisn).Scan(&detail.FinalStatus, &finalPath, &finalizedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &snapshot, nil
		}
		return nil, err
	}
	if finalPath != nil {
		detail.FinalPath = *finalPath
	}
	detail.FinalizedAt = finalizedAt

	switch detail.FinalStatus {
	case "PT":
		var pt OutcomePTDetail
		var tahunDiterima *int
		if err := tx.QueryRow(ctx, `
            SELECT COALESCE(kampus, ''), COALESCE(prodi, ''), COALESCE(jalur::text, ''), tahun_diterima
            FROM outcome_pt_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, schoolID, tahunLulus, nisn).Scan(&pt.Kampus, &pt.Prodi, &pt.Jalur, &tahunDiterima); err == nil {
			pt.TahunDiterima = tahunDiterima
			detail.PtDetail = &pt
		}
	case "KERJA":
		var wk OutcomeWorkDetail
		var tahunMulai *int
		if err := tx.QueryRow(ctx, `
            SELECT COALESCE(perusahaan, ''), COALESCE(jabatan, ''), tahun_mulai
            FROM outcome_work_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, schoolID, tahunLulus, nisn).Scan(&wk.Perusahaan, &wk.Jabatan, &tahunMulai); err == nil {
			wk.TahunMulai = tahunMulai
			detail.WorkDetail = &wk
		}
	case "WIRAUSAHA":
		var wb OutcomeBusinessDetail
		var tahunMulai *int
		if err := tx.QueryRow(ctx, `
            SELECT COALESCE(bidang_usaha, ''), COALESCE(nama_usaha, ''), tahun_mulai
            FROM outcome_business_details
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, schoolID, tahunLulus, nisn).Scan(&wb.Bidang, &wb.NamaUsaha, &tahunMulai); err == nil {
			wb.TahunMulai = tahunMulai
			detail.BusinessDetail = &wb
		}
	}

	snapshot.Outcome = &detail

	if includePlan {
		var planJalur []string
		var planChoicesRaw []byte
		var planSource string
		if err := tx.QueryRow(ctx, `
            SELECT plan_jalur, plan_choices, plan_source
            FROM student_plans
            WHERE school_id = $1 AND tahun_lulus = $2 AND nisn = $3
        `, schoolID, tahunLulus, nisn).Scan(&planJalur, &planChoicesRaw, &planSource); err == nil {
			planRecord := &SiswaPlanRecord{
				PlanJalur:   planJalur,
				PlanChoices: decodePlanChoices(planChoicesRaw),
				PlanSource:  planSource,
			}
			snapshot.Plan = planRecord
		}
	}

	return &snapshot, nil
}
