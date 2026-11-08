package services

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/utils"
)

type AdminService struct {
	DB *db.DB
}

var cabdinAllowedLevels = map[string]struct{}{
	"SMA": {},
	"SMK": {},
	"SLB": {},
}

func normalizeCabdinLevels(levels []string) ([]string, error) {
	normalized := auth.NormalizeAllowedLevels(levels)
	if len(normalized) == 0 {
		return nil, errors.New("LEVELS_REQUIRED")
	}
	for _, level := range normalized {
		if _, ok := cabdinAllowedLevels[level]; !ok {
			return nil, errors.New("LEVEL_INVALID")
		}
	}
	return normalized, nil
}

type InitialPasswordSupport struct {
	V1 bool
	V2 bool
}

func (s AdminService) InitialPasswordSupport(ctx context.Context) (InitialPasswordSupport, error) {
	var v1 bool
	var v2Cipher bool
	var v2Nonce bool
	err := s.DB.Pool.QueryRow(ctx, `
        SELECT
            EXISTS(
                SELECT 1
                FROM information_schema.columns
                WHERE table_schema = 'public'
                  AND table_name = 'users'
                  AND column_name = 'initial_password_encrypted'
            ) AS v1,
            EXISTS(
                SELECT 1
                FROM information_schema.columns
                WHERE table_schema = 'public'
                  AND table_name = 'users'
                  AND column_name = 'initial_password_ciphertext'
            ) AS v2_cipher,
            EXISTS(
                SELECT 1
                FROM information_schema.columns
                WHERE table_schema = 'public'
                  AND table_name = 'users'
                  AND column_name = 'initial_password_nonce'
            ) AS v2_nonce
    `).Scan(&v1, &v2Cipher, &v2Nonce)
	if err != nil {
		return InitialPasswordSupport{}, err
	}
	return InitialPasswordSupport{V1: v1, V2: v2Cipher && v2Nonce}, nil
}

type SchoolListItem struct {
	ID          int
	NPSN        *string
	NamaSekolah string
}

type SchoolSuggestion struct {
	ID          int
	NPSN        *string
	NamaSekolah string
}

type SchoolAdminAccount struct {
	ID                 int
	Username           string
	SchoolID           *int
	NamaSekolah        *string
	NamaInstansi       *string
	NPSNInstansi       *string
	LastSeenAt         *time.Time
	IsOnline           bool
	HasInitialPassword bool
	LastUpdatePrestasi *time.Time
	LastUpdateSiswaPT  *time.Time
}

type CabdinAdminAccount struct {
	ID        int
	Name      string
	Username  string
	Levels    []string
	CreatedAt time.Time
}

type CabdinAdminCreate struct {
	Name     string
	Username string
	Password string
	Levels   []string
}

type SchoolAdminFilters struct {
	SortBy   string
	Order    string
	Query    string
	SchoolID *int
	Limit    int
	Offset   int
	AllowedLevels []string
}

type MonitoringSchoolFilters struct {
	Jenjang      string
	Status       string
	Akreditasi   string
	SchoolID     *int
	Query        string
	SortBy       string
	Order        string
	Limit        int
	Offset       int
	GuruAsnMin   *int
	GuruAsnMax   *int
	TendikAsnMin *int
	TendikAsnMax *int
	AllowedLevels []string
}

type MonitoringSchoolRow struct {
	SchoolID         int
	NamaSekolah      string
	BentukPendidikan string
	Akreditasi       string
	GuruAsn          int
	GuruHonorer      int
	TendikAsn        int
	TendikHonorer    int
}

type AdminUserRequest struct {
	Username                  string
	Password                  string
	Role                      string
	SchoolID                  *int
	NamaInstansi              *string
	NPSNInstansi              *string
	InitialPasswordCiphertext *string
	InitialPasswordNonce      *string
	InitialPasswordEncrypted  *string
}

func (s AdminService) CreateSchoolUser(ctx context.Context, req AdminUserRequest) (UserRecord, error) {
	if req.Username == "" || req.Password == "" || req.Role == "" {
		return UserRecord{}, errors.New("invalid payload")
	}
	if req.Role != "SCHOOL_ADMIN" {
		return UserRecord{}, errors.New("only SCHOOL_ADMIN is supported")
	}

	schoolID := req.SchoolID
	if schoolID == nil {
		if req.NamaInstansi == nil || *req.NamaInstansi == "" {
			return UserRecord{}, errors.New("school info is required")
		}

		row := s.DB.Pool.QueryRow(ctx, `
            INSERT INTO data_sekolah (nama_sekolah, npsn_sekolah)
            VALUES ($1, $2)
            RETURNING id
        `, *req.NamaInstansi, req.NPSNInstansi)
		var newID int
		if err := row.Scan(&newID); err != nil {
			return UserRecord{}, err
		}
		schoolID = &newID
	} else {
		var exists bool
		if err := s.DB.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM data_sekolah WHERE id = $1)", *schoolID).Scan(&exists); err != nil {
			return UserRecord{}, err
		}
		if !exists {
			return UserRecord{}, errors.New("school not found")
		}
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		return UserRecord{}, err
	}

	support, err := s.InitialPasswordSupport(ctx)
	if err != nil {
		return UserRecord{}, err
	}

	columns := "username, password_hash, role, school_id, npsn_instansi, nama_instansi"
	placeholders := "$1, $2, $3, $4, $5, $6"
	args := []interface{}{req.Username, hash, req.Role, schoolID, req.NPSNInstansi, req.NamaInstansi}
	if support.V2 {
		columns += ", initial_password_ciphertext, initial_password_nonce, initial_password_set_at"
		placeholders += ", $7, $8, NOW()"
		args = append(args, req.InitialPasswordCiphertext, req.InitialPasswordNonce)
	} else if support.V1 {
		columns += ", initial_password_encrypted"
		placeholders += ", $7"
		args = append(args, req.InitialPasswordEncrypted)
	}

	row := s.DB.Pool.QueryRow(ctx, `
        INSERT INTO users (`+columns+`)
        VALUES (`+placeholders+`)
        RETURNING id, username, role, school_id, npsn_instansi, nama_instansi
    `, args...)

	var record UserRecord
	err = row.Scan(&record.ID, &record.Username, &record.Role, &record.SchoolID, &record.NPSNInstansi, &record.NamaInstansi)
	return record, err
}

