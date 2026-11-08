package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/image/draw"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
)

const (
	hardiknasPosterMaxBytes = 5 * 1024 * 1024
	hardiknasTheme          = "Kartini Masa Kini : Berdaya dan Berkarya menuju Indonesia emas 2045"
	hardiknasCutoffLabel    = "24 April 2026 12:00 WIB"
	hardiknasMediaMissing   = "File poster tidak tersedia"

	hardiknasSubmitterTypeSchoolAdmin = "SCHOOL_ADMIN"
	hardiknasSubmitterTypePublic      = "PUBLIC"
)

var (
	ErrHardiknasInvalidPayload   = errors.New("INVALID_PAYLOAD")
	ErrHardiknasInvalidSchool    = errors.New("INVALID_SCHOOL")
	ErrHardiknasInvalidDriveLink = errors.New("INVALID_GOOGLE_DRIVE_LINK")
	ErrHardiknasInvalidImage     = errors.New("INVALID_IMAGE_FILE")
	ErrHardiknasInvalidImageType = errors.New("INVALID_IMAGE_TYPE")
	ErrHardiknasFileTooLarge     = errors.New("FILE_TOO_LARGE")
	ErrHardiknasCreateClosed     = errors.New("CREATE_CUTOFF_PASSED")
	ErrHardiknasDeleteClosed     = errors.New("DELETE_CUTOFF_PASSED")
	ErrHardiknasActionNotAllowed = errors.New("ACTION_NOT_ALLOWED")
	ErrHardiknasStorageMisconfig = errors.New("STORAGE_NOT_CONFIGURED")
	ErrHardiknasFileNotAvailable = errors.New("FILE_NOT_AVAILABLE")
)

type HardiknasService struct {
	DB              *db.DB
	StorageLocalDir string
	Clock           SystemClock
}

type HardiknasCreateInput struct {
	SchoolID                        *int
	NISN                            string
	NamaSiswa                       string
	Rombel                          string
	LinkSuratPernyataanOrisinalitas string
	JudulPoster                     string
	OriginalFilename                string
	FileBytes                       []byte
}

type HardiknasWinnerInput struct {
	Rank    int `json:"rank"`
	EntryID int `json:"entryId"`
}

type HardiknasSaveWinnersInput struct {
	Published bool                   `json:"published"`
	Winners   []HardiknasWinnerInput `json:"winners"`
}

type HardiknasGalleryItem struct {
	ID               int       `json:"id"`
	Judul            string    `json:"judul"`
	Nama             string    `json:"nama"`
	Rombel           string    `json:"rombel"`
	Sekolah          string    `json:"sekolah"`
	TanggalInput     time.Time `json:"tanggalInput"`
	ThumbnailURL     string    `json:"thumbnailUrl"`
	ImageURL         string    `json:"imageUrl"`
	CanDelete        bool      `json:"canDelete"`
	SubmitterType    string    `json:"submitterType"`
	MediaAvailable   bool      `json:"mediaAvailable"`
	MediaStatusLabel string    `json:"mediaStatusLabel,omitempty"`
}

type HardiknasWinnerItem struct {
	Rank             int       `json:"rank"`
	EntryID          int       `json:"entryId"`
	Judul            string    `json:"judul"`
	Nama             string    `json:"nama"`
	Rombel           string    `json:"rombel"`
	Sekolah          string    `json:"sekolah"`
	TanggalInput     time.Time `json:"tanggalInput"`
	ThumbnailURL     string    `json:"thumbnailUrl"`
	ImageURL         string    `json:"imageUrl"`
	MediaAvailable   bool      `json:"mediaAvailable"`
	MediaStatusLabel string    `json:"mediaStatusLabel,omitempty"`
}

type HardiknasTableRow struct {
	ID               int       `json:"id"`
	JudulPoster      string    `json:"judulPoster"`
	NamaSiswa        string    `json:"namaSiswa"`
	Rombel           string    `json:"rombel"`
	Sekolah          string    `json:"sekolah"`
	TanggalInput     time.Time `json:"tanggalInput"`
	CanDelete        bool      `json:"canDelete"`
	SubmitterType    string    `json:"submitterType"`
	MediaAvailable   bool      `json:"mediaAvailable"`
	MediaStatusLabel string    `json:"mediaStatusLabel,omitempty"`
}

type HardiknasTableFilters struct {
	SchoolKeyword string
	PosterTitle   string
	StudentName   string
}

type HardiknasTableResponse struct {
	Rows  []HardiknasTableRow `json:"rows"`
	Total int                 `json:"total"`
}

type HardiknasListResponse struct {
	Items          []HardiknasGalleryItem `json:"items"`
	Rows           []HardiknasTableRow    `json:"rows"`
	Total          int                    `json:"total"`
	Theme          string                 `json:"theme"`
	CutoffAtLabel  string                 `json:"cutoffAtLabel"`
	IsCutoffPassed bool                   `json:"isCutoffPassed"`
	CanCreate      bool                   `json:"canCreate"`
	CanDelete      bool                   `json:"canDelete"`
}

type HardiknasSummaryResponse struct {
	JumlahKarya                      int     `json:"jumlahKarya"`
	JumlahSekolahBerpartisipasi      int     `json:"jumlahSekolahBerpartisipasi"`
	JumlahSekolahBelumBerpartisipasi int     `json:"jumlahSekolahBelumBerpartisipasi"`
	RataRataKaryaPerSekolah          float64 `json:"rataRataKaryaPerSekolah"`
	TotalSekolahDalamScope           int     `json:"totalSekolahDalamScope"`
}

type HardiknasWinnersResponse struct {
	Published bool                  `json:"published"`
	Items     []HardiknasWinnerItem `json:"items"`
}

type HardiknasPublicDetail struct {
	ID               int       `json:"id"`
	JudulPoster      string    `json:"judulPoster"`
	NamaSiswa        string    `json:"namaSiswa"`
	NamaSekolah      string    `json:"namaSekolah"`
	Rombel           string    `json:"rombel"`
	TanggalInput     time.Time `json:"tanggalInput"`
	ImageURL         string    `json:"imageUrl"`
	DownloadURL      string    `json:"downloadUrl"`
	MediaAvailable   bool      `json:"mediaAvailable"`
	MediaStatusLabel string    `json:"mediaStatusLabel,omitempty"`
}

