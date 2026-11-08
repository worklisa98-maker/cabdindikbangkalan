package services

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"cabdin-bangkalan-backend/internal/db"
)

type PublicAchievementService struct {
	DB *db.DB
}

type AchievementDashboardSummary struct {
	TotalPrestasi     int                      `json:"totalPrestasi"`
	DistribusiJenjang []map[string]interface{} `json:"distribusiJenjang"`
	DistribusiTingkat []map[string]interface{} `json:"distribusiTingkat"`
	TrenPerTahun      []map[string]interface{} `json:"trenPerTahun"`
}

type PublicAchievementTrend struct {
	Category string `json:"category"`
	Year     string `json:"year"`
	Total    int    `json:"total"`
}

type PublicJenjangTingkatCount struct {
	Jenjang string `json:"jenjang"`
	Tingkat string `json:"tingkat"`
	Total   int    `json:"total"`
}

type PublicTopSchool struct {
	School string `json:"school"`
	Total  int    `json:"total"`
}

type PublicSchoolDetail struct {
	School  string `json:"school"`
	Tingkat string `json:"tingkat"`
	Total   int    `json:"total"`
}

type PublicAchievementFull struct {
	CategoryTrends          []PublicAchievementTrend   `json:"categoryTrends"`
	DistribusiJenjangTingkat []PublicJenjangTingkatCount `json:"distribusiJenjangTingkat"`
	TopSchools              []PublicTopSchool          `json:"topSchools"`
	SchoolDetails           []PublicSchoolDetail       `json:"schoolDetails"`
	AvailableYears          []string                   `json:"availableYears"`
}

func normalizeJenisLombaLabel(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SISWA":
		return "Siswa"
	case "GURU":
		return "Guru"
	case "TENDIK":
		return "Tendik"
	default:
		return ""
	}
}

func (s PublicAchievementService) GetDashboard(ctx context.Context, bentuk string) (AchievementDashboardSummary, error) {
	normalizedBentuk := strings.ToUpper(strings.TrimSpace(bentuk))
	query := `
        SELECT
            COALESCE(ps.jenjang_pendidikan, ds.bentuk_pendidikan, '') AS jenjang,
            COALESCE(ps.tingkat_lomba, '') AS tingkat,
            COALESCE(ps.tahun_lomba, '') AS tahun
        FROM prestasi_siswa ps
        JOIN data_sekolah ds ON ds.id = ps.school_id
    `

	var args []interface{}
	if normalizedBentuk != "" && normalizedBentuk != "ALL" {
		query += " WHERE UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $1"
		args = append(args, normalizedBentuk)
	}

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return AchievementDashboardSummary{}, err
	}
	defer rows.Close()

	total := 0
	jenjangCounts := map[string]int{}
	tingkatCounts := map[string]int{}
	yearCounts := map[string]int{}

	for rows.Next() {
		var jenjang, tingkat, tahun string
		if err := rows.Scan(&jenjang, &tingkat, &tahun); err != nil {
			return AchievementDashboardSummary{}, err
		}
		total++
		normalizedJenjang := strings.ToUpper(strings.TrimSpace(jenjang))
		if normalizedJenjang != "" {
			jenjangCounts[normalizedJenjang] = jenjangCounts[normalizedJenjang] + 1
		}
		normalizedTingkat := strings.ToUpper(strings.TrimSpace(tingkat))
		if normalizedTingkat != "" {
			tingkatCounts[normalizedTingkat] = tingkatCounts[normalizedTingkat] + 1
		}
		normalizedYear := strings.TrimSpace(tahun)
		if normalizedYear != "" {
			yearCounts[normalizedYear] = yearCounts[normalizedYear] + 1
		}
	}

	toDistribution := func(values map[string]int) []map[string]interface{} {
		var result []map[string]interface{}
		for key, value := range values {
			result = append(result, map[string]interface{}{
				"label": key,
				"value": value,
			})
		}
		return result
	}

	trend := make([]map[string]interface{}, 0, len(yearCounts))
	for year, value := range yearCounts {
		trend = append(trend, map[string]interface{}{
			"year":  year,
			"total": value,
		})
	}
	sort.Slice(trend, func(i, j int) bool {
		left, _ := strconv.Atoi(trend[i]["year"].(string))
		right, _ := strconv.Atoi(trend[j]["year"].(string))
		return left < right
	})

	return AchievementDashboardSummary{
		TotalPrestasi:     total,
		DistribusiJenjang: toDistribution(jenjangCounts),
		DistribusiTingkat: toDistribution(tingkatCounts),
		TrenPerTahun:      trend,
	}, nil
}

