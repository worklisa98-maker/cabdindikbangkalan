package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/logging"
)

type SiswaOutcomeExportFilters struct {
	SchoolID      *int
	Status        string
	Year          int
	Search        string
	Jalur         string
	Jenjang       string
	StatusSekolah string
}

type SiswaOutcomeExportRow struct {
	NPSN             string
	NamaSekolah      string
	BentukPendidikan string
	StatusSekolah    string
	NISN             string
	NamaSiswa        string
	Rombel           string
	TahunLulus       int
	StatusFinal      string
	JalurFinal       string
	PTN              string
	Prodi            string
	JalurPT          string
	TahunDiterimaPT  *int
	Perusahaan       string
	Jabatan          string
	TahunMulaiKerja  *int
	BidangUsaha      string
	NamaUsaha        string
	TahunMulaiUsaha  *int
	FinalizedAt      *time.Time
}

type SiswaPTRankingFilters struct {
	SchoolID      *int
	Year          int
	Jenjang       string
	StatusSekolah string
	Limit         int
}

type SiswaPTRankingItem struct {
	PTNName string `json:"ptn_name"`
	Count   int    `json:"count"`
}

func normalizeOutcomeExportStatus(raw string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	switch value {
	case "ALL":
		return "ALL", nil
	case "PT", "KERJA", "WIRAUSAHA", "BELUM":
		return value, nil
	default:
		return "", ErrInvalidOutcome
	}
}

func (s SiswaService) ExportOutcomesRows(
	ctx context.Context,
	user auth.User,
	filters SiswaOutcomeExportFilters,
	limit int,
) ([]SiswaOutcomeExportRow, int, error) {
	statusFilter, err := normalizeOutcomeExportStatus(filters.Status)
	if err != nil {
		return nil, 0, err
	}

	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return nil, 0, err
	}

	var selectedSchoolID *int
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return nil, 0, auth.ErrForbidden
		}
		schoolID := *filters.SchoolID
		selectedSchoolID = &schoolID
	}

	requiredTables := []string{
		"class12_students",
		"student_outcomes",
		"data_sekolah",
		"outcome_pt_details",
		"outcome_work_details",
		"outcome_business_details",
	}
	if statusFilter == "PT" {
		requiredTables = append(requiredTables, "outcome_pt_offers", "student_higher_ed_applications", "student_cohorts")
	}
	ok, missing, err := tablesExist(ctx, s.DB, requiredTables...)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "export_rows", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaOutcomeExportRow{}, 0, nil
	}
	if statusFilter == "PT" {
		return s.exportPTOutcomesRows(ctx, user, filters, limit, scope, selectedSchoolID)
	}

	fromQuery := `
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
    `

	conditions := []string{"o.finalized_at IS NOT NULL"}
	args := make([]interface{}, 0)
	addClause := func(clause string, value interface{}) {
		conditions = append(conditions, fmt.Sprintf(clause, len(args)+1))
		args = append(args, value)
	}

	switch statusFilter {
	case "PT":
		conditions = append(conditions, "(o.final_status = 'PT' OR pt.id IS NOT NULL)")
	case "KERJA":
		conditions = append(conditions, "(o.final_status = 'KERJA' OR wk.id IS NOT NULL)")
	case "WIRAUSAHA":
		conditions = append(conditions, "(o.final_status = 'WIRAUSAHA' OR wb.id IS NOT NULL)")
	case "BELUM":
		conditions = append(conditions, "o.final_status = 'BELUM'")
	}

	if selectedSchoolID != nil {
		addClause("c.school_id = $%d", *selectedSchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return []SiswaOutcomeExportRow{}, 0, nil
		}
		addClause("c.school_id = ANY($%d)", scope.SchoolIDs)
	}

	if user.Role == auth.RoleAdminCabdin {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			conditions = append(conditions, "1=0")
		} else {
			addClause("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", allowed)
		}
	}

	if auth.IsAdmin(user) {
		jenjang := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
		if jenjang != "" && jenjang != "ALL" {
			addClause("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", jenjang)
		}
		statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if statusSekolah != "" && statusSekolah != "ALL" {
			addClause("UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $%d", statusSekolah)
		}
	}

	if filters.Year > 0 {
		addClause("c.tahun_lulus = $%d", filters.Year)
	}

	if search := strings.ToLower(strings.TrimSpace(filters.Search)); search != "" {
		term := "%" + search + "%"
		addClause(`(
            LOWER(COALESCE(c.nama_siswa, '')) LIKE $%d OR
            LOWER(COALESCE(c.nisn, '')) LIKE $%d OR
            LOWER(COALESCE(ds.nama_sekolah, '')) LIKE $%d OR
            LOWER(COALESCE(pt.kampus, '')) LIKE $%d OR
            LOWER(COALESCE(pt.prodi, '')) LIKE $%d OR
            LOWER(COALESCE(wk.perusahaan, '')) LIKE $%d OR
            LOWER(COALESCE(wb.nama_usaha, '')) LIKE $%d
        )`, term)
	}

	if jalurFilter := strings.ToUpper(strings.TrimSpace(filters.Jalur)); jalurFilter != "" && jalurFilter != "ALL" {
		if statusFilter != "PT" && statusFilter != "ALL" {
			conditions = append(conditions, "1=0")
		} else {
			conditions = append(conditions, "(o.final_status = 'PT' OR pt.id IS NOT NULL)")
			addClause("UPPER(TRIM(COALESCE(pt.jalur::text, o.final_path::text, ''))) = $%d", jalurFilter)
		}
	}

	whereQuery := " WHERE " + strings.Join(conditions, " AND ")
	countQuery := "SELECT COUNT(*) " + fromQuery + whereQuery

	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit > 0 && total > limit {
		return nil, total, ErrExportLimitExceeded
	}

	query := `
        SELECT COALESCE(ds.npsn_sekolah, ''),
               COALESCE(ds.nama_sekolah, ''),
               COALESCE(ds.bentuk_pendidikan, ''),
               COALESCE(ds.status_sekolah, ''),
               COALESCE(c.nisn, ''),
               COALESCE(c.nama_siswa, ''),
               COALESCE(c.rombel, ''),
               COALESCE(c.tahun_lulus, 0),
               COALESCE(o.final_status, ''),
               COALESCE(o.final_path::text, ''),
               COALESCE(pt.kampus, ''),
               COALESCE(pt.prodi, ''),
               COALESCE(pt.jalur::text, ''),
               pt.tahun_diterima,
               COALESCE(wk.perusahaan, ''),
               COALESCE(wk.jabatan, ''),
               wk.tahun_mulai,
               COALESCE(wb.bidang_usaha, ''),
               COALESCE(wb.nama_usaha, ''),
               wb.tahun_mulai,
               o.finalized_at
    ` + fromQuery + whereQuery + `
        ORDER BY c.tahun_lulus DESC, c.nama_siswa ASC
    `

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaOutcomeExportRow, 0, total)
	for rows.Next() {
		var row SiswaOutcomeExportRow
		if err := rows.Scan(
			&row.NPSN,
			&row.NamaSekolah,
			&row.BentukPendidikan,
			&row.StatusSekolah,
			&row.NISN,
			&row.NamaSiswa,
			&row.Rombel,
			&row.TahunLulus,
			&row.StatusFinal,
			&row.JalurFinal,
			&row.PTN,
			&row.Prodi,
			&row.JalurPT,
			&row.TahunDiterimaPT,
			&row.Perusahaan,
			&row.Jabatan,
			&row.TahunMulaiKerja,
			&row.BidangUsaha,
			&row.NamaUsaha,
			&row.TahunMulaiUsaha,
			&row.FinalizedAt,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}

	return result, total, nil
}