type HardiknasSchoolSuggestion struct {
	ID              int    `json:"id"`
	NamaSekolah     string `json:"namaSekolah"`
	NPSN            string `json:"npsn"`
	HasSubmission   bool   `json:"hasSubmission"`
	SubmissionCount int    `json:"submissionCount"`
}

type HardiknasExportRow struct {
	ID                              int
	JudulPoster                     string
	LinkSuratPernyataanOrisinalitas string
	NISN                            string
	NamaSiswa                       string
	Rombel                          string
	NamaSekolah                     string
}

type HardiknasMediaResult struct {
	ContentType string
	Filename    string
	Content     []byte
}

type hardiknasScope struct {
	All       bool
	SchoolIDs []int
	Role      string
}

type hardiknasEntryRecord struct {
	ID            int
	SchoolID      int
	SchoolName    string
	JudulPoster   string
	NamaSiswa     string
	Rombel        string
	CreatedAt     time.Time
	OriginalPath  string
	ThumbnailPath string
	OriginalMIME  string
	NISN          string
	LinkSurat     string
	CreatedBy     sql.NullInt64
	SubmitterType string
}

func (s HardiknasService) ListForUser(ctx context.Context, user auth.User) (HardiknasListResponse, error) {
	scope, err := s.resolveScope(ctx, user)
	if err != nil {
		return HardiknasListResponse{}, err
	}
	return s.listWithScope(ctx, scope)
}

func (s HardiknasService) ListPublic(ctx context.Context) (HardiknasListResponse, error) {
	return s.listWithScope(ctx, hardiknasScope{All: true, Role: "PUBLIC"})
}

func (s HardiknasService) ListTableForUser(ctx context.Context, user auth.User, filters HardiknasTableFilters) (HardiknasTableResponse, error) {
	scope, err := s.resolveScope(ctx, user)
	if err != nil {
		return HardiknasTableResponse{}, err
	}
	return s.tableWithScope(ctx, scope, filters)
}

func (s HardiknasService) ListTablePublic(ctx context.Context, filters HardiknasTableFilters) (HardiknasTableResponse, error) {
	return s.tableWithScope(ctx, hardiknasScope{All: true, Role: "PUBLIC"}, filters)
}

func (s HardiknasService) SummaryForUser(ctx context.Context, user auth.User) (HardiknasSummaryResponse, error) {
	scope, err := s.resolveScope(ctx, user)
	if err != nil {
		return HardiknasSummaryResponse{}, err
	}
	return s.summaryWithScope(ctx, scope)
}

func (s HardiknasService) SummaryPublic(ctx context.Context) (HardiknasSummaryResponse, error) {
	return s.summaryWithScope(ctx, hardiknasScope{All: true, Role: "PUBLIC"})
}

func (s HardiknasService) WinnersPublic(ctx context.Context) (HardiknasWinnersResponse, error) {
	return s.winners(ctx, false)
}

func (s HardiknasService) WinnersForUser(ctx context.Context, user auth.User) (HardiknasWinnersResponse, error) {
	return s.winners(ctx, auth.NormalizeRole(user.Role) == auth.RoleAdminMaster)
}

