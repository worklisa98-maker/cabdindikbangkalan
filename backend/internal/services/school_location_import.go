package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const locationSeedFile = "data-sekolah-map-bangkalan.xlsx"

// LocationSeedPath is server-controlled; HTTP callers cannot select a file.
func LocationSeedPath() string {
	if configured := os.Getenv("SCHOOL_LOCATION_SEED_PATH"); configured != "" {
		return configured
	}
	candidates := []string{filepath.Join("assets", locationSeedFile), filepath.Join("backend", "assets", locationSeedFile)}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "assets", locationSeedFile))
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return candidates[0]
}

type LocationImportRow struct {
	NPSN      string  `json:"npsn"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type LocationImportReport struct {
	TotalRows        int                 `json:"total_rows"`
	ValidCoordinates int                 `json:"valid_coordinates"`
	Matched          int                 `json:"matched"`
	Eligible         int                 `json:"eligible"`
	Imported         int                 `json:"imported"`
	SkippedExisting  int                 `json:"skipped_existing"`
	SkippedManual    int                 `json:"skipped_manual"`
	SkippedInvalid   int                 `json:"skipped_invalid"`
	NotFound         int                 `json:"not_found"`
	Errors           []string            `json:"errors"`
	Rows             []LocationImportRow `json:"rows"`
}

// RunLocationImport shares parsing, matching and eligibility across CLI/API.
// Preview uses a database-enforced read-only transaction. Apply locks each matched
// row before evaluating eligibility, including concurrent manual edits/imports.
func RunLocationImport(ctx context.Context, pool *pgxpool.Pool, path string, apply bool, actorID *int) (LocationImportReport, error) {
	report := LocationImportReport{Errors: []string{}, Rows: []LocationImportRow{}}
	seeds, parsed, err := ReadLocationSeed(path)
	if err != nil {
		return report, err
	}
	report.TotalRows, report.ValidCoordinates, report.SkippedInvalid = parsed.Rows, parsed.Rows-parsed.InvalidCoordinates, parsed.Skipped
	options := pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}
	if apply {
		options = pgx.TxOptions{}
	}
	tx, err := pool.BeginTx(ctx, options)
	if err != nil {
		return report, fmt.Errorf("begin school location import: %w", err)
	}
	defer tx.Rollback(ctx)
	imported := 0
	for _, seed := range seeds {
		var id int
		var name, source string
		var lat, lon *float64
		var updated *time.Time
		var updatedBy *int
		var verified bool
		query := `SELECT id,nama_sekolah,latitude,longitude,location_source,location_verified,location_updated_at,location_updated_by FROM data_sekolah WHERE npsn_sekolah=$1`
		if apply {
			query += " FOR UPDATE"
		}
		err := tx.QueryRow(ctx, query, seed.NPSN).Scan(&id, &name, &lat, &lon, &source, &verified, &updated, &updatedBy)
		if err == pgx.ErrNoRows {
			report.NotFound++
			continue
		}
		if err != nil {
			return report, fmt.Errorf("match school location (verify migration 066): %w", err)
		}
		report.Matched++
		// Existing importer stores workbook provenance, not an EXCEL_SEED enum.
		// Preserve that convention; never replace any populated or audited location.
		if updatedBy != nil {
			report.SkippedManual++
			continue
		}
		if lat != nil || lon != nil {
			report.SkippedExisting++
			continue
		}
		if source != "" || verified || updated != nil {
			report.SkippedManual++
			continue
		}
		report.Eligible++
		if len(report.Rows) < 50 {
			report.Rows = append(report.Rows, LocationImportRow{seed.NPSN, name, seed.Latitude, seed.Longitude})
		}
		if apply {
			_, err := tx.Exec(ctx, `UPDATE data_sekolah SET latitude=$1,longitude=$2,location_source=$3,location_verified=$4,location_updated_at=NOW() WHERE id=$5`, seed.Latitude, seed.Longitude, seed.Source, seed.Verified, id)
			if err != nil {
				return report, fmt.Errorf("school location import rolled back: %w", err)
			}
			imported++
		}
	}
	if apply {
		if actorID != nil {
			if _, err := tx.Exec(ctx, `INSERT INTO admin_audit_log (admin_id,action,count) VALUES ($1,'SCHOOL_LOCATION_INITIAL_IMPORT',$2)`, *actorID, imported); err != nil {
				return report, fmt.Errorf("audit school location import: %w", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return report, fmt.Errorf("commit school location import: %w", err)
		}
		report.Imported = imported
	}
	return report, nil
}