func (s AdminService) ListCabdinAdmins(ctx context.Context) ([]CabdinAdminAccount, error) {
	rows, err := s.DB.Pool.Query(ctx, `
        SELECT id,
               username,
               COALESCE(NULLIF(TRIM(nama_instansi), ''), username) AS name,
               COALESCE(allowed_levels, '{}'::text[]) AS allowed_levels,
               created_at
        FROM users
        WHERE role = 'ADMIN'
        ORDER BY created_at DESC, id DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CabdinAdminAccount
	for rows.Next() {
		var item CabdinAdminAccount
		if err := rows.Scan(&item.ID, &item.Username, &item.Name, &item.Levels, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (s AdminService) CreateCabdinAdmin(ctx context.Context, req CabdinAdminCreate) (CabdinAdminAccount, error) {
	name := strings.TrimSpace(req.Name)
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)

	if name == "" || username == "" || password == "" {
		return CabdinAdminAccount{}, errors.New("INVALID_PAYLOAD")
	}
	if len(password) < 8 {
		return CabdinAdminAccount{}, errors.New("PASSWORD_MIN_8")
	}
	normalizedLevels, err := normalizeCabdinLevels(req.Levels)
	if err != nil {
		return CabdinAdminAccount{}, err
	}

	var exists bool
	if err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists); err != nil {
		return CabdinAdminAccount{}, err
	}
	if exists {
		return CabdinAdminAccount{}, errors.New("USERNAME_EXISTS")
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return CabdinAdminAccount{}, err
	}

	row := s.DB.Pool.QueryRow(ctx, `
        INSERT INTO users (username, password_hash, role, nama_instansi, allowed_levels)
        VALUES ($1, $2, 'ADMIN', $3, $4)
        RETURNING id,
                  username,
                  COALESCE(NULLIF(TRIM(nama_instansi), ''), username) AS name,
                  COALESCE(allowed_levels, '{}'::text[]) AS allowed_levels,
                  created_at
    `, username, hash, name, normalizedLevels)

	var record CabdinAdminAccount
	if err := row.Scan(&record.ID, &record.Username, &record.Name, &record.Levels, &record.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CabdinAdminAccount{}, errors.New("USER_NOT_FOUND")
		}
		return CabdinAdminAccount{}, err
	}
	return record, nil
}

func (s AdminService) UpdateCabdinAdminLevels(ctx context.Context, userID int, levels []string) (CabdinAdminAccount, error) {
	if userID <= 0 {
		return CabdinAdminAccount{}, errors.New("INVALID_USER_ID")
	}

	normalizedLevels, err := normalizeCabdinLevels(levels)
	if err != nil {
		return CabdinAdminAccount{}, err
	}

	var role string
	if err := s.DB.Pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CabdinAdminAccount{}, errors.New("USER_NOT_FOUND")
		}
		return CabdinAdminAccount{}, err
	}
	if role != "ADMIN" {
		return CabdinAdminAccount{}, errors.New("USER_NOT_CABDIN")
	}

	row := s.DB.Pool.QueryRow(ctx, `
        UPDATE users
        SET allowed_levels = $1,
            updated_at = NOW()
        WHERE id = $2
        RETURNING id,
                  username,
                  COALESCE(NULLIF(TRIM(nama_instansi), ''), username) AS name,
                  COALESCE(allowed_levels, '{}'::text[]) AS allowed_levels,
                  created_at
    `, normalizedLevels, userID)

	var record CabdinAdminAccount
	if err := row.Scan(&record.ID, &record.Username, &record.Name, &record.Levels, &record.CreatedAt); err != nil {
		return CabdinAdminAccount{}, err
	}
	return record, nil
}

func (s AdminService) ListSchools(ctx context.Context, allowedLevels []string) ([]SchoolListItem, error) {
	args := []interface{}{}
	query := `
        SELECT id, npsn_sekolah, nama_sekolah
        FROM data_sekolah
        WHERE 1=1
    `
	if allowedLevels != nil {
		if len(allowedLevels) == 0 {
			query += " AND 1=0"
		} else {
			query += " AND COALESCE(UPPER(TRIM(bentuk_pendidikan)), '') = ANY($1)"
			args = append(args, allowedLevels)
		}
	}
	query += " ORDER BY nama_sekolah ASC"
	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SchoolListItem
	for rows.Next() {
		var item SchoolListItem
		if err := rows.Scan(&item.ID, &item.NPSN, &item.NamaSekolah); err == nil {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s AdminService) ListMonitoringSchools(ctx context.Context, filters MonitoringSchoolFilters) ([]MonitoringSchoolRow, error) {
	normalizedJenjang := normalizeUpperValue(filters.Jenjang)
	normalizedStatus := normalizeUpperValue(filters.Status)
	normalizedAkreditasi := normalizeUpperValue(filters.Akreditasi)

	if normalizedJenjang == "ALL" {
		normalizedJenjang = ""
	}
	if normalizedStatus == "ALL" {
		normalizedStatus = ""
	}
	if normalizedAkreditasi == "ALL" {
		normalizedAkreditasi = ""
	}
	switch normalizedAkreditasi {
	case "", "A", "B", "C", "BELUM":
	default:
		normalizedAkreditasi = ""
	}

	query := `
        WITH normalized AS (
            SELECT id,
                   npsn_sekolah,
                   nama_sekolah,
                   COALESCE(UPPER(TRIM(bentuk_pendidikan)), '') AS bentuk_pendidikan,
                   COALESCE(UPPER(TRIM(status_sekolah)), '') AS status,
                   CASE
                       WHEN UPPER(TRIM(akreditasi)) IN ('A', 'B', 'C') THEN UPPER(TRIM(akreditasi))
                       ELSE 'BELUM'
                   END AS akreditasi,
                   COALESCE(jumlah_guru_asn, 0) AS guru_asn,
                   COALESCE(jumlah_guru_honorer, 0) AS guru_honorer,
                   COALESCE(jumlah_tendik_asn, 0) AS tendik_asn,
                   COALESCE(jumlah_tendik_honorer, 0) AS tendik_honorer
            FROM data_sekolah
        )
        SELECT id, nama_sekolah, bentuk_pendidikan, akreditasi, guru_asn, guru_honorer, tendik_asn, tendik_honorer
        FROM normalized
        WHERE 1=1
    `

	args := []interface{}{}
	idx := 1

	if normalizedJenjang != "" {
		query += " AND bentuk_pendidikan = $" + strconv.Itoa(idx)
		args = append(args, normalizedJenjang)
		idx++
	}
	if normalizedStatus != "" {
		query += " AND status = $" + strconv.Itoa(idx)
		args = append(args, normalizedStatus)
		idx++
	}
	if normalizedAkreditasi != "" {
		query += " AND akreditasi = $" + strconv.Itoa(idx)
		args = append(args, normalizedAkreditasi)
		idx++
	}
	if filters.AllowedLevels != nil {
		if len(filters.AllowedLevels) == 0 {
			query += " AND 1=0"
		} else {
			query += " AND bentuk_pendidikan = ANY($" + strconv.Itoa(idx) + ")"
			args = append(args, filters.AllowedLevels)
			idx++
		}
	}
	if filters.SchoolID != nil {
		query += " AND id = $" + strconv.Itoa(idx)
		args = append(args, *filters.SchoolID)
		idx++
	}
	if strings.TrimSpace(filters.Query) != "" {
		query += " AND (LOWER(nama_sekolah) LIKE $" + strconv.Itoa(idx) + " OR LOWER(COALESCE(npsn_sekolah, '')) LIKE $" + strconv.Itoa(idx) + ")"
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(filters.Query))+"%")
		idx++
	}
	if filters.GuruAsnMin != nil {
		query += " AND guru_asn >= $" + strconv.Itoa(idx)
		args = append(args, *filters.GuruAsnMin)
		idx++
	}
	if filters.GuruAsnMax != nil {
		query += " AND guru_asn <= $" + strconv.Itoa(idx)
		args = append(args, *filters.GuruAsnMax)
		idx++
	}
	if filters.TendikAsnMin != nil {
		query += " AND tendik_asn >= $" + strconv.Itoa(idx)
		args = append(args, *filters.TendikAsnMin)
		idx++
	}
	if filters.TendikAsnMax != nil {
		query += " AND tendik_asn <= $" + strconv.Itoa(idx)
		args = append(args, *filters.TendikAsnMax)
		idx++
	}

	sortBy := strings.ToLower(strings.TrimSpace(filters.SortBy))
	order := strings.ToLower(strings.TrimSpace(filters.Order))
	if order != "asc" {
		order = "desc"
	}

	switch sortBy {
	case "bentuk_pendidikan", "akreditasi", "guru_asn", "guru_honorer", "tendik_asn", "tendik_honorer":
		query += " ORDER BY " + sortBy + " " + order
	default:
		query += " ORDER BY nama_sekolah " + order
	}

	if filters.Limit > 0 {
		limit := filters.Limit
		if limit > 50 {
			limit = 50
		}
		offset := filters.Offset
		if offset < 0 {
			offset = 0
		}
		query += " LIMIT $" + strconv.Itoa(idx) + " OFFSET $" + strconv.Itoa(idx+1)
		args = append(args, limit, offset)
	}

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MonitoringSchoolRow
	for rows.Next() {
		var item MonitoringSchoolRow
		if err := rows.Scan(
			&item.SchoolID,
			&item.NamaSekolah,
			&item.BentukPendidikan,
			&item.Akreditasi,
			&item.GuruAsn,
			&item.GuruHonorer,
			&item.TendikAsn,
			&item.TendikHonorer,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, nil
}

func (s AdminService) SuggestSchools(ctx context.Context, query string, limit int, allowedLevels []string) ([]SchoolSuggestion, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}
	trimmed := strings.TrimSpace(query)
	pattern := "%" + strings.ToLower(trimmed) + "%"
	prefix := strings.ToLower(trimmed) + "%"
	args := []interface{}{pattern, prefix}
	querySQL := `
        SELECT id, npsn_sekolah, nama_sekolah
        FROM data_sekolah
        WHERE (LOWER(nama_sekolah) LIKE $1
           OR LOWER(COALESCE(npsn_sekolah, '')) LIKE $1)
    `
	if allowedLevels != nil {
		if len(allowedLevels) == 0 {
			querySQL += " AND 1=0"
		} else {
			querySQL += " AND COALESCE(UPPER(TRIM(bentuk_pendidikan)), '') = ANY($3)"
			args = append(args, allowedLevels)
		}
	}
	orderStart := 3
	if allowedLevels != nil && len(allowedLevels) > 0 {
		orderStart = 4
	}
	querySQL += `
        ORDER BY
            CASE
                WHEN LOWER(nama_sekolah) LIKE $2 THEN 0
                WHEN LOWER(COALESCE(npsn_sekolah, '')) LIKE $2 THEN 0
                ELSE 1
            END,
            nama_sekolah ASC
        LIMIT $` + strconv.Itoa(orderStart) + `
    `
	args = append(args, limit)
	rows, err := s.DB.Pool.Query(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SchoolSuggestion
	for rows.Next() {
		var item SchoolSuggestion
		if err := rows.Scan(&item.ID, &item.NPSN, &item.NamaSekolah); err == nil {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s AdminService) CreateSchoolAdmin(ctx context.Context, schoolID int, username, password string, initialPasswordCiphertext *string, initialPasswordNonce *string, initialPasswordEncrypted *string) (UserRecord, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return UserRecord{}, errors.New("invalid payload")
	}
	if len(password) < 8 {
		return UserRecord{}, errors.New("password too short")
	}

	var exists bool
	if err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists); err != nil {
		return UserRecord{}, err
	}
	if exists {
		return UserRecord{}, errors.New("username already exists")
	}

	var schoolNPSN *string
	var schoolName string
	if err := s.DB.Pool.QueryRow(ctx, `
        SELECT npsn_sekolah, nama_sekolah
        FROM data_sekolah
        WHERE id = $1
    `, schoolID).Scan(&schoolNPSN, &schoolName); err != nil {
		return UserRecord{}, errors.New("school not found")
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return UserRecord{}, err
	}

	support, err := s.InitialPasswordSupport(ctx)
	if err != nil {
		return UserRecord{}, err
	}

	columns := "username, password_hash, role, school_id, npsn_instansi, nama_instansi"
	placeholders := "$1, $2, 'SCHOOL_ADMIN', $3, $4, $5"
	args := []interface{}{username, hash, schoolID, schoolNPSN, schoolName}
	if support.V2 {
		columns += ", initial_password_ciphertext, initial_password_nonce, initial_password_set_at"
		placeholders += ", $6, $7, NOW()"
		args = append(args, initialPasswordCiphertext, initialPasswordNonce)
	} else if support.V1 {
		columns += ", initial_password_encrypted"
		placeholders += ", $6"
		args = append(args, initialPasswordEncrypted)
	}

	row := s.DB.Pool.QueryRow(ctx, `
        INSERT INTO users (`+columns+`)
        VALUES (`+placeholders+`)
        RETURNING id, username, role, school_id, npsn_instansi, nama_instansi
    `, args...)

	var record UserRecord
	err = row.Scan(&record.ID, &record.Username, &record.Role, &record.SchoolID, &record.NPSNInstansi, &record.NamaInstansi)
	return record, err
}

type SchoolAdminUpdate struct {
	Username     string
	SchoolID     *int
	NamaInstansi *string
	NPSNInstansi *string
}

func (s AdminService) ListSchoolAdmins(ctx context.Context, now time.Time, filters SchoolAdminFilters) ([]SchoolAdminAccount, error) {
	sortBy := strings.ToLower(strings.TrimSpace(filters.SortBy))
	order := strings.ToLower(strings.TrimSpace(filters.Order))
	if order != "asc" {
		order = "desc"
	}

	support, err := s.InitialPasswordSupport(ctx)
	if err != nil {
		return nil, err
	}
	initialPasswordExpr := "FALSE AS has_initial_password"
	if support.V2 {
		initialPasswordExpr = "(u.initial_password_ciphertext IS NOT NULL AND u.initial_password_nonce IS NOT NULL) AS has_initial_password"
	} else if support.V1 {
		initialPasswordExpr = "(u.initial_password_encrypted IS NOT NULL) AS has_initial_password"
	}

	query := `
        SELECT u.id, u.username, u.school_id, u.npsn_instansi, u.nama_instansi, u.last_seen_at,
               ds.nama_sekolah, ds.last_update_prestasi, ds.last_update_siswa_pt,
               (u.last_seen_at IS NOT NULL AND u.last_seen_at >= ($1::timestamp - interval '10 minutes')) AS is_online,
               ` + initialPasswordExpr + `
        FROM users u
        LEFT JOIN data_sekolah ds ON ds.id = u.school_id
        WHERE u.role = 'SCHOOL_ADMIN'
    `

	args := []interface{}{now}
	idx := 2
	if filters.SchoolID != nil {
		query += " AND u.school_id = $" + strconv.Itoa(idx)
		args = append(args, *filters.SchoolID)
		idx++
	}
	if filters.AllowedLevels != nil {
		if len(filters.AllowedLevels) == 0 {
			query += " AND 1=0"
		} else {
			query += " AND COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)), '') = ANY($" + strconv.Itoa(idx) + ")"
			args = append(args, filters.AllowedLevels)
			idx++
		}
	}
	if strings.TrimSpace(filters.Query) != "" {
		query += " AND ds.nama_sekolah ILIKE $" + strconv.Itoa(idx)
		args = append(args, "%"+strings.TrimSpace(filters.Query)+"%")
		idx++
	}

	orderClause := ""
	nulls := "NULLS LAST"
	if order == "asc" {
		nulls = "NULLS FIRST"
	}

	switch sortBy {
	case "nama_sekolah":
		orderClause = " ORDER BY COALESCE(ds.nama_sekolah, '') " + order + " " + nulls
	case "last_update_prestasi":
		orderClause = " ORDER BY ds.last_update_prestasi " + order + " " + nulls
	case "last_update_siswa_pt":
		orderClause = " ORDER BY ds.last_update_siswa_pt " + order + " " + nulls
	case "status":
		orderClause = " ORDER BY is_online " + order + ", u.username ASC"
	default:
		orderClause = " ORDER BY is_online DESC, u.username ASC"
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	query += orderClause + " LIMIT $" + strconv.Itoa(idx) + " OFFSET $" + strconv.Itoa(idx+1)
	args = append(args, limit, offset)

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SchoolAdminAccount
	for rows.Next() {
		var item SchoolAdminAccount
		if err := rows.Scan(
			&item.ID,
			&item.Username,
			&item.SchoolID,
			&item.NPSNInstansi,
			&item.NamaInstansi,
			&item.LastSeenAt,
			&item.NamaSekolah,
			&item.LastUpdatePrestasi,
			&item.LastUpdateSiswaPT,
			&item.IsOnline,
			&item.HasInitialPassword,
		); err == nil {
			result = append(result, item)
		}
	}

	return result, nil
}

func (s AdminService) CountSchoolAdmins(ctx context.Context, filters SchoolAdminFilters) (int, error) {
	query := `
        SELECT COUNT(1)
        FROM users u
        LEFT JOIN data_sekolah ds ON ds.id = u.school_id
        WHERE u.role = 'SCHOOL_ADMIN'
    `

	args := []interface{}{}
	idx := 1
	if filters.SchoolID != nil {
		query += " AND u.school_id = $" + strconv.Itoa(idx)
		args = append(args, *filters.SchoolID)
		idx++
	}
	if filters.AllowedLevels != nil {
		if len(filters.AllowedLevels) == 0 {
			query += " AND 1=0"
		} else {
			query += " AND COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)), '') = ANY($" + strconv.Itoa(idx) + ")"
			args = append(args, filters.AllowedLevels)
			idx++
		}
	}
	if strings.TrimSpace(filters.Query) != "" {
		query += " AND ds.nama_sekolah ILIKE $" + strconv.Itoa(idx)
		args = append(args, "%"+strings.TrimSpace(filters.Query)+"%")
		idx++
	}

	var total int
	if err := s.DB.Pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

type BulkGenerateFilters struct {
	Jenjang string
	Status  string
}

const (
	bulkGenerateReasonAlreadyHasAccount        = "ALREADY_HAS_ACCOUNT"
	bulkGenerateReasonInvalidSchoolName        = "INVALID_SCHOOL_NAME"
	bulkGenerateReasonUsernameCollisionResolved = "USERNAME_COLLISION_RESOLVED"
	bulkGenerateReasonUsernameCollisionUnresolved = "USERNAME_COLLISION_UNRESOLVED"
	bulkGenerateReasonQueryFailed              = "QUERY_FAILED"
	bulkGenerateReasonInsertFailed             = "INSERT_FAILED"
)

type GeneratedSchoolAdmin struct {
	SchoolID    int
	NamaSekolah string
	Username    string
	Password    string // plaintext (HANYA untuk response download)
	NPSN        *string
	Reason      string
}

type SkippedSchoolAdmin struct {
	SchoolID    int
	NamaSekolah string
	NPSN        *string
	Reason      string
}

type RejectedSchoolAdmin struct {
	SchoolID    int
	NamaSekolah string
	NPSN        *string
	Reason      string
}

type BulkGenerateSummary struct {
	Generated []GeneratedSchoolAdmin
	Skipped   []SkippedSchoolAdmin
	Rejected  []RejectedSchoolAdmin
}

func (s AdminService) GenerateSchoolAdmins(ctx context.Context, filters BulkGenerateFilters, encryptionKey []byte) (BulkGenerateSummary, error) {
	normalizedJenjang := normalizeUpperValue(filters.Jenjang)
	normalizedStatus := normalizeUpperValue(filters.Status)

	switch normalizedJenjang {
	case "SMA", "SMK", "SLB":
	default:
		return BulkGenerateSummary{}, errors.New("invalid jenjang")
	}
	switch normalizedStatus {
	case "NEGERI", "SWASTA":
	default:
		return BulkGenerateSummary{}, errors.New("invalid status")
	}

	rows, err := s.DB.Pool.Query(ctx, `
        SELECT id, npsn_sekolah, nama_sekolah
        FROM data_sekolah
        WHERE COALESCE(UPPER(TRIM(bentuk_pendidikan)), '') = $1
          AND COALESCE(UPPER(TRIM(status_sekolah)), '') = $2
        ORDER BY nama_sekolah ASC
    `, normalizedJenjang, normalizedStatus)
	if err != nil {
		return BulkGenerateSummary{}, err
	}
	defer rows.Close()

	type schoolRow struct {
		ID          int
		NPSN        *string
		NamaSekolah string
	}
	var schools []schoolRow
	for rows.Next() {
		var row schoolRow
		if err := rows.Scan(&row.ID, &row.NPSN, &row.NamaSekolah); err != nil {
			return BulkGenerateSummary{}, err
		}
		schools = append(schools, row)
	}

	summary := BulkGenerateSummary{
		Generated: []GeneratedSchoolAdmin{},
		Skipped:   []SkippedSchoolAdmin{},
		Rejected:  []RejectedSchoolAdmin{},
	}

	support, err := s.InitialPasswordSupport(ctx)
	if err != nil {
		return summary, err
	}

	hasSchoolAdmin := func(schoolID int) (bool, error) {
		var exists bool
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT EXISTS(
                SELECT 1 FROM users WHERE role = 'SCHOOL_ADMIN' AND school_id = $1
            )
        `, schoolID).Scan(&exists); err != nil {
			return false, err
		}
		return exists, nil
	}

	usernameExists := func(username string) (bool, error) {
		var exists bool
		if err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists); err != nil {
			return false, err
		}
		return exists, nil
	}

	insertSchoolAdmin := func(username, password string, school schoolRow) error {
		hash, err := utils.HashPassword(password)
		if err != nil {
			return err
		}

		columns := "username, password_hash, role, school_id, npsn_instansi, nama_instansi"
		placeholders := "$1, $2, 'SCHOOL_ADMIN', $3, $4, $5"
		args := []interface{}{username, hash, school.ID, school.NPSN, school.NamaSekolah}
		if support.V2 {
			ciphertext, nonce, encErr := utils.EncryptInitialPassword(encryptionKey, password)
			if encErr != nil {
				return encErr
			}
			columns += ", initial_password_ciphertext, initial_password_nonce, initial_password_set_at"
			placeholders += ", $6, $7, NOW()"
			args = append(args, ciphertext, nonce)
		} else if support.V1 {
			encrypted, encErr := utils.EncryptString(encryptionKey, password)
			if encErr != nil {
				return encErr
			}
			columns += ", initial_password_encrypted"
			placeholders += ", $6"
			args = append(args, encrypted)
		}

		_, err = s.DB.Pool.Exec(ctx, `
            INSERT INTO users (`+columns+`)
            VALUES (`+placeholders+`)
        `, args...)
		return err
	}

	for _, school := range schools {
		exists, err := hasSchoolAdmin(school.ID)
		if err != nil {
			logging.Errorf("admin:service", "generate_school_admins", "school_id=%d err=%v", school.ID, err)
			summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
				SchoolID:    school.ID,
				NamaSekolah: school.NamaSekolah,
				NPSN:        school.NPSN,
				Reason:      bulkGenerateReasonQueryFailed,
			})
			continue
		}
		if exists {
			summary.Skipped = append(summary.Skipped, SkippedSchoolAdmin{
				SchoolID:    school.ID,
				NamaSekolah: school.NamaSekolah,
				NPSN:        school.NPSN,
				Reason:      bulkGenerateReasonAlreadyHasAccount,
			})
			continue
		}

		slug := utils.SlugifySchoolName(school.NamaSekolah)
		if slug == "" {
			summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
				SchoolID:    school.ID,
				NamaSekolah: school.NamaSekolah,
				NPSN:        school.NPSN,
				Reason:      bulkGenerateReasonInvalidSchoolName,
			})
			continue
		}

		baseUsername := "wakakurikulum-" + slug
		username := baseUsername
		reason := ""

		exists, err = usernameExists(username)
		if err != nil {
			logging.Errorf("admin:service", "generate_school_admins", "school_id=%d err=%v", school.ID, err)
			summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
				SchoolID:    school.ID,
				NamaSekolah: school.NamaSekolah,
				NPSN:        school.NPSN,
				Reason:      bulkGenerateReasonQueryFailed,
			})
			continue
		}
		if exists {
			username = baseUsername + "-" + strconv.Itoa(school.ID)
			reason = bulkGenerateReasonUsernameCollisionResolved
			exists, err = usernameExists(username)
			if err != nil {
				logging.Errorf("admin:service", "generate_school_admins", "school_id=%d err=%v", school.ID, err)
				summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
					SchoolID:    school.ID,
					NamaSekolah: school.NamaSekolah,
					NPSN:        school.NPSN,
					Reason:      bulkGenerateReasonQueryFailed,
				})
				continue
			}
			if exists {
				summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
					SchoolID:    school.ID,
					NamaSekolah: school.NamaSekolah,
					NPSN:        school.NPSN,
					Reason:      bulkGenerateReasonUsernameCollisionUnresolved,
				})
				continue
			}
		}

		password, err := utils.GenerateStrongPassword(10)
		if err != nil {
			logging.Errorf("admin:service", "generate_school_admins", "school_id=%d err=%v", school.ID, err)
			summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
				SchoolID:    school.ID,
				NamaSekolah: school.NamaSekolah,
				NPSN:        school.NPSN,
				Reason:      bulkGenerateReasonInsertFailed,
			})
			continue
		}

		if err := insertSchoolAdmin(username, password, school); err != nil {
			if isUniqueViolation(err) {
				if reason == "" {
					alternateUsername := baseUsername + "-" + strconv.Itoa(school.ID)
					altExists, altErr := usernameExists(alternateUsername)
					if altErr != nil {
						logging.Errorf("admin:service", "generate_school_admins", "school_id=%d err=%v", school.ID, altErr)
						summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
							SchoolID:    school.ID,
							NamaSekolah: school.NamaSekolah,
							NPSN:        school.NPSN,
							Reason:      bulkGenerateReasonQueryFailed,
						})
						continue
					}
					if altExists {
						summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
							SchoolID:    school.ID,
							NamaSekolah: school.NamaSekolah,
							NPSN:        school.NPSN,
							Reason:      bulkGenerateReasonUsernameCollisionUnresolved,
						})
						continue
					}
					if retryErr := insertSchoolAdmin(alternateUsername, password, school); retryErr != nil {
						logging.Errorf("admin:service", "generate_school_admins", "school_id=%d err=%v", school.ID, retryErr)
						summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
							SchoolID:    school.ID,
							NamaSekolah: school.NamaSekolah,
							NPSN:        school.NPSN,
							Reason:      bulkGenerateReasonInsertFailed,
						})
						continue
					}
					username = alternateUsername
					reason = bulkGenerateReasonUsernameCollisionResolved
				} else {
					summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
						SchoolID:    school.ID,
						NamaSekolah: school.NamaSekolah,
						NPSN:        school.NPSN,
						Reason:      bulkGenerateReasonUsernameCollisionUnresolved,
					})
					continue
				}
			} else {
				logging.Errorf("admin:service", "generate_school_admins", "school_id=%d err=%v", school.ID, err)
				summary.Rejected = append(summary.Rejected, RejectedSchoolAdmin{
					SchoolID:    school.ID,
					NamaSekolah: school.NamaSekolah,
					NPSN:        school.NPSN,
					Reason:      bulkGenerateReasonInsertFailed,
				})
				continue
			}
		}

		summary.Generated = append(summary.Generated, GeneratedSchoolAdmin{
			SchoolID:    school.ID,
			NamaSekolah: school.NamaSekolah,
			Username:    username,
			Password:    password,    // used for Excel download
			NPSN:        school.NPSN, // keep NPSN for download
			Reason:      reason,
		})
	}

	logging.Debugf(
		"admin:service",
		"generate_school_admins_done",
		"generated=%d skipped=%d rejected=%d",
		len(summary.Generated),
		len(summary.Skipped),
		len(summary.Rejected),
	)

	return summary, nil
}

