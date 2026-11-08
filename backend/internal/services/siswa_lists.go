package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type SiswaCohortListFilters struct {
	SchoolID       *int
	Page           int
	PageSize       int
	Search         string
	Year           int
	IncludePlanned bool
}

type SiswaCohortListItem struct {
	ID         int64
	SchoolID   int
	SchoolName string
	NISN       string
	NamaSiswa  string
	Rombel     string
	TahunLulus int
	Status     string
}

type SiswaApplicationListFilters struct {
	SchoolID *int
	Page     int
	PageSize int
	Search   string
	Year     int
}

type SiswaApplicationListItem struct {
	ID                  int64
	SchoolID            int
	NISN                string
	NamaSiswa           string
	Rombel              string
	JalurPenerimaan     string
	NamaPerguruanTinggi string
	Status              string
	FinalStatus         string
	FinalizedAt         *time.Time
}

func (s SiswaService) ListCohorts(ctx context.Context, user auth.User, filters SiswaCohortListFilters) ([]SiswaCohortListItem, int, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}
	if filters.Year == 0 {
		filters.Year = time.Now().Year()
	}
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_plans", "student_outcomes", "data_sekolah")
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_cohorts", "list", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaCohortListItem{}, 0, nil
	}
	hasFinalRequests, _, err := tablesExist(ctx, s.DB, "student_final_requests")
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT c.id,
               c.school_id,
               COALESCE(ds.nama_sekolah, '') AS school_name,
               COALESCE(c.nisn, '') AS nisn,
               COALESCE(c.nama_siswa, '') AS nama_siswa,
               COALESCE(c.rombel, '') AS rombel_siswa,
               COALESCE(c.tahun_lulus, 0) AS tahun_lulus,
               CASE
                 WHEN EXISTS (
                   SELECT 1 FROM student_outcomes o
                   WHERE o.school_id = c.school_id
                     AND o.tahun_lulus = c.tahun_lulus
                     AND o.nisn = c.nisn
                     AND o.finalized_at IS NOT NULL
                 ) THEN 'Final'
                 WHEN EXISTS (
                   SELECT 1 FROM student_plans p
                   WHERE p.school_id = c.school_id
                     AND p.tahun_lulus = c.tahun_lulus
                     AND p.nisn = c.nisn
                     AND (
                       cardinality(p.plan_jalur) > 0
                       OR (
                         p.plan_choices IS NOT NULL
                         AND jsonb_typeof(p.plan_choices) = 'array'
                         AND jsonb_array_length(p.plan_choices) > 0
                       )
                     )
                 ) THEN 'Belum Final'
                 ELSE 'Belum Isi Rencana'
               END AS status
        FROM class12_students c
        JOIN data_sekolah ds ON ds.id = c.school_id
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

	if !scope.All {
		if len(scope.IDs) == 0 {
			return []SiswaCohortListItem{}, 0, nil
		}
		addClause(fmt.Sprintf("c.school_id = ANY($%d)", idx), scope.IDs)
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

	planCondition := `
        EXISTS (
            SELECT 1 FROM student_plans p
            WHERE p.school_id = c.school_id
              AND p.tahun_lulus = c.tahun_lulus
              AND p.nisn = c.nisn
              AND (
                cardinality(p.plan_jalur) > 0
                OR (
                  p.plan_choices IS NOT NULL
                  AND jsonb_typeof(p.plan_choices) = 'array'
                  AND jsonb_array_length(p.plan_choices) > 0
                )
              )
        )
    `
	finalCondition := `
        EXISTS (
            SELECT 1 FROM student_outcomes o
            WHERE o.school_id = c.school_id
              AND o.tahun_lulus = c.tahun_lulus
              AND o.nisn = c.nisn
              AND o.finalized_at IS NOT NULL
        )
    `

	addRaw("NOT " + finalCondition)
	if hasFinalRequests {
		addRaw(`
			NOT EXISTS (
				SELECT 1 FROM student_final_requests r
				WHERE r.student_id = c.id
				  AND r.status = 'APPROVED'
			)
		`)
	}
	if !filters.IncludePlanned {
		addRaw("NOT " + planCondition)
	}

	countQuery := "SELECT COUNT(*) FROM class12_students c JOIN data_sekolah ds ON ds.id = c.school_id" + whereClause
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

	query += whereClause + " ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaCohortListItem, 0)
	for rows.Next() {
		var item SiswaCohortListItem
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.SchoolName,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.TahunLulus,
			&item.Status,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, total, nil
}

func (s SiswaService) ListApplications(ctx context.Context, user auth.User, filters SiswaApplicationListFilters) ([]SiswaApplicationListItem, int, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT a.id,
               a.school_id,
               COALESCE(c.nisn, '') AS nisn,
               COALESCE(c.nama_siswa, '') AS nama_siswa,
               COALESCE(c.rombel_siswa, '') AS rombel_siswa,
               COALESCE(a.jalur_penerimaan::text, '') AS jalur_penerimaan,
               COALESCE(a.nama_perguruan_tinggi, '') AS nama_perguruan_tinggi,
               COALESCE(f.final_type, '') AS final_status,
               f.finalized_at,
               CASE
                 WHEN EXISTS (
                   SELECT 1 FROM student_final_outcomes f
                   WHERE f.school_id = a.school_id AND f.cohort_id = a.cohort_id
                 ) THEN 'Final'
                 WHEN EXISTS (
                   SELECT 1 FROM student_higher_ed_acceptance_offers o
                   WHERE o.school_id = a.school_id AND o.cohort_id = a.cohort_id
                 ) THEN 'Menunggu Final'
                 ELSE 'Belum Diterima'
               END AS status
        FROM student_higher_ed_applications a
        JOIN student_cohorts c
          ON c.id = a.cohort_id
         AND c.school_id = a.school_id
        JOIN data_sekolah ds ON ds.id = a.school_id
        LEFT JOIN student_final_outcomes f
          ON f.school_id = a.school_id
         AND f.cohort_id = a.cohort_id
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

	if !scope.All {
		if len(scope.IDs) == 0 {
			return []SiswaApplicationListItem{}, 0, nil
		}
		addClause(fmt.Sprintf("a.school_id = ANY($%d)", idx), scope.IDs)
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

	countQuery := "SELECT COUNT(*) FROM student_higher_ed_applications a JOIN student_cohorts c ON c.id = a.cohort_id AND c.school_id = a.school_id JOIN data_sekolah ds ON ds.id = a.school_id" + whereClause
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

	query += whereClause + " ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC, a.pilihan_ke ASC, a.id ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaApplicationListItem, 0)
	for rows.Next() {
		var item SiswaApplicationListItem
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.NISN,
			&item.NamaSiswa,
			&item.Rombel,
			&item.JalurPenerimaan,
			&item.NamaPerguruanTinggi,
			&item.FinalStatus,
			&item.FinalizedAt,
			&item.Status,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, total, nil
}