func (s HardiknasService) SaveWinners(ctx context.Context, user auth.User, input HardiknasSaveWinnersInput) (HardiknasWinnersResponse, error) {
	if auth.NormalizeRole(user.Role) != auth.RoleAdminMaster {
		return HardiknasWinnersResponse{}, auth.ErrForbidden
	}
	if err := validateHardiknasWinnerInput(input); err != nil {
		return HardiknasWinnersResponse{}, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return HardiknasWinnersResponse{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if len(input.Winners) > 0 {
		entryIDs := make([]int, 0, len(input.Winners))
		for _, winner := range input.Winners {
			entryIDs = append(entryIDs, winner.EntryID)
		}
		rows, err := tx.Query(ctx, `
			SELECT id
			FROM competition_poster_entries
			WHERE deleted_at IS NULL
			  AND id = ANY($1)
		`, entryIDs)
		if err != nil {
			return HardiknasWinnersResponse{}, err
		}
		found := map[int]struct{}{}
		for rows.Next() {
			var entryID int
			if err := rows.Scan(&entryID); err != nil {
				rows.Close()
				return HardiknasWinnersResponse{}, err
			}
			found[entryID] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return HardiknasWinnersResponse{}, err
		}
		rows.Close()
		if len(found) != len(entryIDs) {
			return HardiknasWinnersResponse{}, ErrHardiknasInvalidPayload
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM competition_poster_winners`); err != nil {
		return HardiknasWinnersResponse{}, err
	}
	now, err := s.currentTime(ctx)
	if err != nil {
		return HardiknasWinnersResponse{}, err
	}
	for _, winner := range input.Winners {
		if _, err := tx.Exec(ctx, `
			INSERT INTO competition_poster_winners (
				rank,
				entry_id,
				is_published,
				created_by,
				updated_by,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $4, $5, $5)
		`, winner.Rank, winner.EntryID, input.Published, user.ID, now); err != nil {
			return HardiknasWinnersResponse{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return HardiknasWinnersResponse{}, err
	}
	return s.WinnersForUser(ctx, user)
}

func (s HardiknasService) PublicDetail(ctx context.Context, entryID int) (HardiknasPublicDetail, error) {
	record, err := s.findEntryByID(ctx, entryID)
	if err != nil {
		return HardiknasPublicDetail{}, err
	}
	mediaAvailable, mediaStatusLabel := s.describeMediaAvailability(record.OriginalPath)
	return HardiknasPublicDetail{
		ID:               record.ID,
		JudulPoster:      record.JudulPoster,
		NamaSiswa:        record.NamaSiswa,
		NamaSekolah:      record.SchoolName,
		Rombel:           record.Rombel,
		TanggalInput:     record.CreatedAt,
		ImageURL:         hardiknasOriginalURL(record.ID),
		DownloadURL:      hardiknasOriginalURL(record.ID),
		MediaAvailable:   mediaAvailable,
		MediaStatusLabel: mediaStatusLabel,
	}, nil
}

func (s HardiknasService) SuggestSchoolsForUser(ctx context.Context, user auth.User, keyword string, limit int) ([]HardiknasSchoolSuggestion, error) {
	scope, err := s.resolveScope(ctx, user)
	if err != nil {
		return nil, err
	}
	return s.suggestSchoolsWithScope(ctx, scope, keyword, limit)
}

func (s HardiknasService) SuggestSchoolsPublic(ctx context.Context, keyword string, limit int) ([]HardiknasSchoolSuggestion, error) {
	return s.suggestSchoolsWithScope(ctx, hardiknasScope{All: true, Role: "PUBLIC"}, keyword, limit)
}

func (s HardiknasService) ExportRowsForUser(ctx context.Context, user auth.User, filters HardiknasTableFilters) ([]HardiknasExportRow, error) {
	if !auth.IsAdmin(user) {
		return nil, auth.ErrForbidden
	}
	scope, err := s.resolveScope(ctx, user)
	if err != nil {
		return nil, err
	}
	records, err := s.listEntries(ctx, scope, filters)
	if err != nil {
		return nil, err
	}
	rows := make([]HardiknasExportRow, 0, len(records))
	for _, record := range records {
		rows = append(rows, HardiknasExportRow{
			ID:                              record.ID,
			JudulPoster:                     record.JudulPoster,
			LinkSuratPernyataanOrisinalitas: record.LinkSurat,
			NISN:                            record.NISN,
			NamaSiswa:                       record.NamaSiswa,
			Rombel:                          record.Rombel,
			NamaSekolah:                     record.SchoolName,
		})
	}
	return rows, nil
}

func (s HardiknasService) Create(ctx context.Context, user auth.User, input HardiknasCreateInput) (HardiknasGalleryItem, error) {
	if !auth.IsSchoolAdmin(user) {
		return HardiknasGalleryItem{}, auth.ErrForbidden
	}
	if user.SchoolID == nil || *user.SchoolID <= 0 {
		return HardiknasGalleryItem{}, auth.ErrUnauthorized
	}
	return s.createWithSchoolID(ctx, *user.SchoolID, input, hardiknasCreateOptions{
		CreatedBy:      &user.ID,
		SubmitterType:  hardiknasSubmitterTypeSchoolAdmin,
		CanDelete:      true,
		ValidateSchool: false,
	})
}

func (s HardiknasService) CreatePublic(ctx context.Context, input HardiknasCreateInput) (HardiknasGalleryItem, error) {
	if input.SchoolID == nil || *input.SchoolID <= 0 {
		return HardiknasGalleryItem{}, ErrHardiknasInvalidSchool
	}
	return s.createWithSchoolID(ctx, *input.SchoolID, input, hardiknasCreateOptions{
		SubmitterType:  hardiknasSubmitterTypePublic,
		CanDelete:      false,
		ValidateSchool: true,
	})
}

func (s HardiknasService) Delete(ctx context.Context, user auth.User, entryID int) error {
	scope, err := s.resolveScope(ctx, user)
	if err != nil {
		return err
	}
	if entryID <= 0 {
		return ErrHardiknasInvalidPayload
	}
	if passed, err := s.isCutoffPassed(ctx); err != nil {
		return err
	} else if passed {
		return ErrHardiknasDeleteClosed
	}

	record, err := s.findEntryByID(ctx, entryID)
	if err != nil {
		return err
	}
	if !s.canDeleteEntry(scope, record, false) {
		return ErrHardiknasActionNotAllowed
	}

	deletedAt, err := s.currentTime(ctx)
	if err != nil {
		return err
	}

	var (
		originalPath  string
		thumbnailPath string
	)
	err = s.DB.Pool.QueryRow(ctx, `
		UPDATE competition_poster_entries
		SET deleted_at = $3,
		    updated_at = $3
		WHERE id = $1
		  AND school_id = $2
		  AND deleted_at IS NULL
		RETURNING hasil_karya_path, hasil_karya_thumbnail_path
	`, entryID, record.SchoolID, deletedAt).Scan(&originalPath, &thumbnailPath)
	if err != nil {
		return err
	}

	for _, relativePath := range []string{originalPath, thumbnailPath} {
		absolutePath, resolveErr := s.resolveStoragePath(relativePath)
		if resolveErr != nil {
			logging.Warnf("hardiknas", "resolve_delete_path", "path=%q err=%v", relativePath, resolveErr)
			continue
		}
		if removeErr := os.Remove(absolutePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			logging.Warnf("hardiknas", "delete_file", "path=%q err=%v", absolutePath, removeErr)
		}
	}

	return nil
}

func (s HardiknasService) PublicMedia(ctx context.Context, entryID int, thumbnail bool) (HardiknasMediaResult, error) {
	record, err := s.findEntryByID(ctx, entryID)
	if err != nil {
		return HardiknasMediaResult{}, err
	}

	dbPath := record.OriginalPath
	if thumbnail {
		dbPath = record.ThumbnailPath
	}
	normalizedPath, absolutePath, storageRoot, err := s.resolveStoragePathWithMeta(dbPath)
	if err != nil {
		logging.Warnf(
			"hardiknas",
			"hardiknas_media_missing",
			"entry_id=%d db_path=%q resolved_path=%q storage_root=%q err=%v",
			entryID,
			dbPath,
			"",
			storageRoot,
			err,
		)
		return HardiknasMediaResult{}, err
	}
	content, err := os.ReadFile(absolutePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logging.Warnf(
				"hardiknas",
				"hardiknas_media_missing",
				"entry_id=%d db_path=%q resolved_path=%q storage_root=%q err=%v",
				entryID,
				dbPath,
				absolutePath,
				storageRoot,
				err,
			)
			return HardiknasMediaResult{}, ErrHardiknasFileNotAvailable
		}
		return HardiknasMediaResult{}, err
	}
	contentType := http.DetectContentType(content)
	if thumbnail {
		contentType = normalizeImageContentType(contentType)
	} else if strings.TrimSpace(record.OriginalMIME) != "" {
		contentType = record.OriginalMIME
	}

	return HardiknasMediaResult{
		ContentType: contentType,
		Filename:    filepath.Base(normalizedPath),
		Content:     content,
	}, nil
}

func (s HardiknasService) listWithScope(ctx context.Context, scope hardiknasScope) (HardiknasListResponse, error) {
	records, err := s.listEntries(ctx, scope, HardiknasTableFilters{})
	if err != nil {
		return HardiknasListResponse{}, err
	}
	passed, err := s.isCutoffPassed(ctx)
	if err != nil {
		return HardiknasListResponse{}, err
	}

	items := make([]HardiknasGalleryItem, 0, len(records))
	rows := make([]HardiknasTableRow, 0, len(records))
	hasDeletePermission := false
	for _, record := range records {
		canDelete := s.canDeleteEntry(scope, record, passed)
		mediaAvailable, mediaStatusLabel := s.describeMediaAvailability(record.OriginalPath)
		if canDelete {
			hasDeletePermission = true
		}
		items = append(items, HardiknasGalleryItem{
			ID:               record.ID,
			Judul:            record.JudulPoster,
			Nama:             record.NamaSiswa,
			Rombel:           record.Rombel,
			Sekolah:          record.SchoolName,
			TanggalInput:     record.CreatedAt,
			ThumbnailURL:     hardiknasThumbnailURL(record.ID),
			ImageURL:         hardiknasOriginalURL(record.ID),
			CanDelete:        canDelete,
			SubmitterType:    record.SubmitterType,
			MediaAvailable:   mediaAvailable,
			MediaStatusLabel: mediaStatusLabel,
		})
		rows = append(rows, HardiknasTableRow{
			ID:               record.ID,
			JudulPoster:      record.JudulPoster,
			NamaSiswa:        record.NamaSiswa,
			Rombel:           record.Rombel,
			Sekolah:          record.SchoolName,
			TanggalInput:     record.CreatedAt,
			CanDelete:        canDelete,
			SubmitterType:    record.SubmitterType,
			MediaAvailable:   mediaAvailable,
			MediaStatusLabel: mediaStatusLabel,
		})
	}

	return HardiknasListResponse{
		Items:          items,
		Rows:           rows,
		Total:          len(records),
		Theme:          hardiknasTheme,
		CutoffAtLabel:  hardiknasCutoffLabel,
		IsCutoffPassed: passed,
		CanCreate:      s.canCreate(scope, passed),
		CanDelete:      hasDeletePermission,
	}, nil
}

func (s HardiknasService) tableWithScope(ctx context.Context, scope hardiknasScope, filters HardiknasTableFilters) (HardiknasTableResponse, error) {
	records, err := s.listEntries(ctx, scope, filters)
	if err != nil {
		return HardiknasTableResponse{}, err
	}
	passed, err := s.isCutoffPassed(ctx)
	if err != nil {
		return HardiknasTableResponse{}, err
	}

	rows := make([]HardiknasTableRow, 0, len(records))
	for _, record := range records {
		canDelete := s.canDeleteEntry(scope, record, passed)
		mediaAvailable, mediaStatusLabel := s.describeMediaAvailability(record.OriginalPath)
		rows = append(rows, HardiknasTableRow{
			ID:               record.ID,
			JudulPoster:      record.JudulPoster,
			NamaSiswa:        record.NamaSiswa,
			Rombel:           record.Rombel,
			Sekolah:          record.SchoolName,
			TanggalInput:     record.CreatedAt,
			CanDelete:        canDelete,
			SubmitterType:    record.SubmitterType,
			MediaAvailable:   mediaAvailable,
			MediaStatusLabel: mediaStatusLabel,
		})
	}

	return HardiknasTableResponse{
		Rows:  rows,
		Total: len(rows),
	}, nil
}

func (s HardiknasService) summaryWithScope(ctx context.Context, scope hardiknasScope) (HardiknasSummaryResponse, error) {
	var totalEntries int
	entryQuery := `
		SELECT COUNT(*)
		FROM competition_poster_entries e
		WHERE e.deleted_at IS NULL
	`
	entryArgs := make([]interface{}, 0)
	if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return HardiknasSummaryResponse{}, nil
		}
		entryQuery += " AND e.school_id = ANY($1)"
		entryArgs = append(entryArgs, scope.SchoolIDs)
	}
	if err := s.DB.Pool.QueryRow(ctx, entryQuery, entryArgs...).Scan(&totalEntries); err != nil {
		return HardiknasSummaryResponse{}, err
	}

	var participatingSchools int
	participatingQuery := `
		SELECT COUNT(DISTINCT e.school_id)
		FROM competition_poster_entries e
		WHERE e.deleted_at IS NULL
	`
	if !scope.All {
		participatingQuery += " AND e.school_id = ANY($1)"
	}
	if err := s.DB.Pool.QueryRow(ctx, participatingQuery, entryArgs...).Scan(&participatingSchools); err != nil {
		return HardiknasSummaryResponse{}, err
	}

	totalSchools, err := s.countSchoolsInScope(ctx, scope)
	if err != nil {
		return HardiknasSummaryResponse{}, err
	}

	average := 0.0
	if participatingSchools > 0 {
		average = float64(totalEntries) / float64(participatingSchools)
	}
	notParticipating := totalSchools - participatingSchools
	if notParticipating < 0 {
		notParticipating = 0
	}

	return HardiknasSummaryResponse{
		JumlahKarya:                      totalEntries,
		JumlahSekolahBerpartisipasi:      participatingSchools,
		JumlahSekolahBelumBerpartisipasi: notParticipating,
		RataRataKaryaPerSekolah:          average,
		TotalSekolahDalamScope:           totalSchools,
	}, nil
}

func (s HardiknasService) winners(ctx context.Context, includeDraft bool) (HardiknasWinnersResponse, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT w.rank,
		       e.id,
		       COALESCE(e.judul_poster, ''),
		       COALESCE(e.nama_siswa, ''),
		       COALESCE(e.rombel, ''),
		       COALESCE(ds.nama_sekolah, ''),
		       e.created_at,
		       COALESCE(e.hasil_karya_path, ''),
		       BOOL_AND(w.is_published) OVER () AS published
		FROM competition_poster_winners w
		JOIN competition_poster_entries e ON e.id = w.entry_id
		JOIN data_sekolah ds ON ds.id = e.school_id
		WHERE e.deleted_at IS NULL
		ORDER BY w.rank ASC
	`)
	if err != nil {
		return HardiknasWinnersResponse{}, err
	}
	defer rows.Close()

	items := make([]HardiknasWinnerItem, 0, 3)
	published := false
	for rows.Next() {
		var (
			item         HardiknasWinnerItem
			originalPath string
			rowPublished bool
		)
		if err := rows.Scan(
			&item.Rank,
			&item.EntryID,
			&item.Judul,
			&item.Nama,
			&item.Rombel,
			&item.Sekolah,
			&item.TanggalInput,
			&originalPath,
			&rowPublished,
		); err != nil {
			return HardiknasWinnersResponse{}, err
		}
		published = rowPublished
		item.TanggalInput = normalizeHardiknasTimestamp(item.TanggalInput)
		item.ThumbnailURL = hardiknasThumbnailURL(item.EntryID)
		item.ImageURL = hardiknasOriginalURL(item.EntryID)
		item.MediaAvailable, item.MediaStatusLabel = s.describeMediaAvailability(originalPath)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return HardiknasWinnersResponse{}, err
	}
	if !published && !includeDraft {
		items = []HardiknasWinnerItem{}
	}
	if len(items) != 3 {
		published = false
		if !includeDraft {
			items = []HardiknasWinnerItem{}
		}
	}
	return HardiknasWinnersResponse{
		Published: published,
		Items:     items,
	}, nil
}

func (s HardiknasService) listEntries(ctx context.Context, scope hardiknasScope, filters HardiknasTableFilters) ([]hardiknasEntryRecord, error) {
	query := `
		SELECT e.id,
		       e.school_id,
		       COALESCE(ds.nama_sekolah, '') AS school_name,
		       e.judul_poster,
		       e.nama_siswa,
		       e.rombel,
		       e.created_at,
		       e.hasil_karya_path,
		       e.hasil_karya_thumbnail_path,
		       e.hasil_karya_mime_type,
		       COALESCE(e.nisn, ''),
		       COALESCE(e.link_surat_pernyataan_orisinalitas, ''),
		       e.created_by,
		       COALESCE(e.submitter_type, 'SCHOOL_ADMIN')
		FROM competition_poster_entries e
		JOIN data_sekolah ds ON ds.id = e.school_id
		WHERE e.deleted_at IS NULL
	`
	args := make([]interface{}, 0)
	argIndex := 1
	if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return []hardiknasEntryRecord{}, nil
		}
		query += " AND e.school_id = ANY($" + strconv.Itoa(argIndex) + ")"
		args = append(args, scope.SchoolIDs)
		argIndex++
	}
	if normalized := normalizeHardiknasFilterValue(filters.SchoolKeyword); normalized != "" {
		query += " AND LOWER(COALESCE(ds.nama_sekolah, '')) LIKE $" + strconv.Itoa(argIndex)
		args = append(args, hardiknasLikePattern(normalized))
		argIndex++
	}
	if normalized := normalizeHardiknasFilterValue(filters.PosterTitle); normalized != "" {
		query += " AND LOWER(COALESCE(e.judul_poster, '')) LIKE $" + strconv.Itoa(argIndex)
		args = append(args, hardiknasLikePattern(normalized))
		argIndex++
	}
	if normalized := normalizeHardiknasFilterValue(filters.StudentName); normalized != "" {
		query += " AND LOWER(COALESCE(e.nama_siswa, '')) LIKE $" + strconv.Itoa(argIndex)
		args = append(args, hardiknasLikePattern(normalized))
	}
	query += " ORDER BY e.created_at DESC, e.id DESC"

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]hardiknasEntryRecord, 0)
	for rows.Next() {
		var item hardiknasEntryRecord
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.SchoolName,
			&item.JudulPoster,
			&item.NamaSiswa,
			&item.Rombel,
			&item.CreatedAt,
			&item.OriginalPath,
			&item.ThumbnailPath,
			&item.OriginalMIME,
			&item.NISN,
			&item.LinkSurat,
			&item.CreatedBy,
			&item.SubmitterType,
		); err != nil {
			return nil, err
		}
		item.CreatedAt = normalizeHardiknasTimestamp(item.CreatedAt)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s HardiknasService) countSchoolsInScope(ctx context.Context, scope hardiknasScope) (int, error) {
	query := `SELECT COUNT(*) FROM data_sekolah`
	args := make([]interface{}, 0)
	if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return 0, nil
		}
		query += " WHERE id = ANY($1)"
		args = append(args, scope.SchoolIDs)
	}
	var total int
	if err := s.DB.Pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s HardiknasService) suggestSchoolsWithScope(ctx context.Context, scope hardiknasScope, keyword string, limit int) ([]HardiknasSchoolSuggestion, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	args := make([]interface{}, 0)
	argIndex := 1
	whereClause := "WHERE 1=1"
	if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			return []HardiknasSchoolSuggestion{}, nil
		}
		whereClause += " AND ds.id = ANY($" + strconv.Itoa(argIndex) + ")"
		args = append(args, scope.SchoolIDs)
		argIndex++
	}

	trimmed := normalizeHardiknasFilterValue(keyword)
	pattern := "%"
	prefix := "%"
	if trimmed != "" {
		pattern = hardiknasLikePattern(trimmed)
		prefix = trimmed + "%"
	}
	args = append(args, pattern, prefix, limit)

	query := `
		WITH school_scope AS (
			SELECT ds.id,
			       COALESCE(ds.nama_sekolah, '') AS nama_sekolah,
			       COALESCE(ds.npsn_sekolah, '') AS npsn_sekolah
			FROM data_sekolah ds
			` + whereClause + `
		),
		submission_counts AS (
			SELECT e.school_id,
			       COUNT(*)::int AS submission_count
			FROM competition_poster_entries e
			WHERE e.deleted_at IS NULL
			GROUP BY e.school_id
		)
		SELECT sc.id,
		       sc.nama_sekolah,
		       sc.npsn_sekolah,
		       COALESCE(sub.submission_count, 0) AS submission_count
		FROM school_scope sc
		LEFT JOIN submission_counts sub ON sub.school_id = sc.id
		WHERE (
			LOWER(sc.nama_sekolah) LIKE $` + strconv.Itoa(argIndex) + `
			OR LOWER(sc.npsn_sekolah) LIKE $` + strconv.Itoa(argIndex) + `
		)
		ORDER BY
			CASE WHEN COALESCE(sub.submission_count, 0) > 0 THEN 0 ELSE 1 END,
			CASE
				WHEN LOWER(sc.nama_sekolah) LIKE $` + strconv.Itoa(argIndex+1) + ` THEN 0
				WHEN LOWER(sc.npsn_sekolah) LIKE $` + strconv.Itoa(argIndex+1) + ` THEN 0
				ELSE 1
			END,
			sc.nama_sekolah ASC
		LIMIT $` + strconv.Itoa(argIndex+2)
	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]HardiknasSchoolSuggestion, 0)
	for rows.Next() {
		var item HardiknasSchoolSuggestion
		if err := rows.Scan(&item.ID, &item.NamaSekolah, &item.NPSN, &item.SubmissionCount); err != nil {
			return nil, err
		}
		item.HasSubmission = item.SubmissionCount > 0
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s HardiknasService) resolveScope(ctx context.Context, user auth.User) (hardiknasScope, error) {
	scope, err := auth.GetMultiSchoolScope(ctx, s.DB, user)
	if err != nil {
		return hardiknasScope{}, err
	}
	return hardiknasScope{
		All:       scope.All,
		SchoolIDs: append([]int(nil), scope.SchoolIDs...),
		Role:      scope.Role,
	}, nil
}

