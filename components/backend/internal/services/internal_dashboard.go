package services

import (
	"context"
	"fmt"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

type InternalDashboardService struct {
	DB *db.DB
}

type InternalDashboardFilters struct {
	SchoolID *int
	Jenjang  string
	Status   string
	Tahun    int
}

type InternalDashboardExtras struct {
	GuruAsn            int `json:"guruAsn"`
	GuruHonorer        int `json:"guruHonorer"`
	TendikAsn          int `json:"tendikAsn"`
	TendikHonorer      int `json:"tendikHonorer"`
	MissingAkreditasi  int `json:"missingAkreditasi"`
	MissingTmt         int `json:"missingTmt"`
	MissingNPSN        int `json:"missingNpsn"`
	MissingGuru        int `json:"missingGuru"`
	MissingTendik      int `json:"missingTendik"`
}

type InternalDashboardSummary struct {
	DashboardSummary
	Extras InternalDashboardExtras `json:"extras"`
	InternalIndex *InternalIndex `json:"internalIndex,omitempty"`
}

type InternalIndex struct {
	Value      float64 `json:"value"`
	Label      string  `json:"label"`
	Disclaimer bool    `json:"disclaimer"`
}

func (s InternalDashboardService) GetSummary(ctx context.Context, user auth.User, filters InternalDashboardFilters) (InternalDashboardSummary, error) {
	scope, err := auth.GetSchoolScope(user, filters.SchoolID)
	if err != nil {
		return InternalDashboardSummary{}, err
	}

	normalizedJenjang := normalizeUpperValue(filters.Jenjang)
	normalizedStatus := normalizeUpperValue(filters.Status)
	allowedLevels := []string(nil)
	if user.Role == "ADMIN" {
		allowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if allowedLevels == nil {
			allowedLevels = []string{}
		}
	}

	cte := `
        WITH normalized AS (
            SELECT id,
                   npsn_sekolah,
                   tmt_akreditasi,
                   COALESCE(UPPER(TRIM(bentuk_pendidikan)), '') AS jenjang,
                   COALESCE(UPPER(TRIM(status_sekolah)), '') AS status,
                   CASE
                       WHEN UPPER(TRIM(akreditasi)) IN ('A', 'B', 'C') THEN UPPER(TRIM(akreditasi))
                       ELSE 'BELUM'
                   END AS akreditasi,
                   COALESCE(jumlah_guru_asn, 0) AS guru_asn,
                   COALESCE(jumlah_guru_honorer, 0) AS guru_honorer,
                   COALESCE(jumlah_tendik_asn, 0) AS tendik_asn,
                   COALESCE(jumlah_tendik_honorer, 0) AS tendik_honorer,
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
		if scope != nil {
			query += fmt.Sprintf(" AND id = $%d", *idx)
			*args = append(*args, *scope)
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
		if allowedLevels != nil {
			if len(allowedLevels) == 0 {
				query += " AND 1=0"
			} else {
				query += fmt.Sprintf(" AND jenjang = ANY($%d)", *idx)
				*args = append(*args, allowedLevels)
				*idx++
			}
		}
		return query
	}

	var args []interface{}
	idx := 1
	baseFilter := " FROM normalized WHERE 1=1"
	filterClause := buildFilter(baseFilter, &args, &idx)

	publicService := PublicDashboardService{DB: s.DB}
	publicSummary, err := publicService.GetSummary(ctx, DashboardFilters{
		SchoolID: scope,
		Jenjang:  normalizedJenjang,
		Status:   normalizedStatus,
		Tahun:    filters.Tahun,
		AllowedLevels: allowedLevels,
	})
	if err != nil {
		return InternalDashboardSummary{}, err
	}

	extrasQuery := cte + `
        SELECT
            COALESCE(SUM(guru_asn), 0) AS guru_asn,
            COALESCE(SUM(guru_honorer), 0) AS guru_honorer,
            COALESCE(SUM(tendik_asn), 0) AS tendik_asn,
            COALESCE(SUM(tendik_honorer), 0) AS tendik_honorer,
            COALESCE(SUM(CASE WHEN akreditasi = 'BELUM' THEN 1 ELSE 0 END), 0) AS missing_akreditasi,
            COALESCE(SUM(CASE WHEN tmt_akreditasi IS NULL OR TRIM(tmt_akreditasi::text) = '' THEN 1 ELSE 0 END), 0) AS missing_tmt,
            COALESCE(SUM(CASE WHEN npsn_sekolah IS NULL OR TRIM(npsn_sekolah) = '' THEN 1 ELSE 0 END), 0) AS missing_npsn,
            COALESCE(SUM(CASE WHEN total_guru = 0 THEN 1 ELSE 0 END), 0) AS missing_guru,
            COALESCE(SUM(CASE WHEN total_tendik = 0 THEN 1 ELSE 0 END), 0) AS missing_tendik
    ` + filterClause

	row := s.DB.Pool.QueryRow(ctx, extrasQuery, args...)
	var extras InternalDashboardExtras
	if err := row.Scan(
		&extras.GuruAsn,
		&extras.GuruHonorer,
		&extras.TendikAsn,
		&extras.TendikHonorer,
		&extras.MissingAkreditasi,
		&extras.MissingTmt,
		&extras.MissingNPSN,
		&extras.MissingGuru,
		&extras.MissingTendik,
	); err != nil {
		return InternalDashboardSummary{}, err
	}

	var internalIndex *InternalIndex
	if auth.IsAdmin(user) && publicSummary.Totals.TotalSchools > 0 {
		counts := publicSummary.Akreditasi
		total := float64(publicSummary.Totals.TotalSchools)
		value := (float64(counts.A)*100 +
			float64(counts.B)*80 +
			float64(counts.C)*60 +
			float64(counts.Belum)*40) / total
		internalIndex = &InternalIndex{
			Value:      value,
			Label:      "Indeks Mutu Akreditasi (Internal)",
			Disclaimer: true,
		}
	}

	return InternalDashboardSummary{
		DashboardSummary: publicSummary,
		Extras:           extras,
		InternalIndex:    internalIndex,
	}, nil
}
