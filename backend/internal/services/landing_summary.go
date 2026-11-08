package services

import (
	"context"
	"database/sql"
	"time"

	"cabdin-bangkalan-backend/internal/db"
)

type LandingSummaryService struct {
	DB *db.DB
}

type LandingSummary struct {
	PortalGuruTendik      int     `json:"portalGuruTendik"`
	PortalPrestasiYear    int     `json:"portalPrestasiYear"`
	PortalPrestasiTotal   int     `json:"portalPrestasiTotal"`
	PrestasiTotal         int     `json:"prestasiTotal"`
	ActiveSchools         int     `json:"activeSchools"`
	YearMin               *int    `json:"yearMin"`
	YearMax               *int    `json:"yearMax"`
	AkreditasiABPercent   float64 `json:"akreditasiABPercent"`
	LatestYear            *int    `json:"latestYear"`
	PrevYear              *int    `json:"prevYear"`
	YoYSiswaPercent       float64 `json:"yoySiswaPercent"`
	YoYGuruTendikPercent  float64 `json:"yoyGuruTendikPercent"`
	CurrentYearSiswa      int     `json:"currentYearSiswa"`
	PrevYearSiswa         int     `json:"prevYearSiswa"`
	CurrentYearGuruTendik int     `json:"currentYearGuruTendik"`
	PrevYearGuruTendik    int     `json:"prevYearGuruTendik"`
}

