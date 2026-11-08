package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"cabdin-bangkalan-backend/internal/db"
	"github.com/jackc/pgx/v5"
)

const (
	HaloSettingsKey  = "service_halo_kepegawaian"
	SinomSettingsKey = "service_sinom"
	LinksSettingsKey = "service_pusat_tautan"
)

var ErrSettingsConflict = errors.New("settings changed since last load")
var ErrInvalidSettings = errors.New("invalid service settings")

type ExternalService struct {
	URL          string `json:"url"`
	Description  string `json:"description"`
	IsActive     bool   `json:"is_active"`
	DialogTitle  string `json:"dialog_title,omitempty"`
	DialogBody   string `json:"dialog_body,omitempty"`
	DialogButton string `json:"dialog_button,omitempty"`
}

type ServiceLink struct {
	ID          string `json:"id"`
	CategoryID  string `json:"category_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Icon        string `json:"icon"`
	SortOrder   int    `json:"sort_order"`
	IsActive    bool   `json:"is_active"`
	IsPublic    *bool  `json:"is_public"`
}

type LinkCategory struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	SortOrder int    `json:"sort_order"`
}

type LinkSettings struct {
	Categories []LinkCategory `json:"categories"`
	Links      []ServiceLink  `json:"links"`
}

type VersionedLinks struct {
	LinkSettings
	Version string `json:"version"`
}

type ServiceLinksService struct{ DB *db.DB }

var defaultHalo = ExternalService{URL: "http://halokepegawaian.web.id/", Description: "Akses layanan informasi dan pengurusan administrasi kepegawaian.", IsActive: true, DialogTitle: "Informasi Halo Kepegawaian", DialogBody: "Gunakan akun Gmail untuk mengakses layanan Halo Kepegawaian.", DialogButton: "Lanjutkan ke Halo Kepegawaian"}
var defaultSinom = ExternalService{Description: "Akses layanan SINOM.", IsActive: false}
var publicByDefault = true

func defaultLinks() LinkSettings {
	return LinkSettings{
		Categories: []LinkCategory{{ID: "laporan", Title: "Laporan", SortOrder: 0}, {ID: "data-sekolah", Title: "Data Sekolah", SortOrder: 1}},
		Links: []ServiceLink{
			{ID: "laporan-bpopp", CategoryID: "laporan", Title: "Laporan BPOPP", Icon: "📊", SortOrder: 0, IsActive: false, IsPublic: &publicByDefault},
			{ID: "laporan-bos", CategoryID: "laporan", Title: "Laporan BOS", Icon: "📊", SortOrder: 1, IsActive: false, IsPublic: &publicByDefault},
			{ID: "data-sekolah", CategoryID: "data-sekolah", Title: "Data Sekolah", Icon: "🏫", SortOrder: 0, IsActive: false, IsPublic: &publicByDefault},
		},
	}
}

func (s ServiceLinksService) read(ctx context.Context, key string, destination any) (string, error) {
	var raw []byte
	err := s.DB.Pool.QueryRow(ctx, `SELECT setting_value FROM site_settings WHERE setting_key=$1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		initial, marshalErr := json.Marshal(destination)
		if marshalErr != nil {
			return "", marshalErr
		}
		if _, insertErr := s.DB.Pool.Exec(ctx, `INSERT INTO site_settings(setting_key,setting_value) VALUES($1,$2) ON CONFLICT(setting_key) DO NOTHING`, key, initial); insertErr != nil {
			return "", insertErr
		}
		err = s.DB.Pool.QueryRow(ctx, `SELECT setting_value FROM site_settings WHERE setting_key=$1`, key).Scan(&raw)
	}
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return "", err
	}
	return string(raw), nil
}

func (s ServiceLinksService) GetExternal(ctx context.Context, key string) (ExternalService, error) {
	result := defaultSinom
	if key == HaloSettingsKey {
		result = defaultHalo
	} else if key != SinomSettingsKey {
		return ExternalService{}, ErrInvalidSettings
	}
	_, err := s.read(ctx, key, &result)
	return result, err
}

func validExternalURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != ""
}