type SchoolAdminCredential struct {
	ID                        int
	Username                  string
	SchoolID                  *int
	NPSN                      *string
	NamaSekolah               *string
	InitialPasswordCiphertext *string
	InitialPasswordNonce      *string
	InitialPasswordEncrypted  *string
}

func (s AdminService) GetSchoolAdminCredential(ctx context.Context, userID int) (SchoolAdminCredential, error) {
	support, err := s.InitialPasswordSupport(ctx)
	if err != nil {
		return SchoolAdminCredential{}, err
	}

	cipherExpr := "NULL::text AS initial_password_ciphertext"
	nonceExpr := "NULL::text AS initial_password_nonce"
	legacyExpr := "NULL::text AS initial_password_encrypted"
	if support.V2 {
		cipherExpr = "u.initial_password_ciphertext"
		nonceExpr = "u.initial_password_nonce"
	}
	if support.V1 {
		legacyExpr = "u.initial_password_encrypted"
	}

	var result SchoolAdminCredential
	err = s.DB.Pool.QueryRow(ctx, `
        SELECT
            u.id,
            u.username,
            u.school_id,
            COALESCE(ds.npsn_sekolah, u.npsn_instansi) AS npsn,
            COALESCE(ds.nama_sekolah, u.nama_instansi) AS nama_sekolah,
            `+cipherExpr+`,
            `+nonceExpr+`,
            `+legacyExpr+`
        FROM users u
        LEFT JOIN data_sekolah ds ON ds.id = u.school_id
        WHERE u.id = $1 AND u.role = 'SCHOOL_ADMIN'
    `, userID).Scan(
		&result.ID,
		&result.Username,
		&result.SchoolID,
		&result.NPSN,
		&result.NamaSekolah,
		&result.InitialPasswordCiphertext,
		&result.InitialPasswordNonce,
		&result.InitialPasswordEncrypted,
	)
	if err != nil {
		return SchoolAdminCredential{}, err
	}
	return result, nil
}

