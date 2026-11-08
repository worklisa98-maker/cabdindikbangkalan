package migrations

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upSeedRefKampusFromCSV, downSeedRefKampusFromCSV)
}

func upSeedRefKampusFromCSV(ctx context.Context, tx *sql.Tx) error {
	file, sourcePath, err := openRefKampusCSV()
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	firstRow, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("failed reading first row from %s: %w", sourcePath, err)
	}

	nameColumnIdx := 0
	hasHeader := rowLooksLikeHeader(firstRow)
	if hasHeader {
		nameColumnIdx = detectNameColumn(firstRow)
		if nameColumnIdx < 0 {
			nameColumnIdx = 0
		}
	}

	seen := map[string]struct{}{}
	insertCampus := func(raw string) error {
		normalized := normalizeRefKampusName(raw)
		if normalized == "" {
			return nil
		}
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			return nil
		}
		seen[key] = struct{}{}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ref_kampus (nama)
			SELECT $1
			WHERE NOT EXISTS (
				SELECT 1
				FROM ref_kampus
				WHERE LOWER(nama) = LOWER($1)
			)
			ON CONFLICT (nama) DO NOTHING
		`, normalized); err != nil {
			return fmt.Errorf("failed inserting ref_kampus %q: %w", normalized, err)
		}
		return nil
	}

	if !hasHeader && len(firstRow) > 0 {
		if err := insertCampus(firstRow[0]); err != nil {
			return err
		}
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed reading csv %s: %w", sourcePath, err)
		}
		if len(record) == 0 {
			continue
		}

		column := 0
		if hasHeader {
			column = nameColumnIdx
		}
		if column >= len(record) {
			continue
		}
		if err := insertCampus(record[column]); err != nil {
			return err
		}
	}

	return nil
}

func downSeedRefKampusFromCSV(context.Context, *sql.Tx) error {
	// No-op. Reference data should not be removed on migration rollback.
	return nil
}

func openRefKampusCSV() (*os.File, string, error) {
	candidates := []string{
		"./backend/assets/kampus_indonesia.csv",
		"../assets/kampus_indonesia.csv",
		"assets/kampus_indonesia.csv",
	}
	for _, candidate := range candidates {
		file, err := os.Open(candidate)
		if err == nil {
			return file, candidate, nil
		}
	}
	return nil, "", fmt.Errorf(
		"failed to open kampus CSV. tried paths: %s",
		strings.Join(candidates, ", "),
	)
}

func rowLooksLikeHeader(row []string) bool {
	for _, cell := range row {
		lower := strings.ToLower(strings.TrimSpace(cell))
		if strings.Contains(lower, "nama") || strings.Contains(lower, "kampus") {
			return true
		}
	}
	return false
}

func detectNameColumn(row []string) int {
	bestIdx := -1
	bestScore := -1
	for idx, cell := range row {
		lower := strings.ToLower(strings.TrimSpace(cell))
		score := 0
		if strings.Contains(lower, "nama") {
			score++
		}
		if strings.Contains(lower, "kampus") {
			score++
		}
		if score > bestScore {
			bestScore = score
			bestIdx = idx
		}
	}
	return bestIdx
}

func normalizeRefKampusName(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}