func (s HardiknasService) currentTime(ctx context.Context) (time.Time, error) {
	if s.Clock.DB == nil {
		return normalizeJakartaTime(time.Now()), nil
	}
	now, err := s.Clock.Now(ctx)
	if err != nil {
		return time.Time{}, err
	}
	return normalizeHardiknasTimestamp(now), nil
}

func (s HardiknasService) isCutoffPassed(ctx context.Context) (bool, error) {
	now, err := s.currentTime(ctx)
	if err != nil {
		return false, err
	}
	return isHardiknasCutoffPassedAt(now), nil
}

func hardiknasCutoffBoundary() time.Time {
	return time.Date(2026, 4, 24, 12, 0, 0, 0, JakartaLocation())
}

func isHardiknasCutoffPassedAt(now time.Time) bool {
	return !normalizeHardiknasTimestamp(now).Before(hardiknasCutoffBoundary())
}

func validateHardiknasWinnerInput(input HardiknasSaveWinnersInput) error {
	if input.Published && len(input.Winners) != 3 {
		return ErrHardiknasInvalidPayload
	}
	seenRanks := map[int]struct{}{}
	seenEntryIDs := map[int]struct{}{}
	for _, winner := range input.Winners {
		if winner.Rank < 1 || winner.Rank > 3 || winner.EntryID <= 0 {
			return ErrHardiknasInvalidPayload
		}
		if _, exists := seenRanks[winner.Rank]; exists {
			return ErrHardiknasInvalidPayload
		}
		if _, exists := seenEntryIDs[winner.EntryID]; exists {
			return ErrHardiknasInvalidPayload
		}
		seenRanks[winner.Rank] = struct{}{}
		seenEntryIDs[winner.EntryID] = struct{}{}
	}
	if input.Published {
		for rank := 1; rank <= 3; rank++ {
			if _, exists := seenRanks[rank]; !exists {
				return ErrHardiknasInvalidPayload
			}
		}
	}
	return nil
}

