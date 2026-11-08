package services

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	refKampusSourceSeedCSV = "seed_csv"
	refKampusSourceManual  = "manual"
)

type RefKampusAdminFilters struct {
	Query           string
	Limit           int
	IncludeInactive bool
}

type RefKampusCreateInput struct {
	Nama string
}

type RefKampusUpdateInput struct {
	Nama     *string
	IsActive *bool
}

func normalizeManagedRefKampusName(value string) (string, error) {
	name := normalizeRefKampusName(value)
	if name == "" {
		return "", errors.New("REF_KAMPUS_NAME_REQUIRED")
	}
	return name, nil
}

func formatRefKampusTimestamp(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func scanManagedRefKampusItem(scanner interface {
	Scan(dest ...any) error
}) (RefKampusItem, error) {
	var (
		item      RefKampusItem
		createdAt *time.Time
		updatedAt *time.Time
	)
	if err := scanner.Scan(&item.ID, &item.Nama, &item.IsActive, &item.Source, &createdAt, &updatedAt); err != nil {
		return RefKampusItem{}, err
	}
	item.CreatedAt = formatRefKampusTimestamp(createdAt)
	item.UpdatedAt = formatRefKampusTimestamp(updatedAt)
	return item, nil
}

func (s AdminService) ListManagedRefKampus(ctx context.Context, filters RefKampusAdminFilters) ([]RefKampusItem, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	query := strings.TrimSpace(filters.Query)
	sql := `
		SELECT id,
		       nama,
		       COALESCE(is_active, TRUE) AS is_active,
		       COALESCE(source, '') AS source,
		       created_at,
		       updated_at
		FROM ref_kampus
		WHERE 1=1
	`
	args := make([]any, 0, 3)
	argIdx := 1
	if !filters.IncludeInactive {
		sql += " AND COALESCE(is_active, TRUE) = TRUE"
	}
	if query != "" {
		sql += " AND nama ILIKE '%' || $" + strconv.Itoa(argIdx) + " || '%'"
		args = append(args, query)
		argIdx++
	}
	sql += " ORDER BY nama ASC LIMIT $" + strconv.Itoa(argIdx)
	args = append(args, limit)

	rows, err := s.DB.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RefKampusItem, 0)
	for rows.Next() {
		item, scanErr := scanManagedRefKampusItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s AdminService) CreateManagedRefKampus(ctx context.Context, input RefKampusCreateInput) (RefKampusItem, error) {
	name, err := normalizeManagedRefKampusName(input.Nama)
	if err != nil {
		return RefKampusItem{}, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return RefKampusItem{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var existingID int
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM ref_kampus
		WHERE LOWER(REGEXP_REPLACE(TRIM(COALESCE(nama, '')), '\s+', ' ', 'g')) = $1
		LIMIT 1
	`, normalizeRefKampusKey(name)).Scan(&existingID)
	if err == nil {
		return RefKampusItem{}, errors.New("REF_KAMPUS_NAME_EXISTS")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return RefKampusItem{}, err
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO ref_kampus (nama, is_active, source, updated_at)
		VALUES ($1, TRUE, $2, NOW())
		RETURNING id,
		          nama,
		          COALESCE(is_active, TRUE) AS is_active,
		          COALESCE(source, '') AS source,
		          created_at,
		          updated_at
	`, name, refKampusSourceManual)
	item, err := scanManagedRefKampusItem(row)
	if err != nil {
		return RefKampusItem{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return RefKampusItem{}, err
	}
	return item, nil
}

func (s AdminService) UpdateManagedRefKampus(ctx context.Context, id int, input RefKampusUpdateInput) (RefKampusItem, error) {
	if id <= 0 {
		return RefKampusItem{}, errors.New("INVALID_REF_KAMPUS_ID")
	}
	if input.Nama == nil && input.IsActive == nil {
		return RefKampusItem{}, errors.New("INVALID_PAYLOAD")
	}

	var normalizedName *string
	if input.Nama != nil {
		name, err := normalizeManagedRefKampusName(*input.Nama)
		if err != nil {
			return RefKampusItem{}, err
		}
		normalizedName = &name
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return RefKampusItem{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM ref_kampus WHERE id = $1)`, id).Scan(&exists); err != nil {
		return RefKampusItem{}, err
	}
	if !exists {
		return RefKampusItem{}, errors.New("REF_KAMPUS_NOT_FOUND")
	}

	if normalizedName != nil {
		var duplicateID int
		err = tx.QueryRow(ctx, `
			SELECT id
			FROM ref_kampus
			WHERE id <> $1
			  AND LOWER(REGEXP_REPLACE(TRIM(COALESCE(nama, '')), '\s+', ' ', 'g')) = $2
			LIMIT 1
		`, id, normalizeRefKampusKey(*normalizedName)).Scan(&duplicateID)
		if err == nil {
			return RefKampusItem{}, errors.New("REF_KAMPUS_NAME_EXISTS")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return RefKampusItem{}, err
		}
	}

	row := tx.QueryRow(ctx, `
		UPDATE ref_kampus
		SET nama = COALESCE($2, nama),
		    is_active = COALESCE($3, is_active),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id,
		          nama,
		          COALESCE(is_active, TRUE) AS is_active,
		          COALESCE(source, '') AS source,
		          created_at,
		          updated_at
	`, id, normalizedName, input.IsActive)
	item, err := scanManagedRefKampusItem(row)
	if err != nil {
		return RefKampusItem{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return RefKampusItem{}, err
	}
	return item, nil
}