func (s ServiceLinksService) SaveExternal(ctx context.Context, key string, value ExternalService, userID int) error {
	if key != HaloSettingsKey && key != SinomSettingsKey {
		return ErrInvalidSettings
	}
	value.URL = strings.TrimSpace(value.URL)
	if !validExternalURL(value.URL) || (value.IsActive && value.URL == "") {
		return ErrInvalidSettings
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.DB.Pool.Exec(ctx, `INSERT INTO site_settings(setting_key,setting_value,updated_by,updated_at) VALUES($1,$2,$3,NOW()) ON CONFLICT(setting_key) DO UPDATE SET setting_value=EXCLUDED.setting_value,updated_by=EXCLUDED.updated_by,updated_at=NOW()`, key, raw, userID)
	return err
}

func (s ServiceLinksService) GetLinks(ctx context.Context) (VersionedLinks, error) {
	result := defaultLinks()
	raw, err := s.read(ctx, LinksSettingsKey, &result)
	if err != nil {
		return VersionedLinks{}, err
	}
	// An existing empty document is intentional and must stay empty.
	if raw != "" {
		result = LinkSettings{}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return VersionedLinks{}, err
		}
	}
	if result.Categories == nil {
		result.Categories = []LinkCategory{}
	}
	if result.Links == nil {
		result.Links = []ServiceLink{}
	}
	normalizeLinkVisibility(result.Links)
	return VersionedLinks{LinkSettings: result, Version: raw}, nil
}

func normalizeLinkVisibility(links []ServiceLink) {
	for i := range links {
		if links[i].IsPublic == nil {
			visible := true
			links[i].IsPublic = &visible
		}
	}
}

func (s ServiceLinksService) SaveLinks(ctx context.Context, value LinkSettings, expectedVersion string, userID int) (VersionedLinks, error) {
	if value.Categories == nil {
		value.Categories = []LinkCategory{}
	}
	if value.Links == nil {
		value.Links = []ServiceLink{}
	}
	categories := map[string]bool{}
	for _, category := range value.Categories {
		if strings.TrimSpace(category.ID) == "" || strings.TrimSpace(category.Title) == "" || categories[category.ID] {
			return VersionedLinks{}, ErrInvalidSettings
		}
		categories[category.ID] = true
	}
	ids := map[string]bool{}
	for i := range value.Links {
		link := &value.Links[i]
		link.URL = strings.TrimSpace(link.URL)
		if link.IsPublic == nil {
			public := true
			link.IsPublic = &public
		}
		if link.ID == "" || link.Title == "" || !categories[link.CategoryID] || ids[link.ID] || !validExternalURL(link.URL) || (link.IsActive && link.URL == "") {
			return VersionedLinks{}, ErrInvalidSettings
		}
		ids[link.ID] = true
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return VersionedLinks{}, err
	}
	// Compare the stored JSON in the same statement. The row lock taken by UPDATE
	// makes a concurrent editor's stale version fail instead of losing changes.
	var saved []byte
	if expectedVersion == "" {
		err = s.DB.Pool.QueryRow(ctx, `INSERT INTO site_settings(setting_key,setting_value,updated_by,updated_at) VALUES($1,$2,$3,NOW()) ON CONFLICT(setting_key) DO NOTHING RETURNING setting_value`, LinksSettingsKey, raw, userID).Scan(&saved)
	} else {
		err = s.DB.Pool.QueryRow(ctx, `UPDATE site_settings SET setting_value=$2,updated_by=$3,updated_at=NOW() WHERE setting_key=$1 AND setting_value=$4::jsonb RETURNING setting_value`, LinksSettingsKey, raw, userID, expectedVersion).Scan(&saved)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return VersionedLinks{}, ErrSettingsConflict
	}
	if err != nil {
		return VersionedLinks{}, err
	}
	return VersionedLinks{LinkSettings: value, Version: string(saved)}, nil
}

func FilterLinks(settings VersionedLinks, authenticated bool) LinkSettings {
	result := LinkSettings{Categories: []LinkCategory{}, Links: []ServiceLink{}}
	used := map[string]bool{}
	for _, link := range settings.Links {
		if link.IsActive && (authenticated || (link.IsPublic != nil && *link.IsPublic)) {
			result.Links = append(result.Links, link)
			used[link.CategoryID] = true
		}
	}
	for _, category := range settings.Categories {
		if used[category.ID] {
			result.Categories = append(result.Categories, category)
		}
	}
	return result
}