func normalizeHardiknasTimestamp(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return normalizeJakartaTime(value)
}

func normalizeHardiknasCreateInput(input HardiknasCreateInput) (HardiknasCreateInput, error) {
	normalized := HardiknasCreateInput{
		SchoolID:                        input.SchoolID,
		NISN:                            strings.TrimSpace(input.NISN),
		NamaSiswa:                       strings.TrimSpace(input.NamaSiswa),
		Rombel:                          strings.TrimSpace(input.Rombel),
		LinkSuratPernyataanOrisinalitas: strings.TrimSpace(input.LinkSuratPernyataanOrisinalitas),
		JudulPoster:                     strings.TrimSpace(input.JudulPoster),
		OriginalFilename:                strings.TrimSpace(input.OriginalFilename),
		FileBytes:                       input.FileBytes,
	}

	switch {
	case normalized.NISN == "":
		return HardiknasCreateInput{}, ErrHardiknasInvalidPayload
	case normalized.NamaSiswa == "":
		return HardiknasCreateInput{}, ErrHardiknasInvalidPayload
	case normalized.Rombel == "":
		return HardiknasCreateInput{}, ErrHardiknasInvalidPayload
	case normalized.JudulPoster == "":
		return HardiknasCreateInput{}, ErrHardiknasInvalidPayload
	case normalized.LinkSuratPernyataanOrisinalitas == "":
		return HardiknasCreateInput{}, ErrHardiknasInvalidPayload
	case len(normalized.FileBytes) == 0:
		return HardiknasCreateInput{}, ErrHardiknasInvalidPayload
	}

	if !isReasonableGoogleDriveURL(normalized.LinkSuratPernyataanOrisinalitas) {
		return HardiknasCreateInput{}, ErrHardiknasInvalidDriveLink
	}
	if len(normalized.FileBytes) > hardiknasPosterMaxBytes {
		return HardiknasCreateInput{}, ErrHardiknasFileTooLarge
	}
	if normalized.OriginalFilename == "" {
		normalized.OriginalFilename = "poster-upload"
	}

	return normalized, nil
}

