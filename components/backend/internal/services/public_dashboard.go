package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/db"
)

type PublicDashboardService struct {
	DB *db.DB
}

type DashboardFilters struct {
	SchoolID *int
	Jenjang string
	Status  string
	Tahun   int
	AllowedLevels []string
}

type DashboardTotals struct {
	TotalSchools        int     `json:"totalSchools"`
	TotalGuru           int     `json:"totalGuru"`
	TotalTendik         int     `json:"totalTendik"`
	AvgGuruPerSchool    float64 `json:"avgGuruPerSchool"`
	PercentAkreditasiAB float64 `json:"percentAkreditasiAB"`
}

type DashboardKPI struct {
	TotalSchools        int     `json:"totalSchools"`
	TotalGuru           int     `json:"totalGuru"`
	TotalTendik         int     `json:"totalTendik"`
	PercentAkreditasiAB float64 `json:"percentAkreditasiAB"`
}

type AkreditasiCounts struct {
	A     int `json:"A"`
	B     int `json:"B"`
	C     int `json:"C"`
	Belum int `json:"BELUM"`
}

type TopSekolahItem struct {
	NamaSekolah string `json:"namaSekolah"`
	Value       int    `json:"value"`
}

type AkreditasiByJenjang struct {
	Jenjang string `json:"jenjang"`
	A       int    `json:"A"`
	B       int    `json:"B"`
	C       int    `json:"C"`
	Belum   int    `json:"Belum"`
}

type AkreditasiComposition struct {
	Label string `json:"label"`
	Total int    `json:"total"`
}

type StatusByJenjang struct {
	Jenjang string `json:"jenjang"`
	Negeri  int    `json:"Negeri"`
	Swasta  int    `json:"Swasta"`
}

type JenjangSummary struct {
	Jenjang              string  `json:"jenjang"`
	TotalSekolah         int     `json:"totalSekolah"`
	TotalGuru            int     `json:"totalGuru"`
	TotalTendik          int     `json:"totalTendik"`
	PercentAkreditasiAB  float64 `json:"percentAkreditasiAB"`
}

type DashboardSummary struct {
	Kpi                   DashboardKPI             `json:"kpi"`
	Akreditasi            AkreditasiCounts          `json:"akreditasi"`
	PctAB                 float64                  `json:"pctAB"`
	TopSekolah            []TopSekolahItem          `json:"topSekolah"`
	Totals                DashboardTotals          `json:"totals"`
	AkreditasiPerJenjang  []AkreditasiByJenjang    `json:"akreditasiPerJenjang"`
	AkreditasiComposition []AkreditasiComposition  `json:"akreditasiComposition"`
	StatusByJenjang       []StatusByJenjang        `json:"statusByJenjang"`
	GuruVsTendik          []map[string]interface{} `json:"guruVsTendik"`
	JenjangSummaries      []JenjangSummary         `json:"jenjangSummaries"`
	LastUpdated           *time.Time               `json:"lastUpdated"`
}

