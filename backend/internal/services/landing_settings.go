package services

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

const landingSettingsKey = "landing_settings"

type LandingSettingsService struct {
	DB *db.DB
}

type LandingSettings struct {
	HeroTitle       string `json:"hero_title"`
	HeroSubtitle    string `json:"hero_subtitle"`
	HeroImageURL    string `json:"hero_image_url"`
	HeroImageAlt    string `json:"hero_image_alt"`
	ContactWhatsapp string `json:"contact_whatsapp"`
	ContactPhone    string `json:"contact_phone"`
	ContactEmail    string `json:"contact_email"`
	ContactAddress  string `json:"contact_address"`
	ContactHours    string `json:"contact_hours"`
	GuidebookURL    string `json:"guidebook_url"`
}

var defaultLandingSettings = LandingSettings{
	HeroTitle:       "Satu Sistem. Satu Data. Prestasi Terintegrasi.",
	HeroSubtitle:    "SIPRES membantu sekolah dan Cabdin memantau prestasi, rencana lulusan, dan analitik secara terstruktur.",
	HeroImageURL:    "",
	HeroImageAlt:    "Visual SIPRES",
	ContactWhatsapp: "0812-xxxx-xxxx",
	ContactPhone:    "(031) xxx xxxx",
	ContactEmail:    "cabdin.bangkalan@example.go.id",
	ContactAddress:  "Cabang Dinas Pendidikan Wilayah Bangkalan",
	ContactHours:    "Senin - Jumat, 08.00 - 15.00 WIB",
	GuidebookURL:    "",
}

func NormalizeLandingSettings(value map[string]any) LandingSettings {
	settings := defaultLandingSettings
	if value == nil {
		return settings
	}
	settings.HeroTitle = landingString(value, "hero_title", settings.HeroTitle)
	settings.HeroSubtitle = landingString(value, "hero_subtitle", settings.HeroSubtitle)
	settings.HeroImageURL = landingString(value, "hero_image_url", settings.HeroImageURL)
	settings.HeroImageAlt = landingString(value, "hero_image_alt", settings.HeroImageAlt)
	settings.ContactWhatsapp = landingString(value, "contact_whatsapp", settings.ContactWhatsapp)
	settings.ContactPhone = landingString(value, "contact_phone", settings.ContactPhone)
	settings.ContactEmail = landingString(value, "contact_email", settings.ContactEmail)
	settings.ContactAddress = landingString(value, "contact_address", settings.ContactAddress)
	settings.ContactHours = landingString(value, "contact_hours", settings.ContactHours)
	settings.GuidebookURL = landingString(value, "guidebook_url", settings.GuidebookURL)
	return settings
}

func (s LandingSettingsService) Get(ctx context.Context) (LandingSettings, error) {
	var raw []byte
	err := s.DB.Pool.QueryRow(ctx, `
		SELECT setting_value
		FROM site_settings
		WHERE setting_key = $1
	`, landingSettingsKey).Scan(&raw)
	if err != nil {
		if err == pgx.ErrNoRows {
			return defaultLandingSettings, nil
		}
		return LandingSettings{}, err
	}

	var value map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &value); err != nil {
			return LandingSettings{}, err
		}
	}
	return NormalizeLandingSettings(value), nil
}

func (s LandingSettingsService) Update(ctx context.Context, user auth.User, patch map[string]any) (LandingSettings, error) {
	if !auth.IsAdminMaster(user) {
		return LandingSettings{}, auth.ErrForbidden
	}

	current, err := s.Get(ctx)
	if err != nil {
		return LandingSettings{}, err
	}
	merged := landingSettingsToMap(current)
	for key, value := range patch {
		merged[key] = value
	}
	settings := NormalizeLandingSettings(merged)
	raw, err := json.Marshal(settings)
	if err != nil {
		return LandingSettings{}, err
	}

	if _, err := s.DB.Pool.Exec(ctx, `
		INSERT INTO site_settings (setting_key, setting_value, updated_by, updated_at)
		VALUES ($1, $2::jsonb, $3, NOW())
		ON CONFLICT (setting_key) DO UPDATE
		SET setting_value = EXCLUDED.setting_value,
		    updated_by = EXCLUDED.updated_by,
		    updated_at = NOW()
	`, landingSettingsKey, string(raw), user.ID); err != nil {
		return LandingSettings{}, err
	}

	return settings, nil
}

func landingString(source map[string]any, key string, fallback string) string {
	if value, ok := source[key].(string); ok {
		return value
	}
	return fallback
}

func landingSettingsToMap(settings LandingSettings) map[string]any {
	return map[string]any{
		"hero_title":       settings.HeroTitle,
		"hero_subtitle":    settings.HeroSubtitle,
		"hero_image_url":   settings.HeroImageURL,
		"hero_image_alt":   settings.HeroImageAlt,
		"contact_whatsapp": settings.ContactWhatsapp,
		"contact_phone":    settings.ContactPhone,
		"contact_email":    settings.ContactEmail,
		"contact_address":  settings.ContactAddress,
		"contact_hours":    settings.ContactHours,
		"guidebook_url":    settings.GuidebookURL,
	}
}