func validateHardiknasImage(content []byte) (image.Image, string, string, error) {
	contentType := http.DetectContentType(content)
	switch contentType {
	case "image/jpeg", "image/png":
	default:
		return nil, "", "", ErrHardiknasInvalidImageType
	}

	decodedImage, format, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, "", "", ErrHardiknasInvalidImage
	}

	switch format {
	case "jpeg":
		return decodedImage, format, "image/jpeg", nil
	case "png":
		return decodedImage, format, "image/png", nil
	default:
		return nil, "", "", ErrHardiknasInvalidImageType
	}
}

func (s HardiknasService) savePosterAssets(src image.Image, format string, originalFilename string, originalBytes []byte) (string, string, []string, error) {
	_, originalDir, _, err := s.resolveStoragePathWithMeta(hardiknasOriginalRelativeDir)
	if err != nil {
		return "", "", nil, err
	}
	_, thumbnailDir, _, err := s.resolveStoragePathWithMeta(hardiknasThumbnailRelativeDir)
	if err != nil {
		return "", "", nil, err
	}
	if err := os.MkdirAll(originalDir, 0o755); err != nil {
		return "", "", nil, err
	}
	if err := os.MkdirAll(thumbnailDir, 0o755); err != nil {
		return "", "", nil, err
	}

	extension := hardiknasExtensionForFormat(format)
	uniqueName, err := buildHardiknasUniqueFilename(originalFilename, extension)
	if err != nil {
		return "", "", nil, err
	}

	thumbnailBytes, thumbnailExt, err := buildHardiknasThumbnail(src, format)
	if err != nil {
		return "", "", nil, err
	}

	originalAbsPath := filepath.Join(originalDir, uniqueName)
	thumbnailName := strings.TrimSuffix(uniqueName, filepath.Ext(uniqueName)) + "_thumb" + thumbnailExt
	thumbnailAbsPath := filepath.Join(thumbnailDir, thumbnailName)

	if err := os.WriteFile(originalAbsPath, originalBytes, 0o644); err != nil {
		return "", "", nil, err
	}
	if err := os.WriteFile(thumbnailAbsPath, thumbnailBytes, 0o644); err != nil {
		return "", "", []string{originalAbsPath}, err
	}

	return filepath.ToSlash(filepath.Join(hardiknasOriginalRelativeDir, uniqueName)),
		filepath.ToSlash(filepath.Join(hardiknasThumbnailRelativeDir, thumbnailName)),
		[]string{originalAbsPath, thumbnailAbsPath},
		nil
}

