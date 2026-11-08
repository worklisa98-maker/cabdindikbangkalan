package services

import (
	"context"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

const jenjangProdiUnknown = "UNKNOWN"
const strictRefKampusRejectReason = "nama Perguruan tinggi tidak ada yang cocok dengan DB tolong jangan disingkat atau check excel templatenya"

type RefKampusItem struct {
	ID        int     `json:"id"`
	Nama      string  `json:"nama"`
	IsActive  bool    `json:"isActive"`
	Source    string  `json:"source,omitempty"`
	CreatedAt *string `json:"createdAt,omitempty"`
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

type refKampusQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func normalizeRefKampusName(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normalizeRefKampusKey(value string) string {
	return strings.ToLower(normalizeRefKampusName(value))
}

func collectRefKampusKeys(values ...string) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0, len(values))
	for _, value := range values {
		key := normalizeRefKampusKey(value)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func resolveRefKampusStrict(value string, campusByKey map[string]RefKampusItem) (RefKampusItem, bool) {
	item, ok := campusByKey[normalizeRefKampusKey(value)]
	return item, ok
}

func normalizeJenjangProdi(value string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if upper == "" {
		return jenjangProdiUnknown, true
	}
	switch upper {
	case "D1", "D2", "D3", "D4", "S1", "S2", "S3", "UNKNOWN":
		return upper, true
	default:
		return "", false
	}
}

func (s SiswaService) SearchRefKampus(ctx context.Context, query string, limit int) ([]RefKampusItem, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	trimmed := strings.TrimSpace(query)
	sql := `
		SELECT id, nama
		FROM ref_kampus
		WHERE COALESCE(is_active, TRUE) = TRUE
	`
	args := []any{}
	if trimmed != "" {
		sql += " AND nama ILIKE '%' || $1 || '%'"
		args = append(args, trimmed)
		sql += " ORDER BY nama ASC LIMIT $2"
		args = append(args, limit)
	} else {
		sql += " ORDER BY nama ASC LIMIT $1"
		args = append(args, limit)
	}

	rows, err := s.DB.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RefKampusItem, 0)
	for rows.Next() {
		var item RefKampusItem
		if scanErr := rows.Scan(&item.ID, &item.Nama); scanErr == nil {
			item.IsActive = true
			items = append(items, item)
		}
	}
	return items, nil
}

func (s SiswaService) ListAllRefKampus(ctx context.Context) ([]RefKampusItem, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT id, nama
		FROM ref_kampus
		WHERE COALESCE(is_active, TRUE) = TRUE
		ORDER BY nama ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RefKampusItem, 0)
	for rows.Next() {
		var item RefKampusItem
		if scanErr := rows.Scan(&item.ID, &item.Nama); scanErr == nil {
			item.IsActive = true
			items = append(items, item)
		}
	}
	return items, nil
}

func fetchRefKampusByIDs(ctx context.Context, q refKampusQuerier, ids []int32) (map[int]RefKampusItem, error) {
	result := map[int]RefKampusItem{}
	if len(ids) == 0 {
		return result, nil
	}

	rows, err := q.Query(ctx, `
		SELECT id, nama, COALESCE(is_active, TRUE) AS is_active
		FROM ref_kampus
		WHERE id = ANY($1)
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item RefKampusItem
		if scanErr := rows.Scan(&item.ID, &item.Nama, &item.IsActive); scanErr != nil {
			return nil, scanErr
		}
		result[item.ID] = item
	}
	return result, nil
}

func fetchRefKampusByKeys(ctx context.Context, q refKampusQuerier, keys []string) (map[string]RefKampusItem, error) {
	result := map[string]RefKampusItem{}
	if len(keys) == 0 {
		return result, nil
	}

	rows, err := q.Query(ctx, `
		SELECT id,
		       nama,
		       COALESCE(is_active, TRUE) AS is_active,
		       LOWER(REGEXP_REPLACE(TRIM(COALESCE(nama, '')), '\s+', ' ', 'g')) AS normalized_key
		FROM ref_kampus
		WHERE LOWER(REGEXP_REPLACE(TRIM(COALESCE(nama, '')), '\s+', ' ', 'g')) = ANY($1)
	`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			item          RefKampusItem
			normalizedKey string
		)
		if scanErr := rows.Scan(&item.ID, &item.Nama, &item.IsActive, &normalizedKey); scanErr != nil {
			return nil, scanErr
		}
		normalizedKey = strings.TrimSpace(strings.ToLower(normalizedKey))
		if normalizedKey == "" {
			continue
		}
		if _, exists := result[normalizedKey]; exists {
			continue
		}
		result[normalizedKey] = item
	}
	return result, nil
}
