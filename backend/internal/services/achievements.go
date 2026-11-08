package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

type AchievementService struct {
	DB *db.DB
}

type Achievement struct {
	IDPrestasi       string
	SchoolID         int
	NamaSekolah      string
	JenisLomba       string
	KategoriPrestasi string
	Jenjang          string
	TingkatLomba     string
	TahunLomba       string
	NamaLomba        string
	Peringkat        string
}

type AchievementFilters struct {
	SchoolID *int
	Jenjang  string
	Tahun    string
	Bentuk   string
}

type AchievementListFilters struct {
	SchoolID      *int
	Year          string
	Tingkat       string
	JenisPeserta  string
	Bentuk        string
	StatusSekolah string
	Search        string
	Page          int
	PageSize      int
}

type AchievementExportFilters struct {
	SchoolID      *int
	Year          string
	Tingkat       string
	JenisPeserta  string
	Bentuk        string
	StatusSekolah string
	Search        string
}

type AchievementExportRow struct {
	NamaSekolah       string
	NamaLomba         string
	TingkatLomba      string
	KategoriPrestasi  string
	PeringkatPrestasi string
	NamaPeserta       string
	TahunLomba        string
	ModePelaksanaan   string
	Penyelenggaraan   string
	JenisPeserta      string
	JenisLomba        string
}

type AchievementListItem struct {
	IDPrestasi       string
	SchoolID         int
	NamaSekolah      string
	NamaLomba        string
	TingkatLomba     string
	KategoriPrestasi string
	Peringkat        string
	NamaPeserta      string
	JenisLomba       string
	TahunLomba       string
}

type AchievementAnalyticsFilters struct {
	SchoolID   *int
	Year       *int
	Kategori   string
	JenisLomba string
	Bentuk     string
}

type AchievementTrendPoint struct {
	Year          int `json:"year"`
	Kabupaten     int `json:"KABUPATEN"`
	Provinsi      int `json:"PROVINSI"`
	Nasional      int `json:"NASIONAL"`
	Internasional int `json:"INTERNASIONAL"`
}

type AchievementNamedValue struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type AchievementLevelYearValue struct {
	Tingkat string `json:"tingkat"`
	Tahun   int    `json:"tahun"`
	Total   int    `json:"total"`
}

type AchievementAnalytics struct {
	TrendByYear           []AchievementTrendPoint     `json:"trendByYear"`
	LevelComparison       []AchievementNamedValue     `json:"levelComparison"`
	LevelComparisonByYear []AchievementLevelYearValue `json:"byTingkatByYear"`
	CategoryComposition   []AchievementNamedValue     `json:"categoryComposition"`
}

type AchievementAdminSummaryFilters struct {
	SchoolID *int
	Year     string
	Kategori string
	Bentuk   string
	Status   string
}

type AchievementJenjangTingkat struct {
	Jenjang       string `json:"jenjang"`
	Kabupaten     int    `json:"kabupaten"`
	Provinsi      int    `json:"provinsi"`
	Nasional      int    `json:"nasional"`
	Internasional int    `json:"internasional"`
}

type AchievementSchoolDetail struct {
	NamaSekolah   string `json:"nama_sekolah"`
	Kabupaten     int    `json:"kabupaten"`
	Provinsi      int    `json:"provinsi"`
	Nasional      int    `json:"nasional"`
	Internasional int    `json:"internasional"`
	Total         int    `json:"total"`
}

type AchievementAdminSummary struct {
	RekapJenjangTingkat []AchievementJenjangTingkat `json:"rekapJenjangTingkat"`
	DetailSekolah       []AchievementSchoolDetail   `json:"detailSekolah"`
}

