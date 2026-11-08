package services

import (
	"fmt"
	"github.com/xuri/excelize/v2"
	"math"
	"strconv"
	"strings"
)

type LocationSeed struct {
	NPSN                string
	Latitude, Longitude float64
	Source              string
	Verified            bool
}
type LocationSeedReport struct {
	InvalidCoordinates int
	InvalidNPSN        int
	DuplicateNPSN      int
	Rows               int
	Valid              int
	Skipped            int
	Reasons            []string
}

func ReadLocationSeed(path string) ([]LocationSeed, LocationSeedReport, error) {
	report := LocationSeedReport{}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, report, fmt.Errorf("school location seed file unavailable: expected path=%s: %w", path, err)
	}
	defer f.Close()
	rows, err := f.GetRows("Data Map")
	if err != nil {
		return nil, report, err
	}
	if len(rows) == 0 {
		return nil, report, fmt.Errorf("Data Map sheet is empty")
	}
	columns := map[string]int{}
	for i, name := range rows[0] {
		columns[strings.ToLower(strings.TrimSpace(name))] = i
	}
	for _, key := range []string{"npsn", "latitude", "longitude", "sumber resmi", "status koordinat"} {
		if _, ok := columns[key]; !ok {
			return nil, report, fmt.Errorf("missing column %s", key)
		}
	}
	result := []LocationSeed{}
	seen := map[string]bool{}
	for i, row := range rows[1:] {
		report.Rows++
		cell := func(key string) string {
			col := columns[key]
			if col >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[col])
		}
		npsn := cell("npsn")
		lat, latErr := strconv.ParseFloat(cell("latitude"), 64)
		lon, lonErr := strconv.ParseFloat(cell("longitude"), 64)
		reason := ""
		if latErr != nil || lonErr != nil || math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			report.InvalidCoordinates++
			reason = "invalid/missing coordinate"
		} else if len(npsn) != 8 || strings.Trim(npsn, "0123456789") != "" {
			report.InvalidNPSN++
			reason = "invalid NPSN"
		} else if seen[npsn] {
			report.DuplicateNPSN++
			reason = "duplicate NPSN"
		}
		if reason != "" {
			report.Skipped++
			report.Reasons = append(report.Reasons, fmt.Sprintf("row %d: %s", i+2, reason))
			continue
		}
		seen[npsn] = true
		result = append(result, LocationSeed{NPSN: npsn, Latitude: lat, Longitude: lon, Source: cell("sumber resmi"), Verified: strings.HasPrefix(strings.ToLower(cell("status koordinat")), "terverifikasi")})
	}
	report.Valid = len(result)
	return result, report, nil
}
