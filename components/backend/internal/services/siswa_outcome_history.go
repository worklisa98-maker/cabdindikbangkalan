package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type SiswaOutcomeImportHistoryFilters struct {
	SchoolID *int
	Page     int
	PageSize int
	Search   string
	Year     int
}

type SiswaOutcomeImportHistoryItem struct {
	ID          int64     `json:"id"`
	SchoolID    int       `json:"school_id"`
	SchoolName  string    `json:"school_name"`
	SchoolLevel string    `json:"school_level"`
	NISN        string    `json:"nisn"`
	NamaSiswa   string    `json:"nama_siswa"`
	Rombel      string    `json:"rombel"`
	TahunLulus  int       `json:"tahun_lulus"`
	FinalStatus string    `json:"final_status"`
	FinalPath   string    `json:"final_path"`
	ChangedAt   time.Time `json:"changed_at"`
	ChangedBy   string    `json:"changed_by"`
}

func (s SiswaService) ListOutcomeImportHistory(
	ctx context.Context,
	user auth.User,
	filters SiswaOutcomeImportHistoryFilters,
) ([]SiswaOutcomeImportHistoryItem, int, error) {
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
		"student_outcome_audit",
		"class12_students",
		"data_sekolah",
		"users",
		"student_outcomes",
	)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "import_history", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaOutcomeImportHistoryItem{}, 0, nil
	}

	conditions := []string{"a.reason ILIKE 'IMPORT%'"}
	args := make([]interface{}, 0)
	addClause := func(condition string, value interface{}) {
		conditions = append(conditions, fmt.Sprintf(condition, len(args)+1))
		args = append(args, value)
	}

	if scope != nil {
		addClause("a.school_id = $%d", *scope)
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			conditions = append(conditions, "1=0")
		} else {
			addClause("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", allowed)
		}
	}
	if filters.Year > 0 {
		addClause("a.tahun_lulus = $%d", filters.Year)
	}
	if strings.TrimSpace(filters.Search) != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		conditions = append(conditions, fmt.Sprintf("(LOWER(COALESCE(c.nisn, '')) LIKE $%d OR LOWER(COALESCE(c.nama_siswa, '')) LIKE $%d)", len(args)+1, len(args)+2))
		args = append(args, search, search)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := `
        SELECT COUNT(*)
        FROM student_outcome_audit a
        JOIN data_sekolah ds ON ds.id = a.school_id
        LEFT JOIN class12_students c
          ON c.school_id = a.school_id
         AND c.tahun_lulus = a.tahun_lulus
         AND c.nisn = a.nisn
    ` + whereClause
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

	dataArgs := append([]interface{}{}, args...)
	limitPos := len(dataArgs) + 1
	offsetPos := len(dataArgs) + 2
	dataArgs = append(dataArgs, pageSize, offset)

	query := fmt.Sprintf(`
        SELECT a.id,
               a.school_id,
               COALESCE(ds.nama_sekolah, '') AS school_name,
               COALESCE(ds.bentuk_pendidikan, '') AS school_level,
               COALESCE(c.nisn, '') AS nisn,
               COALESCE(c.nama_siswa, '') AS nama_siswa,
               COALESCE(c.rombel, '') AS rombel,
               COALESCE(c.tahun_lulus, 0) AS tahun_lulus,
               COALESCE(o.final_status, '') AS final_status,
               COALESCE(o.final_path::text, '') AS final_path,
               a.changed_at,
               COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS changed_by
        FROM student_outcome_audit a
        JOIN data_sekolah ds ON ds.id = a.school_id
        LEFT JOIN class12_students c
          ON c.school_id = a.school_id
         AND c.tahun_lulus = a.tahun_lulus
         AND c.nisn = a.nisn
        LEFT JOIN users u ON u.id = a.changed_by
        LEFT JOIN student_outcomes o
          ON o.school_id = a.school_id
         AND o.tahun_lulus = a.tahun_lulus
         AND o.nisn = a.nisn
        %s
        ORDER BY a.changed_at DESC
        LIMIT $%d OFFSET $%d
    `, whereClause, limitPos, offsetPos)

	rows, err := s.DB.Pool.Query(ctx, query, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaOutcomeImportHistoryItem, 0)
	for rows.Next() {
		var item SiswaOutcomeImportHistoryItem
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
			&item.ChangedAt,
			&item.ChangedBy,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, item)
	}

	return result, total, nil
}