func (s AchievementService) List(ctx context.Context, user auth.User, filters AchievementFilters) ([]Achievement, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, err
	}

	query := `
        SELECT ps.id_prestasi,
               ps.school_id,
               ds.nama_sekolah,
               COALESCE(ps.jenis_lomba, 'Siswa') AS jenis_lomba,
               COALESCE(ps.kategori_prestasi, '') AS kategori_prestasi,
               COALESCE(ps.jenjang_pendidikan, ds.bentuk_pendidikan, '') AS jenjang_pendidikan,
               COALESCE(ps.tingkat_lomba, '') AS tingkat_lomba,
               COALESCE(ps.tahun_lomba, '') AS tahun_lomba,
               COALESCE(ps.nama_lomba, '') AS nama_lomba,
               COALESCE(ps.peringkat_prestasi, '') AS peringkat_prestasi
        FROM prestasi_siswa ps
        JOIN data_sekolah ds ON ds.id = ps.school_id
    `

	var args []interface{}
	idx := 1
	if !scope.All {
		if len(scope.IDs) == 0 {
			return []Achievement{}, nil
		}
		query += " WHERE ps.school_id = ANY($1)"
		args = append(args, scope.IDs)
		idx++
	}
	if filters.Jenjang != "" {
		clause := fmt.Sprintf("COALESCE(ps.jenjang_pendidikan, ds.bentuk_pendidikan, '') = $%d", idx)
		if len(args) == 0 {
			query += " WHERE " + clause
		} else {
			query += " AND " + clause
		}
		args = append(args, filters.Jenjang)
		idx++
	}
	if filters.Tahun != "" {
		clause := fmt.Sprintf("ps.tahun_lomba = $%d", idx)
		if len(args) == 0 {
			query += " WHERE " + clause
		} else {
			query += " AND " + clause
		}
		args = append(args, filters.Tahun)
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
	if auth.IsAdmin(user) {
		normalizedBentuk := strings.ToUpper(strings.TrimSpace(filters.Bentuk))
		if normalizedBentuk != "" && normalizedBentuk != "ALL" {
			clause := fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx)
			if len(args) == 0 {
				query += " WHERE " + clause
			} else {
				query += " AND " + clause
			}
			args = append(args, normalizedBentuk)
			idx++
		}
	}

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Achievement
	for rows.Next() {
		var item Achievement
		if err := rows.Scan(
			&item.IDPrestasi,
			&item.SchoolID,
			&item.NamaSekolah,
			&item.JenisLomba,
			&item.KategoriPrestasi,
			&item.Jenjang,
			&item.TingkatLomba,
			&item.TahunLomba,
			&item.NamaLomba,
			&item.Peringkat,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, nil
}

func (s AchievementService) ListTable(ctx context.Context, user auth.User, filters AchievementListFilters) ([]AchievementListItem, int, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT ps.id_prestasi,
               ps.school_id,
               ds.nama_sekolah,
               COALESCE(ps.nama_lomba, '') AS nama_lomba,
               COALESCE(ps.tingkat_lomba, '') AS tingkat_lomba,
               COALESCE(ps.kategori_prestasi, '') AS kategori_prestasi,
               COALESCE(ps.peringkat_prestasi, '') AS peringkat_prestasi,
               COALESCE(ps.nama_peserta, '') AS nama_peserta,
               COALESCE(ps.jenis_lomba, 'Siswa') AS jenis_lomba,
               COALESCE(ps.tahun_lomba, '') AS tahun_lomba
        FROM prestasi_siswa ps
        JOIN data_sekolah ds ON ds.id = ps.school_id
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
			return []AchievementListItem{}, 0, nil
		}
		addClause(fmt.Sprintf("ps.school_id = ANY($%d)", idx), scope.IDs)
	}
	if filters.Year != "" && filters.Year != "ALL" {
		addClause(fmt.Sprintf("ps.tahun_lomba = $%d", idx), filters.Year)
	}
	if filters.Tingkat != "" && filters.Tingkat != "ALL" {
		addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) = $%d", idx), strings.ToUpper(filters.Tingkat))
	}
	if filters.JenisPeserta != "" && filters.JenisPeserta != "ALL" {
		addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ps.jenis_peserta, ''))) = $%d", idx), strings.ToUpper(filters.JenisPeserta))
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
		normalizedBentuk := strings.ToUpper(strings.TrimSpace(filters.Bentuk))
		if normalizedBentuk != "" && normalizedBentuk != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx), normalizedBentuk)
		}
		normalizedStatusSekolah := strings.ToUpper(strings.TrimSpace(filters.StatusSekolah))
		if normalizedStatusSekolah != "" && normalizedStatusSekolah != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $%d", idx), normalizedStatusSekolah)
		}
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		condition := fmt.Sprintf("(LOWER(COALESCE(ps.nama_lomba, '')) LIKE $%d OR LOWER(COALESCE(ps.nama_peserta, '')) LIKE $%d)", idx, idx+1)
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, search, search)
		idx += 2
	}

	countQuery := "SELECT COUNT(*) FROM prestasi_siswa ps JOIN data_sekolah ds ON ds.id = ps.school_id" + whereClause
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

	query += whereClause + " ORDER BY ps.tahun_lomba DESC, ps.nama_lomba ASC LIMIT $" + fmt.Sprint(idx) + " OFFSET $" + fmt.Sprint(idx+1)
	args = append(args, pageSize, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []AchievementListItem
	for rows.Next() {
		var item AchievementListItem
		if err := rows.Scan(
			&item.IDPrestasi,
			&item.SchoolID,
			&item.NamaSekolah,
			&item.NamaLomba,
			&item.TingkatLomba,
			&item.KategoriPrestasi,
			&item.Peringkat,
			&item.NamaPeserta,
			&item.JenisLomba,
			&item.TahunLomba,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, total, nil
}

func (s AchievementService) ExportRows(ctx context.Context, user auth.User, filters AchievementExportFilters, limit int) ([]AchievementExportRow, int, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return nil, 0, err
	}

	query := `
        SELECT ds.nama_sekolah,
               COALESCE(ps.nama_lomba, '') AS nama_lomba,
               COALESCE(ps.tingkat_lomba, '') AS tingkat_lomba,
               COALESCE(ps.kategori_prestasi, '') AS kategori_prestasi,
               COALESCE(ps.peringkat_prestasi, '') AS peringkat_prestasi,
               COALESCE(ps.nama_peserta, '') AS nama_peserta,
               COALESCE(ps.tahun_lomba, '') AS tahun_lomba,
               COALESCE(ps.mode_pelaksanaan::text, '') AS mode_pelaksanaan,
               COALESCE(ps.penyelenggaraan_lomba, '') AS penyelenggaraan_lomba,
               COALESCE(ps.jenis_peserta::text, '') AS jenis_peserta,
               COALESCE(ps.jenis_lomba, '') AS jenis_lomba
        FROM prestasi_siswa ps
        JOIN data_sekolah ds ON ds.id = ps.school_id
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
			return []AchievementExportRow{}, 0, nil
		}
		addClause(fmt.Sprintf("ps.school_id = ANY($%d)", idx), scope.IDs)
	}
	if filters.Year != "" && filters.Year != "ALL" {
		addClause(fmt.Sprintf("ps.tahun_lomba = $%d", idx), filters.Year)
	}
	if filters.Tingkat != "" && filters.Tingkat != "ALL" {
		addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) = $%d", idx), strings.ToUpper(filters.Tingkat))
	}
	if filters.JenisPeserta != "" && filters.JenisPeserta != "ALL" {
		addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ps.jenis_peserta::text, ''))) = $%d", idx), strings.ToUpper(filters.JenisPeserta))
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
		normalizedBentuk := strings.ToUpper(strings.TrimSpace(filters.Bentuk))
		if normalizedBentuk != "" && normalizedBentuk != "ALL" {
			addClause(fmt.Sprintf("UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $%d", idx), normalizedBentuk)
		}
	}
	if filters.Search != "" {
		search := "%" + strings.ToLower(strings.TrimSpace(filters.Search)) + "%"
		condition := fmt.Sprintf("(LOWER(COALESCE(ps.nama_lomba, '')) LIKE $%d OR LOWER(COALESCE(ps.nama_peserta, '')) LIKE $%d)", idx, idx+1)
		if whereClause == "" {
			whereClause = " WHERE " + condition
		} else {
			whereClause += " AND " + condition
		}
		args = append(args, search, search)
		idx += 2
	}

	countQuery := "SELECT COUNT(*) FROM prestasi_siswa ps JOIN data_sekolah ds ON ds.id = ps.school_id" + whereClause
	var total int
	if err := s.DB.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit > 0 && total > limit {
		return nil, total, ErrExportLimitExceeded
	}

	query += whereClause + " ORDER BY ps.tahun_lomba DESC, ps.nama_lomba ASC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []AchievementExportRow
	for rows.Next() {
		var item AchievementExportRow
		if err := rows.Scan(
			&item.NamaSekolah,
			&item.NamaLomba,
			&item.TingkatLomba,
			&item.KategoriPrestasi,
			&item.PeringkatPrestasi,
			&item.NamaPeserta,
			&item.TahunLomba,
			&item.ModePelaksanaan,
			&item.Penyelenggaraan,
			&item.JenisPeserta,
			&item.JenisLomba,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, total, nil
}

func (s AchievementService) Analytics(ctx context.Context, user auth.User, filters AchievementAnalyticsFilters) (AchievementAnalytics, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return AchievementAnalytics{}, err
	}
	if scope.All || len(scope.IDs) == 0 {
		return AchievementAnalytics{}, auth.ErrForbidden
	}

	levelKeys := []string{"KABUPATEN", "PROVINSI", "NASIONAL", "INTERNASIONAL"}
	levelKeySet := map[string]struct{}{}
	for _, key := range levelKeys {
		levelKeySet[key] = struct{}{}
	}
	categoryKeys := []string{"OSN", "O2SN", "FLS3N", "LKSN", "LAINNYA"}
	categoryLabel := map[string]string{
		"OSN":     "OSN",
		"O2SN":    "O2SN",
		"FLS3N":   "FLS3N",
		"LKSN":    "LKSN",
		"LAINNYA": "Lainnya",
	}

	normalizedKategori := strings.ToUpper(strings.TrimSpace(filters.Kategori))
	normalizedJenisLomba := strings.ToUpper(strings.TrimSpace(filters.JenisLomba))

	base := `
        WITH normalized AS (
            SELECT ps.school_id,
                   CASE
                       WHEN ps.tahun_lomba ~ '^\d{4}$' THEN ps.tahun_lomba::int
                       ELSE NULL
                   END AS year,
                   UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) AS tingkat,
                   CASE
                       WHEN UPPER(TRIM(COALESCE(ps.kategori_prestasi, ''))) IN ('OSN', 'O2SN', 'FLS3N', 'LKSN')
                           THEN UPPER(TRIM(COALESCE(ps.kategori_prestasi, '')))
                       ELSE 'LAINNYA'
                   END AS kategori,
                   UPPER(TRIM(COALESCE(ps.jenis_lomba, ''))) AS jenis_lomba,
                   UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) AS bentuk
            FROM prestasi_siswa ps
            JOIN data_sekolah ds ON ds.id = ps.school_id
        )
    `

	buildWhere := func(includeYear bool) (string, []interface{}) {
		whereClause := " WHERE school_id = ANY($1) AND year IS NOT NULL"
		args := []interface{}{scope.IDs}
		idx := 2
		if includeYear && filters.Year != nil {
			whereClause += fmt.Sprintf(" AND year = $%d", idx)
			args = append(args, *filters.Year)
			idx++
		}
		if normalizedKategori != "" && normalizedKategori != "ALL" {
			whereClause += fmt.Sprintf(" AND kategori = $%d", idx)
			args = append(args, normalizedKategori)
			idx++
		}
		if normalizedJenisLomba != "" && normalizedJenisLomba != "ALL" {
			whereClause += fmt.Sprintf(" AND jenis_lomba = $%d", idx)
			args = append(args, normalizedJenisLomba)
			idx++
		}
		if user.Role == "ADMIN" {
			allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
			if len(allowed) == 0 {
				whereClause += " AND 1=0"
			} else {
				whereClause += fmt.Sprintf(" AND bentuk = ANY($%d)", idx)
				args = append(args, allowed)
				idx++
			}
		}
		if auth.IsAdmin(user) {
			normalizedBentuk := strings.ToUpper(strings.TrimSpace(filters.Bentuk))
			if normalizedBentuk != "" && normalizedBentuk != "ALL" {
				whereClause += fmt.Sprintf(" AND bentuk = $%d", idx)
				args = append(args, normalizedBentuk)
				idx++
			}
		}
		return whereClause, args
	}

	trendWhere, trendArgs := buildWhere(true)
	trendQuery := base + `
        SELECT year, tingkat, COUNT(*)
        FROM normalized
    ` + trendWhere + `
        GROUP BY year, tingkat
        ORDER BY year ASC
    `

	rows, err := s.DB.Pool.Query(ctx, trendQuery, trendArgs...)
	if err != nil {
		return AchievementAnalytics{}, err
	}
	defer rows.Close()

	type trendRow struct {
		Year    int
		Tingkat string
		Total   int
	}
	var trendRows []trendRow
	yearSet := map[int]struct{}{}
	for rows.Next() {
		var row trendRow
		if err := rows.Scan(&row.Year, &row.Tingkat, &row.Total); err == nil {
			trendRows = append(trendRows, row)
			yearSet[row.Year] = struct{}{}
		}
	}

	var years []int
	for year := range yearSet {
		years = append(years, year)
	}
	sort.Ints(years)
	if len(years) > 5 {
		years = years[len(years)-5:]
	}
	yearIncluded := map[int]struct{}{}
	for _, year := range years {
		yearIncluded[year] = struct{}{}
	}

	trendMap := map[int]*AchievementTrendPoint{}
	for _, year := range years {
		trendMap[year] = &AchievementTrendPoint{Year: year}
	}
	for _, row := range trendRows {
		if _, ok := yearIncluded[row.Year]; !ok {
			continue
		}
		point := trendMap[row.Year]
		switch row.Tingkat {
		case "KABUPATEN":
			point.Kabupaten = row.Total
		case "PROVINSI":
			point.Provinsi = row.Total
		case "NASIONAL":
			point.Nasional = row.Total
		case "INTERNASIONAL":
			point.Internasional = row.Total
		}
	}

	var trend []AchievementTrendPoint
	for _, year := range years {
		if point, ok := trendMap[year]; ok {
			trend = append(trend, *point)
		}
	}

	levelWhere, levelArgs := buildWhere(true)
	levelQuery := base + `
        SELECT tingkat, COUNT(*)
        FROM normalized
    ` + levelWhere + `
        GROUP BY tingkat
    `

	levelRows, err := s.DB.Pool.Query(ctx, levelQuery, levelArgs...)
	if err != nil {
		return AchievementAnalytics{}, err
	}
	defer levelRows.Close()

	levelCounts := map[string]int{}
	for levelRows.Next() {
		var tingkat string
		var total int
		if err := levelRows.Scan(&tingkat, &total); err == nil {
			levelCounts[tingkat] = total
		}
	}

	var levelComparison []AchievementNamedValue
	for _, key := range levelKeys {
		levelComparison = append(levelComparison, AchievementNamedValue{
			Name:  key,
			Value: levelCounts[key],
		})
	}

	var levelComparisonByYear []AchievementLevelYearValue
	if filters.Year == nil {
		levelYearWhere, levelYearArgs := buildWhere(false)
		levelYearQuery := base + `
        SELECT tingkat, year, COUNT(*)
        FROM normalized
    ` + levelYearWhere + `
        GROUP BY tingkat, year
        ORDER BY year ASC
    `

		levelYearRows, err := s.DB.Pool.Query(ctx, levelYearQuery, levelYearArgs...)
		if err != nil {
			return AchievementAnalytics{}, err
		}
		defer levelYearRows.Close()

		for levelYearRows.Next() {
			var tingkat string
			var year int
			var total int
			if err := levelYearRows.Scan(&tingkat, &year, &total); err == nil {
				if _, ok := levelKeySet[tingkat]; !ok {
					continue
				}
				levelComparisonByYear = append(levelComparisonByYear, AchievementLevelYearValue{
					Tingkat: tingkat,
					Tahun:   year,
					Total:   total,
				})
			}
		}
	}

	categoryWhere, categoryArgs := buildWhere(true)
	categoryQuery := base + `
        SELECT kategori, COUNT(*)
        FROM normalized
    ` + categoryWhere + `
        GROUP BY kategori
    `

	categoryRows, err := s.DB.Pool.Query(ctx, categoryQuery, categoryArgs...)
	if err != nil {
		return AchievementAnalytics{}, err
	}
	defer categoryRows.Close()

	categoryCounts := map[string]int{}
	for categoryRows.Next() {
		var kategori string
		var total int
		if err := categoryRows.Scan(&kategori, &total); err == nil {
			categoryCounts[kategori] = total
		}
	}

	var categoryComposition []AchievementNamedValue
	for _, key := range categoryKeys {
		categoryComposition = append(categoryComposition, AchievementNamedValue{
			Name:  categoryLabel[key],
			Value: categoryCounts[key],
		})
	}

	return AchievementAnalytics{
		TrendByYear:           trend,
		LevelComparison:       levelComparison,
		LevelComparisonByYear: levelComparisonByYear,
		CategoryComposition:   categoryComposition,
	}, nil
}

func (s AchievementService) AdminSummary(ctx context.Context, user auth.User, filters AchievementAdminSummaryFilters) (AchievementAdminSummary, error) {
	scope, err := auth.GetSchoolScopeV2(user, filters.SchoolID)
	if err != nil {
		return AchievementAdminSummary{}, err
	}

	normalizedYear := strings.TrimSpace(filters.Year)
	if strings.EqualFold(normalizedYear, "ALL") {
		normalizedYear = ""
	}
	normalizedKategori := strings.ToUpper(strings.TrimSpace(filters.Kategori))
	if normalizedKategori == "ALL" {
		normalizedKategori = ""
	}
	normalizedBentuk := strings.ToUpper(strings.TrimSpace(filters.Bentuk))
	if normalizedBentuk == "ALL" {
		normalizedBentuk = ""
	}
	normalizedStatus := strings.ToUpper(strings.TrimSpace(filters.Status))
	if normalizedStatus == "ALL" {
		normalizedStatus = ""
	}

	base := `
        WITH normalized AS (
            SELECT ds.nama_sekolah,
                   UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) AS jenjang,
                   UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) AS status,
                   CASE
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('KAB', 'KABUPATEN', 'KOTA', 'KABUPATEN/KOTA', 'KOTA/KABUPATEN') THEN 'KABUPATEN'
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('PROV', 'PROVINSI') THEN 'PROVINSI'
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('NAS', 'NASIONAL') THEN 'NASIONAL'
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('INT', 'INTERNASIONAL') THEN 'INTERNASIONAL'
                       ELSE NULL
                   END AS tingkat,
                   UPPER(TRIM(COALESCE(ps.jenis_lomba, ''))) AS jenis_lomba,
                   COALESCE(ps.tahun_lomba, '') AS tahun,
                   ps.school_id
            FROM prestasi_siswa ps
            JOIN data_sekolah ds ON ds.id = ps.school_id
        )
    `

	whereClause := " WHERE jenjang IN ('SMA', 'SMK', 'SLB') AND tingkat IS NOT NULL"
	var args []interface{}
	idx := 1
	if !scope.All {
		if len(scope.IDs) == 0 {
			return AchievementAdminSummary{}, nil
		}
		whereClause += fmt.Sprintf(" AND school_id = ANY($%d)", idx)
		args = append(args, scope.IDs)
		idx++
	}
	if user.Role == "ADMIN" {
		allowed := auth.NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			whereClause += " AND 1=0"
		} else {
			whereClause += fmt.Sprintf(" AND jenjang = ANY($%d)", idx)
			args = append(args, allowed)
			idx++
		}
	}
	if normalizedYear != "" {
		whereClause += fmt.Sprintf(" AND tahun = $%d", idx)
		args = append(args, normalizedYear)
		idx++
	}
	if normalizedKategori != "" {
		whereClause += fmt.Sprintf(" AND jenis_lomba = $%d", idx)
		args = append(args, normalizedKategori)
		idx++
	}
	if normalizedBentuk != "" {
		whereClause += fmt.Sprintf(" AND jenjang = $%d", idx)
		args = append(args, normalizedBentuk)
		idx++
	}
	if normalizedStatus != "" {
		whereClause += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, normalizedStatus)
		idx++
	}

	rekapQuery := base + `
        SELECT jenjang,
               SUM(CASE WHEN tingkat = 'KABUPATEN' THEN 1 ELSE 0 END) AS kabupaten,
               SUM(CASE WHEN tingkat = 'PROVINSI' THEN 1 ELSE 0 END) AS provinsi,
               SUM(CASE WHEN tingkat = 'NASIONAL' THEN 1 ELSE 0 END) AS nasional,
               SUM(CASE WHEN tingkat = 'INTERNASIONAL' THEN 1 ELSE 0 END) AS internasional
        FROM normalized
    ` + whereClause + `
        GROUP BY jenjang
    `
	rows, err := s.DB.Pool.Query(ctx, rekapQuery, args...)
	if err != nil {
		return AchievementAdminSummary{}, err
	}
	defer rows.Close()

	rekapMap := make(map[string]AchievementJenjangTingkat)
	for rows.Next() {
		var row AchievementJenjangTingkat
		if err := rows.Scan(
			&row.Jenjang,
			&row.Kabupaten,
			&row.Provinsi,
			&row.Nasional,
			&row.Internasional,
		); err != nil {
			return AchievementAdminSummary{}, err
		}
		rekapMap[row.Jenjang] = row
	}

	jenjangOrder := []string{"SMA", "SMK", "SLB"}
	rekap := make([]AchievementJenjangTingkat, 0, len(jenjangOrder))
	for _, jenjang := range jenjangOrder {
		if row, ok := rekapMap[jenjang]; ok {
			rekap = append(rekap, row)
		} else {
			rekap = append(rekap, AchievementJenjangTingkat{Jenjang: jenjang})
		}
	}

	detailQuery := base + `
        SELECT nama_sekolah,
               SUM(CASE WHEN tingkat = 'KABUPATEN' THEN 1 ELSE 0 END) AS kabupaten,
               SUM(CASE WHEN tingkat = 'PROVINSI' THEN 1 ELSE 0 END) AS provinsi,
               SUM(CASE WHEN tingkat = 'NASIONAL' THEN 1 ELSE 0 END) AS nasional,
               SUM(CASE WHEN tingkat = 'INTERNASIONAL' THEN 1 ELSE 0 END) AS internasional,
               COUNT(*) AS total
        FROM normalized
    ` + whereClause + `
        GROUP BY nama_sekolah
        ORDER BY total DESC, nama_sekolah ASC
    `
	rows, err = s.DB.Pool.Query(ctx, detailQuery, args...)
	if err != nil {
		return AchievementAdminSummary{}, err
	}
	defer rows.Close()

	var detail []AchievementSchoolDetail
	for rows.Next() {
		var row AchievementSchoolDetail
		if err := rows.Scan(
			&row.NamaSekolah,
			&row.Kabupaten,
			&row.Provinsi,
			&row.Nasional,
			&row.Internasional,
			&row.Total,
		); err != nil {
			return AchievementAdminSummary{}, err
		}
		detail = append(detail, row)
	}

	return AchievementAdminSummary{
		RekapJenjangTingkat: rekap,
		DetailSekolah:       detail,
	}, nil
}
