package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/db"
)

type LegacyImportService struct {
	DB *db.DB
}

type LegacyImportSettings struct {
	GlobalEnabled   bool
	AutoDisableYear int
	ForceEnabled    bool
	MaxYear         int
}

type LegacyImportSettingsUpdate struct {
	GlobalEnabled   *bool
	AutoDisableYear *int
	ForceEnabled    *bool
	MaxYear         *int
}

type LegacyImportLevelSetting struct {
	Level     string
	Enabled   bool
	ScopeType string
	ScopeID   *int
	UpdatedBy *int
	UpdatedAt time.Time
}

var legacyImportAllowedLevels = map[string]struct{}{
	"SMA": {},
	"SMK": {},
	"SLB": {},
}

var legacyImportLevels = []string{"SMA", "SMK", "SLB"}

func normalizeLegacyLevel(level string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(level))
	if normalized == "" {
		return "", errors.New("LEVEL_REQUIRED")
	}
	if _, ok := legacyImportAllowedLevels[normalized]; !ok {
		return "", errors.New("LEVEL_INVALID")
	}
	return normalized, nil
}

func (s LegacyImportService) EnsureSettingsRow(ctx context.Context) error {
	_, err := s.DB.Pool.Exec(ctx, `
		INSERT INTO legacy_import_settings (id)
		VALUES (1)
		ON CONFLICT (id) DO NOTHING
	`)
	return err
}

func (s LegacyImportService) GetSettings(ctx context.Context) (LegacyImportSettings, error) {
	if err := s.EnsureSettingsRow(ctx); err != nil {
		return LegacyImportSettings{}, err
	}

	var settings LegacyImportSettings
	err := s.DB.Pool.QueryRow(ctx, `
		SELECT global_enabled, auto_disable_year, force_enabled, max_year
		FROM legacy_import_settings
		ORDER BY id
		LIMIT 1
	`).Scan(
		&settings.GlobalEnabled,
		&settings.AutoDisableYear,
		&settings.ForceEnabled,
		&settings.MaxYear,
	)
	if err != nil {
		return LegacyImportSettings{}, err
	}
	return settings, nil
}

func (s LegacyImportService) UpdateSettings(ctx context.Context, update LegacyImportSettingsUpdate, updatedBy int) (LegacyImportSettings, error) {
	if update.AutoDisableYear != nil && *update.AutoDisableYear <= 0 {
		return LegacyImportSettings{}, errors.New("AUTO_DISABLE_YEAR_INVALID")
	}
	if update.MaxYear != nil && *update.MaxYear <= 0 {
		return LegacyImportSettings{}, errors.New("MAX_YEAR_INVALID")
	}

	if err := s.EnsureSettingsRow(ctx); err != nil {
		return LegacyImportSettings{}, err
	}

	var settings LegacyImportSettings
	err := s.DB.Pool.QueryRow(ctx, `
		UPDATE legacy_import_settings
		SET global_enabled = COALESCE($1, global_enabled),
			auto_disable_year = COALESCE($2, auto_disable_year),
			force_enabled = COALESCE($3, force_enabled),
			max_year = COALESCE($4, max_year),
			updated_by = $5,
			updated_at = NOW()
		WHERE id = 1
		RETURNING global_enabled, auto_disable_year, force_enabled, max_year
	`, update.GlobalEnabled, update.AutoDisableYear, update.ForceEnabled, update.MaxYear, updatedBy).Scan(
		&settings.GlobalEnabled,
		&settings.AutoDisableYear,
		&settings.ForceEnabled,
		&settings.MaxYear,
	)
	if err != nil {
		return LegacyImportSettings{}, err
	}
	return settings, nil
}