func buildHardiknasThumbnail(src image.Image, format string) ([]byte, string, error) {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, "", ErrHardiknasInvalidImage
	}

	maxDimension := 480
	targetWidth := width
	targetHeight := height
	if width > maxDimension || height > maxDimension {
		if width >= height {
			targetWidth = maxDimension
			targetHeight = int(float64(height) * (float64(maxDimension) / float64(width)))
		} else {
			targetHeight = maxDimension
			targetWidth = int(float64(width) * (float64(maxDimension) / float64(height)))
		}
	}
	if targetWidth <= 0 {
		targetWidth = 1
	}
	if targetHeight <= 0 {
		targetHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	buffer := bytes.NewBuffer(nil)
	switch format {
	case "png":
		if err := png.Encode(buffer, dst); err != nil {
			return nil, "", err
		}
		return buffer.Bytes(), ".png", nil
	case "jpeg":
		if err := jpeg.Encode(buffer, dst, &jpeg.Options{Quality: 85}); err != nil {
			return nil, "", err
		}
		return buffer.Bytes(), ".jpg", nil
	default:
		return nil, "", ErrHardiknasInvalidImageType
	}
}

func buildHardiknasUniqueFilename(originalFilename string, extension string) (string, error) {
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	baseName := sanitizeHardiknasFilename(strings.TrimSuffix(filepath.Base(originalFilename), filepath.Ext(originalFilename)))
	if baseName == "" {
		baseName = "poster"
	}
	return fmt.Sprintf("%s_%s_%s%s",
		time.Now().In(JakartaLocation()).Format("20060102150405"),
		hex.EncodeToString(randomBytes),
		baseName,
		extension,
	), nil
}

func sanitizeHardiknasFilename(name string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '_'
		}
	}, strings.TrimSpace(name))
	return strings.Trim(cleaned, "_")
}

func hardiknasExtensionForFormat(format string) string {
	if format == "png" {
		return ".png"
	}
	return ".jpg"
}

func isReasonableGoogleDriveURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host != "drive.google.com" && host != "docs.google.com" {
		return false
	}
	path := strings.ToLower(strings.TrimSpace(parsed.Path))
	if path == "" || path == "/" {
		return parsed.Query().Get("id") != ""
	}
	return strings.Contains(path, "/file/") ||
		strings.Contains(path, "/document/") ||
		strings.Contains(path, "/spreadsheets/") ||
		strings.Contains(path, "/presentation/") ||
		strings.Contains(path, "/forms/") ||
		parsed.Query().Get("id") != ""
}

func hardiknasOriginalURL(entryID int) string {
	return fmt.Sprintf("/api/public/lomba-hardiknas/media/%d", entryID)
}

func hardiknasThumbnailURL(entryID int) string {
	return fmt.Sprintf("/api/public/lomba-hardiknas/media/%d/thumbnail", entryID)
}

func normalizeHardiknasFilterValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func hardiknasLikePattern(value string) string {
	return "%" + value + "%"
}

func (s HardiknasService) findEntryByID(ctx context.Context, entryID int) (hardiknasEntryRecord, error) {
	var item hardiknasEntryRecord
	err := s.DB.Pool.QueryRow(ctx, `
		SELECT e.id,
		       e.school_id,
		       COALESCE(ds.nama_sekolah, ''),
		       COALESCE(e.judul_poster, ''),
		       COALESCE(e.nama_siswa, ''),
		       COALESCE(e.rombel, ''),
		       e.created_at,
		       COALESCE(e.hasil_karya_path, ''),
		       COALESCE(e.hasil_karya_thumbnail_path, ''),
		       COALESCE(e.hasil_karya_mime_type, ''),
		       COALESCE(e.nisn, ''),
		       COALESCE(e.link_surat_pernyataan_orisinalitas, ''),
		       e.created_by,
		       COALESCE(e.submitter_type, 'SCHOOL_ADMIN')
		FROM competition_poster_entries e
		JOIN data_sekolah ds ON ds.id = e.school_id
		WHERE e.id = $1
		  AND e.deleted_at IS NULL
	`, entryID).Scan(
		&item.ID,
		&item.SchoolID,
		&item.SchoolName,
		&item.JudulPoster,
		&item.NamaSiswa,
		&item.Rombel,
		&item.CreatedAt,
		&item.OriginalPath,
		&item.ThumbnailPath,
		&item.OriginalMIME,
		&item.NISN,
		&item.LinkSurat,
		&item.CreatedBy,
		&item.SubmitterType,
	)
	if err == nil {
		item.CreatedAt = normalizeHardiknasTimestamp(item.CreatedAt)
	}
	return item, err
}

type hardiknasCreateOptions struct {
	CreatedBy      *int
	SubmitterType  string
	CanDelete      bool
	ValidateSchool bool
}