func (s PublicAchievementService) GetFullDashboard(
	ctx context.Context,
	category string,
	year string,
	bentuk string,
	status string,
) (PublicAchievementFull, error) {
	normalizedCategory := strings.ToUpper(strings.TrimSpace(category))
	if normalizedCategory == "ALL" {
		normalizedCategory = ""
	}
	normalizedYear := strings.TrimSpace(year)
	if strings.EqualFold(normalizedYear, "ALL") {
		normalizedYear = ""
	}
	normalizedBentuk := strings.ToUpper(strings.TrimSpace(bentuk))
	if normalizedBentuk == "ALL" {
		normalizedBentuk = ""
	}
	normalizedStatus := strings.ToUpper(strings.TrimSpace(status))
	if normalizedStatus == "ALL" {
		normalizedStatus = ""
	}

	trendQuery := `
        SELECT UPPER(COALESCE(NULLIF(TRIM(ps.jenis_lomba), ''), 'SISWA')) AS kategori,
               COALESCE(ps.tahun_lomba, '') AS tahun,
               COUNT(*)
        FROM prestasi_siswa ps
        JOIN data_sekolah ds ON ds.id = ps.school_id
    `
	trendArgs := []interface{}{}
	trendFilter := "WHERE COALESCE(ps.tahun_lomba, '') <> ''"
	if normalizedBentuk != "" && normalizedBentuk != "ALL" {
		trendFilter += " AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $1"
		trendArgs = append(trendArgs, normalizedBentuk)
	}
	if normalizedStatus != "" && normalizedStatus != "ALL" {
		trendFilter += " AND UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $" + strconv.Itoa(len(trendArgs)+1)
		trendArgs = append(trendArgs, normalizedStatus)
	}
	trendQuery += " " + trendFilter + `
        GROUP BY kategori, tahun
        ORDER BY tahun ASC
    `
	rows, err := s.DB.Pool.Query(ctx, trendQuery, trendArgs...)
	if err != nil {
		return PublicAchievementFull{}, err
	}
	defer rows.Close()

	var categoryTrends []PublicAchievementTrend
	for rows.Next() {
		var trend PublicAchievementTrend
		if err := rows.Scan(&trend.Category, &trend.Year, &trend.Total); err != nil {
			return PublicAchievementFull{}, err
		}
		label := normalizeJenisLombaLabel(trend.Category)
		if label == "" {
			continue
		}
		trend.Category = label
		categoryTrends = append(categoryTrends, trend)
	}

	yearArgs := []interface{}{}
	yearFilter := "WHERE COALESCE(ps.tahun_lomba, '') <> ''"
	argIndex := 1
	if normalizedBentuk != "" && normalizedBentuk != "ALL" {
		yearFilter += " AND UPPER(TRIM(COALESCE(ds.bentuk_pendidikan, ''))) = $" + strconv.Itoa(argIndex)
		yearArgs = append(yearArgs, normalizedBentuk)
		argIndex++
	}
	if normalizedStatus != "" && normalizedStatus != "ALL" {
		yearFilter += " AND UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $" + strconv.Itoa(argIndex)
		yearArgs = append(yearArgs, normalizedStatus)
		argIndex++
	}
	if normalizedCategory != "" {
		yearFilter += " AND UPPER(COALESCE(NULLIF(TRIM(ps.jenis_lomba), ''), 'SISWA')) = $" + strconv.Itoa(argIndex)
		yearArgs = append(yearArgs, normalizedCategory)
		argIndex++
	}
	yearsQuery := `
        SELECT DISTINCT COALESCE(ps.tahun_lomba, '')
        FROM prestasi_siswa ps
        JOIN data_sekolah ds ON ds.id = ps.school_id
    ` + yearFilter + `
        ORDER BY COALESCE(ps.tahun_lomba, '') DESC
    `
	rows, err = s.DB.Pool.Query(ctx, yearsQuery, yearArgs...)
	if err != nil {
		return PublicAchievementFull{}, err
	}
	defer rows.Close()

	var availableYears []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return PublicAchievementFull{}, err
		}
		if value != "" {
			availableYears = append(availableYears, value)
		}
	}

	args := []interface{}{}
	filters := "WHERE 1=1"
	argIndex = 1
	if normalizedCategory != "" {
		filters += " AND jenis_lomba = $" + strconv.Itoa(argIndex)
		args = append(args, normalizedCategory)
		argIndex++
	}
	if normalizedYear != "" {
		filters += " AND tahun = $" + strconv.Itoa(argIndex)
		args = append(args, normalizedYear)
		argIndex++
	}
	if normalizedBentuk != "" {
		filters += " AND jenjang = $" + strconv.Itoa(argIndex)
		args = append(args, normalizedBentuk)
		argIndex++
	}
	if normalizedStatus != "" {
		filters += " AND status = $" + strconv.Itoa(argIndex)
		args = append(args, normalizedStatus)
		argIndex++
	}

	base := `
        WITH normalized AS (
            SELECT ds.nama_sekolah,
                   UPPER(TRIM(COALESCE(ps.jenjang_pendidikan, ds.bentuk_pendidikan, ''))) AS jenjang,
                   CASE
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('KAB', 'KABUPATEN', 'KOTA', 'KABUPATEN/KOTA', 'KOTA/KABUPATEN') THEN 'Kabupaten'
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('PROV', 'PROVINSI') THEN 'Provinsi'
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('NAS', 'NASIONAL') THEN 'Nasional'
                       WHEN UPPER(TRIM(COALESCE(ps.tingkat_lomba, ''))) IN ('INT', 'INTERNASIONAL') THEN 'Internasional'
                       ELSE NULL
                   END AS tingkat,
                   UPPER(COALESCE(NULLIF(TRIM(ps.jenis_lomba), ''), 'SISWA')) AS jenis_lomba,
                   COALESCE(ps.tahun_lomba, '') AS tahun,
                   UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) AS status
            FROM prestasi_siswa ps
            JOIN data_sekolah ds ON ds.id = ps.school_id
        )
    `

	distributionQuery := base + `
        SELECT jenjang,
               tingkat,
               COUNT(*)
        FROM normalized
    ` + filters + `
        AND jenjang IN ('SMA', 'SMK', 'SLB')
        AND tingkat IS NOT NULL
        GROUP BY jenjang, tingkat
        ORDER BY jenjang ASC, tingkat ASC
    `
	rows, err = s.DB.Pool.Query(ctx, distributionQuery, args...)
	if err != nil {
		return PublicAchievementFull{}, err
	}
	defer rows.Close()

	var distribusi []PublicJenjangTingkatCount
	for rows.Next() {
		var item PublicJenjangTingkatCount
		if err := rows.Scan(&item.Jenjang, &item.Tingkat, &item.Total); err != nil {
			return PublicAchievementFull{}, err
		}
		distribusi = append(distribusi, item)
	}

	topQuery := base + `
        SELECT nama_sekolah,
               COUNT(*) AS total
        FROM normalized
    ` + filters + `
        GROUP BY nama_sekolah
        ORDER BY total DESC, nama_sekolah ASC
        LIMIT 10
    `
	rows, err = s.DB.Pool.Query(ctx, topQuery, args...)
	if err != nil {
		return PublicAchievementFull{}, err
	}
	defer rows.Close()

	var topSchools []PublicTopSchool
	for rows.Next() {
		var item PublicTopSchool
		if err := rows.Scan(&item.School, &item.Total); err != nil {
			return PublicAchievementFull{}, err
		}
		topSchools = append(topSchools, item)
	}

	detailQuery := base + `
        SELECT nama_sekolah,
               tingkat,
               COUNT(*) AS total
        FROM normalized
    ` + filters + `
        AND jenjang IN ('SMA', 'SMK', 'SLB')
        AND tingkat IS NOT NULL
        GROUP BY nama_sekolah, tingkat
        ORDER BY nama_sekolah ASC
    `
	rows, err = s.DB.Pool.Query(ctx, detailQuery, args...)
	if err != nil {
		return PublicAchievementFull{}, err
	}
	defer rows.Close()

	var schoolDetails []PublicSchoolDetail
	for rows.Next() {
		var item PublicSchoolDetail
		if err := rows.Scan(&item.School, &item.Tingkat, &item.Total); err != nil {
			return PublicAchievementFull{}, err
		}
		schoolDetails = append(schoolDetails, item)
	}

	return PublicAchievementFull{
		CategoryTrends:          categoryTrends,
		DistribusiJenjangTingkat: distribusi,
		TopSchools:              topSchools,
		SchoolDetails:           schoolDetails,
		AvailableYears:          availableYears,
	}, nil
}