func (s AdminService) ListSchoolAdminCredentials(ctx context.Context, allowedLevels []string) ([]SchoolAdminCredential, error) {
	support, err := s.InitialPasswordSupport(ctx)
	if err != nil {
		return nil, err
	}

	cipherExpr := "NULL::text AS initial_password_ciphertext"
	nonceExpr := "NULL::text AS initial_password_nonce"
	legacyExpr := "NULL::text AS initial_password_encrypted"
	if support.V2 {
		cipherExpr = "u.initial_password_ciphertext"
		nonceExpr = "u.initial_password_nonce"
	}
	if support.V1 {
		legacyExpr = "u.initial_password_encrypted"
	}

	query := `
        SELECT
            u.id,
            u.username,
            u.school_id,
            COALESCE(ds.npsn_sekolah, u.npsn_instansi) AS npsn,
            COALESCE(ds.nama_sekolah, u.nama_instansi) AS nama_sekolah,
            `+cipherExpr+`,
            `+nonceExpr+`,
            `+legacyExpr+`
        FROM users u
        LEFT JOIN data_sekolah ds ON ds.id = u.school_id
        WHERE u.role = 'SCHOOL_ADMIN'
    `
	args := []interface{}{}
	if allowedLevels != nil {
		if len(allowedLevels) == 0 {
			query += " AND 1=0"
		} else {
			query += " AND COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)), '') = ANY($1)"
			args = append(args, allowedLevels)
		}
	}
	query += `
        ORDER BY COALESCE(ds.nama_sekolah, u.nama_instansi) ASC, u.username ASC
    `
	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SchoolAdminCredential
	for rows.Next() {
		var item SchoolAdminCredential
		if err := rows.Scan(
			&item.ID,
			&item.Username,
			&item.SchoolID,
			&item.NPSN,
			&item.NamaSekolah,
			&item.InitialPasswordCiphertext,
			&item.InitialPasswordNonce,
			&item.InitialPasswordEncrypted,
		); err == nil {
			result = append(result, item)
		}
	}
	return result, nil
}

