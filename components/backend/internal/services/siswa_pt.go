package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type PtPipelineListFilters struct {
	SchoolID         *int
	Page             int
	PageSize         int
	Search           string
	Year             int
	IncludeFinalized bool
}

type PtPipelineListItem struct {
	ID          int64
	SchoolID    int
	NISN        string
	NamaSiswa   string
	Rombel      string
	TahunLulus  int
	Status      string
	PlanCount   int
	FinalStatus string
	FinalPath   string
	FinalizedAt *time.Time
	YearClosed  bool
}

type PtApplicationChoice struct {
	PilihanKe int
	Jalur     string
	Kampus    string
	Prodi     string
}

func (s SiswaService) ListPtPipeline(ctx context.Context, user auth.User, filters PtPipelineListFilters) ([]PtPipelineListItem, int, error) {
	scope, err := auth.GetSchoolScope(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_cohorts", "student_higher_ed_applications", "student_outcomes", "data_sekolah")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("pt_pipeline", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []PtPipelineListItem{}, 0, nil
	}

	lockOK, _, lockErr := tablesExist(ctx, s.DB, "graduation_year_locks")
	if lockErr != nil {
		return nil, 0, lockErr
	}
	yearClosedExpr := "BOOL_OR(false)"
	if lockOK {
		yearClosedExpr = `
			BOOL_OR(EXISTS (
				SELECT 1
				FROM graduation_year_locks gyl
				WHERE gyl.tahun_lulus = c.tahun_lulus
				  AND gyl.is_closed = TRUE
				  AND (gyl.school_id = c.school_id OR gyl.school_id IS NULL)
			))
		`
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(c.nisn, '') AS nisn,
               COALESCE(c.nama_siswa, '') AS nama_siswa,
               COALESCE(c.rombel, '') AS rombel,
               COALESCE(c.tahun_lulus, 0) AS tahun_lulus,
               COUNT(a.id) AS plan_count,
               COALESCE(o.final_status, '') AS final_status,
               COALESCE(o.final_path::text, '') AS final_path,
               o.finalized_at,
               ` + yearClosedExpr + ` AS year_closed
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_cohorts sc
          ON sc.school_id = c.school_id
         AND sc.nisn = c.nisn
         AND sc.tahun_lulus = c.tahun_lulus
        LEFT JOIN student_higher_ed_applications a
          ON a.school_id = c.school_id
         AND a.cohort_id = sc.id
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
    `

	whereClause := ""
	args := make([]interface{}, 0)
	idx := 1
	addClause := func(condition string, value interface{}) {
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, value)
		idx++
	}
	addRaw := func(condition string) {
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
	}

	if scope != nil {
		addClause(fmt.Sprintf("c.school_id = $%d", idx), *scope)
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			if whereClause == "" {
				whereClause = " WHERE 1=0"
			} else {
				whereClause += " AND 1=0"
			}
		} else {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx), allowed)
		}
	}
	if filters.Year != 0 {
		addClause(fmt.Sprintf("c.tahun_lulus = $%d", idx), filters.Year)
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		condition := fmt.Sprintf("(LOWER(COALESCE(c.nisn, '')) LIKE $%d OR LOWER(COALESCE(c.nama_siswa, '')) LIKE $%d)", idx, idx+1)
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, search, search)
		idx += 2
	}
	if !filters.IncludeFinalized {
		addRaw("o.finalized_at IS NULL")
	}

	countQuery := `
        SELECT COUNT(*)
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        LEFT JOIN student_outcomes o
          ON o.school_id = c.school_id
         AND o.tahun_lulus = c.tahun_lulus
         AND o.nisn = c.nisn
    ` + whereClause
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []PtPipelineListItem{}, 0, nil
		} else {
			return nil, 0, err
		}
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

	query += whereClause + " GROUP BY c.id, c.school_id, c.nisn, c.nama_siswa, c.rombel, c.tahun_lulus, o.final_status, o.final_path, o.finalized_at" +
		" ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]PtPipelineListItem, 0)
	for rows.Next() {
		var item PtPipelineListItem
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.PlanCount,
			&item.FinalStatus,
			&item.FinalPath,
			&item.FinalizedAt,
			&item.YearClosed,
		); err == nil {
			if item.PlanCount > 0 {
				item.Status = "Sudah Diisi"
			} else {
				item.Status = "Belum Diisi"
			}
			result = append(result, item)
		}
	}

	return result, total, nil
}

func (s SiswaService) GetPtApplications(ctx context.Context, user auth.User, schoolID int, nisn string, tahunLulus int) (SiswaPlanCohort, []PtApplicationChoice, string, *time.Time, error) {
	var cohort SiswaPlanCohort
	var bentuk string
	err := s.DB.Pool.QueryRow(ctx, `
        SELECT c.id,
               c.school_id,
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel, ''),
               COALESCE(c.tahun_lulus, 0),
               COALESCE(ds.bentuk_pendidikan, '')
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
        WHERE c.school_id = $1
          AND c.nisn = $2
          AND c.tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(
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
			return SiswaPlanCohort{}, nil, "", nil, ErrSiswaNotFound
		}
		return SiswaPlanCohort{}, nil, "", nil, err
	}

	if user.Role == "SCHOOL_ADMIN" {
		if user.SchoolID == nil || *user.SchoolID != cohort.SchoolID {
			return SiswaPlanCohort{}, nil, "", nil, auth.ErrForbidden
		}
	} else if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return SiswaPlanCohort{}, nil, "", nil, auth.ErrForbidden
		}
		normalized := strings.ToUpper(strings.TrimSpace(bentuk))
		if normalized == "" {
			return SiswaPlanCohort{}, nil, "", nil, auth.ErrForbidden
		}
		allowedSet := map[string]struct{}{}
		for _, level := range allowed {
			allowedSet[level] = struct{}{}
		}
		if _, ok := allowedSet[normalized]; !ok {
			return SiswaPlanCohort{}, nil, "", nil, auth.ErrForbidden
		}
	}

	var cohortID int64
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT id
        FROM student_cohorts
        WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&cohortID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiswaPlanCohort{}, nil, "", nil, ErrSiswaNotFound
		}
		return SiswaPlanCohort{}, nil, "", nil, err
	}

	choices := make([]PtApplicationChoice, 0)
	rows, err := s.DB.Pool.Query(ctx, `
        SELECT pilihan_ke,
               COALESCE(jalur_penerimaan::text, ''),
               COALESCE(nama_perguruan_tinggi, ''),
               COALESCE(jurusan_perguruan_tinggi, '')
        FROM student_higher_ed_applications
        WHERE school_id = $1 AND cohort_id = $2
        ORDER BY pilihan_ke ASC, id ASC
    `, schoolID, cohortID)
	if err != nil {
		return SiswaPlanCohort{}, nil, "", nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item PtApplicationChoice
		if scanErr := rows.Scan(&item.PilihanKe, &item.Jalur, &item.Kampus, &item.Prodi); scanErr == nil {
			choices = append(choices, item)
		}
	}

	var finalizedAt *time.Time
	var finalStatus string
	_ = s.DB.Pool.QueryRow(ctx, `
        SELECT COALESCE(final_status, ''), finalized_at
        FROM student_outcomes
        WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&finalStatus, &finalizedAt)

	status := "Belum Diisi"
	if len(choices) > 0 {
		status = "Sudah Diisi"
	}

	return cohort, choices, status, finalizedAt, nil
}

func (s SiswaService) UpsertPtApplications(ctx context.Context, user auth.User, schoolID int, nisn string, tahunLulus int, choices []PtApplicationChoice) error {
	if user.Role != "SCHOOL_ADMIN" {
		return auth.ErrForbidden
	}
	if user.SchoolID == nil || *user.SchoolID != schoolID {
		return auth.ErrForbidden
	}

	if strings.TrimSpace(nisn) == "" || tahunLulus <= 0 {
		return ErrInvalidPlan
	}

	if err := s.ensureNotFinalized(ctx, schoolID, nisn, tahunLulus); err != nil {
		return err
	}
	if err := s.ensureYearOpen(ctx, schoolID, tahunLulus); err != nil {
		return err
	}

	var cohortID int64
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT id
        FROM student_cohorts
        WHERE school_id = $1 AND nisn = $2 AND tahun_lulus = $3
    `, schoolID, nisn, tahunLulus).Scan(&cohortID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSiswaNotFound
		}
		return err
	}

	filtered := make([]PtApplicationChoice, 0, len(choices))
	seen := map[int]struct{}{}
	hasSNBP := false
	hasSNBT := false

	for _, choice := range choices {
		choiceNo := choice.PilihanKe
		if choiceNo < 1 || choiceNo > 3 {
			return ErrInvalidPlan
		}
		jalur, ok := normalizeJalurPenerimaan(choice.Jalur)
		if !ok {
			return ErrInvalidPlan
		}
		kampus := strings.TrimSpace(choice.Kampus)
		prodi := strings.TrimSpace(choice.Prodi)

		if kampus == "" && prodi == "" {
			continue
		}
		if kampus == "" || prodi == "" {
			return ErrInvalidPlan
		}
		if _, exists := seen[choiceNo]; exists {
			return ErrInvalidPlan
		}
		seen[choiceNo] = struct{}{}

		switch jalur {
		case "SNBP":
			hasSNBP = true
		case "SNBT":
			hasSNBT = true
		}

		filtered = append(filtered, PtApplicationChoice{
			PilihanKe: choiceNo,
			Jalur:     jalur,
			Kampus:    kampus,
			Prodi:     prodi,
		})
	}

	if hasSNBP && hasSNBT {
		return ErrInvalidPlan
	}
	if len(filtered) > 3 {
		return ErrInvalidPlan
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
        DELETE FROM student_higher_ed_applications
        WHERE school_id = $1 AND cohort_id = $2
    `, schoolID, cohortID); err != nil {
		return err
	}

	for _, choice := range filtered {
		_, err := tx.Exec(ctx, `
            INSERT INTO student_higher_ed_applications (
                school_id, cohort_id, pilihan_ke, jalur_penerimaan,
                nama_perguruan_tinggi, jurusan_perguruan_tinggi, source, updated_at
            ) VALUES ($1, $2, $3, $4, $5, $6, 'MANUAL', NOW())
        `, schoolID, cohortID, choice.PilihanKe, choice.Jalur, choice.Kampus, choice.Prodi)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
