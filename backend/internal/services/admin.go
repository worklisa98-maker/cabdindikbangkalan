package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgconn"
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

func normalizeCabdinAdminEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	if email == "" {
		return "", errors.New("EMAIL_REQUIRED")
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed == nil || !strings.EqualFold(strings.TrimSpace(parsed.Address), email) {
		return "", errors.New("EMAIL_INVALID")
	}
	return email, nil
}

func isCabdinAdminUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
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
	ID                 int
	NPSN               *string
	NamaSekolah        string
	SupervisorUserID   *int
	SupervisorUsername *string
	SupervisorName     *string
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
	SupervisorName     *string
	SupervisorUsername *string
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

type PengawasAccount struct {
	ID          int
	Username    string
	Name        string
	SchoolCount int
	CreatedAt   time.Time
	Schools     []PengawasSchool
}

type PengawasCreate struct {
	Username  string
	Password  string
	Name      string
	SchoolIDs []int
}

type PengawasUpdate struct {
	Username *string
	Name     *string
}

type PengawasSchool struct {
	ID               int
	NPSN             *string
	NamaSekolah      string
	BentukPendidikan *string
}

type SchoolAssignmentConflictError struct {
	Schools []PengawasSchool
}

func (e *SchoolAssignmentConflictError) Error() string {
	return "SCHOOL_ASSIGNMENT_CONFLICT"
}

type CabdinAdminCreate struct {
	Name     string
	Username string
	Password string
	Levels   []string
}

type CabdinAdminUpdate struct {
	Name     *string
	Username *string
	Levels   *[]string
}

type SchoolAdminFilters struct {
	SortBy        string
	Order         string
	Query         string
	SchoolID      *int
	Limit         int
	Offset        int
	AllowedLevels []string
}

type MonitoringSchoolFilters struct {
	Jenjang        string
	Status         string
	Akreditasi     string
	SchoolID       *int
	ScopeAll       bool
	ScopeSchoolIDs []int
	Query          string
	SortBy         string
	Order          string
	Limit          int
	Offset         int
	GuruAsnMin     *int
	GuruAsnMax     *int
	TendikAsnMin   *int
	TendikAsnMax   *int
	AllowedLevels  []string
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

func normalizeSchoolIDs(ids []int) ([]int, error) {
	if len(ids) == 0 {
		return []int{}, nil
	}
	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, schoolID := range ids {
		if schoolID <= 0 {
			return nil, errors.New("INVALID_SCHOOL_ID")
		}
		if _, exists := seen[schoolID]; exists {
			continue
		}
		seen[schoolID] = struct{}{}
		normalized = append(normalized, schoolID)
	}
	return normalized, nil
}

func nullableStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func (s AdminService) listPengawasAccountsByID(ctx context.Context, userID *int) ([]PengawasAccount, error) {
	query := `
        SELECT u.id,
               u.username,
               COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS name,
               u.created_at,
               ds.id AS school_id,
               ds.npsn_sekolah,
               ds.nama_sekolah,
               ds.bentuk_pendidikan
        FROM users u
        LEFT JOIN data_sekolah ds ON ds.supervisor_user_id = u.id
        WHERE u.role = 'PENGAWAS'
    `
	args := []interface{}{}
	if userID != nil {
		query += " AND u.id = $1"
		args = append(args, *userID)
	}
	query += `
        ORDER BY u.created_at DESC, u.id DESC, ds.nama_sekolah ASC
    `

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]PengawasAccount, 0)
	accountIndex := map[int]int{}
	for rows.Next() {
		var (
			accountID        int
			username         string
			name             string
			createdAt        time.Time
			schoolID         sql.NullInt32
			npsnSekolah      sql.NullString
			namaSekolah      sql.NullString
			bentukPendidikan sql.NullString
		)

		if scanErr := rows.Scan(
			&accountID,
			&username,
			&name,
			&createdAt,
			&schoolID,
			&npsnSekolah,
			&namaSekolah,
			&bentukPendidikan,
		); scanErr != nil {
			return nil, scanErr
		}

		idx, exists := accountIndex[accountID]
		if !exists {
			accounts = append(accounts, PengawasAccount{
				ID:        accountID,
				Username:  username,
				Name:      name,
				CreatedAt: createdAt,
				Schools:   []PengawasSchool{},
			})
			idx = len(accounts) - 1
			accountIndex[accountID] = idx
		}

		if schoolID.Valid {
			accounts[idx].Schools = append(accounts[idx].Schools, PengawasSchool{
				ID:               int(schoolID.Int32),
				NPSN:             nullableStringPtr(npsnSekolah),
				NamaSekolah:      strings.TrimSpace(namaSekolah.String),
				BentukPendidikan: nullableStringPtr(bentukPendidikan),
			})
		}
	}

	for index := range accounts {
		accounts[index].SchoolCount = len(accounts[index].Schools)
	}

	return accounts, nil
}