func IsUserOnline(lastSeenAt *time.Time, now time.Time) bool {
	if lastSeenAt == nil {
		return false
	}
	if lastSeenAt.After(now) {
		return false
	}
	return now.Sub(*lastSeenAt) <= 10*time.Minute
}

func (s AdminService) UpdateSchoolAdmin(ctx context.Context, userID int, update SchoolAdminUpdate) (SchoolAdminAccount, error) {
	if update.Username == "" {
		return SchoolAdminAccount{}, errors.New("username is required")
	}

	support, err := s.InitialPasswordSupport(ctx)
	if err != nil {
		return SchoolAdminAccount{}, err
	}
	initialPasswordExpr := "FALSE AS has_initial_password"
	if support.V2 {
		initialPasswordExpr = "(initial_password_ciphertext IS NOT NULL AND initial_password_nonce IS NOT NULL) AS has_initial_password"
	} else if support.V1 {
		initialPasswordExpr = "(initial_password_encrypted IS NOT NULL) AS has_initial_password"
	}

	row := s.DB.Pool.QueryRow(ctx, `
        UPDATE users
        SET username = $1,
            school_id = COALESCE($2, school_id),
            nama_instansi = COALESCE($3, nama_instansi),
            npsn_instansi = COALESCE($4, npsn_instansi),
            updated_at = NOW()
        WHERE id = $5 AND role = 'SCHOOL_ADMIN'
        RETURNING id, username, school_id, npsn_instansi, nama_instansi, last_seen_at,
                  `+initialPasswordExpr+`
    `, update.Username, update.SchoolID, update.NamaInstansi, update.NPSNInstansi, userID)

	var result SchoolAdminAccount
	if err := row.Scan(
		&result.ID,
		&result.Username,
		&result.SchoolID,
		&result.NPSNInstansi,
		&result.NamaInstansi,
		&result.LastSeenAt,
		&result.HasInitialPassword,
	); err != nil {
		return SchoolAdminAccount{}, err
	}

	if result.SchoolID != nil {
		_ = s.DB.Pool.QueryRow(ctx, `SELECT nama_sekolah FROM data_sekolah WHERE id = $1`, *result.SchoolID).Scan(&result.NamaSekolah)
	}

	return result, nil
}