func (s SiswaService) exportPTOutcomesRows(
	ctx context.Context,
	user auth.User,
	filters SiswaOutcomeExportFilters,
	limit int,
	scope auth.MultiSchoolScope,
	selectedSchoolID *int,
) ([]SiswaOutcomeExportRow, int, error) {
	query := buildFinalPTReconciledCTE() + `
		SELECT x.npsn,
		       x.school_name,
		       x.school_level,
		       x.school_status,
		       x.nisn,
		       x.nama_siswa,
		       x.rombel,
		       x.tahun_lulus,
		       x.final_status,
		       x.final_path,
		       x.pt_kampus,
		       x.pt_prodi,
		       x.pt_jalur,
		       x.pt_tahun_diterima,
		       ''::text AS perusahaan,
		       ''::text AS jabatan,
		       NULL::integer AS tahun_mulai_kerja,
		       ''::text AS bidang_usaha,
		       ''::text AS nama_usaha,
		       NULL::integer AS tahun_mulai_usaha,
		       x.finalized_at
		FROM final_pt x
		WHERE 1=1
	`

	args := make([]interface{}, 0)
	idx := 1
	addClause := func(condition string, value interface{}) {
		query += " AND " + condition
		args = append(args, value)
		idx++
	}

	if selectedSchoolID != nil {
		addClause(fmt.Sprintf("x.school_id = $%d", idx), *selectedSchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return []SiswaOutcomeExportRow{}, 0, nil
		}
		addClause(fmt.Sprintf("x.school_id = ANY($%d)", idx), scope.SchoolIDs)
	}

	if user.Role == auth.RoleAdminCabdin {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			query += " AND 1=0"
		} else {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(x.school_level, ''))) = ANY($%d)", idx), allowed)
		}
	}

	if auth.IsAdmin(user) {
		jenjang := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
		if jenjang != "" && jenjang != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(x.school_level, ''))) = $%d", idx), jenjang)
		}
		statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if statusSekolah != "" && statusSekolah != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(x.school_status, ''))) = $%d", idx), statusSekolah)
		}
	}

	if filters.Year > 0 {
		addClause(fmt.Sprintf("x.tahun_lulus = $%d", idx), filters.Year)
	}

	if search := strings.ToLower(strings.TrimSpace(filters.Search)); search != "" {
		term := "%" + search + "%"
		query += fmt.Sprintf(
			" AND (LOWER(COALESCE(x.nama_siswa, '')) LIKE $%d OR LOWER(COALESCE(x.nisn, '')) LIKE $%d OR LOWER(COALESCE(x.school_name, '')) LIKE $%d OR LOWER(COALESCE(x.pt_kampus, '')) LIKE $%d OR LOWER(COALESCE(x.pt_prodi, '')) LIKE $%d)",
			idx, idx+1, idx+2, idx+3, idx+4,
		)
		args = append(args, term, term, term, term, term)
		idx += 5
	}

	if rawJalur := strings.TrimSpace(filters.Jalur); rawJalur != "" && !strings.EqualFold(rawJalur, "ALL") {
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
		jalurCase := `
			CASE
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%snbp%' THEN 'SNBP'
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%snbt%' THEN 'SNBT'
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%mandiri%' OR LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%snbm%' THEN 'SNBM (Mandiri)'
			  WHEN LOWER(COALESCE(x.pt_jalur, x.final_path, '')) LIKE '%kedinas%' THEN 'Kedinasan'
			  ELSE 'Lainnya'
			END
		`
		addClause(fmt.Sprintf("(%s) = $%d", jalurCase, idx), jalurFilter)
	}

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS filtered"
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit > 0 && total > limit {
		return nil, total, ErrExportLimitExceeded
	}

	query += " ORDER BY x.tahun_lulus DESC, x.nama_siswa ASC"
	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]SiswaOutcomeExportRow, 0, total)
	for rows.Next() {
		var row SiswaOutcomeExportRow
		if err := rows.Scan(
			&row.NPSN,
			&row.NamaSekolah,
			&row.BentukPendidikan,
			&row.StatusSekolah,
			&row.NISN,
			&row.NamaSiswa,
			&row.Rombel,
			&row.TahunLulus,
			&row.StatusFinal,
			&row.JalurFinal,
			&row.PTN,
			&row.Prodi,
			&row.JalurPT,
			&row.TahunDiterimaPT,
			&row.Perusahaan,
			&row.Jabatan,
			&row.TahunMulaiKerja,
			&row.BidangUsaha,
			&row.NamaUsaha,
			&row.TahunMulaiUsaha,
			&row.FinalizedAt,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return result, total, nil
}

func (s SiswaService) PTRanking(
	ctx context.Context,
	user auth.User,
	filters SiswaPTRankingFilters,
) ([]SiswaPTRankingItem, error) {
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return nil, err
	}

	var selectedSchoolID *int
	if filters.SchoolID != nil {
		if err := auth.EnsureSchoolInScope(scope, *filters.SchoolID); err != nil {
			return nil, auth.ErrForbidden
		}
		schoolID := *filters.SchoolID
		selectedSchoolID = &schoolID
	}

	ok, missing, err := tablesExist(
		ctx,
		s.DB,
		"student_outcomes",
		"outcome_pt_details",
		"outcome_pt_offers",
		"class12_students",
		"student_higher_ed_applications",
		"student_cohorts",
		"data_sekolah",
	)
	if err != nil {
		return nil, err
	}
	if !ok {
		logging.Warnf("siswa_outcomes", "pt_ranking", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return []SiswaPTRankingItem{}, nil
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 2000
	}
	if limit > 2000 {
		limit = 2000
	}

	query := buildFinalPTReconciledCTE() + `
        SELECT TRIM(COALESCE(x.pt_kampus, '')) AS ptn_name,
               COUNT(*) AS total
        FROM final_pt x
        WHERE TRIM(COALESCE(x.pt_kampus, '')) <> ''
    `

	args := make([]interface{}, 0)
	addClause := func(clause string, value interface{}) {
		query += fmt.Sprintf(" AND "+clause, len(args)+1)
		args = append(args, value)
	}

	if selectedSchoolID != nil {
		addClause("x.school_id = $%d", *selectedSchoolID)
	} else if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return []SiswaPTRankingItem{}, nil
		}
		addClause("x.school_id = ANY($%d)", scope.SchoolIDs)
	}

	if user.Role == auth.RoleAdminCabdin {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			query += " AND 1=0"
		} else {
			addClause("UPPER(TRIM(COALESCE(x.school_level, ''))) = ANY($%d)", allowed)
		}
	}

	if auth.IsAdmin(user) {
		jenjang := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
		if jenjang != "" && jenjang != "ALL" {
			addClause("UPPER(TRIM(COALESCE(x.school_level, ''))) = $%d", jenjang)
		}
		statusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if statusSekolah != "" && statusSekolah != "ALL" {
			addClause("UPPER(TRIM(COALESCE(x.school_status, ''))) = $%d", statusSekolah)
		}
	}

	if filters.Year > 0 {
		addClause("x.tahun_lulus = $%d", filters.Year)
	}

	query += fmt.Sprintf(" GROUP BY ptn_name ORDER BY total DESC, ptn_name ASC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]SiswaPTRankingItem, 0)
	for rows.Next() {
		var item SiswaPTRankingItem
		if err := rows.Scan(&item.PTNName, &item.Count); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, nil
}