func (s LegacyImportService) ListLevels(ctx context.Context, scopeType string, scopeID *int) (map[string]LegacyImportLevelSetting, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT level, enabled, scope_type, scope_id, updated_by, updated_at
		FROM legacy_import_level_settings
		WHERE scope_type = $1
		  AND scope_id IS NOT DISTINCT FROM $2
	`, scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]LegacyImportLevelSetting)
	for rows.Next() {
		var item LegacyImportLevelSetting
		var scopeIDValue *int
		if scanErr := rows.Scan(&item.Level, &item.Enabled, &item.ScopeType, &scopeIDValue, &item.UpdatedBy, &item.UpdatedAt); scanErr != nil {
			return nil, scanErr
		}
		item.Level = strings.ToUpper(strings.TrimSpace(item.Level))
		item.ScopeID = scopeIDValue
		result[item.Level] = item
	}
	return result, nil
}

func (s LegacyImportService) GetLevel(ctx context.Context, scopeType string, scopeID *int, level string) (*LegacyImportLevelSetting, error) {
	normalized, err := normalizeLegacyLevel(level)
	if err != nil {
		return nil, err
	}

	var item LegacyImportLevelSetting
	var scopeIDValue *int
	err = s.DB.Pool.QueryRow(ctx, `
		SELECT level, enabled, scope_type, scope_id, updated_by, updated_at
		FROM legacy_import_level_settings
		WHERE scope_type = $1
		  AND scope_id IS NOT DISTINCT FROM $2
		  AND level = $3
	`, scopeType, scopeID, normalized).Scan(
		&item.Level,
		&item.Enabled,
		&item.ScopeType,
		&scopeIDValue,
		&item.UpdatedBy,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.Level = strings.ToUpper(strings.TrimSpace(item.Level))
	item.ScopeID = scopeIDValue
	return &item, nil
}

func (s LegacyImportService) UpsertLevelSettings(ctx context.Context, scopeType string, scopeID *int, updates map[string]bool, updatedBy int) error {
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for level, enabled := range updates {
		normalized, err := normalizeLegacyLevel(level)
		if err != nil {
			return err
		}
		if _, execErr := tx.Exec(ctx, `
			INSERT INTO legacy_import_level_settings (
				scope_type, scope_id, level, enabled, updated_by, updated_at
			) VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (scope_type, scope_id, level)
			DO UPDATE SET
				enabled = EXCLUDED.enabled,
				updated_by = EXCLUDED.updated_by,
				updated_at = NOW()
		`, scopeType, scopeID, normalized, enabled, updatedBy); execErr != nil {
			return execErr
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s LegacyImportService) DeleteLevel(ctx context.Context, scopeType string, scopeID *int, level string) error {
	normalized, err := normalizeLegacyLevel(level)
	if err != nil {
		return err
	}
	_, execErr := s.DB.Pool.Exec(ctx, `
		DELETE FROM legacy_import_level_settings
		WHERE scope_type = $1
		  AND scope_id IS NOT DISTINCT FROM $2
		  AND level = $3
	`, scopeType, scopeID, normalized)
	return execErr
}

func (s LegacyImportService) EffectiveLegacyEnabled(ctx context.Context, level string, cabdinUserID *int, now time.Time) (bool, LegacyImportSettings, string, error) {
	normalized, err := normalizeLegacyLevel(level)
	if err != nil {
		return false, LegacyImportSettings{}, "", err
	}

	settings, err := s.GetSettings(ctx)
	if err != nil {
		return false, LegacyImportSettings{}, "", err
	}

	if !settings.GlobalEnabled {
		return false, settings, "GLOBAL_DISABLED", nil
	}
	if now.Year() >= settings.AutoDisableYear && !settings.ForceEnabled {
		return false, settings, "AUTO_DISABLED", nil
	}

	masterSetting, err := s.GetLevel(ctx, "MASTER", nil, normalized)
	if err == nil && masterSetting != nil {
		return masterSetting.Enabled, settings, "MASTER", nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, settings, "", err
	}

	if cabdinUserID != nil {
		cabdinSetting, cabdinErr := s.GetLevel(ctx, "CABDIN", cabdinUserID, normalized)
		if cabdinErr == nil && cabdinSetting != nil {
			return cabdinSetting.Enabled, settings, "CABDIN", nil
		}
		if cabdinErr != nil && !errors.Is(cabdinErr, pgx.ErrNoRows) {
			return false, settings, "", cabdinErr
		}
	}

	return true, settings, "DEFAULT", nil
}

func (s LegacyImportService) ValidateLevelList(levels []string) ([]string, error) {
	if len(levels) == 0 {
		return nil, errors.New("LEVELS_REQUIRED")
	}

	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(levels))
	for _, level := range levels {
		value, err := normalizeLegacyLevel(level)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func LegacyImportLevels() []string {
	return append([]string{}, legacyImportLevels...)
}

func FormatLegacyImportDisabledReason(source string, settings LegacyImportSettings, level string) string {
	switch source {
	case "GLOBAL_DISABLED":
		return "LEGACY_IMPORT_DISABLED_GLOBAL"
	case "AUTO_DISABLED":
		return fmt.Sprintf("LEGACY_IMPORT_AUTO_DISABLED_%d", settings.AutoDisableYear)
	default:
		return fmt.Sprintf("LEGACY_IMPORT_DISABLED_%s_%s", level, source)
	}
}