func (s AdminService) UpdateSchoolAdminPassword(ctx context.Context, userID int, newPassword string, encryptionKey []byte) error {
	if strings.TrimSpace(newPassword) == "" {
		return errors.New("invalid payload")
	}
	if len(newPassword) < 8 {
		return errors.New("password too short")
	}

	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}

	support, err := s.InitialPasswordSupport(ctx)
	if err != nil {
		return err
	}

	updateQuery := `
        UPDATE users
        SET password_hash = $1,
            token_version = token_version + 1,
            password_updated_at = NOW(),
            updated_at = NOW()
    `
	args := []interface{}{hash, userID}
	argIdx := 3

	if support.V2 {
		ciphertext, nonce, encErr := utils.EncryptInitialPassword(encryptionKey, newPassword)
		if encErr != nil {
			return encErr
		}
		updateQuery += `,
            initial_password_ciphertext = $` + strconv.Itoa(argIdx) + `,
            initial_password_nonce = $` + strconv.Itoa(argIdx+1) + `,
            initial_password_set_at = NOW()
        `
		args = append(args, ciphertext, nonce)
		argIdx += 2
	} else if support.V1 {
		encrypted, encErr := utils.EncryptString(encryptionKey, newPassword)
		if encErr != nil {
			return encErr
		}
		updateQuery += `,
            initial_password_encrypted = $` + strconv.Itoa(argIdx) + `
        `
		args = append(args, encrypted)
		argIdx++
	}

	updateQuery += `
        WHERE id = $2 AND role = 'SCHOOL_ADMIN'
    `

	result, err := s.DB.Pool.Exec(ctx, updateQuery, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("school admin not found")
	}
	return nil
}