func normalizeUpperValue(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func (s PublicDashboardService) GetSummary(ctx context.Context, filters DashboardFilters) (DashboardSummary, error) {
	normalizedJenjang := normalizeUpperValue(filters.Jenjang)
	normalizedStatus := normalizeUpperValue(filters.Status)

	cte := `
        WITH normalized AS (
            SELECT id,
                   updated_at,
                   COALESCE(UPPER(TRIM(bentuk_pendidikan)), '') AS jenjang,
                   COALESCE(UPPER(TRIM(status_sekolah)), '') AS status,
                   CASE
                       WHEN UPPER(TRIM(akreditasi)) IN ('A', 'B', 'C') THEN UPPER(TRIM(akreditasi))
                       ELSE 'BELUM'
                   END AS akreditasi,
                   CASE
                       WHEN jumlah_total_guru > 0 THEN jumlah_total_guru
                       ELSE COALESCE(jumlah_guru_asn, 0) + COALESCE(jumlah_guru_honorer, 0)
                   END AS total_guru,
                   CASE
                       WHEN jumlah_total_tendik > 0 THEN jumlah_total_tendik
                       ELSE COALESCE(jumlah_tendik_asn, 0) + COALESCE(jumlah_tendik_honorer, 0)
                   END AS total_tendik
            FROM data_sekolah
        )
    `

	buildFilter := func(base string, args *[]interface{}, idx *int) string {
		query := base
		if filters.SchoolID != nil {
			query += fmt.Sprintf(" AND id = $%d", *idx)
			*args = append(*args, *filters.SchoolID)
			*idx++
		}
		if normalizedJenjang != "" {
			query += fmt.Sprintf(" AND jenjang = $%d", *idx)
			*args = append(*args, normalizedJenjang)
			*idx++
		}
		if normalizedStatus != "" {
			query += fmt.Sprintf(" AND status = $%d", *idx)
			*args = append(*args, normalizedStatus)
			*idx++
		}
		if filters.AllowedLevels != nil {
			if len(filters.AllowedLevels) == 0 {
				query += " AND 1=0"
			} else {
				query += fmt.Sprintf(" AND jenjang = ANY($%d)", *idx)
				*args = append(*args, filters.AllowedLevels)
				*idx++
			}
		}
		return query
	}
	buildFilterWithAlias := func(base string, alias string, args *[]interface{}, idx *int) string {
		query := base
		if filters.SchoolID != nil {
			query += fmt.Sprintf(" AND %s.id = $%d", alias, *idx)
			*args = append(*args, *filters.SchoolID)
			*idx++
		}
		if normalizedJenjang != "" {
			query += fmt.Sprintf(" AND %s.jenjang = $%d", alias, *idx)
			*args = append(*args, normalizedJenjang)
			*idx++
		}
		if normalizedStatus != "" {
			query += fmt.Sprintf(" AND %s.status = $%d", alias, *idx)
			*args = append(*args, normalizedStatus)
			*idx++
		}
		if filters.AllowedLevels != nil {
			if len(filters.AllowedLevels) == 0 {
				query += " AND 1=0"
			} else {
				query += fmt.Sprintf(" AND %s.jenjang = ANY($%d)", alias, *idx)
				*args = append(*args, filters.AllowedLevels)
				*idx++
			}
		}
		return query
	}

	var args []interface{}
	idx := 1
	baseFilter := " FROM normalized WHERE 1=1"
	filterClause := buildFilter(baseFilter, &args, &idx)

	var totals DashboardTotals
	totalsQuery := cte + `
        SELECT
            COUNT(*) AS total_schools,
            COALESCE(SUM(total_guru), 0) AS total_guru,
            COALESCE(SUM(total_tendik), 0) AS total_tendik,
            COALESCE(SUM(CASE WHEN akreditasi IN ('A', 'B') THEN 1 ELSE 0 END), 0) AS total_ab
    ` + filterClause
	row := s.DB.Pool.QueryRow(ctx, totalsQuery, args...)
	var totalAB int
	if err := row.Scan(&totals.TotalSchools, &totals.TotalGuru, &totals.TotalTendik, &totalAB); err != nil {
		return DashboardSummary{}, err
	}
	if totals.TotalSchools > 0 {
		totals.AvgGuruPerSchool = float64(totals.TotalGuru) / float64(totals.TotalSchools)
		totals.PercentAkreditasiAB = (float64(totalAB) / float64(totals.TotalSchools)) * 100
	}

	akreditasiQuery := cte + `
        SELECT jenjang, akreditasi, COUNT(*)
    ` + filterClause + `
        GROUP BY jenjang, akreditasi
        ORDER BY jenjang ASC
    `
	rows, err := s.DB.Pool.Query(ctx, akreditasiQuery, args...)
	if err != nil {
		return DashboardSummary{}, err
	}
	defer rows.Close()

	akreditasiMap := map[string]*AkreditasiByJenjang{}
	for rows.Next() {
		var jenjang, akreditasi string
		var total int
		if err := rows.Scan(&jenjang, &akreditasi, &total); err != nil {
			return DashboardSummary{}, err
		}
		entry := akreditasiMap[jenjang]
		if entry == nil {
			entry = &AkreditasiByJenjang{Jenjang: jenjang}
			akreditasiMap[jenjang] = entry
		}
		switch akreditasi {
		case "A":
			entry.A = total
		case "B":
			entry.B = total
		case "C":
			entry.C = total
		default:
			entry.Belum = total
		}
	}

	var akreditasiPerJenjang []AkreditasiByJenjang
	for _, value := range akreditasiMap {
		akreditasiPerJenjang = append(akreditasiPerJenjang, *value)
	}

	compositionQuery := cte + `
        SELECT akreditasi, COUNT(*)
    ` + filterClause + `
        GROUP BY akreditasi
        ORDER BY akreditasi ASC
    `
	rows, err = s.DB.Pool.Query(ctx, compositionQuery, args...)
	if err != nil {
		return DashboardSummary{}, err
	}
	defer rows.Close()

	var compositions []AkreditasiComposition
	compositionCounts := map[string]int{}
	for rows.Next() {
		var label string
		var total int
		if err := rows.Scan(&label, &total); err != nil {
			return DashboardSummary{}, err
		}
		compositions = append(compositions, AkreditasiComposition{
			Label: label,
			Total: total,
		})
		compositionCounts[label] = total
	}

	akreditasiCounts := AkreditasiCounts{
		A:     compositionCounts["A"],
		B:     compositionCounts["B"],
		C:     compositionCounts["C"],
		Belum: compositionCounts["BELUM"],
	}
	kpi := DashboardKPI{
		TotalSchools:        totals.TotalSchools,
		TotalGuru:           totals.TotalGuru,
		TotalTendik:         totals.TotalTendik,
		PercentAkreditasiAB: totals.PercentAkreditasiAB,
	}

	statusQuery := cte + `
        SELECT jenjang, status, COUNT(*)
    ` + filterClause + `
        GROUP BY jenjang, status
        ORDER BY jenjang ASC
    `
	rows, err = s.DB.Pool.Query(ctx, statusQuery, args...)
	if err != nil {
		return DashboardSummary{}, err
	}
	defer rows.Close()

	statusMap := map[string]*StatusByJenjang{}
	for rows.Next() {
		var jenjang, status string
		var total int
		if err := rows.Scan(&jenjang, &status, &total); err != nil {
			return DashboardSummary{}, err
		}
		entry := statusMap[jenjang]
		if entry == nil {
			entry = &StatusByJenjang{Jenjang: jenjang}
			statusMap[jenjang] = entry
		}
		switch status {
		case "NEGERI":
			entry.Negeri = total
		case "SWASTA":
			entry.Swasta = total
		}
	}

	var statusByJenjang []StatusByJenjang
	for _, value := range statusMap {
		statusByJenjang = append(statusByJenjang, *value)
	}

	jenjangQuery := cte + `
        SELECT jenjang,
               COUNT(*) AS total_schools,
               COALESCE(SUM(total_guru), 0) AS total_guru,
               COALESCE(SUM(total_tendik), 0) AS total_tendik,
               COALESCE(SUM(CASE WHEN akreditasi IN ('A', 'B') THEN 1 ELSE 0 END), 0) AS total_ab
    ` + filterClause + `
        GROUP BY jenjang
        ORDER BY jenjang ASC
    `
	rows, err = s.DB.Pool.Query(ctx, jenjangQuery, args...)
	if err != nil {
		return DashboardSummary{}, err
	}
	defer rows.Close()

	var summaries []JenjangSummary
	for rows.Next() {
		var summary JenjangSummary
		var totalAB int
		if err := rows.Scan(
			&summary.Jenjang,
			&summary.TotalSekolah,
			&summary.TotalGuru,
			&summary.TotalTendik,
			&totalAB,
		); err != nil {
			return DashboardSummary{}, err
		}
		if summary.TotalSekolah > 0 {
			summary.PercentAkreditasiAB =
				(float64(totalAB) / float64(summary.TotalSekolah)) * 100
		}
		summaries = append(summaries, summary)
	}

	var lastUpdated *time.Time
	updatedQuery := cte + `SELECT MAX(updated_at)` + filterClause
	if err := s.DB.Pool.QueryRow(ctx, updatedQuery, args...).Scan(&lastUpdated); err != nil {
		return DashboardSummary{}, err
	}

	topArgs := []interface{}{}
	topIdx := 1
	topFilterClause := buildFilterWithAlias(" WHERE 1=1", "n", &topArgs, &topIdx)
	topQuery := cte + `
        SELECT ds.nama_sekolah,
               COUNT(ps.id_prestasi) AS total
        FROM prestasi_siswa ps
        JOIN normalized n ON n.id = ps.school_id
        JOIN data_sekolah ds ON ds.id = n.id
    ` + topFilterClause + `
        GROUP BY ds.nama_sekolah
        ORDER BY total DESC, ds.nama_sekolah ASC
        LIMIT 10
    `
	rows, err = s.DB.Pool.Query(ctx, topQuery, topArgs...)
	if err != nil {
		return DashboardSummary{}, err
	}
	defer rows.Close()

	var topSekolah []TopSekolahItem
	for rows.Next() {
		var name string
		var total int
		if err := rows.Scan(&name, &total); err != nil {
			return DashboardSummary{}, err
		}
		topSekolah = append(topSekolah, TopSekolahItem{
			NamaSekolah: name,
			Value:       total,
		})
	}

	return DashboardSummary{
		Kpi:                  kpi,
		Akreditasi:           akreditasiCounts,
		PctAB:                totals.PercentAkreditasiAB,
		TopSekolah:           topSekolah,
		Totals:               totals,
		AkreditasiPerJenjang: akreditasiPerJenjang,
		AkreditasiComposition: compositions,
		StatusByJenjang:      statusByJenjang,
		GuruVsTendik: []map[string]interface{}{
			{"label": "Guru", "value": totals.TotalGuru},
			{"label": "Tendik", "value": totals.TotalTendik},
		},
		JenjangSummaries: summaries,
		LastUpdated:     lastUpdated,
	}, nil
}
