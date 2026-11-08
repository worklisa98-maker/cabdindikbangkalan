package services

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"cabdin-bangkalan-backend/internal/db"
)

type PublicAdmissionService struct {
	DB *db.DB
}

type AdmissionDashboardSummary struct {
	TrenPerTahun       []map[string]interface{} `json:"trenPerTahun"`
	TopPerguruanTinggi []map[string]interface{} `json:"topPerguruanTinggi"`
	AvailableYears     []int                    `json:"availableYears"`
}

type TopUniversity struct {
	Name       string         `json:"name"`
	Total      int            `json:"total"`
	YearCounts map[string]int `json:"yearCounts,omitempty"`
}

func (s PublicAdmissionService) GetDashboard(ctx context.Context, year int, jenjang string, status string) (AdmissionDashboardSummary, error) {
	normalizedJenjang := strings.ToUpper(strings.TrimSpace(jenjang))
	if normalizedJenjang == "ALL" {
		normalizedJenjang = ""
	}
	normalizedStatus := strings.ToUpper(strings.TrimSpace(status))
	if normalizedStatus == "ALL" {
		normalizedStatus = ""
	}

	buildFilters := func() (string, []interface{}) {
		filters := `
        WHERE spt.tahun_diterima IS NOT NULL
          AND spt.tahun_diterima <> 0
    `
		var args []interface{}
		idx := 1
		if normalizedJenjang != "" {
			filters += " AND UPPER(TRIM(COALESCE(spt.jenjang_pendidikan, ds.bentuk_pendidikan, ''))) = $" + strconv.Itoa(idx)
			args = append(args, normalizedJenjang)
			idx++
		}
		if normalizedStatus != "" {
			filters += " AND UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $" + strconv.Itoa(idx)
			args = append(args, normalizedStatus)
			idx++
		}
		return filters, args
	}

	trendFilter, trendArgs := buildFilters()
	if year != 0 {
		trendFilter += " AND spt.tahun_diterima = $" + strconv.Itoa(len(trendArgs)+1)
		trendArgs = append(trendArgs, year)
	}
	trendQuery := `
        SELECT spt.tahun_diterima,
               SUM(CASE WHEN spt.jalur_penerimaan = 'SNBP' THEN 1 ELSE 0 END) AS snbp,
               SUM(CASE WHEN spt.jalur_penerimaan = 'SNBT' THEN 1 ELSE 0 END) AS snbt,
               SUM(CASE WHEN spt.jalur_penerimaan = 'MANDIRI' THEN 1 ELSE 0 END) AS mandiri,
               SUM(CASE WHEN spt.jalur_penerimaan = 'KEDINASAN' THEN 1 ELSE 0 END) AS kedinasan,
               SUM(CASE WHEN spt.jalur_penerimaan = 'LAINNYA' THEN 1 ELSE 0 END) AS lainnya
        FROM siswa_perguruan_tinggi spt
        JOIN data_sekolah ds ON ds.id = spt.school_id
    ` + trendFilter + `
        GROUP BY spt.tahun_diterima
        ORDER BY spt.tahun_diterima ASC
    `
	rows, err := s.DB.Pool.Query(ctx, trendQuery, trendArgs...)
	if err != nil {
		return AdmissionDashboardSummary{}, err
	}
	defer rows.Close()

	var trend []map[string]interface{}
	for rows.Next() {
		var year int
		var snbp, snbt, mandiri, kedinasan, lainnya int
		if err := rows.Scan(&year, &snbp, &snbt, &mandiri, &kedinasan, &lainnya); err != nil {
			return AdmissionDashboardSummary{}, err
		}
		trend = append(trend, map[string]interface{}{
			"year":      year,
			"SNBP":      snbp,
			"SNBT":      snbt,
			"MANDIRI":   mandiri,
			"KEDINASAN": kedinasan,
			"LAINNYA":   lainnya,
		})
	}
	if err := rows.Err(); err != nil {
		return AdmissionDashboardSummary{}, err
	}

	yearFilter, yearArgs := buildFilters()
	yearsQuery := `
        SELECT DISTINCT spt.tahun_diterima
        FROM siswa_perguruan_tinggi spt
        JOIN data_sekolah ds ON ds.id = spt.school_id
    ` + yearFilter + `
        ORDER BY spt.tahun_diterima DESC
    `
	rows, err = s.DB.Pool.Query(ctx, yearsQuery, yearArgs...)
	if err != nil {
		return AdmissionDashboardSummary{}, err
	}
	defer rows.Close()

	var years []int
	for rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			return AdmissionDashboardSummary{}, err
		}
		if value != 0 {
			years = append(years, value)
		}
	}
	if err := rows.Err(); err != nil {
		return AdmissionDashboardSummary{}, err
	}

	topUniversities, err := s.GetTopUniversities(ctx, year, normalizedJenjang, normalizedStatus)
	if err != nil {
		return AdmissionDashboardSummary{}, err
	}
	top := make([]map[string]interface{}, 0, len(topUniversities))
	for _, entry := range topUniversities {
		top = append(top, map[string]interface{}{
			"name":  entry.Name,
			"total": entry.Total,
		})
	}

	return AdmissionDashboardSummary{
		TrenPerTahun:       trend,
		TopPerguruanTinggi: top,
		AvailableYears:     years,
	}, nil
}