func (s HardiknasService) createWithSchoolID(ctx context.Context, schoolID int, input HardiknasCreateInput, options hardiknasCreateOptions) (_ HardiknasGalleryItem, err error) {
	if s.DB == nil || s.DB.Pool == nil {
		return HardiknasGalleryItem{}, ErrHardiknasActionNotAllowed
	}
	if passed, cutoffErr := s.isCutoffPassed(ctx); cutoffErr != nil {
		return HardiknasGalleryItem{}, cutoffErr
	} else if passed {
		return HardiknasGalleryItem{}, ErrHardiknasCreateClosed
	}

	normalizedInput, err := normalizeHardiknasCreateInput(input)
	if err != nil {
		return HardiknasGalleryItem{}, err
	}

	decodedImage, format, mimeType, err := validateHardiknasImage(normalizedInput.FileBytes)
	if err != nil {
		return HardiknasGalleryItem{}, err
	}

	if strings.TrimSpace(s.StorageLocalDir) == "" {
		return HardiknasGalleryItem{}, ErrHardiknasStorageMisconfig
	}

	relativeOriginalPath, relativeThumbnailPath, cleanupPaths, err := s.savePosterAssets(
		decodedImage,
		format,
		normalizedInput.OriginalFilename,
		normalizedInput.FileBytes,
	)
	if err != nil {
		return HardiknasGalleryItem{}, err
	}

	defer func() {
		if err == nil {
			return
		}
		for _, path := range cleanupPaths {
			if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				logging.Warnf("hardiknas", "cleanup_file", "path=%q err=%v", path, removeErr)
			}
		}
	}()

	createdAt, err := s.currentTime(ctx)
	if err != nil {
		return HardiknasGalleryItem{}, err
	}

	schoolName, err := s.lookupSchoolName(ctx, schoolID, options.ValidateSchool)
	if err != nil {
		return HardiknasGalleryItem{}, err
	}

	var (
		entryID        int
		createdByValue interface{}
	)
	if options.CreatedBy != nil && *options.CreatedBy > 0 {
		createdByValue = *options.CreatedBy
	}

	submitterType := strings.TrimSpace(options.SubmitterType)
	if submitterType == "" {
		submitterType = hardiknasSubmitterTypeSchoolAdmin
	}

	err = s.DB.Pool.QueryRow(ctx, `
		INSERT INTO competition_poster_entries (
			school_id,
			nisn,
			nama_siswa,
			rombel,
			link_surat_pernyataan_orisinalitas,
			judul_poster,
			hasil_karya_path,
			hasil_karya_thumbnail_path,
			hasil_karya_mime_type,
			hasil_karya_size,
			created_by,
			submitter_type,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13)
		RETURNING id
	`, schoolID,
		normalizedInput.NISN,
		normalizedInput.NamaSiswa,
		normalizedInput.Rombel,
		normalizedInput.LinkSuratPernyataanOrisinalitas,
		normalizedInput.JudulPoster,
		relativeOriginalPath,
		relativeThumbnailPath,
		mimeType,
		int64(len(normalizedInput.FileBytes)),
		createdByValue,
		submitterType,
		createdAt,
	).Scan(&entryID)
	if err != nil {
		return HardiknasGalleryItem{}, err
	}

	return HardiknasGalleryItem{
		ID:            entryID,
		Judul:         normalizedInput.JudulPoster,
		Nama:          normalizedInput.NamaSiswa,
		Rombel:        normalizedInput.Rombel,
		Sekolah:       schoolName,
		TanggalInput:  normalizeHardiknasTimestamp(createdAt),
		ThumbnailURL:  hardiknasThumbnailURL(entryID),
		ImageURL:      hardiknasOriginalURL(entryID),
		CanDelete:     options.CanDelete,
		SubmitterType: submitterType,
	}, nil
}

func (s HardiknasService) lookupSchoolName(ctx context.Context, schoolID int, strict bool) (string, error) {
	if schoolID <= 0 {
		if strict {
			return "", ErrHardiknasInvalidSchool
		}
		return "", ErrHardiknasInvalidPayload
	}

	var schoolName string
	err := s.DB.Pool.QueryRow(ctx, `
		SELECT COALESCE(nama_sekolah, '')
		FROM data_sekolah
		WHERE id = $1
	`, schoolID).Scan(&schoolName)
	if err != nil {
		if strict && errors.Is(err, pgx.ErrNoRows) {
			return "", ErrHardiknasInvalidSchool
		}
		return "", err
	}
	return schoolName, nil
}

func (s HardiknasService) canCreate(scope hardiknasScope, passed bool) bool {
	if passed {
		return false
	}
	switch scope.Role {
	case "PUBLIC", auth.RoleSchoolAdmin:
		return true
	default:
		return false
	}
}

func (s HardiknasService) canDeleteEntry(scope hardiknasScope, record hardiknasEntryRecord, passed bool) bool {
	if passed {
		return false
	}

	switch scope.Role {
	case "PUBLIC":
		return false
	case auth.RoleSchoolAdmin:
		return record.SchoolID > 0 &&
			len(scope.SchoolIDs) == 1 &&
			scope.SchoolIDs[0] == record.SchoolID &&
			record.SubmitterType == hardiknasSubmitterTypeSchoolAdmin &&
			record.CreatedBy.Valid
	case auth.RoleAdminCabdin, auth.RoleAdminMaster:
		if record.SubmitterType != hardiknasSubmitterTypePublic {
			return false
		}
		return scope.All || containsHardiknasSchoolID(scope.SchoolIDs, record.SchoolID)
	default:
		return false
	}
}

func containsHardiknasSchoolID(ids []int, target int) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func (s HardiknasService) resolveStoragePath(relativePath string) (string, error) {
	_, absolutePath, _, err := s.resolveStoragePathWithMeta(relativePath)
	return absolutePath, err
}

func (s HardiknasService) resolveStoragePathWithMeta(storedPath string) (string, string, string, error) {
	storageRoot, err := resolveHardiknasStorageRoot(s.StorageLocalDir)
	if err != nil {
		return "", "", "", err
	}
	normalizedPath, err := normalizeHardiknasStoredPath(storedPath)
	if err != nil {
		return "", "", storageRoot, err
	}
	absolutePath := filepath.Clean(filepath.Join(storageRoot, filepath.FromSlash(normalizedPath)))
	baseWithSeparator := storageRoot + string(os.PathSeparator)
	if absolutePath != storageRoot && !strings.HasPrefix(absolutePath, baseWithSeparator) {
		return "", "", storageRoot, ErrHardiknasFileNotAvailable
	}
	return normalizedPath, absolutePath, storageRoot, nil
}

func (s HardiknasService) describeMediaAvailability(storedPath string) (bool, string) {
	if _, absolutePath, _, err := s.resolveStoragePathWithMeta(storedPath); err == nil {
		if info, statErr := os.Stat(absolutePath); statErr == nil && !info.IsDir() {
			return true, ""
		}
	}
	return false, hardiknasMediaMissing
}

func normalizeImageContentType(contentType string) string {
	switch contentType {
	case "image/png", "image/jpeg":
		return contentType
	default:
		return "image/jpeg"
	}
}