func (s LandingSummaryService) Get(ctx context.Context, schoolIDs []int) (LandingSummary, error) {
	portalYear := time.Now().In(time.FixedZone("WIB", 7*60*60)).Year()
	const query = `
		WITH scoped_schools AS (
			SELECT
				id,
				COALESCE(UPPER(TRIM(akreditasi)), '') AS akreditasi,
				COALESCE(jumlah_guru_asn,0) + COALESCE(jumlah_guru_honorer,0) + COALESCE(jumlah_tendik_asn,0) + COALESCE(jumlah_tendik_honorer,0) AS staff_total,
                last_update_prestasi,
				last_update_siswa_pt
			FROM data_sekolah
			WHERE ($1::int[] IS NULL OR id = ANY($1))
		),
		scoped_prestasi AS (
			SELECT
				school_id,
				CASE
					WHEN TRIM(COALESCE(tahun_lomba, '')) ~ '^\d{4}$' THEN TRIM(tahun_lomba)::int
					ELSE NULL
				END AS year_value,
				COALESCE(UPPER(TRIM(jenis_lomba)), '') AS jenis_lomba
			FROM prestasi_siswa
			WHERE ($1::int[] IS NULL OR school_id = ANY($1))
		),
		year_bounds AS (
			SELECT
				MIN(year_value) AS year_min,
				MAX(year_value) AS year_max
			FROM scoped_prestasi
			WHERE year_value IS NOT NULL
		),
		ranked_years AS (
			SELECT DISTINCT year_value
			FROM scoped_prestasi
			WHERE year_value IS NOT NULL
		),
		top_years AS (
			SELECT year_value, ROW_NUMBER() OVER (ORDER BY year_value DESC) AS rn
			FROM ranked_years
			ORDER BY year_value DESC
			LIMIT 2
		),
		resolved_years AS (
			SELECT
				CASE
					WHEN COUNT(*) = 0 THEN NULL
					ELSE MAX(CASE WHEN rn = 1 THEN year_value END)
				END AS latest_year,
				CASE
					WHEN COUNT(*) = 0 THEN NULL
					WHEN COUNT(*) = 1 THEN MAX(CASE WHEN rn = 1 THEN year_value END)
					ELSE MAX(CASE WHEN rn = 2 THEN year_value END)
				END AS prev_year
			FROM top_years
		),
		prestasi_counts AS (
			SELECT COUNT(*) AS prestasi_total
			FROM scoped_prestasi
		),
		school_counts AS (
			SELECT
				COUNT(*) AS total_sekolah,
				COALESCE(SUM(CASE WHEN akreditasi IN ('A', 'B') THEN 1 ELSE 0 END), 0) AS total_ab,
				COALESCE(SUM(CASE WHEN last_update_prestasi IS NOT NULL OR last_update_siswa_pt IS NOT NULL THEN 1 ELSE 0 END), 0) AS active_schools
			FROM scoped_schools
		),
		siswa_counts AS (
			SELECT
				COALESCE(SUM(CASE WHEN sp.year_value = ry.latest_year AND sp.jenis_lomba = 'SISWA' THEN 1 ELSE 0 END), 0) AS current_total,
				COALESCE(SUM(CASE WHEN sp.year_value = ry.prev_year AND sp.jenis_lomba = 'SISWA' THEN 1 ELSE 0 END), 0) AS prev_total
			FROM scoped_prestasi sp
			CROSS JOIN resolved_years ry
		),
		guru_tendik_counts AS (
			SELECT
				COALESCE(SUM(CASE WHEN sp.year_value = ry.latest_year AND sp.jenis_lomba IN ('GURU', 'TENDIK') THEN 1 ELSE 0 END), 0) AS current_total,
				COALESCE(SUM(CASE WHEN sp.year_value = ry.prev_year AND sp.jenis_lomba IN ('GURU', 'TENDIK') THEN 1 ELSE 0 END), 0) AS prev_total
			FROM scoped_prestasi sp
			CROSS JOIN resolved_years ry
		)
		SELECT
			pc.prestasi_total,
            (SELECT COUNT(*) FROM scoped_prestasi WHERE year_value = $2),
            (SELECT COALESCE(SUM(staff_total),0) FROM scoped_schools),
			sc.active_schools,
			yb.year_min,
			yb.year_max,
			sc.total_sekolah,
			sc.total_ab,
			ry.latest_year,
			ry.prev_year,
			siswa.current_total,
			siswa.prev_total,
			gt.current_total,
			gt.prev_total
		FROM prestasi_counts pc
		CROSS JOIN school_counts sc
		CROSS JOIN year_bounds yb
		CROSS JOIN resolved_years ry
		CROSS JOIN siswa_counts siswa
		CROSS JOIN guru_tendik_counts gt
	`

	var (
		payload                  LandingSummary
		yearMin                  sql.NullInt64
		yearMax                  sql.NullInt64
		totalSekolah             int
		totalSekolahAkreditasiAB int
		latestYear               sql.NullInt64
		prevYear                 sql.NullInt64
	)

	if err := s.DB.Pool.QueryRow(ctx, query, schoolIDs, portalYear).Scan(
		&payload.PrestasiTotal,
		&payload.PortalPrestasiTotal,
		&payload.PortalGuruTendik,
		&payload.ActiveSchools,
		&yearMin,
		&yearMax,
		&totalSekolah,
		&totalSekolahAkreditasiAB,
		&latestYear,
		&prevYear,
		&payload.CurrentYearSiswa,
		&payload.PrevYearSiswa,
		&payload.CurrentYearGuruTendik,
		&payload.PrevYearGuruTendik,
	); err != nil {
		return LandingSummary{}, err
	}

	payload.PortalPrestasiYear = portalYear
	payload.YearMin = intPtr(yearMin)
	payload.YearMax = intPtr(yearMax)
	payload.LatestYear = intPtr(latestYear)
	payload.PrevYear = intPtr(prevYear)
	payload.AkreditasiABPercent = calculatePercent(totalSekolahAkreditasiAB, totalSekolah)
	payload.YoYSiswaPercent = calculateYoY(payload.CurrentYearSiswa, payload.PrevYearSiswa)
	payload.YoYGuruTendikPercent = calculateYoY(payload.CurrentYearGuruTendik, payload.PrevYearGuruTendik)

	return payload, nil
}

func calculatePercent(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return (float64(numerator) / float64(denominator)) * 100
}

func calculateYoY(currentTotal, prevTotal int) float64 {
	if prevTotal <= 0 {
		if currentTotal > 0 {
			return 100
		}
		return 0
	}
	return (float64(currentTotal-prevTotal) / float64(prevTotal)) * 100
}

func intPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}