type ResetDataOptions struct {
	Achievement          bool
	SiswaPerguruanTinggi bool
}

type ResetDataResult struct {
	AchievementDeleted          int64
	SiswaPerguruanTinggiDeleted int64
	LastUpdatePrestasi          *time.Time
	LastUpdateSiswaPT           *time.Time
}

func (s AdminService) ResetSchoolData(ctx context.Context, schoolID int, options ResetDataOptions) (ResetDataResult, error) {
	result := ResetDataResult{}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if options.Achievement {
		res, execErr := tx.Exec(ctx, `DELETE FROM prestasi_siswa WHERE school_id = $1`, schoolID)
		if execErr != nil {
			return result, execErr
		}
		result.AchievementDeleted = res.RowsAffected()
		if _, execErr := tx.Exec(ctx, `UPDATE data_sekolah SET last_update_prestasi = NULL WHERE id = $1`, schoolID); execErr != nil {
			return result, execErr
		}
	}

	if options.SiswaPerguruanTinggi {
		res, execErr := tx.Exec(ctx, `DELETE FROM siswa_perguruan_tinggi WHERE school_id = $1`, schoolID)
		if execErr != nil {
			return result, execErr
		}
		result.SiswaPerguruanTinggiDeleted = res.RowsAffected()
		if _, execErr := tx.Exec(ctx, `UPDATE data_sekolah SET last_update_siswa_pt = NULL WHERE id = $1`, schoolID); execErr != nil {
			return result, execErr
		}
	}

	if err := tx.QueryRow(ctx, `
        SELECT last_update_prestasi, last_update_siswa_pt
        FROM data_sekolah
        WHERE id = $1
    `, schoolID).Scan(&result.LastUpdatePrestasi, &result.LastUpdateSiswaPT); err != nil {
		return result, err
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}

	return result, nil
}

func (s AdminService) LogAdminAction(ctx context.Context, adminID int, action string) error {
	if adminID <= 0 || strings.TrimSpace(action) == "" {
		return errors.New("invalid payload")
	}
	_, err := s.DB.Pool.Exec(ctx, `
        INSERT INTO admin_audit_log (admin_id, action)
        VALUES ($1, $2)
    `, adminID, strings.TrimSpace(action))
	return err
}

func (s AdminService) LogAdminActionWithDetails(ctx context.Context, adminID int, action string, targetUserID *int, count *int) error {
	if adminID <= 0 || strings.TrimSpace(action) == "" {
		return errors.New("invalid payload")
	}
	_, err := s.DB.Pool.Exec(ctx, `
        INSERT INTO admin_audit_log (admin_id, action, target_user_id, count)
        VALUES ($1, $2, $3, $4)
    `, adminID, strings.TrimSpace(action), targetUserID, count)
	return err
}
