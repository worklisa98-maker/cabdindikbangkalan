package services

import (
	"context"
	"fmt"
	"strings"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

type AdmissionService struct {
	DB *db.DB
}

type Admission struct {
	ID                  string
	SchoolID            int
	NamaSekolah         string
	JenjangPendidikan   string
	NamaSiswa           string
	RombelSiswa         string
	TahunLulus          int
	NamaPerguruanTinggi string
	Jurusan             string
	TahunDiterima       int
	JalurPenerimaan     string
}

type AdmissionFilters struct {
	SchoolID *int
	Jenjang  string
	Tahun    int
	Jalur    string
	Status   string
}

type AdmissionListFilters struct {
	SchoolID      *int
	Year          int
	PtName        string
	Jenjang       string
	StatusSekolah string
	Search        string
	Page          int
	PageSize      int
}

type AdmissionExportFilters struct {
	SchoolID      *int
	Year          int
	PtName        string
	Search        string
	Jalur         string
	Jenjang       string
	StatusSekolah string
}

type AdmissionExportRow struct {
	NamaSekolah         string
	NISNSiswa           string
	NamaSiswa           string
	NamaPerguruanTinggi string
	Jurusan             string
	JalurPenerimaan     string
	TahunDiterima       int
}

type AdmissionListItem struct {
	ID                  string
	SchoolID            int
	NamaSekolah         string
	NISNSiswa           string
	NamaSiswa           string
	NamaPerguruanTinggi string
	Jurusan             string
	JalurPenerimaan     string
	TahunDiterima       int
}

type AdmissionSummary struct {
	Year      int
	SNBP      int
	SNBT      int
	MANDIRI   int
	KEDINASAN int
	LAINNYA   int
}

type AdmissionUniversitySummary struct {
	UniversityName string
	SNBP           int
	SNBT           int
	MANDIRI        int
	KEDINASAN      int
	LAINNYA        int
	Total          int
}

type UniversitySuggestion struct {
	Nama string
}

func (s AdmissionService) List(ctx context.Context, user auth.User, filters AdmissionFilters) ([]Admission, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, err
	}

	query := `
        SELECT spt.id,
               spt.school_id,
               ds.nama_sekolah,
               COALESCE(spt.jenjang_pendidikan, ds.bentuk_pendidikan, '') AS jenjang_pendidikan,
               spt.nama_siswa,
               COALESCE(spt.rombel_siswa, '') AS rombel_siswa,
               COALESCE(spt.tahun_lulus, 0) AS tahun_lulus,
               spt.nama_perguruan_tinggi,
               COALESCE(spt.jurusan_perguruan_tinggi, '') AS jurusan,
               COALESCE(spt.tahun_diterima, 0) AS tahun_diterima,
               spt.jalur_penerimaan
        FROM siswa_perguruan_tinggi spt
        JOIN data_sekolah ds ON ds.id = spt.school_id
    `

	var args []interface{}
	idx := 1
	if !scope.All {
		if len(scope.IDs) == 0 {
			return []Admission{}, nil
		}
		query += " WHERE spt.school_id = ANY($1)"
		args = append(args, scope.IDs)
		idx++
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			if len(args) == 0 {
				query += " WHERE 1=0"
			} else {
				query += " AND 1=0"
			}
		} else {
			clause := fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx)
			if len(args) == 0 {
				query += " WHERE " + clause
			} else {
				query += " AND " + clause
			}
			args = append(args, allowed)
			idx++
		}
	}
	if filters.Jalur != "" {
		clause := fmt.Sprintf("spt.jalur_penerimaan = $%d", idx)
		if len(args) == 0 {
			query += " WHERE " + clause
		} else {
			query += " AND " + clause
		}
		args = append(args, strings.ToUpper(filters.Jalur))
		idx++
	}
	if filters.Jenjang != "" {
		clause := fmt.Sprintf("COALESCE(spt.jenjang_pendidikan, ds.bentuk_pendidikan, '') = $%d", idx)
		if len(args) == 0 {
			query += " WHERE " + clause
		} else {
			query += " AND " + clause
		}
		args = append(args, filters.Jenjang)
		idx++
	}
	if filters.Status != "" && strings.ToUpper(strings.TrimSpace(filters.Status)) != "ALL" {
		status := strings.ToUpper(strings.TrimSpace(filters.Status))
		clause := fmt.Sprintf("UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $%d", idx)
		if len(args) == 0 {
			query += " WHERE " + clause
		} else {
			query += " AND " + clause
		}
		args = append(args, status)
		idx++
	}
	if filters.Tahun != 0 {
		clause := fmt.Sprintf("spt.tahun_diterima = $%d", idx)
		if len(args) == 0 {
			query += " WHERE " + clause
		} else {
			query += " AND " + clause
		}
		args = append(args, filters.Tahun)
	}

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Admission
	for rows.Next() {
		var item Admission
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.NamaSekolah,
			&item.JenjangPendidikan,
			&item.NamaSiswa,
			&item.RombelSiswa,
			&item.TahunLulus,
			&item.NamaPerguruanTinggi,
			&item.Jurusan,
			&item.TahunDiterima,
			&item.JalurPenerimaan,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, nil
}