func (s PublicAdmissionService) GetTopUniversities(ctx context.Context, year int, jenjang string, status string) ([]TopUniversity, error) {
	normalizedJenjang := strings.ToUpper(strings.TrimSpace(jenjang))
	if normalizedJenjang == "ALL" {
		normalizedJenjang = ""
	}
	normalizedStatus := strings.ToUpper(strings.TrimSpace(status))
	if normalizedStatus == "ALL" {
		normalizedStatus = ""
	}

	args := []interface{}{}
	filter := `
        WHERE spt.nama_perguruan_tinggi IS NOT NULL
          AND TRIM(spt.nama_perguruan_tinggi) <> ''
    `
	idx := 1
	if year != 0 {
		filter += " AND spt.tahun_diterima = $" + strconv.Itoa(idx)
		args = append(args, year)
		idx++
	}
	if normalizedJenjang != "" {
		filter += " AND UPPER(TRIM(COALESCE(spt.jenjang_pendidikan, ds.bentuk_pendidikan, ''))) = $" + strconv.Itoa(idx)
		args = append(args, normalizedJenjang)
		idx++
	}
	if normalizedStatus != "" {
		filter += " AND UPPER(TRIM(COALESCE(ds.status_sekolah, ''))) = $" + strconv.Itoa(idx)
		args = append(args, normalizedStatus)
	}

	topQuery := `
        SELECT spt.nama_perguruan_tinggi,
               COUNT(*) AS total
        FROM siswa_perguruan_tinggi spt
        JOIN data_sekolah ds ON ds.id = spt.school_id
    ` + filter + `
        GROUP BY spt.nama_perguruan_tinggi
        ORDER BY total DESC, spt.nama_perguruan_tinggi ASC
        LIMIT 10
    `
	rows, err := s.DB.Pool.Query(ctx, topQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var top []TopUniversity
	for rows.Next() {
		var name string
		var totalCount int
		if err := rows.Scan(&name, &totalCount); err != nil {
			return nil, err
		}
		top = append(top, TopUniversity{
			Name:  name,
			Total: totalCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return top, nil
}

func (s PublicAdmissionService) GetTopUniversitiesByYear(ctx context.Context) ([]TopUniversity, []int, error) {
	query := `
        SELECT spt.nama_perguruan_tinggi,
               spt.tahun_diterima,
               COUNT(*) AS total
        FROM siswa_perguruan_tinggi spt
        WHERE spt.nama_perguruan_tinggi IS NOT NULL
          AND TRIM(spt.nama_perguruan_tinggi) <> ''
          AND spt.tahun_diterima IS NOT NULL
          AND spt.tahun_diterima <> 0
        GROUP BY spt.nama_perguruan_tinggi, spt.tahun_diterima
    `
	rows, err := s.DB.Pool.Query(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	type aggRow struct {
		name  string
		year  int
		total int
	}
	var rowsData []aggRow
	yearSet := map[int]struct{}{}
	for rows.Next() {
		var row aggRow
		if err := rows.Scan(&row.name, &row.year, &row.total); err != nil {
			return nil, nil, err
		}
		rowsData = append(rowsData, row)
		yearSet[row.year] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	years := make([]int, 0, len(yearSet))
	for year := range yearSet {
		years = append(years, year)
	}
	sort.Slice(years, func(i, j int) bool { return years[i] < years[j] })

	byName := map[string]*TopUniversity{}
	for _, row := range rowsData {
		entry := byName[row.name]
		if entry == nil {
			entry = &TopUniversity{
				Name:       row.name,
				Total:      0,
				YearCounts: map[string]int{},
			}
			byName[row.name] = entry
		}
		entry.Total += row.total
		entry.YearCounts[strconv.Itoa(row.year)] += row.total
	}

	var result []TopUniversity
	for _, entry := range byName {
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Total != result[j].Total {
			return result[i].Total > result[j].Total
		}
		return result[i].Name < result[j].Name
	})
	if len(result) > 10 {
		result = result[:10]
	}

	return result, years, nil
}