func (s AdminService) ListPengawasAccounts(ctx context.Context) ([]PengawasAccount, error) {
	return s.listPengawasAccountsByID(ctx, nil)
}

func (s AdminService) GetPengawasAccount(ctx context.Context, userID int) (PengawasAccount, error) {
	if userID <= 0 {
		return PengawasAccount{}, errors.New("INVALID_USER_ID")
	}

	records, err := s.listPengawasAccountsByID(ctx, &userID)
	if err != nil {
		return PengawasAccount{}, err
	}
	if len(records) == 0 {
		return PengawasAccount{}, errors.New("USER_NOT_FOUND")
	}
	return records[0], nil
}

func (s AdminService) CreatePengawasAccount(ctx context.Context, req PengawasCreate) (PengawasAccount, error) {
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	name := strings.TrimSpace(req.Name)
	schoolIDs, err := normalizeSchoolIDs(req.SchoolIDs)
	if err != nil {
		return PengawasAccount{}, err
	}

	if username == "" || password == "" {
		return PengawasAccount{}, errors.New("INVALID_PAYLOAD")
	}
	if len(password) < 8 {
		return PengawasAccount{}, errors.New("PASSWORD_MIN_8")
	}
	if name == "" {
		name = username
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return PengawasAccount{}, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return PengawasAccount{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	var createdID int
	if err := tx.QueryRow(ctx, `
        INSERT INTO users (username, password_hash, role, nama_instansi)
        VALUES ($1, $2, 'PENGAWAS', $3)
        RETURNING id
    `, username, hash, name).Scan(&createdID); err != nil {
		if isCabdinAdminUniqueViolation(err) {
			return PengawasAccount{}, errors.New("USERNAME_EXISTS")
		}
		return PengawasAccount{}, err
	}

	if len(schoolIDs) > 0 {
		rows, queryErr := tx.Query(ctx, `
            SELECT id,
                   npsn_sekolah,
                   nama_sekolah,
                   bentuk_pendidikan,
                   supervisor_user_id
            FROM data_sekolah
            WHERE id = ANY($1)
            ORDER BY nama_sekolah ASC
            FOR UPDATE
        `, schoolIDs)
		if queryErr != nil {
			return PengawasAccount{}, queryErr
		}

		requestedByID := make(map[int]struct{}, len(schoolIDs))
		for _, schoolID := range schoolIDs {
			requestedByID[schoolID] = struct{}{}
		}

		foundCount := 0
		conflicts := make([]PengawasSchool, 0)
		for rows.Next() {
			var (
				schoolID            int
				npsnSekolah         sql.NullString
				namaSekolah         string
				bentukPendidikan    sql.NullString
				supervisorUserIDRaw sql.NullInt32
			)
			if scanErr := rows.Scan(
				&schoolID,
				&npsnSekolah,
				&namaSekolah,
				&bentukPendidikan,
				&supervisorUserIDRaw,
			); scanErr != nil {
				rows.Close()
				return PengawasAccount{}, scanErr
			}
			if _, exists := requestedByID[schoolID]; exists {
				foundCount++
			}
			if supervisorUserIDRaw.Valid {
				conflicts = append(conflicts, PengawasSchool{
					ID:               schoolID,
					NPSN:             nullableStringPtr(npsnSekolah),
					NamaSekolah:      namaSekolah,
					BentukPendidikan: nullableStringPtr(bentukPendidikan),
				})
			}
		}
		if rows.Err() != nil {
			rows.Close()
			return PengawasAccount{}, rows.Err()
		}
		rows.Close()

		if foundCount != len(schoolIDs) {
			return PengawasAccount{}, errors.New("SCHOOL_NOT_FOUND")
		}
		if len(conflicts) > 0 {
			return PengawasAccount{}, &SchoolAssignmentConflictError{Schools: conflicts}
		}

		if _, execErr := tx.Exec(ctx, `
            UPDATE data_sekolah
            SET supervisor_user_id = $1,
                updated_at = NOW()
            WHERE id = ANY($2)
        `, createdID, schoolIDs); execErr != nil {
			return PengawasAccount{}, execErr
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return PengawasAccount{}, err
	}
	committed = true

	return s.GetPengawasAccount(ctx, createdID)
}

func (s AdminService) UpdatePengawasAccount(ctx context.Context, userID int, req PengawasUpdate) (PengawasAccount, error) {
	if userID <= 0 {
		return PengawasAccount{}, errors.New("INVALID_USER_ID")
	}

	var normalizedUsername *string
	if req.Username != nil {
		trimmed := strings.TrimSpace(*req.Username)
		if trimmed == "" {
			return PengawasAccount{}, errors.New("INVALID_PAYLOAD")
		}
		normalizedUsername = &trimmed
	}

	var normalizedName *string
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return PengawasAccount{}, errors.New("INVALID_PAYLOAD")
		}
		normalizedName = &trimmed
	}

	if normalizedUsername == nil && normalizedName == nil {
		return PengawasAccount{}, errors.New("INVALID_PAYLOAD")
	}

	var role string
	if err := s.DB.Pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PengawasAccount{}, errors.New("USER_NOT_FOUND")
		}
		return PengawasAccount{}, err
	}
	if role != auth.RolePengawas {
		return PengawasAccount{}, errors.New("USER_NOT_PENGAWAS")
	}

	if normalizedUsername != nil {
		var exists bool
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT EXISTS(
                SELECT 1
                FROM users
                WHERE username = $1
                  AND id <> $2
            )
        `, *normalizedUsername, userID).Scan(&exists); err != nil {
			return PengawasAccount{}, err
		}
		if exists {
			return PengawasAccount{}, errors.New("USERNAME_EXISTS")
		}
	}

	setParts := make([]string, 0, 3)
	args := make([]interface{}, 0, 3)
	argPos := 1
	if normalizedUsername != nil {
		setParts = append(setParts, "username = $"+strconv.Itoa(argPos))
		args = append(args, *normalizedUsername)
		argPos++
	}
	if normalizedName != nil {
		setParts = append(setParts, "nama_instansi = $"+strconv.Itoa(argPos))
		args = append(args, *normalizedName)
		argPos++
	}
	setParts = append(setParts, "updated_at = NOW()")

	query := `
        UPDATE users
        SET ` + strings.Join(setParts, ", ") + `
        WHERE id = $` + strconv.Itoa(argPos) + `
          AND role = 'PENGAWAS'
    `
	args = append(args, userID)
	tag, err := s.DB.Pool.Exec(ctx, query, args...)
	if err != nil {
		if isCabdinAdminUniqueViolation(err) {
			return PengawasAccount{}, errors.New("USERNAME_EXISTS")
		}
		return PengawasAccount{}, err
	}
	if tag.RowsAffected() == 0 {
		return PengawasAccount{}, errors.New("USER_NOT_FOUND")
	}

	return s.GetPengawasAccount(ctx, userID)
}

func (s AdminService) ResetPengawasPassword(ctx context.Context, userID int, newPassword string) error {
	if userID <= 0 {
		return errors.New("INVALID_USER_ID")
	}
	password := strings.TrimSpace(newPassword)
	if len(password) < 8 {
		return errors.New("PASSWORD_MIN_8")
	}

	var role string
	if err := s.DB.Pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("USER_NOT_FOUND")
		}
		return err
	}
	if role != auth.RolePengawas {
		return errors.New("USER_NOT_PENGAWAS")
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	tag, err := s.DB.Pool.Exec(ctx, `
        UPDATE users
        SET password_hash = $1,
            token_version = token_version + 1,
            password_updated_at = NOW(),
            updated_at = NOW()
        WHERE id = $2
          AND role = 'PENGAWAS'
    `, hash, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("USER_NOT_FOUND")
	}
	return nil
}

func (s AdminService) UpdatePengawasSchools(ctx context.Context, userID int, schoolIDs []int) ([]PengawasSchool, error) {
	if userID <= 0 {
		return nil, errors.New("INVALID_USER_ID")
	}
	normalizedSchoolIDs, err := normalizeSchoolIDs(schoolIDs)
	if err != nil {
		return nil, err
	}

	var role string
	if err := s.DB.Pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("USER_NOT_FOUND")
		}
		return nil, err
	}
	if role != auth.RolePengawas {
		return nil, errors.New("USER_NOT_PENGAWAS")
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if len(normalizedSchoolIDs) > 0 {
		rows, queryErr := tx.Query(ctx, `
            SELECT id,
                   npsn_sekolah,
                   nama_sekolah,
                   bentuk_pendidikan,
                   supervisor_user_id
            FROM data_sekolah
            WHERE id = ANY($1)
            ORDER BY nama_sekolah ASC
            FOR UPDATE
        `, normalizedSchoolIDs)
		if queryErr != nil {
			return nil, queryErr
		}

		requestedByID := make(map[int]struct{}, len(normalizedSchoolIDs))
		for _, schoolID := range normalizedSchoolIDs {
			requestedByID[schoolID] = struct{}{}
		}

		foundCount := 0
		conflicts := make([]PengawasSchool, 0)
		for rows.Next() {
			var (
				schoolID            int
				npsnSekolah         sql.NullString
				namaSekolah         string
				bentukPendidikan    sql.NullString
				supervisorUserIDRaw sql.NullInt32
			)
			if scanErr := rows.Scan(
				&schoolID,
				&npsnSekolah,
				&namaSekolah,
				&bentukPendidikan,
				&supervisorUserIDRaw,
			); scanErr != nil {
				rows.Close()
				return nil, scanErr
			}
			if _, exists := requestedByID[schoolID]; exists {
				foundCount++
			}
			if supervisorUserIDRaw.Valid && int(supervisorUserIDRaw.Int32) != userID {
				conflicts = append(conflicts, PengawasSchool{
					ID:               schoolID,
					NPSN:             nullableStringPtr(npsnSekolah),
					NamaSekolah:      namaSekolah,
					BentukPendidikan: nullableStringPtr(bentukPendidikan),
				})
			}
		}
		if rows.Err() != nil {
			rows.Close()
			return nil, rows.Err()
		}
		rows.Close()

		if foundCount != len(normalizedSchoolIDs) {
			return nil, errors.New("SCHOOL_NOT_FOUND")
		}
		if len(conflicts) > 0 {
			return nil, &SchoolAssignmentConflictError{Schools: conflicts}
		}
	}

	if len(normalizedSchoolIDs) == 0 {
		if _, execErr := tx.Exec(ctx, `
            UPDATE data_sekolah
            SET supervisor_user_id = NULL,
                updated_at = NOW()
            WHERE supervisor_user_id = $1
        `, userID); execErr != nil {
			return nil, execErr
		}
	} else {
		if _, execErr := tx.Exec(ctx, `
            UPDATE data_sekolah
            SET supervisor_user_id = NULL,
                updated_at = NOW()
            WHERE supervisor_user_id = $1
              AND NOT (id = ANY($2))
        `, userID, normalizedSchoolIDs); execErr != nil {
			return nil, execErr
		}
		if _, execErr := tx.Exec(ctx, `
            UPDATE data_sekolah
            SET supervisor_user_id = $1,
                updated_at = NOW()
            WHERE id = ANY($2)
        `, userID, normalizedSchoolIDs); execErr != nil {
			return nil, execErr
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	committed = true

	account, err := s.GetPengawasAccount(ctx, userID)
	if err != nil {
		return nil, err
	}
	return account.Schools, nil
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

func (s AdminService) UpdateSchoolSupervisor(ctx context.Context, schoolID int, supervisorUserID *int) (SchoolListItem, error) {
	if schoolID <= 0 {
		return SchoolListItem{}, errors.New("INVALID_SCHOOL_ID")
	}
	if supervisorUserID != nil {
		if *supervisorUserID <= 0 {
			return SchoolListItem{}, errors.New("INVALID_SUPERVISOR_ID")
		}
		var role string
		if err := s.DB.Pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, *supervisorUserID).Scan(&role); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return SchoolListItem{}, errors.New("SUPERVISOR_NOT_FOUND")
			}
			return SchoolListItem{}, err
		}
		if role != auth.RolePengawas {
			return SchoolListItem{}, errors.New("SUPERVISOR_ROLE_INVALID")
		}
	}

	row := s.DB.Pool.QueryRow(ctx, `
        WITH updated AS (
            UPDATE data_sekolah
            SET supervisor_user_id = $1,
                updated_at = NOW()
            WHERE id = $2
            RETURNING id, npsn_sekolah, nama_sekolah, supervisor_user_id
        )
        SELECT u.id,
               u.npsn_sekolah,
               u.nama_sekolah,
               u.supervisor_user_id,
               su.username AS supervisor_username,
               COALESCE(NULLIF(TRIM(su.nama_instansi), ''), su.username) AS supervisor_name
        FROM updated u
        LEFT JOIN users su ON su.id = u.supervisor_user_id
    `, supervisorUserID, schoolID)

	var record SchoolListItem
	if err := row.Scan(
		&record.ID,
		&record.NPSN,
		&record.NamaSekolah,
		&record.SupervisorUserID,
		&record.SupervisorUsername,
		&record.SupervisorName,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SchoolListItem{}, errors.New("SCHOOL_NOT_FOUND")
		}
		return SchoolListItem{}, err
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

func (s AdminService) UpdateCabdinAdmin(ctx context.Context, userID int, req CabdinAdminUpdate) (CabdinAdminAccount, error) {
	if userID <= 0 {
		return CabdinAdminAccount{}, errors.New("INVALID_USER_ID")
	}
	if req.Name == nil && req.Username == nil && req.Levels == nil {
		return CabdinAdminAccount{}, errors.New("INVALID_PAYLOAD")
	}

	var normalizedName *string
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return CabdinAdminAccount{}, errors.New("NAME_REQUIRED")
		}
		normalizedName = &name
	}

	var normalizedUsername *string
	if req.Username != nil {
		email, err := normalizeCabdinAdminEmail(*req.Username)
		if err != nil {
			return CabdinAdminAccount{}, err
		}
		normalizedUsername = &email
	}

	var normalizedLevels *[]string
	if req.Levels != nil {
		levels, err := normalizeCabdinLevels(*req.Levels)
		if err != nil {
			return CabdinAdminAccount{}, err
		}
		normalizedLevels = &levels
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

	if normalizedUsername != nil {
		var exists bool
		if err := s.DB.Pool.QueryRow(ctx, `
            SELECT EXISTS(
                SELECT 1
                FROM users
                WHERE username = $1
                  AND id <> $2
            )
        `, *normalizedUsername, userID).Scan(&exists); err != nil {
			return CabdinAdminAccount{}, err
		}
		if exists {
			return CabdinAdminAccount{}, errors.New("EMAIL_EXISTS")
		}
	}

	setParts := make([]string, 0, 4)
	args := make([]interface{}, 0, 4)
	argPos := 1

	if normalizedUsername != nil {
		setParts = append(setParts, "username = $"+strconv.Itoa(argPos))
		args = append(args, *normalizedUsername)
		argPos++
	}
	if normalizedName != nil {
		setParts = append(setParts, "nama_instansi = $"+strconv.Itoa(argPos))
		args = append(args, *normalizedName)
		argPos++
	}
	if normalizedLevels != nil {
		setParts = append(setParts, "allowed_levels = $"+strconv.Itoa(argPos))
		args = append(args, *normalizedLevels)
		argPos++
	}
	setParts = append(setParts, "updated_at = NOW()")

	query := `
        UPDATE users
        SET ` + strings.Join(setParts, ", ") + `
        WHERE id = $` + strconv.Itoa(argPos) + `
        RETURNING id,
                  username,
                  COALESCE(NULLIF(TRIM(nama_instansi), ''), username) AS name,
                  COALESCE(allowed_levels, '{}'::text[]) AS allowed_levels,
                  created_at
    `
	args = append(args, userID)

	row := s.DB.Pool.QueryRow(ctx, query, args...)
	var record CabdinAdminAccount
	if err := row.Scan(&record.ID, &record.Username, &record.Name, &record.Levels, &record.CreatedAt); err != nil {
		if isCabdinAdminUniqueViolation(err) {
			return CabdinAdminAccount{}, errors.New("EMAIL_EXISTS")
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return CabdinAdminAccount{}, errors.New("USER_NOT_FOUND")
		}
		return CabdinAdminAccount{}, err
	}
	return record, nil
}

func (s AdminService) ResetCabdinAdminPassword(ctx context.Context, userID int, newPassword string) error {
	if userID <= 0 {
		return errors.New("INVALID_USER_ID")
	}
	password := strings.TrimSpace(newPassword)
	if len(password) < 8 {
		return errors.New("PASSWORD_MIN_8")
	}

	var role string
	if err := s.DB.Pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("USER_NOT_FOUND")
		}
		return err
	}
	if role != "ADMIN" {
		return errors.New("USER_NOT_CABDIN")
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return err
	}

	commandTag, err := s.DB.Pool.Exec(ctx, `
        UPDATE users
        SET password_hash = $1,
            updated_at = NOW()
        WHERE id = $2
    `, hash, userID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return errors.New("USER_NOT_FOUND")
	}
	return nil
}

func (s AdminService) ListSchools(ctx context.Context, allowedLevels []string) ([]SchoolListItem, error) {
	args := []interface{}{}
	query := `
        SELECT ds.id,
               ds.npsn_sekolah,
               ds.nama_sekolah,
               ds.supervisor_user_id,
               u.username AS supervisor_username,
               COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS supervisor_name
        FROM data_sekolah ds
        LEFT JOIN users u ON u.id = ds.supervisor_user_id
        WHERE 1=1
    `
	if allowedLevels != nil {
		if len(allowedLevels) == 0 {
			query += " AND 1=0"
		} else {
			query += " AND COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)), '') = ANY($1)"
			args = append(args, allowedLevels)
		}
	}
	query += " ORDER BY ds.nama_sekolah ASC"
	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SchoolListItem
	for rows.Next() {
		var item SchoolListItem
		if err := rows.Scan(
			&item.ID,
			&item.NPSN,
			&item.NamaSekolah,
			&item.SupervisorUserID,
			&item.SupervisorUsername,
			&item.SupervisorName,
		); err == nil {
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
	if len(filters.ScopeSchoolIDs) > 0 {
		query += " AND id = ANY($" + strconv.Itoa(idx) + ")"
		args = append(args, filters.ScopeSchoolIDs)
		idx++
	} else if !filters.ScopeAll {
		query += " AND 1=0"
	} else if filters.SchoolID != nil {
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
	if len(allowedLevels) > 0 {
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
               COALESCE(NULLIF(TRIM(su.nama_instansi), ''), su.username) AS supervisor_name,
               su.username AS supervisor_username,
               (u.last_seen_at IS NOT NULL AND u.last_seen_at >= ($1::timestamp - interval '10 minutes')) AS is_online,
               ` + initialPasswordExpr + `
        FROM users u
        LEFT JOIN data_sekolah ds ON ds.id = u.school_id
        LEFT JOIN users su ON su.id = ds.supervisor_user_id
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
			&item.SupervisorName,
			&item.SupervisorUsername,
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
	bulkGenerateReasonAlreadyHasAccount           = "ALREADY_HAS_ACCOUNT"
	bulkGenerateReasonInvalidSchoolName           = "INVALID_SCHOOL_NAME"
	bulkGenerateReasonUsernameCollisionResolved   = "USERNAME_COLLISION_RESOLVED"
	bulkGenerateReasonUsernameCollisionUnresolved = "USERNAME_COLLISION_UNRESOLVED"
	bulkGenerateReasonQueryFailed                 = "QUERY_FAILED"
	bulkGenerateReasonInsertFailed                = "INSERT_FAILED"
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
            ` + cipherExpr + `,
            ` + nonceExpr + `,
            ` + legacyExpr + `
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
	isGaspolSchoolEntry := false
	if err := s.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM gaspol_user_roles
			WHERE user_id = $1
			  AND role = 'SCHOOL_ENTRY'
		)
	`, userID).Scan(&isGaspolSchoolEntry); err != nil {
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

	if !isGaspolSchoolEntry && support.V2 {
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
	} else if !isGaspolSchoolEntry && support.V1 {
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
	TahunLulus    *int
	ResetPrestasi bool
	ResetSiswa    bool
	Override      bool
}

type ResetDataDeletedCounts struct {
	PrestasiSiswa          int64 `json:"prestasi_siswa"`
	Class12Students        int64 `json:"class12_students"`
	StudentPlans           int64 `json:"student_plans"`
	StudentOutcomes        int64 `json:"student_outcomes"`
	OutcomePTDetails       int64 `json:"outcome_pt_details"`
	OutcomeWorkDetails     int64 `json:"outcome_work_details"`
	OutcomeBusinessDetails int64 `json:"outcome_business_details"`
	Applications           int64 `json:"applications"`
	AcceptanceOffers       int64 `json:"acceptance_offers"`
	Wirausaha              int64 `json:"wirausaha"`
	StudentCohorts         int64 `json:"student_cohorts"`
	StudentFinalOutcomes   int64 `json:"student_final_outcomes"`
}

type ResetDataResult struct {
	SchoolID   int                    `json:"school_id"`
	TahunLulus *int                   `json:"tahun_lulus"`
	Deleted    ResetDataDeletedCounts `json:"deleted"`
}

func (s AdminService) tableExistsCached(ctx context.Context, cache map[string]bool, table string) (bool, error) {
	if value, ok := cache[table]; ok {
		return value, nil
	}
	exists, err := tableExists(ctx, s.DB, table)
	if err != nil {
		return false, err
	}
	cache[table] = exists
	return exists, nil
}

func deleteBySchoolAndYear(ctx context.Context, tx pgx.Tx, table string, schoolID int, tahunLulus *int) (int64, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE school_id = $1", table)
	args := []interface{}{schoolID}
	if tahunLulus != nil {
		query += " AND tahun_lulus = $2"
		args = append(args, *tahunLulus)
	}
	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func deleteLegacyByCohortScope(ctx context.Context, tx pgx.Tx, table string, schoolID int, tahunLulus *int) (int64, error) {
	var (
		query string
		args  []interface{}
	)
	if tahunLulus != nil {
		query = fmt.Sprintf(`
			DELETE FROM %s
			WHERE school_id = $1
			  AND cohort_id IN (
				SELECT id
				FROM student_cohorts
				WHERE school_id = $1
				  AND tahun_lulus = $2
			  )
		`, table)
		args = []interface{}{schoolID, *tahunLulus}
	} else {
		query = fmt.Sprintf(`
			DELETE FROM %s
			WHERE school_id = $1
			  AND cohort_id IN (
				SELECT id
				FROM student_cohorts
				WHERE school_id = $1
			  )
		`, table)
		args = []interface{}{schoolID}
	}
	tag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s AdminService) ResetSchoolData(ctx context.Context, schoolID int, options ResetDataOptions) (ResetDataResult, error) {
	result := ResetDataResult{
		SchoolID:   schoolID,
		TahunLulus: options.TahunLulus,
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	tableCache := map[string]bool{}

	if options.ResetPrestasi {
		hasPrestasi, existsErr := s.tableExistsCached(ctx, tableCache, "prestasi_siswa")
		if existsErr != nil {
			return result, existsErr
		}
		if hasPrestasi {
			deleted, deleteErr := deleteBySchoolAndYear(ctx, tx, "prestasi_siswa", schoolID, nil)
			if deleteErr != nil {
				return result, deleteErr
			}
			result.Deleted.PrestasiSiswa = deleted
		}
		if _, execErr := tx.Exec(ctx, `UPDATE data_sekolah SET last_update_prestasi = NULL WHERE id = $1`, schoolID); execErr != nil {
			return result, execErr
		}
		hasUploads, existsErr := s.tableExistsCached(ctx, tableCache, "uploads")
		if existsErr != nil {
			return result, existsErr
		}
		if hasUploads {
			if _, execErr := tx.Exec(ctx, `
				UPDATE uploads
				SET deleted_at = NOW(),
				    updated_at = NOW()
				WHERE school_id = $1
				  AND type = 'ACHIEVEMENT'
				  AND deleted_at IS NULL
			`, schoolID); execErr != nil {
				return result, execErr
			}
		}
	}

	if options.ResetSiswa {
		type scopedTable struct {
			name  string
			store *int64
		}
		newSchemaTables := []scopedTable{
			{name: "outcome_pt_details", store: &result.Deleted.OutcomePTDetails},
			{name: "outcome_work_details", store: &result.Deleted.OutcomeWorkDetails},
			{name: "outcome_business_details", store: &result.Deleted.OutcomeBusinessDetails},
			{name: "student_outcomes", store: &result.Deleted.StudentOutcomes},
			{name: "student_plans", store: &result.Deleted.StudentPlans},
			{name: "class12_students", store: &result.Deleted.Class12Students},
		}
		for _, item := range newSchemaTables {
			hasTable, existsErr := s.tableExistsCached(ctx, tableCache, item.name)
			if existsErr != nil {
				return result, existsErr
			}
			if !hasTable {
				continue
			}
			deleted, deleteErr := deleteBySchoolAndYear(ctx, tx, item.name, schoolID, options.TahunLulus)
			if deleteErr != nil {
				return result, deleteErr
			}
			*item.store = deleted
		}

		hasCohorts, existsErr := s.tableExistsCached(ctx, tableCache, "student_cohorts")
		if existsErr != nil {
			return result, existsErr
		}
		legacyTables := []struct {
			name  string
			store *int64
		}{
			{name: "student_higher_ed_applications", store: &result.Deleted.Applications},
			{name: "student_higher_ed_acceptance_offers", store: &result.Deleted.AcceptanceOffers},
			{name: "student_entrepreneur_plans", store: &result.Deleted.Wirausaha},
			{name: "student_final_outcomes", store: &result.Deleted.StudentFinalOutcomes},
		}
		if hasCohorts {
			for _, item := range legacyTables {
				hasTable, tableErr := s.tableExistsCached(ctx, tableCache, item.name)
				if tableErr != nil {
					return result, tableErr
				}
				if !hasTable {
					continue
				}
				deleted, deleteErr := deleteLegacyByCohortScope(ctx, tx, item.name, schoolID, options.TahunLulus)
				if deleteErr != nil {
					return result, deleteErr
				}
				*item.store = deleted
			}

			deletedCohorts, deleteErr := deleteBySchoolAndYear(ctx, tx, "student_cohorts", schoolID, options.TahunLulus)
			if deleteErr != nil {
				return result, deleteErr
			}
			result.Deleted.StudentCohorts = deletedCohorts
		} else if options.TahunLulus == nil {
			for _, item := range legacyTables {
				hasTable, tableErr := s.tableExistsCached(ctx, tableCache, item.name)
				if tableErr != nil {
					return result, tableErr
				}
				if !hasTable {
					continue
				}
				deleted, deleteErr := deleteBySchoolAndYear(ctx, tx, item.name, schoolID, nil)
				if deleteErr != nil {
					return result, deleteErr
				}
				*item.store = deleted
			}
		}

		hasUploads, existsErr := s.tableExistsCached(ctx, tableCache, "uploads")
		if existsErr != nil {
			return result, existsErr
		}
		if hasUploads {
			if _, execErr := tx.Exec(ctx, `
				UPDATE uploads
				SET deleted_at = NOW(),
				    updated_at = NOW()
				WHERE school_id = $1
				  AND type IN ('COHORTS','APPLICATIONS','ACCEPTANCE')
				  AND deleted_at IS NULL
			`, schoolID); execErr != nil {
				return result, execErr
			}
		}

		if _, execErr := tx.Exec(ctx, `UPDATE data_sekolah SET last_update_siswa_pt = NULL WHERE id = $1`, schoolID); execErr != nil {
			return result, execErr
		}
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