func (s AdmissionService) ListTable(ctx context.Context, user auth.User, filters AdmissionListFilters) ([]AdmissionListItem, int, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT spt.id,
               spt.school_id,
               ds.nama_sekolah,
               COALESCE(spt.nisn_siswa, '') AS nisn_siswa,
               COALESCE(spt.nama_siswa, '') AS nama_siswa,
               COALESCE(spt.nama_perguruan_tinggi, '') AS nama_perguruan_tinggi,
               COALESCE(spt.jurusan_perguruan_tinggi, '') AS jurusan_perguruan_tinggi,
               COALESCE(spt.jalur_penerimaan::text, '') AS jalur_penerimaan,
               COALESCE(spt.tahun_diterima, 0) AS tahun_diterima
        FROM siswa_perguruan_tinggi spt
        JOIN data_sekolah ds ON ds.id = spt.school_id
    `

	whereClause := ""
	var args []interface{}
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
			return []AdmissionListItem{}, 0, nil
		}
		addClause(fmt.Sprintf("spt.school_id = ANY($%d)", idx), scope.IDs)
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
	if auth.IsAdmin(user) {
		normalizedJenjang := strings.ToUpper(strings.TrimSpace(filters.Jenjang))
		if normalizedJenjang != "" && normalizedJenjang != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx), normalizedJenjang)
		}
		normalizedStatusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if normalizedStatusSekolah != "" && normalizedStatusSekolah != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $%d", idx), normalizedStatusSekolah)
		}
	}
	if filters.Year != 0 {
		addClause(fmt.Sprintf("spt.tahun_diterima = $%d", idx), filters.Year)
	}
	if filters.PtName != "" && filters.PtName != "ALL" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.PtName)) + "%"
		condition := fmt.Sprintf("LOWER(COALESCE(spt.nama_perguruan_tinggi, '')) LIKE $%d", idx)
		addClause(condition, search)
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		condition := fmt.Sprintf("(LOWER(COALESCE(spt.nisn_siswa, '')) LIKE $%d OR LOWER(COALESCE(spt.nama_siswa, '')) LIKE $%d)", idx, idx+1)
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, search, search)
		idx += 2
	}

	countQuery := "SELECT COUNT(*) FROM siswa_perguruan_tinggi spt JOIN data_sekolah ds ON ds.id = spt.school_id" + whereClause
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
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	query += whereClause + " ORDER BY spt.tahun_diterima DESC, spt.nama_siswa ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []AdmissionListItem
	for rows.Next() {
		var item AdmissionListItem
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.NamaSekolah,
			&item.NISNSiswa,
			&item.NamaSiswa,
			&item.NamaPerguruanTinggi,
			&item.Jurusan,
			&item.JalurPenerimaan,
			&item.TahunDiterima,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, total, nil
}

func (s AdmissionService) ExportRows(ctx context.Context, user auth.User, filters AdmissionExportFilters, limit int) ([]AdmissionExportRow, int, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT ds.nama_sekolah,
               COALESCE(spt.nisn_siswa, '') AS nisn_siswa,
               COALESCE(spt.nama_siswa, '') AS nama_siswa,
               COALESCE(spt.nama_perguruan_tinggi, '') AS nama_perguruan_tinggi,
               COALESCE(spt.jurusan_perguruan_tinggi, '') AS jurusan_perguruan_tinggi,
               COALESCE(spt.jalur_penerimaan::text, '') AS jalur_penerimaan,
               COALESCE(spt.tahun_diterima, 0) AS tahun_diterima
        FROM siswa_perguruan_tinggi spt
        JOIN data_sekolah ds ON ds.id = spt.school_id
    `

	whereClause := ""
	var args []interface{}
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
			return []AdmissionExportRow{}, 0, nil
		}
		addClause(fmt.Sprintf("spt.school_id = ANY($%d)", idx), scope.IDs)
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
		addClause(fmt.Sprintf("spt.tahun_diterima = $%d", idx), filters.Year)
	}
	if filters.PtName != "" && filters.PtName != "ALL" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.PtName)) + "%"
		condition := fmt.Sprintf("LOWER(COALESCE(spt.nama_perguruan_tinggi, '')) LIKE $%d", idx)
		addClause(condition, search)
	}
	if filters.Jalur != "" && filters.Jalur != "ALL" {
		addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(spt.jalur_penerimaan::text, ''))) = $%d", idx), strings.ToUpper(filters.Jalur))
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		condition := fmt.Sprintf("(LOWER(COALESCE(spt.nisn_siswa, '')) LIKE $%d OR LOWER(COALESCE(spt.nama_siswa, '')) LIKE $%d)", idx, idx+1)
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, search, search)
		idx += 2
	}

	countQuery := "SELECT COUNT(*) FROM siswa_perguruan_tinggi spt JOIN data_sekolah ds ON ds.id = spt.school_id" + whereClause
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit > 0 && total > limit {
		return nil, total, ErrExportLimitExceeded
	}

	query += whereClause + " ORDER BY spt.tahun_diterima DESC, spt.nama_siswa ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []AdmissionExportRow
	for rows.Next() {
		var item AdmissionExportRow
		if err := rows.Scan(
			&item.NamaSekolah,
			&item.NISNSiswa,
			&item.NamaSiswa,
			&item.NamaPerguruanTinggi,
			&item.Jurusan,
			&item.JalurPenerimaan,
			&item.TahunDiterima,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, total, nil
}

func (s AdmissionService) SummaryByYear(ctx context.Context, user auth.User, filters AdmissionFilters) ([]AdmissionSummary, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, err
	}

	query := `
        SELECT spt.tahun_diterima,
               SUM(CASE WHEN spt.jalur_penerimaan = 'SNBP' THEN 1 ELSE 0 END) AS snbp,
               SUM(CASE WHEN spt.jalur_penerimaan = 'SNBT' THEN 1 ELSE 0 END) AS snbt,
               SUM(CASE WHEN spt.jalur_penerimaan = 'MANDIRI' THEN 1 ELSE 0 END) AS mandiri,
               SUM(CASE WHEN spt.jalur_penerimaan = 'KEDINASAN' THEN 1 ELSE 0 END) AS kedinasan,
               SUM(CASE WHEN spt.jalur_penerimaan = 'LAINNYA' THEN 1 ELSE 0 END) AS lainnya
        FROM siswa_perguruan_tinggi spt
        JOIN data_sekolah ds ON ds.id = spt.school_id
        WHERE spt.tahun_diterima IS NOT NULL
          AND spt.tahun_diterima <> 0
    `

	var args []interface{}
	idx := 1
	if !scope.All {
		if len(scope.IDs) == 0 {
			return []AdmissionSummary{}, nil
		}
		query += " AND spt.school_id = ANY($1)"
		args = append(args, scope.IDs)
		idx++
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			query += " AND 1=0"
		} else {
			query += fmt.Sprintf(" AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx)
			args = append(args, allowed)
			idx++
		}
	}
	if filters.Jenjang != "" {
		clause := fmt.Sprintf("COALESCE(spt.jenjang_pendidikan, ds.bentuk_pendidikan, '') = $%d", idx)
		query += " AND " + clause
		args = append(args, filters.Jenjang)
		idx++
	}
	if filters.Status != "" && strings.ToUpper(strings.TrimSpace(filters.Status)) != "ALL" {
		status := strings.ToUpper(strings.TrimSpace(filters.Status))
		clause := fmt.Sprintf("UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $%d", idx)
		query += " AND " + clause
		args = append(args, status)
		idx++
	}
	if filters.Tahun != 0 {
		clause := fmt.Sprintf("spt.tahun_diterima = $%d", idx)
		query += " AND " + clause
		args = append(args, filters.Tahun)
		idx++
	}

	query += " GROUP BY spt.tahun_diterima ORDER BY spt.tahun_diterima ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AdmissionSummary
	for rows.Next() {
		var item AdmissionSummary
		if err := rows.Scan(
			&item.Year,
			&item.SNBP,
			&item.SNBT,
			&item.MANDIRI,
			&item.KEDINASAN,
			&item.LAINNYA,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, nil
}

func (s AdmissionService) SummaryByUniversity(ctx context.Context, user auth.User, filters AdmissionFilters) ([]AdmissionUniversitySummary, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, err
	}

	query := `
        SELECT COALESCE(spt.nama_perguruan_tinggi, '') AS nama_pt,
               SUM(CASE WHEN spt.jalur_penerimaan = 'SNBP' THEN 1 ELSE 0 END) AS snbp,
               SUM(CASE WHEN spt.jalur_penerimaan = 'SNBT' THEN 1 ELSE 0 END) AS snbt,
               SUM(CASE WHEN spt.jalur_penerimaan = 'MANDIRI' THEN 1 ELSE 0 END) AS mandiri,
               SUM(CASE WHEN spt.jalur_penerimaan = 'KEDINASAN' THEN 1 ELSE 0 END) AS kedinasan,
               SUM(CASE WHEN spt.jalur_penerimaan = 'LAINNYA' THEN 1 ELSE 0 END) AS lainnya,
               COUNT(*) AS total
        FROM siswa_perguruan_tinggi spt
        JOIN data_sekolah ds ON ds.id = spt.school_id
        WHERE spt.nama_perguruan_tinggi IS NOT NULL
          AND TRIM(spt.nama_perguruan_tinggi) <> ''
    `

	var args []interface{}
	idx := 1
	if !scope.All {
		if len(scope.IDs) == 0 {
			return []AdmissionUniversitySummary{}, nil
		}
		query += " AND spt.school_id = ANY($1)"
		args = append(args, scope.IDs)
		idx++
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			query += " AND 1=0"
		} else {
			query += fmt.Sprintf(" AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = ANY($%d)", idx)
			args = append(args, allowed)
			idx++
		}
	}
	if filters.Jenjang != "" {
		clause := fmt.Sprintf("COALESCE(spt.jenjang_pendidikan, ds.bentuk_pendidikan, '') = $%d", idx)
		query += " AND " + clause
		args = append(args, filters.Jenjang)
		idx++
	}
	if filters.Status != "" && strings.ToUpper(strings.TrimSpace(filters.Status)) != "ALL" {
		status := strings.ToUpper(strings.TrimSpace(filters.Status))
		clause := fmt.Sprintf("UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $%d", idx)
		query += " AND " + clause
		args = append(args, status)
		idx++
	}
	if filters.Tahun != 0 {
		clause := fmt.Sprintf("spt.tahun_diterima = $%d", idx)
		query += " AND " + clause
		args = append(args, filters.Tahun)
		idx++
	}

	query += " GROUP BY nama_pt ORDER BY total DESC, nama_pt ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AdmissionUniversitySummary
	for rows.Next() {
		var item AdmissionUniversitySummary
		if err := rows.Scan(
			&item.UniversityName,
			&item.SNBP,
			&item.SNBT,
			&item.MANDIRI,
			&item.KEDINASAN,
			&item.LAINNYA,
			&item.Total,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, nil
}

func (s AdmissionService) SuggestUniversities(ctx context.Context, user auth.User, query string, limit int) ([]UniversitySuggestion, error) {
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return []UniversitySuggestion{}, nil
		}
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}
	pattern := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"

	sql := `
        SELECT COALESCE(rk.nama, '') AS nama_pt
        FROM ref_kampus rk
        WHERE COALESCE(rk.is_active, TRUE) = TRUE
          AND rk.nama IS NOT NULL
          AND TRIM(rk.nama) <> ''
          AND LOWER(rk.nama) LIKE $1
    `
	args := []interface{}{pattern}
	argIdx := 2
	sql += fmt.Sprintf(" ORDER BY nama_pt ASC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := s.DB.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []UniversitySuggestion
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			result = append(result, UniversitySuggestion{Nama: name})
		}
	}
	return result, nil
}
