package services

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
)

var (
	ErrServiceInfoForbidden  = errors.New("service info forbidden")
	ErrServiceInfoValidation = errors.New("service info validation")
	ErrServiceInfoNotFound   = errors.New("service info not found")
)

type ServiceInfoItem struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	SortOrder   int       `json:"sort_order"`
	IsBanner    bool      `json:"is_banner"`
	IsActive    bool      `json:"is_active"`
	IsArchived  bool      `json:"is_archived"`
	FileURL     string    `json:"file_url"`
	DownloadURL string    `json:"download_url"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	FileSize    int64     `json:"file_size"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ServiceInfoUpsertInput struct {
	ID          int64
	Title       string
	Category    string
	SortOrder   int
	IsBanner    bool
	IsActive    bool
	IsArchived  bool
	FileKey     string
	FileName    string
	ContentType string
	FileSize    int64
}

func serviceInfoPublicURL(id int64) string {
	return "/api/public/informasi-pelayanan-cabdin/files/" + strconv.FormatInt(id, 10)
}

func (s SiswaService) CanManageServiceInfo(ctx context.Context, user auth.User) (bool, error) {
	role := auth.NormalizeRole(user.Role)
	if role == auth.RoleAdminMaster {
		return true, nil
	}
	if role != auth.RoleAdminCabdin {
		return false, nil
	}
	if ok, _, err := tablesExist(ctx, s.DB, "cabdin_service_info_delegates"); err != nil || !ok {
		return false, err
	}
	var delegated bool
	err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cabdin_service_info_delegates WHERE user_id = $1)`, user.ID).Scan(&delegated)
	return delegated, err
}

func (s SiswaService) ListServiceInfo(ctx context.Context, includeInactive bool) ([]ServiceInfoItem, error) {
	if ok, _, err := tablesExist(ctx, s.DB, "cabdin_service_info_items"); err != nil || !ok {
		return []ServiceInfoItem{}, err
	}
	query := `
		SELECT id, title, category, sort_order, is_banner, is_active, is_archived,
		       COALESCE(file_name, ''), COALESCE(content_type, ''), file_size, created_at, updated_at
		FROM cabdin_service_info_items
		WHERE is_archived = FALSE
	`
	if !includeInactive {
		query += " AND is_active = TRUE"
	}
	query += " ORDER BY is_banner DESC, sort_order ASC, id ASC"
	rows, err := s.DB.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ServiceInfoItem{}
	for rows.Next() {
		var item ServiceInfoItem
		if err := rows.Scan(&item.ID, &item.Title, &item.Category, &item.SortOrder, &item.IsBanner, &item.IsActive, &item.IsArchived, &item.FileName, &item.ContentType, &item.FileSize, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.FileURL = serviceInfoPublicURL(item.ID)
		item.DownloadURL = item.FileURL + "?download=1"
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s SiswaService) UpsertServiceInfo(ctx context.Context, user auth.User, input ServiceInfoUpsertInput) (ServiceInfoItem, error) {
	canManage, err := s.CanManageServiceInfo(ctx, user)
	if err != nil || !canManage {
		if err != nil {
			return ServiceInfoItem{}, err
		}
		return ServiceInfoItem{}, ErrServiceInfoForbidden
	}
	title := strings.TrimSpace(input.Title)
	category := strings.TrimSpace(input.Category)
	if title == "" || category == "" {
		return ServiceInfoItem{}, ErrServiceInfoValidation
	}
	var id int64
	if input.ID > 0 {
		err = s.DB.Pool.QueryRow(ctx, `
			UPDATE cabdin_service_info_items
			SET title = $1, category = $2, sort_order = $3, is_banner = $4, is_active = $5, is_archived = $6,
			    file_storage_key = COALESCE(NULLIF(TRIM($7), ''), file_storage_key),
			    file_name = COALESCE(NULLIF(TRIM($8), ''), file_name),
			    content_type = COALESCE(NULLIF(TRIM($9), ''), content_type),
			    file_size = CASE WHEN NULLIF(TRIM($7), '') IS NULL THEN file_size ELSE $10 END,
			    updated_by = $11, updated_at = NOW()
			WHERE id = $12
			RETURNING id
		`, title, category, input.SortOrder, input.IsBanner, input.IsActive, input.IsArchived, input.FileKey, input.FileName, input.ContentType, input.FileSize, user.ID, input.ID).Scan(&id)
	} else {
		err = s.DB.Pool.QueryRow(ctx, `
			INSERT INTO cabdin_service_info_items
				(title, category, sort_order, is_banner, is_active, is_archived, file_storage_key, file_name, content_type, file_size, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF(TRIM($7), ''), NULLIF(TRIM($8), ''), NULLIF(TRIM($9), ''), $10, $11, $11)
			RETURNING id
		`, title, category, input.SortOrder, input.IsBanner, input.IsActive, input.IsArchived, input.FileKey, input.FileName, input.ContentType, input.FileSize, user.ID).Scan(&id)
	}
	if err != nil {
		return ServiceInfoItem{}, err
	}
	items, err := s.ListServiceInfo(ctx, true)
	if err != nil {
		return ServiceInfoItem{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return ServiceInfoItem{}, ErrServiceInfoNotFound
}

func (s SiswaService) ListServiceInfoDelegates(ctx context.Context, user auth.User) ([]DelegatedCabdinUser, error) {
	if !auth.IsAdminMaster(user) {
		return []DelegatedCabdinUser{}, auth.ErrForbidden
	}
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT u.id, u.username, COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS name,
		       d.user_id IS NOT NULL AS delegated
		FROM users u
		LEFT JOIN cabdin_service_info_delegates d ON d.user_id = u.id
		WHERE UPPER(TRIM(u.role::text)) IN ('ADMIN', 'ADMIN_CABDIN')
		ORDER BY name, u.username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DelegatedCabdinUser{}
	for rows.Next() {
		var item DelegatedCabdinUser
		if err := rows.Scan(&item.ID, &item.Username, &item.Name, &item.Delegated); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s SiswaService) SetServiceInfoDelegates(ctx context.Context, user auth.User, userIDs []int) error {
	if !auth.IsAdminMaster(user) {
		return auth.ErrForbidden
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM cabdin_service_info_delegates`); err != nil {
		return err
	}
	for _, id := range userIDs {
		if id <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO cabdin_service_info_delegates (user_id, created_by)
			SELECT id, $2 FROM users WHERE id = $1 AND UPPER(TRIM(role::text)) IN ('ADMIN', 'ADMIN_CABDIN')
			ON CONFLICT (user_id) DO NOTHING
		`, id, user.ID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
