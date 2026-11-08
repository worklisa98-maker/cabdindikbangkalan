package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

type GaspolRepository struct {
	DB *db.DB
}

type gaspolAssessmentValidationMetadata struct {
	ValidationDecision string   `json:"validationDecision,omitempty"`
	SupervisorScore    *float64 `json:"supervisorScore,omitempty"`
	ValidatorScore     *float64 `json:"validatorScore,omitempty"`
	FinalMonthlyScore  *float64 `json:"finalMonthlyScore,omitempty"`
	ValidatorNotes     string   `json:"validatorNotes,omitempty"`
}

func (r GaspolRepository) EnsureYearActive(ctx context.Context, year int, actorID int) error {
	_, err := r.DB.Pool.Exec(ctx, `
		INSERT INTO gaspol_school_participants (
			year,
			school_id,
			is_year_configuration,
			is_active,
			created_by
		)
		VALUES ($1, NULL, TRUE, TRUE, $2)
		ON CONFLICT ((year)) WHERE is_year_configuration = TRUE
		DO UPDATE SET
			is_active = TRUE,
			created_by = EXCLUDED.created_by,
			updated_at = NOW()
	`, year, actorID)
	return err
}

func (r GaspolRepository) IsYearActive(ctx context.Context, year int) (bool, error) {
	var active bool
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM gaspol_school_participants
			WHERE year = $1
			  AND is_year_configuration = TRUE
			  AND is_active = TRUE
		)
	`, year).Scan(&active)
	return active, err
}

func (r GaspolRepository) UpsertParticipants(ctx context.Context, year int, schoolIDs []int, actorID int) error {
	tx, err := r.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		INSERT INTO gaspol_school_participants (
			year,
			school_id,
			is_year_configuration,
			is_active,
			created_by
		)
		VALUES ($1, NULL, TRUE, TRUE, $2)
		ON CONFLICT ((year)) WHERE is_year_configuration = TRUE
		DO UPDATE SET
			is_active = TRUE,
			created_by = EXCLUDED.created_by,
			updated_at = NOW()
	`, year, actorID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE gaspol_school_participants
		SET is_active = FALSE,
			updated_at = NOW()
		WHERE year = $1
		  AND is_year_configuration = FALSE
	`, year); err != nil {
		return err
	}

	if len(schoolIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO gaspol_school_participants (
				year,
				school_id,
				is_year_configuration,
				is_active,
				created_by
			)
			SELECT $1, school_id, FALSE, TRUE, $2
			FROM unnest($3::int[]) AS school_id
			ON CONFLICT (year, school_id) WHERE is_year_configuration = FALSE
			DO UPDATE SET
				is_active = TRUE,
				created_by = EXCLUDED.created_by,
				updated_at = NOW()
		`, year, actorID, schoolIDs); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r GaspolRepository) ListParticipants(ctx context.Context, year int) ([]GaspolSchoolParticipant, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT p.id,
		       p.year,
		       p.school_id,
		       COALESCE(ds.npsn_sekolah, '') AS npsn,
		       COALESCE(ds.nama_sekolah, '') AS school_name,
		       COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)), '') AS school_level,
		       COALESCE(UPPER(TRIM(ds.status_sekolah)), '') AS school_status,
		       p.is_year_configuration,
		       p.is_active,
		       p.validator_user_id,
		       COALESCE(NULLIF(TRIM(v.nama_instansi), ''), v.username, '') AS validator_name,
		       p.school_entry_user_id,
		       COALESCE(NULLIF(TRIM(se.nama_instansi), ''), se.username, '') AS school_entry_name,
		       COALESCE(p.notes, ''),
		       p.created_at,
		       p.updated_at
		FROM gaspol_school_participants p
		LEFT JOIN data_sekolah ds ON ds.id = p.school_id
		LEFT JOIN users v ON v.id = p.validator_user_id
		LEFT JOIN users se ON se.id = p.school_entry_user_id
		WHERE p.year = $1
		  AND p.is_year_configuration = FALSE
		  AND p.is_active = TRUE
		ORDER BY ds.nama_sekolah ASC, p.id ASC
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolSchoolParticipant, 0)
	for rows.Next() {
		var item GaspolSchoolParticipant
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.SchoolID,
			&item.NPSN,
			&item.SchoolName,
			&item.SchoolLevel,
			&item.SchoolStatus,
			&item.IsYearConfig,
			&item.IsActive,
			&item.ValidatorUserID,
			&item.ValidatorName,
			&item.SchoolEntryUserID,
			&item.SchoolEntryName,
			&item.Notes,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) GetParticipantBySchool(ctx context.Context, year int, schoolID int) (GaspolSchoolParticipant, error) {
	var item GaspolSchoolParticipant
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT p.id,
		       p.year,
		       p.school_id,
		       COALESCE(ds.npsn_sekolah, '') AS npsn,
		       COALESCE(ds.nama_sekolah, '') AS school_name,
		       COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)), '') AS school_level,
		       COALESCE(UPPER(TRIM(ds.status_sekolah)), '') AS school_status,
		       p.is_year_configuration,
		       p.is_active,
		       p.validator_user_id,
		       COALESCE(NULLIF(TRIM(v.nama_instansi), ''), v.username, '') AS validator_name,
		       p.school_entry_user_id,
		       COALESCE(NULLIF(TRIM(se.nama_instansi), ''), se.username, '') AS school_entry_name,
		       COALESCE(p.notes, ''),
		       p.created_at,
		       p.updated_at
		FROM gaspol_school_participants p
		LEFT JOIN data_sekolah ds ON ds.id = p.school_id
		LEFT JOIN users v ON v.id = p.validator_user_id
		LEFT JOIN users se ON se.id = p.school_entry_user_id
		WHERE p.year = $1
		  AND p.school_id = $2
		  AND p.is_year_configuration = FALSE
		  AND p.is_active = TRUE
	`, year, schoolID).Scan(
		&item.ID,
		&item.Year,
		&item.SchoolID,
		&item.NPSN,
		&item.SchoolName,
		&item.SchoolLevel,
		&item.SchoolStatus,
		&item.IsYearConfig,
		&item.IsActive,
		&item.ValidatorUserID,
		&item.ValidatorName,
		&item.SchoolEntryUserID,
		&item.SchoolEntryName,
		&item.Notes,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func (r GaspolRepository) ListAllSchoolsWithParticipantState(ctx context.Context, year int) ([]GaspolSchoolParticipant, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT COALESCE(p.id, 0) AS id,
		       $1 AS year,
		       ds.id AS school_id,
		       COALESCE(ds.npsn_sekolah, '') AS npsn,
		       ds.nama_sekolah,
		       COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)), '') AS school_level,
		       COALESCE(UPPER(TRIM(ds.status_sekolah)), '') AS school_status,
		       FALSE AS is_year_configuration,
		       COALESCE(p.is_active, FALSE) AS is_active,
		       p.validator_user_id,
		       COALESCE(NULLIF(TRIM(v.nama_instansi), ''), v.username, '') AS validator_name,
		       p.school_entry_user_id,
		       COALESCE(NULLIF(TRIM(se.nama_instansi), ''), se.username, '') AS school_entry_name,
		       COALESCE(p.notes, '') AS notes,
		       p.created_at,
		       p.updated_at
		FROM data_sekolah ds
		LEFT JOIN gaspol_school_participants p
			ON p.school_id = ds.id
		   AND p.year = $1
		   AND p.is_year_configuration = FALSE
		LEFT JOIN users v ON v.id = p.validator_user_id
		LEFT JOIN users se ON se.id = p.school_entry_user_id
		ORDER BY ds.nama_sekolah ASC
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolSchoolParticipant, 0)
	for rows.Next() {
		var item GaspolSchoolParticipant
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.SchoolID,
			&item.NPSN,
			&item.SchoolName,
			&item.SchoolLevel,
			&item.SchoolStatus,
			&item.IsYearConfig,
			&item.IsActive,
			&item.ValidatorUserID,
			&item.ValidatorName,
			&item.SchoolEntryUserID,
			&item.SchoolEntryName,
			&item.Notes,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) CreateGaspolUser(ctx context.Context, username string, passwordHash string, baseRole string, schoolID *int, displayName string) (int, error) {
	var userID int
	err := r.DB.Pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, role, school_id, nama_instansi)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, username, passwordHash, baseRole, schoolID, displayName).Scan(&userID)
	return userID, err
}

func buildCreateGaspolSchoolUserStatement(username string, passwordHash string, baseRole string, schoolID *int, displayName string, initialPassword gaspolInitialPasswordRecord) (string, []interface{}) {
	columns := []string{"username", "password_hash", "role", "school_id", "nama_instansi"}
	values := []string{"$1", "$2", "$3", "$4", "$5"}
	args := []interface{}{username, passwordHash, baseRole, schoolID, displayName}
	nextArg := 6

	if initialPassword.Ciphertext != nil && initialPassword.Nonce != nil {
		columns = append(columns, "initial_password_ciphertext", "initial_password_nonce", "initial_password_set_at")
		values = append(values, fmt.Sprintf("$%d", nextArg), fmt.Sprintf("$%d", nextArg+1), "NOW()")
		args = append(args, *initialPassword.Ciphertext, *initialPassword.Nonce)
		nextArg += 2
	} else if initialPassword.Encrypted != nil {
		columns = append(columns, "initial_password_encrypted")
		values = append(values, fmt.Sprintf("$%d", nextArg))
		args = append(args, *initialPassword.Encrypted)
	}

	query := fmt.Sprintf(`
		INSERT INTO users (%s)
		VALUES (%s)
		RETURNING id
	`, strings.Join(columns, ", "), strings.Join(values, ", "))
	return query, args
}

func (r GaspolRepository) CreateGaspolSchoolUser(ctx context.Context, username string, passwordHash string, baseRole string, schoolID *int, displayName string, initialPassword gaspolInitialPasswordRecord) (int, error) {
	query, args := buildCreateGaspolSchoolUserStatement(username, passwordHash, baseRole, schoolID, displayName, initialPassword)

	var userID int
	err := r.DB.Pool.QueryRow(ctx, query, args...).Scan(&userID)
	return userID, err
}

func (r GaspolRepository) GetUser(ctx context.Context, userID int) (GaspolGeneratedCredential, error) {
	var result GaspolGeneratedCredential
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT u.id,
		       u.username,
		       u.role,
		       u.school_id,
		       COALESCE(ds.nama_sekolah, '') AS school_name,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS display_name
		FROM users u
		LEFT JOIN data_sekolah ds ON ds.id = u.school_id
		WHERE u.id = $1
	`, userID).Scan(
		&result.UserID,
		&result.Username,
		&result.Role,
		&result.SchoolID,
		&result.SchoolName,
		&result.DisplayName,
	)
	return result, err
}

func (r GaspolRepository) ResetUserPassword(ctx context.Context, userID int, passwordHash string) error {
	tag, err := r.DB.Pool.Exec(ctx, `
		UPDATE users
		SET password_hash = $1,
		    token_version = token_version + 1,
		    password_updated_at = NOW(),
		    updated_at = NOW()
		WHERE id = $2
	`, passwordHash, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r GaspolRepository) SetParticipantSchoolEntryUser(ctx context.Context, year int, schoolID int, userID int) error {
	tag, err := r.DB.Pool.Exec(ctx, `
		UPDATE gaspol_school_participants
		SET school_entry_user_id = $1,
		    updated_at = NOW()
		WHERE year = $2
		  AND school_id = $3
		  AND is_year_configuration = FALSE
	`, userID, year, schoolID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r GaspolRepository) SetParticipantValidator(ctx context.Context, year int, schoolID int, userID int) error {
	tag, err := r.DB.Pool.Exec(ctx, `
		UPDATE gaspol_school_participants
		SET validator_user_id = $1,
		    updated_at = NOW()
		WHERE year = $2
		  AND school_id = $3
		  AND is_year_configuration = FALSE
	`, userID, year, schoolID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r GaspolRepository) SetAllParticipantValidators(ctx context.Context, year int, userID int) (int, error) {
	tag, err := r.DB.Pool.Exec(ctx, `
		UPDATE gaspol_school_participants
		SET validator_user_id = $1,
		    updated_at = NOW()
		WHERE year = $2
		  AND is_year_configuration = FALSE
		  AND is_active = TRUE
	`, userID, year)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func buildUpsertUserRoleStatement(year int, userID int, role string, schoolID *int, actorID int) (string, []interface{}) {
	if schoolID != nil {
		return `
			INSERT INTO gaspol_user_roles (year, user_id, role, school_id, is_active, assigned_by)
			VALUES ($1::integer, $2::integer, $3::gaspol_role_type, $4::integer, TRUE, $5::integer)
			ON CONFLICT (year, user_id, role, school_id) WHERE school_id IS NOT NULL
			DO UPDATE SET
				is_active = TRUE,
				assigned_by = EXCLUDED.assigned_by,
				updated_at = NOW()
			RETURNING id, year, user_id, role, school_id, is_active
		`, []interface{}{year, userID, role, *schoolID, actorID}
	}

	return `
		INSERT INTO gaspol_user_roles (year, user_id, role, school_id, is_active, assigned_by)
		VALUES ($1::integer, $2::integer, $3::gaspol_role_type, NULL, TRUE, $4::integer)
		ON CONFLICT (year, user_id, role) WHERE school_id IS NULL
		DO UPDATE SET
			is_active = TRUE,
			assigned_by = EXCLUDED.assigned_by,
			updated_at = NOW()
		RETURNING id, year, user_id, role, school_id, is_active
	`, []interface{}{year, userID, role, actorID}
}

func (r GaspolRepository) UpsertUserRole(ctx context.Context, year int, userID int, role string, schoolID *int, actorID int) (GaspolUserRoleAssignment, error) {
	query, args := buildUpsertUserRoleStatement(year, userID, role, schoolID, actorID)

	var item GaspolUserRoleAssignment
	if err := r.DB.Pool.QueryRow(ctx, query, args...).Scan(
		&item.ID,
		&item.Year,
		&item.UserID,
		&item.Role,
		&item.SchoolID,
		&item.IsActive,
	); err != nil {
		return GaspolUserRoleAssignment{}, err
	}

	meta, err := r.GetUserMeta(ctx, userID)
	if err != nil {
		return GaspolUserRoleAssignment{}, err
	}
	item.Username = meta.Username
	item.DisplayName = meta.DisplayName
	item.BaseRole = meta.BaseRole
	return item, nil
}

func (r GaspolRepository) UserHasRole(ctx context.Context, year int, userID int, role string, schoolID *int) (bool, error) {
	if schoolID != nil {
		var exists bool
		err := r.DB.Pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM gaspol_user_roles
				WHERE year = $1::integer
				  AND user_id = $2::integer
				  AND role = $3::gaspol_role_type
				  AND school_id = $4::integer
				  AND is_active = TRUE
			)
		`, year, userID, role, *schoolID).Scan(&exists)
		return exists, err
	}

	var exists bool
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM gaspol_user_roles
			WHERE year = $1::integer
			  AND user_id = $2::integer
			  AND role = $3::gaspol_role_type
			  AND school_id IS NULL
			  AND is_active = TRUE
		)
	`, year, userID, role).Scan(&exists)
	return exists, err
}

func (r GaspolRepository) UserHasAnyGaspolRole(ctx context.Context, userID int) (bool, error) {
	var exists bool
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM gaspol_user_roles
			WHERE user_id = $1
		)
	`, userID).Scan(&exists)
	return exists, err
}

func (r GaspolRepository) UserHasAnyRole(ctx context.Context, userID int, roles []string) (bool, error) {
	if len(roles) == 0 {
		return false, nil
	}
	var exists bool
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM gaspol_user_roles
			WHERE user_id = $1
			  AND role = ANY($2)
			  AND is_active = TRUE
		)
	`, userID, roles).Scan(&exists)
	return exists, err
}

type gaspolUserMeta struct {
	Username    string
	DisplayName string
	BaseRole    string
	SchoolID    *int
}

func (r GaspolRepository) GetUserMeta(ctx context.Context, userID int) (gaspolUserMeta, error) {
	var meta gaspolUserMeta
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT username,
		       COALESCE(NULLIF(TRIM(nama_instansi), ''), username) AS display_name,
		       role,
		       school_id
		FROM users
		WHERE id = $1
	`, userID).Scan(&meta.Username, &meta.DisplayName, &meta.BaseRole, &meta.SchoolID)
	return meta, err
}

func (r GaspolRepository) ListRoleAssignments(ctx context.Context, year int, role string) ([]GaspolUserRoleAssignment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT gur.id,
		       gur.year,
		       gur.user_id,
		       gur.role,
		       gur.school_id,
		       gur.is_active,
		       u.username,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS display_name,
		       u.role AS base_role
		FROM gaspol_user_roles gur
		JOIN users u ON u.id = gur.user_id
		WHERE gur.year = $1::integer
		  AND gur.role = $2::gaspol_role_type
		  AND gur.is_active = TRUE
		ORDER BY display_name ASC, gur.user_id ASC
	`, year, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolUserRoleAssignment, 0)
	for rows.Next() {
		var item GaspolUserRoleAssignment
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.UserID,
			&item.Role,
			&item.SchoolID,
			&item.IsActive,
			&item.Username,
			&item.DisplayName,
			&item.BaseRole,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) SetUserRoleActive(ctx context.Context, year int, userID int, role string, isActive bool) error {
	tag, err := r.DB.Pool.Exec(ctx, `
		UPDATE gaspol_user_roles
		SET is_active = $4::boolean,
		    updated_at = NOW()
		WHERE year = $1::integer
		  AND user_id = $2::integer
		  AND role = $3::gaspol_role_type
		  AND (
			($3::gaspol_role_type = 'SCHOOL_ENTRY'::gaspol_role_type AND school_id IS NOT NULL)
			OR
			($3::gaspol_role_type <> 'SCHOOL_ENTRY'::gaspol_role_type AND school_id IS NULL)
		  )
	`, year, userID, role, isActive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r GaspolRepository) ListAssignableUsers(ctx context.Context, year int, role string) ([]GaspolRoleCandidate, error) {
	baseRoles := gaspolAssignableBaseRoles(role)
	if len(baseRoles) == 0 {
		return []GaspolRoleCandidate{}, nil
	}
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT gur.id,
		       u.id,
		       u.username,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS display_name,
		       u.role AS base_role,
		       COALESCE(gur.is_active, FALSE) AS role_is_active
		FROM users u
		LEFT JOIN gaspol_user_roles gur
		  ON gur.year = $1::integer
		 AND gur.user_id = u.id
		 AND gur.role = $2::gaspol_role_type
		 AND gur.school_id IS NULL
		WHERE u.school_id IS NULL
		  AND u.role::text = ANY($3::text[])
		ORDER BY display_name ASC, u.id ASC
	`, year, role, baseRoles)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGaspolRoleCandidates(rows)
}

func (r GaspolRepository) ListAssignedRoleUsers(ctx context.Context, year int, role string) ([]GaspolRoleCandidate, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT gur.id,
		       u.id,
		       u.username,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), COALESCE(ds.nama_sekolah, ''), u.username) AS display_name,
		       u.role AS base_role,
		       gur.is_active
		FROM gaspol_user_roles gur
		JOIN users u ON u.id = gur.user_id
		LEFT JOIN data_sekolah ds ON ds.id = u.school_id
		WHERE gur.year = $1::integer
		  AND gur.role = $2::gaspol_role_type
		  AND gur.school_id IS NULL
		ORDER BY display_name ASC, u.id ASC
	`, year, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGaspolRoleCandidates(rows)
}

func (r GaspolRepository) ListStageAssignments(ctx context.Context, year int, assignmentRole string) ([]GaspolStageAssignment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT a.id,a.year,a.stage,a.school_id,a.assessor_user_id,
		       COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username),u.username,u.role::text,
		       a.assignment_role,a.is_active,a.updated_at,
		       COALESCE(ds.nama_sekolah,''),COALESCE(ds.npsn_sekolah,'')
		FROM gaspol_stage_assignments a
		JOIN users u ON u.id=a.assessor_user_id
		LEFT JOIN data_sekolah ds ON ds.id=a.school_id
		WHERE a.year=$1 AND ($2='' OR a.assignment_role=$2)
		ORDER BY a.is_active DESC,a.updated_at DESC,a.id DESC
	`, year, assignmentRole)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []GaspolStageAssignment{}
	for rows.Next() {
		var item GaspolStageAssignment
		if err = rows.Scan(&item.ID, &item.Year, &item.Stage, &item.SchoolID, &item.AssessorUserID, &item.AssessorName, &item.Username, &item.BaseRole, &item.AssignmentRole, &item.IsActive, &item.UpdatedAt, &item.SchoolName, &item.NPSN); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListStageSchoolCandidates(ctx context.Context, year int) ([]GaspolAssignmentSchoolCandidate, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT p.school_id,COALESCE(ds.npsn_sekolah,''),ds.nama_sekolah,
		       COALESCE(array_agg(DISTINCT c.cohort) FILTER (WHERE c.cohort IS NOT NULL AND c.released_at IS NULL),'{}'::text[])
		FROM gaspol_school_participants p
		JOIN data_sekolah ds ON ds.id=p.school_id
		LEFT JOIN gaspol_cohorts c ON c.year=p.year AND c.school_id=p.school_id
		WHERE p.year=$1 AND p.is_active=TRUE AND p.is_year_configuration=FALSE
		GROUP BY p.school_id,ds.npsn_sekolah,ds.nama_sekolah
		ORDER BY ds.nama_sekolah
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []GaspolAssignmentSchoolCandidate{}
	for rows.Next() {
		var item GaspolAssignmentSchoolCandidate
		if err = rows.Scan(&item.SchoolID, &item.NPSN, &item.SchoolName, &item.Cohorts); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListInternalStageCandidates(ctx context.Context, year int) ([]GaspolRoleCandidate, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT u.id,u.username,COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username),u.role::text,
		       EXISTS(SELECT 1 FROM gaspol_user_roles gur WHERE gur.year=$1 AND gur.user_id=u.id AND gur.role IN ('SUPERVISOR','JURY') AND gur.is_active=TRUE)
		FROM users u
		WHERE u.school_id IS NULL AND (
			u.role::text IN ('ADMIN','ADMIN_CABDIN','ADMIN_MASTER','PENGAWAS') OR
			EXISTS(SELECT 1 FROM gaspol_user_roles gur WHERE gur.year=$1 AND gur.user_id=u.id AND gur.role IN ('SUPERVISOR','JURY') AND gur.is_active=TRUE)
		)
		ORDER BY 3,u.id
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []GaspolRoleCandidate{}
	for rows.Next() {
		var item GaspolRoleCandidate
		var hasGaspolRole bool
		if err = rows.Scan(&item.UserID, &item.Username, &item.DisplayName, &item.BaseRole, &hasGaspolRole); err != nil {
			return nil, err
		}
		item.IsActive = hasGaspolRole || auth.NormalizeRole(item.BaseRole) == auth.RoleAdminCabdin || auth.NormalizeRole(item.BaseRole) == auth.RoleAdminMaster || auth.NormalizeRole(item.BaseRole) == auth.RolePengawas
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListStageTasksByAssessor(ctx context.Context, year, userID int) ([]GaspolStageTask, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT a.id,a.stage,a.assignment_role,a.school_id,ds.nama_sekolah,COALESCE(ds.npsn_sekolah,''),
		       sa.id,sa.raw_score,COALESCE(sa.workflow_status,''),COALESCE(sa.notes,''),
		       CASE WHEN sa.workflow_status='RETURNED_BY_MASTER' THEN COALESCE(sa.master_return_notes,'')
		            WHEN sa.workflow_status='RETURNED_BY_KOOR' THEN COALESCE(sa.koor_notes,'')
		            WHEN sa.workflow_status='RETURNED_BY_CABDIN' THEN COALESCE(sa.cabdin_notes,'') ELSE '' END
		FROM gaspol_stage_assignments a
		JOIN data_sekolah ds ON ds.id=a.school_id
		LEFT JOIN LATERAL (
			SELECT x.id,x.raw_score,x.workflow_status,x.notes,
			       x.koor_notes,x.cabdin_notes,x.master_return_notes
			FROM gaspol_stage_assessments x
			WHERE x.year=a.year AND x.stage=a.stage AND x.school_id=a.school_id AND x.assessor_user_id=a.assessor_user_id
			ORDER BY x.updated_at DESC LIMIT 1
		) sa ON TRUE
		WHERE a.year=$1 AND a.assessor_user_id=$2 AND a.is_active=TRUE AND a.school_id IS NOT NULL
		ORDER BY a.stage,ds.nama_sekolah
	`, year, userID)
	if err != nil {
		return nil, fmt.Errorf("ListStageTasksByAssessor query year=%d assessor_user_id=%d (if startup schema_check passed, inspect SQL aliases/projections): %w", year, userID, err)
	}
	defer rows.Close()
	result := []GaspolStageTask{}
	for rows.Next() {
		var item GaspolStageTask
		if err = rows.Scan(&item.AssignmentID, &item.Stage, &item.AssignmentRole, &item.SchoolID, &item.SchoolName, &item.NPSN, &item.AssessmentID, &item.RawScore, &item.WorkflowStatus, &item.Notes, &item.ReturnNotes); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanGaspolRoleCandidates(rows pgx.Rows) ([]GaspolRoleCandidate, error) {
	result := make([]GaspolRoleCandidate, 0)
	for rows.Next() {
		var item GaspolRoleCandidate
		if err := rows.Scan(
			&item.RoleAssignmentID,
			&item.UserID,
			&item.Username,
			&item.DisplayName,
			&item.BaseRole,
			&item.IsActive,
		); err != nil {
			return nil, err
		}
		item.IsAssigned = item.RoleAssignmentID != nil
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListSchoolUsers(ctx context.Context, year int, support InitialPasswordSupport) ([]GaspolSchoolUserRow, error) {
	cipherExpr := "NULL::text AS initial_password_ciphertext"
	nonceExpr := "NULL::text AS initial_password_nonce"
	legacyExpr := "NULL::text AS initial_password_encrypted"
	if support.V2 {
		cipherExpr = "se.initial_password_ciphertext"
		nonceExpr = "se.initial_password_nonce"
	}
	if support.V1 {
		legacyExpr = "se.initial_password_encrypted"
	}

	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ds.id,
		       COALESCE(ds.npsn_sekolah, '') AS npsn,
		       ds.nama_sekolah,
		       COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)), '') AS school_level,
		       COALESCE(UPPER(TRIM(ds.status_sekolah)), '') AS school_status,
		       COALESCE(p.is_active, FALSE) AS is_participant,
		       p.school_entry_user_id,
		       COALESCE(se.username, '') AS username,
		       COALESCE(NULLIF(TRIM(se.nama_instansi), ''), se.username, '') AS display_name,
		       COALESCE(gur.is_active, FALSE) AS role_is_active,
		       `+cipherExpr+`,
		       `+nonceExpr+`,
		       `+legacyExpr+`
		FROM data_sekolah ds
		LEFT JOIN gaspol_school_participants p
		  ON p.year = $1
		 AND p.school_id = ds.id
		 AND p.is_year_configuration = FALSE
		LEFT JOIN users se ON se.id = p.school_entry_user_id
		LEFT JOIN gaspol_user_roles gur
		  ON gur.year = $1
		 AND gur.user_id = p.school_entry_user_id
		 AND gur.role = 'SCHOOL_ENTRY'
		 AND gur.school_id = ds.id
		ORDER BY ds.nama_sekolah ASC, ds.id ASC
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolSchoolUserRow, 0)
	for rows.Next() {
		var item GaspolSchoolUserRow
		if err := rows.Scan(
			&item.SchoolID,
			&item.NPSN,
			&item.SchoolName,
			&item.SchoolLevel,
			&item.SchoolStatus,
			&item.IsParticipant,
			&item.UserID,
			&item.Username,
			&item.DisplayName,
			&item.RoleIsActive,
			&item.InitialPasswordCiphertext,
			&item.InitialPasswordNonce,
			&item.InitialPasswordEncrypted,
		); err != nil {
			return nil, err
		}
		if item.UserID != nil {
			item.AccountStatus = "Sudah tersedia"
		} else {
			item.AccountStatus = "Belum tersedia"
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListRandomAssignmentInputs(ctx context.Context, year int, month int) ([]GaspolAssignmentCandidate, []GaspolSupervisorCandidate, error) {
	schoolRows, err := r.DB.Pool.Query(ctx, `
		SELECT p.school_id,
		       ds.nama_sekolah,
		       prev.supervisor_user_id
		FROM gaspol_school_participants p
		JOIN data_sekolah ds ON ds.id = p.school_id
		LEFT JOIN gaspol_assignments prev
		       ON prev.year = $1
		      AND prev.month = $2 - 1
		      AND prev.school_id = p.school_id
		WHERE p.year = $1
		  AND p.is_year_configuration = FALSE
		  AND p.is_active = TRUE
		ORDER BY ds.nama_sekolah ASC
	`, year, month)
	if err != nil {
		return nil, nil, err
	}
	defer schoolRows.Close()

	schools := make([]GaspolAssignmentCandidate, 0)
	for schoolRows.Next() {
		var item GaspolAssignmentCandidate
		if err := schoolRows.Scan(&item.SchoolID, &item.SchoolName, &item.PreviousSupervisorID); err != nil {
			return nil, nil, err
		}
		schools = append(schools, item)
	}
	if err := schoolRows.Err(); err != nil {
		return nil, nil, err
	}

	supervisorRows, err := r.DB.Pool.Query(ctx, `
		SELECT gur.user_id,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS display_name,
		       0 AS current_load,
		       COALESCE(history.load_count, 0) AS historical_load,
		       COALESCE(previous.load_count, 0) AS previous_month_load
		FROM gaspol_user_roles gur
		JOIN users u ON u.id = gur.user_id
		LEFT JOIN (
			SELECT supervisor_user_id, COUNT(*) AS load_count
			FROM gaspol_assignments
			WHERE year = $1
			  AND month < $2
			  AND supervisor_user_id IS NOT NULL
			GROUP BY supervisor_user_id
		) history ON history.supervisor_user_id = gur.user_id
		LEFT JOIN (
			SELECT supervisor_user_id, COUNT(*) AS load_count
			FROM gaspol_assignments
			WHERE year = $1
			  AND month = $2 - 1
			  AND supervisor_user_id IS NOT NULL
			GROUP BY supervisor_user_id
		) previous ON previous.supervisor_user_id = gur.user_id
		WHERE gur.year = $1
		  AND gur.role = 'SUPERVISOR'
		  AND gur.is_active = TRUE
		ORDER BY display_name ASC
	`, year, month)
	if err != nil {
		return nil, nil, err
	}
	defer supervisorRows.Close()

	supervisors := make([]GaspolSupervisorCandidate, 0)
	for supervisorRows.Next() {
		var item GaspolSupervisorCandidate
		if err := supervisorRows.Scan(&item.UserID, &item.DisplayName, &item.CurrentLoad, &item.HistoricalLoad, &item.PreviousMonthLoad); err != nil {
			return nil, nil, err
		}
		supervisors = append(supervisors, item)
	}
	if err := supervisorRows.Err(); err != nil {
		return nil, nil, err
	}

	return schools, supervisors, nil
}

func (r GaspolRepository) UpsertAssignment(ctx context.Context, year int, month int, schoolID int, supervisorUserID *int, snapshot string, method string, actorID int) (GaspolAssignment, error) {
	var item GaspolAssignment
	err := r.DB.Pool.QueryRow(ctx, `
		INSERT INTO gaspol_assignments (
			year,
			month,
			school_id,
			supervisor_user_id,
			supervisor_name_snapshot,
			method,
			generated_by
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (year, month, school_id)
		DO UPDATE SET
			supervisor_user_id = EXCLUDED.supervisor_user_id,
			supervisor_name_snapshot = EXCLUDED.supervisor_name_snapshot,
			method = EXCLUDED.method,
			generated_by = EXCLUDED.generated_by,
			updated_at = NOW()
		RETURNING id, year, month, school_id, supervisor_user_id, supervisor_name_snapshot, method, created_at, updated_at
	`, year, month, schoolID, supervisorUserID, snapshot, method, actorID).Scan(
		&item.ID,
		&item.Year,
		&item.Month,
		&item.SchoolID,
		&item.SupervisorUserID,
		&item.SupervisorNameSnapshot,
		&item.Method,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return GaspolAssignment{}, err
	}

	if err := r.DB.Pool.QueryRow(ctx, `
		SELECT COALESCE(npsn_sekolah, ''), nama_sekolah
		FROM data_sekolah
		WHERE id = $1
	`, schoolID).Scan(&item.NPSN, &item.SchoolName); err != nil {
		return GaspolAssignment{}, err
	}
	item.SupervisorName = snapshot
	return item, nil
}

func (r GaspolRepository) HasSubmittedSupervisorAssessment(ctx context.Context, year int, month int, schoolID int) (bool, error) {
	var exists bool
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM gaspol_assessments
			WHERE year = $1
			  AND month = $2
			  AND school_id = $3
			  AND stage = 'MONTHLY'
		)
	`, year, month, schoolID).Scan(&exists)
	return exists, err
}

func (r GaspolRepository) ListAssignmentsBySupervisor(ctx context.Context, year int, month int, userID int) ([]GaspolAssignment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       COALESCE(ds.npsn_sekolah, '') AS npsn,
		       ds.nama_sekolah,
		       ga.supervisor_user_id,
		       ga.supervisor_name_snapshot,
		       ga.method,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assignments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		WHERE ga.year = $1
		  AND ga.month = $2
		  AND ga.supervisor_user_id = $3
		ORDER BY ds.nama_sekolah ASC
	`, year, month, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolAssignment, 0)
	for rows.Next() {
		var item GaspolAssignment
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.Month,
			&item.SchoolID,
			&item.NPSN,
			&item.SchoolName,
			&item.SupervisorUserID,
			&item.SupervisorNameSnapshot,
			&item.Method,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.SupervisorName = item.SupervisorNameSnapshot
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListAssignmentsByMonth(ctx context.Context, year int, month int) ([]GaspolAssignment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       COALESCE(ds.npsn_sekolah, '') AS npsn,
		       COALESCE(ds.nama_sekolah, '') AS school_name,
		       ga.supervisor_user_id,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username, '') AS supervisor_name,
		       COALESCE(ga.supervisor_name_snapshot, '') AS supervisor_name_snapshot,
		       ga.method,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assignments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		LEFT JOIN users u ON u.id = ga.supervisor_user_id
		WHERE ga.year = $1
		  AND ga.month = $2
		ORDER BY ds.nama_sekolah ASC, ga.id ASC
	`, year, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolAssignment, 0)
	for rows.Next() {
		var item GaspolAssignment
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.Month,
			&item.SchoolID,
			&item.NPSN,
			&item.SchoolName,
			&item.SupervisorUserID,
			&item.SupervisorName,
			&item.SupervisorNameSnapshot,
			&item.Method,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListAssignments(ctx context.Context, year int, month int) ([]GaspolAssignment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       COALESCE(ds.npsn_sekolah, '') AS npsn,
		       ds.nama_sekolah,
		       ga.supervisor_user_id,
		       ga.supervisor_name_snapshot,
		       ga.method,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assignments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		WHERE ga.year = $1
		  AND ga.month = $2
		ORDER BY ds.nama_sekolah ASC
	`, year, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolAssignment, 0)
	for rows.Next() {
		var item GaspolAssignment
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.Month,
			&item.SchoolID,
			&item.NPSN,
			&item.SchoolName,
			&item.SupervisorUserID,
			&item.SupervisorNameSnapshot,
			&item.Method,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.SupervisorName = item.SupervisorNameSnapshot
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) UpsertSubmission(ctx context.Context, submission GaspolSchoolSubmission, userID int) (GaspolSchoolSubmission, error) {
	attachmentsJSON, err := json.Marshal(submission.AttachmentURLs)
	if err != nil {
		return GaspolSchoolSubmission{}, err
	}

	var item GaspolSchoolSubmission
	var submittedAt *time.Time
	err = r.DB.Pool.QueryRow(ctx, `
		INSERT INTO gaspol_school_submissions (
			year,
			month,
			school_id,
			submitted_by,
			report_title,
			report_body,
			report_link,
			attachment_urls,
			status,
			submitted_at
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8::jsonb,
			$9::gaspol_submission_status_type,
			CASE
				WHEN $9::gaspol_submission_status_type = 'SUBMITTED'::gaspol_submission_status_type THEN NOW()
				ELSE NULL
			END
		)
		ON CONFLICT (year, month, school_id)
		DO UPDATE SET
			submitted_by = EXCLUDED.submitted_by,
			report_title = EXCLUDED.report_title,
			report_body = EXCLUDED.report_body,
			report_link = EXCLUDED.report_link,
			attachment_urls = EXCLUDED.attachment_urls,
			status = EXCLUDED.status,
			submitted_at = CASE
				WHEN EXCLUDED.status = 'SUBMITTED'::gaspol_submission_status_type THEN NOW()
				ELSE gaspol_school_submissions.submitted_at
			END,
			updated_at = NOW()
		RETURNING id, year, month, school_id, report_title, report_body, report_link, attachment_urls, status, submitted_by, submitted_at, updated_at
	`, submission.Year, submission.Month, submission.SchoolID, userID, submission.ReportTitle, submission.ReportBody, submission.ReportLink, string(attachmentsJSON), submission.Status).Scan(
		&item.ID,
		&item.Year,
		&item.Month,
		&item.SchoolID,
		&item.ReportTitle,
		&item.ReportBody,
		&item.ReportLink,
		&attachmentsJSON,
		&item.Status,
		&item.SubmittedBy,
		&submittedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return GaspolSchoolSubmission{}, err
	}
	item.SubmittedAt = submittedAt
	item.AttachmentURLs = parseJSONStringArray(attachmentsJSON)
	if err := r.DB.Pool.QueryRow(ctx, `SELECT nama_sekolah FROM data_sekolah WHERE id = $1`, item.SchoolID).Scan(&item.SchoolName); err != nil {
		return GaspolSchoolSubmission{}, err
	}
	return item, nil
}

func (r GaspolRepository) ListSchoolSubmissions(ctx context.Context, year int, schoolID int) ([]GaspolSchoolSubmission, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT gss.id,
		       gss.year,
		       gss.month,
		       gss.school_id,
		       ds.nama_sekolah,
		       gss.report_title,
		       gss.report_body,
		       gss.report_link,
		       gss.attachment_urls,
		       gss.status,
		       gss.submitted_by,
		       gss.submitted_at,
		       gss.updated_at
		FROM gaspol_school_submissions gss
		JOIN data_sekolah ds ON ds.id = gss.school_id
		WHERE gss.year = $1
		  AND gss.school_id = $2
		ORDER BY gss.month DESC, gss.updated_at DESC
	`, year, schoolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolSchoolSubmission, 0)
	for rows.Next() {
		var item GaspolSchoolSubmission
		var attachmentsJSON []byte
		var submittedAt *time.Time
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.Month,
			&item.SchoolID,
			&item.SchoolName,
			&item.ReportTitle,
			&item.ReportBody,
			&item.ReportLink,
			&attachmentsJSON,
			&item.Status,
			&item.SubmittedBy,
			&submittedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.SubmittedAt = submittedAt
		item.AttachmentURLs = parseJSONStringArray(attachmentsJSON)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListSchoolSubmissionsByMonth(ctx context.Context, year int, month int) ([]GaspolSchoolSubmission, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT gss.id,
		       gss.year,
		       gss.month,
		       gss.school_id,
		       ds.nama_sekolah,
		       gss.report_title,
		       gss.report_body,
		       gss.report_link,
		       gss.attachment_urls,
		       gss.status,
		       gss.submitted_by,
		       gss.submitted_at,
		       gss.updated_at
		FROM gaspol_school_submissions gss
		JOIN gaspol_school_participants p
		  ON p.year = gss.year
		 AND p.school_id = gss.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = gss.school_id
		WHERE gss.year = $1
		  AND gss.month = $2
		ORDER BY ds.nama_sekolah ASC, gss.updated_at DESC
	`, year, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolSchoolSubmission, 0)
	for rows.Next() {
		var item GaspolSchoolSubmission
		var submittedAt *time.Time
		var attachmentsJSON []byte
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.Month,
			&item.SchoolID,
			&item.SchoolName,
			&item.ReportTitle,
			&item.ReportBody,
			&item.ReportLink,
			&attachmentsJSON,
			&item.Status,
			&item.SubmittedBy,
			&submittedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.SubmittedAt = submittedAt
		item.AttachmentURLs = parseJSONStringArray(attachmentsJSON)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) GetSchoolSubmission(ctx context.Context, year int, month int, schoolID int) (GaspolSchoolSubmission, error) {
	var item GaspolSchoolSubmission
	var attachmentsJSON []byte
	var submittedAt *time.Time
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT gss.id,
		       gss.year,
		       gss.month,
		       gss.school_id,
		       ds.nama_sekolah,
		       gss.report_title,
		       gss.report_body,
		       gss.report_link,
		       gss.attachment_urls,
		       gss.status,
		       gss.submitted_by,
		       gss.submitted_at,
		       gss.updated_at
		FROM gaspol_school_submissions gss
		JOIN data_sekolah ds ON ds.id = gss.school_id
		WHERE gss.year = $1
		  AND gss.month = $2
		  AND gss.school_id = $3
	`, year, month, schoolID).Scan(
		&item.ID,
		&item.Year,
		&item.Month,
		&item.SchoolID,
		&item.SchoolName,
		&item.ReportTitle,
		&item.ReportBody,
		&item.ReportLink,
		&attachmentsJSON,
		&item.Status,
		&item.SubmittedBy,
		&submittedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return GaspolSchoolSubmission{}, err
	}
	item.SubmittedAt = submittedAt
	item.AttachmentURLs = parseJSONStringArray(attachmentsJSON)
	return item, nil
}

func (r GaspolRepository) CancelSubmitSubmission(ctx context.Context, year int, month int, schoolID int) (GaspolSchoolSubmission, error) {
	var item GaspolSchoolSubmission
	var attachmentsJSON []byte
	var submittedAt *time.Time
	err := r.DB.Pool.QueryRow(ctx, `
		UPDATE gaspol_school_submissions
		SET
			status = 'DRAFT'::gaspol_submission_status_type,
			submitted_at = NULL,
			submitted_by = NULL,
			updated_at = NOW()
		WHERE year = $1
		  AND month = $2
		  AND school_id = $3
		RETURNING id, year, month, school_id, report_title, report_body, report_link, attachment_urls, status, submitted_by, submitted_at, updated_at
	`, year, month, schoolID).Scan(
		&item.ID,
		&item.Year,
		&item.Month,
		&item.SchoolID,
		&item.ReportTitle,
		&item.ReportBody,
		&item.ReportLink,
		&attachmentsJSON,
		&item.Status,
		&item.SubmittedBy,
		&submittedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return GaspolSchoolSubmission{}, err
	}
	item.SubmittedAt = submittedAt
	item.AttachmentURLs = parseJSONStringArray(attachmentsJSON)
	if err := r.DB.Pool.QueryRow(ctx, `SELECT nama_sekolah FROM data_sekolah WHERE id = $1`, item.SchoolID).Scan(&item.SchoolName); err != nil {
		return GaspolSchoolSubmission{}, err
	}
	return item, nil
}

func parseJSONStringArray(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return []string{}
	}
	if values == nil {
		return []string{}
	}
	return values
}

func (r GaspolRepository) UpsertAssessment(ctx context.Context, assessment GaspolAssessment) (GaspolAssessment, error) {
	var item GaspolAssessment
	var metadataJSON []byte
	err := r.DB.Pool.QueryRow(ctx, `
		INSERT INTO gaspol_assessments (
			year,
			month,
			school_id,
			assignment_id,
			assessor_user_id,
			assessor_role,
			stage,
			score_monthly,
			score_field,
			score_presentation,
			score_total,
			notes,
			metadata,
			is_validated,
			stage_code,
			workflow_status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, '{}'::jsonb, FALSE,
			CASE WHEN $7::text = 'FIELD' AND $6::text = 'JURY' THEN 'FIELD_STAGE_2' WHEN $7::text = 'FIELD' THEN 'FIELD_STAGE_1' ELSE $7::text END, 'IN_PROGRESS')
		ON CONFLICT (year, month, school_id, stage, assessor_user_id)
		DO UPDATE SET
			assignment_id = EXCLUDED.assignment_id,
			score_monthly = EXCLUDED.score_monthly,
			score_field = EXCLUDED.score_field,
			score_presentation = EXCLUDED.score_presentation,
			score_total = EXCLUDED.score_total,
			notes = EXCLUDED.notes,
			metadata = '{}'::jsonb,
			is_validated = FALSE,
			validated_by = NULL,
			validated_at = NULL,
			stage_code = CASE WHEN EXCLUDED.stage::text = 'FIELD' AND EXCLUDED.assessor_role::text = 'JURY' THEN 'FIELD_STAGE_2' WHEN EXCLUDED.stage::text = 'FIELD' THEN 'FIELD_STAGE_1' ELSE EXCLUDED.stage::text END,
			workflow_status = 'IN_PROGRESS',
			koor_user_id = NULL,
			koor_validated_at = NULL,
			cabdin_user_id = NULL,
			cabdin_validated_at = NULL,
			updated_at = NOW()
		RETURNING id, year, month, school_id, assignment_id, assessor_user_id, assessor_role, stage, score_monthly, score_field, score_presentation, score_total, notes, is_validated, validated_by, validated_at, created_at, updated_at, metadata
	`, assessment.Year, assessment.Month, assessment.SchoolID, assessment.AssignmentID, assessment.AssessorUserID, assessment.AssessorRole, assessment.Stage, assessment.ScoreMonthly, assessment.ScoreField, assessment.ScorePresentation, assessment.ScoreTotal, assessment.Notes).Scan(
		&item.ID,
		&item.Year,
		&item.Month,
		&item.SchoolID,
		&item.AssignmentID,
		&item.AssessorUserID,
		&item.AssessorRole,
		&item.Stage,
		&item.ScoreMonthly,
		&item.ScoreField,
		&item.ScorePresentation,
		&item.ScoreTotal,
		&item.Notes,
		&item.IsValidated,
		&item.ValidatedBy,
		&item.ValidatedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&metadataJSON,
	)
	if err != nil {
		return GaspolAssessment{}, err
	}
	applyGaspolAssessmentValidationMetadata(&item, metadataJSON)
	if err := r.DB.Pool.QueryRow(ctx, `
		SELECT ds.nama_sekolah,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS assessor_name
		FROM data_sekolah ds, users u
		WHERE ds.id = $1
		  AND u.id = $2
	`, item.SchoolID, item.AssessorUserID).Scan(&item.SchoolName, &item.AssessorName); err != nil {
		return GaspolAssessment{}, err
	}
	return item, nil
}

// CreateSupervisorAssessment records the one official monthly assessment. The
// transaction-scoped advisory lock serializes retries even when the assignment
// changes supervisor, while the existing actor/stage unique index remains a
// second line of defence.
func (r GaspolRepository) CreateSupervisorAssessment(ctx context.Context, assessment GaspolAssessment) (GaspolAssessment, error) {
	tx, err := r.DB.Pool.Begin(ctx)
	if err != nil {
		return GaspolAssessment{}, err
	}
	defer tx.Rollback(ctx)
	lockKey := assessment.Year*100 + assessment.Month
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1, $2)`, lockKey, assessment.SchoolID); err != nil {
		return GaspolAssessment{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM gaspol_assessments
		WHERE year=$1 AND month=$2 AND school_id=$3
		  AND stage='MONTHLY' AND assessor_role='SUPERVISOR'
	)`, assessment.Year, assessment.Month, assessment.SchoolID).Scan(&exists); err != nil {
		return GaspolAssessment{}, err
	}
	if exists {
		return GaspolAssessment{}, errors.New("ASSESSMENT_ALREADY_SUBMITTED")
	}

	var item GaspolAssessment
	var metadataJSON []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO gaspol_assessments(year,month,school_id,assignment_id,assessor_user_id,assessor_role,stage,score_monthly,score_field,score_presentation,score_total,notes,metadata,is_validated,stage_code,workflow_status)
		VALUES($1,$2,$3,$4,$5,'SUPERVISOR','MONTHLY',$6,$7,$8,$9,$10,'{}'::jsonb,FALSE,'MONTHLY','WAITING_KOOR')
		RETURNING id,year,month,school_id,assignment_id,assessor_user_id,assessor_role,stage,score_monthly,score_field,score_presentation,score_total,notes,is_validated,validated_by,validated_at,created_at,updated_at,metadata
	`, assessment.Year, assessment.Month, assessment.SchoolID, assessment.AssignmentID, assessment.AssessorUserID, assessment.ScoreMonthly, assessment.ScoreField, assessment.ScorePresentation, assessment.ScoreTotal, assessment.Notes).Scan(
		&item.ID, &item.Year, &item.Month, &item.SchoolID, &item.AssignmentID, &item.AssessorUserID, &item.AssessorRole, &item.Stage, &item.ScoreMonthly, &item.ScoreField, &item.ScorePresentation, &item.ScoreTotal, &item.Notes, &item.IsValidated, &item.ValidatedBy, &item.ValidatedAt, &item.CreatedAt, &item.UpdatedAt, &metadataJSON,
	)
	if err != nil {
		return GaspolAssessment{}, err
	}
	if err = tx.QueryRow(ctx, `SELECT ds.nama_sekolah,COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username) FROM data_sekolah ds,users u WHERE ds.id=$1 AND u.id=$2`, item.SchoolID, item.AssessorUserID).Scan(&item.SchoolName, &item.AssessorName); err != nil {
		return GaspolAssessment{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return GaspolAssessment{}, err
	}
	applyGaspolAssessmentValidationMetadata(&item, metadataJSON)
	return item, nil
}

func (r GaspolRepository) ListAssessmentsBySchool(ctx context.Context, year int, schoolID int) ([]GaspolAssessment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       ds.nama_sekolah,
		       ga.assignment_id,
		       ga.assessor_user_id,
		       ga.assessor_role,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS assessor_name,
		       ga.stage,
		       ga.score_monthly,
		       ga.score_field,
		       ga.score_presentation,
		       ga.score_total,
		       ga.notes,
		       ga.metadata,
		       ga.is_validated,
		       ga.validated_by,
		       ga.validated_at,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assessments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		JOIN users u ON u.id = ga.assessor_user_id
		WHERE ga.year = $1
		  AND ga.school_id = $2
		ORDER BY ga.month DESC, ga.created_at DESC
	`, year, schoolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGaspolAssessments(rows)
}

func (r GaspolRepository) ListSupervisorAssessments(ctx context.Context, year int, month int, userID int) ([]GaspolAssessment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       ds.nama_sekolah,
		       ga.assignment_id,
		       ga.assessor_user_id,
		       ga.assessor_role,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS assessor_name,
		       ga.stage,
		       ga.score_monthly,
		       ga.score_field,
		       ga.score_presentation,
		       ga.score_total,
		       ga.notes,
		       ga.metadata,
		       ga.is_validated,
		       ga.validated_by,
		       ga.validated_at,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assessments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		JOIN users u ON u.id = ga.assessor_user_id
		WHERE ga.year = $1
		  AND ga.month = $2
		  AND ga.assessor_user_id = $3
		ORDER BY ds.nama_sekolah ASC, ga.created_at DESC
	`, year, month, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGaspolAssessments(rows)
}

// ListSupervisorMonthlyStageAssessments returns the v2 lifecycle rows used to
// classify the supervisor dashboard. The legacy assessment remains available
// for compatibility, but must not be used as the workflow source of truth.
func (r GaspolRepository) ListSupervisorMonthlyStageAssessments(ctx context.Context, year int, month int, userID int) ([]GaspolStageAssessmentRow, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT a.id,a.year,a.stage,a.school_id,ds.nama_sekolah,a.assessor_user_id,
		       COALESCE(NULLIF(TRIM(u.nama_instansi),''),u.username),a.raw_score,a.final_score,
		       a.workflow_status,a.notes,
		       CASE
		         WHEN a.workflow_status='RETURNED_BY_KOOR' THEN a.koor_notes
		         WHEN a.workflow_status='RETURNED_BY_CABDIN' THEN a.cabdin_notes
		         WHEN a.workflow_status='WAITING_CABDIN' THEN a.koor_notes
		         ELSE ''
		       END,
		       a.updated_at
		FROM gaspol_stage_assessments a
		JOIN data_sekolah ds ON ds.id=a.school_id
		JOIN users u ON u.id=a.assessor_user_id
		WHERE a.year=$1 AND a.month=$2 AND a.assessor_user_id=$3 AND a.stage='MONTHLY'
		ORDER BY ds.nama_sekolah,a.id
	`, year, month, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]GaspolStageAssessmentRow, 0)
	for rows.Next() {
		var item GaspolStageAssessmentRow
		if err := rows.Scan(&item.ID, &item.Year, &item.Stage, &item.SchoolID, &item.SchoolName,
			&item.AssessorUserID, &item.AssessorName, &item.RawScore, &item.FinalScore,
			&item.WorkflowStatus, &item.Notes, &item.ReviewerNotes, &item.SubmittedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListPendingAssessmentsForValidator(ctx context.Context, year int, userID int) ([]GaspolAssessment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       ds.nama_sekolah,
		       ga.assignment_id,
		       ga.assessor_user_id,
		       ga.assessor_role,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS assessor_name,
		       ga.stage,
		       ga.score_monthly,
		       ga.score_field,
		       ga.score_presentation,
		       ga.score_total,
		       ga.notes,
		       ga.metadata,
		       ga.is_validated,
		       ga.validated_by,
		       ga.validated_at,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assessments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		JOIN users u ON u.id = ga.assessor_user_id
		WHERE ga.year = $1
		  AND ga.is_validated = FALSE
		  AND p.validator_user_id = $2
		ORDER BY ga.month ASC, ds.nama_sekolah ASC, ga.created_at ASC
	`, year, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGaspolAssessments(rows)
}

func (r GaspolRepository) CountValidatedAssessmentsForValidator(ctx context.Context, year int, userID int) (int, error) {
	var count int
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM gaspol_assessments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		WHERE ga.year = $1
		  AND ga.is_validated = TRUE
		  AND p.validator_user_id = $2
	`, year, userID).Scan(&count)
	return count, err
}

func (r GaspolRepository) ListValidatedAssessmentsForValidator(ctx context.Context, year int, userID int) ([]GaspolAssessment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       ds.nama_sekolah,
		       ga.assignment_id,
		       ga.assessor_user_id,
		       ga.assessor_role,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS assessor_name,
		       ga.stage,
		       ga.score_monthly,
		       ga.score_field,
		       ga.score_presentation,
		       ga.score_total,
		       ga.notes,
		       ga.metadata,
		       ga.is_validated,
		       ga.validated_by,
		       ga.validated_at,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assessments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		JOIN users u ON u.id = ga.assessor_user_id
		WHERE ga.year = $1
		  AND ga.is_validated = TRUE
		  AND p.validator_user_id = $2
		ORDER BY ga.month DESC, ga.validated_at DESC NULLS LAST, ds.nama_sekolah ASC
	`, year, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGaspolAssessments(rows)
}

func (r GaspolRepository) ListStageAssessments(ctx context.Context, year int, month int, stage string) ([]GaspolAssessment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       ds.nama_sekolah,
		       ga.assignment_id,
		       ga.assessor_user_id,
		       ga.assessor_role,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS assessor_name,
		       ga.stage,
		       ga.score_monthly,
		       ga.score_field,
		       ga.score_presentation,
		       ga.score_total,
		       ga.notes,
		       ga.metadata,
		       ga.is_validated,
		       ga.validated_by,
		       ga.validated_at,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assessments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		JOIN users u ON u.id = ga.assessor_user_id
		WHERE ga.year = $1
		  AND ga.month = $2
		  AND ga.stage = $3
		ORDER BY ds.nama_sekolah ASC, ga.created_at DESC
	`, year, month, stage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGaspolAssessments(rows)
}

func (r GaspolRepository) ListJuryAssessmentsByUser(ctx context.Context, year int, userID int) ([]GaspolAssessment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       ds.nama_sekolah,
		       ga.assignment_id,
		       ga.assessor_user_id,
		       ga.assessor_role,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS assessor_name,
		       ga.stage,
		       ga.score_monthly,
		       ga.score_field,
		       ga.score_presentation,
		       ga.score_total,
		       ga.notes,
		       ga.metadata,
		       ga.is_validated,
		       ga.validated_by,
		       ga.validated_at,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assessments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		JOIN users u ON u.id = ga.assessor_user_id
		WHERE ga.year = $1
		  AND ga.assessor_user_id = $2
		  AND ga.assessor_role = 'JURY'
		ORDER BY ga.month DESC, ds.nama_sekolah ASC, ga.created_at DESC
	`, year, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGaspolAssessments(rows)
}

func scanGaspolAssessments(rows pgx.Rows) ([]GaspolAssessment, error) {
	result := make([]GaspolAssessment, 0)
	for rows.Next() {
		var item GaspolAssessment
		var metadataJSON []byte
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.Month,
			&item.SchoolID,
			&item.SchoolName,
			&item.AssignmentID,
			&item.AssessorUserID,
			&item.AssessorRole,
			&item.AssessorName,
			&item.Stage,
			&item.ScoreMonthly,
			&item.ScoreField,
			&item.ScorePresentation,
			&item.ScoreTotal,
			&item.Notes,
			&metadataJSON,
			&item.IsValidated,
			&item.ValidatedBy,
			&item.ValidatedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		applyGaspolAssessmentValidationMetadata(&item, metadataJSON)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ValidateAssessments(ctx context.Context, ids []int, userID int) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.DB.Pool.Exec(ctx, `
		UPDATE gaspol_assessments
		SET is_validated = TRUE,
		    validated_by = $2,
		    validated_at = NOW(),
		    updated_at = NOW()
		WHERE id = ANY($1)
	`, ids, userID)
	return err
}

func (r GaspolRepository) ApplyValidatorDecision(ctx context.Context, id int, userID int, decision string, validatorScore *float64, validatorNotes string) error {
	finalScore := validatorScore
	if decision == GaspolValidationAgree {
		finalScore = nil
	}
	_, err := r.DB.Pool.Exec(ctx, `
		UPDATE gaspol_assessments
		SET score_monthly = CASE
				WHEN $3::text = 'OBJECT' THEN $4::numeric(8,2)
				ELSE score_monthly
			END,
		    score_total = CASE
				WHEN $3::text = 'OBJECT' THEN $4::numeric(8,2)
				ELSE score_total
			END,
		    metadata = jsonb_strip_nulls(jsonb_build_object(
				'validationDecision', $3::text,
				'supervisorScore', score_monthly,
				'validatorScore', $4::numeric(8,2),
				'finalMonthlyScore', CASE
					WHEN $3::text = 'OBJECT' THEN $4::numeric(8,2)
					ELSE score_monthly
				END,
				'validatorNotes', NULLIF($5::text, '')
			)),
		    is_validated = TRUE,
		    validated_by = $2,
		    validated_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, id, userID, decision, finalScore, strings.TrimSpace(validatorNotes))
	return err
}

func applyGaspolAssessmentValidationMetadata(item *GaspolAssessment, raw []byte) {
	if item == nil {
		return
	}
	finalScore := 0.0
	switch item.Stage {
	case GaspolStageMonthly:
		finalScore = item.ScoreMonthly
	case GaspolStageField:
		finalScore = item.ScoreField
	case GaspolStagePresentation:
		finalScore = item.ScorePresentation
	}
	if item.Stage == GaspolStageMonthly {
		scoreCopy := finalScore
		item.FinalMonthlyScore = &scoreCopy
	}
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return
	}

	var metadata gaspolAssessmentValidationMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return
	}
	if metadata.ValidationDecision != "" {
		item.ValidationDecision = metadata.ValidationDecision
	}
	if metadata.SupervisorScore != nil {
		score := *metadata.SupervisorScore
		item.SupervisorScore = &score
	}
	if metadata.ValidatorScore != nil {
		score := *metadata.ValidatorScore
		item.ValidatorScore = &score
	}
	if metadata.FinalMonthlyScore != nil {
		score := *metadata.FinalMonthlyScore
		item.FinalMonthlyScore = &score
	}
	if metadata.ValidatorNotes != "" {
		item.ValidatorNotes = metadata.ValidatorNotes
	}
	if item.Stage == GaspolStageMonthly && item.SupervisorScore == nil && item.IsValidated {
		score := item.ScoreMonthly
		item.SupervisorScore = &score
	}
}

func (r GaspolRepository) ListValidatedAssessmentScores(ctx context.Context, year int) (map[int]GaspolScoringInput, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT school_id,
		       stage,
		       month,
		       score_monthly,
		       score_field,
		       score_presentation
		FROM gaspol_assessments
		WHERE year = $1
		  AND is_validated = TRUE
		ORDER BY school_id ASC, month ASC
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[int]GaspolScoringInput{}
	for rows.Next() {
		var (
			schoolID          int
			stage             string
			month             int
			scoreMonthly      float64
			scoreField        float64
			scorePresentation float64
		)
		if err := rows.Scan(&schoolID, &stage, &month, &scoreMonthly, &scoreField, &scorePresentation); err != nil {
			return nil, err
		}
		item := result[schoolID]
		switch stage {
		case GaspolStageMonthly:
			if isGaspolMonthlyComponentMonth(month) {
				item.MonthlyComponents = upsertGaspolValidatedMonthlyComponent(item.MonthlyComponents, month, scoreMonthly)
			}
		case GaspolStageField:
			item.FieldScore = scoreField
		case GaspolStagePresentation:
			item.PresentationScore = scorePresentation
		}
		result[schoolID] = item
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListRankingMonthlySubmissions(ctx context.Context, year int) ([]GaspolSchoolSubmission, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT gss.id,
		       gss.year,
		       gss.month,
		       gss.school_id,
		       ds.nama_sekolah,
		       gss.report_title,
		       gss.report_body,
		       gss.report_link,
		       gss.attachment_urls,
		       gss.status,
		       gss.submitted_by,
		       gss.submitted_at,
		       gss.updated_at
		FROM gaspol_school_submissions gss
		JOIN gaspol_school_participants p
		  ON p.year = gss.year
		 AND p.school_id = gss.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = gss.school_id
		WHERE gss.year = $1
		  AND gss.month BETWEEN 4 AND 9
		ORDER BY gss.school_id ASC, gss.month ASC, gss.updated_at DESC
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolSchoolSubmission, 0)
	for rows.Next() {
		var item GaspolSchoolSubmission
		var attachmentsJSON []byte
		var submittedAt *time.Time
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.Month,
			&item.SchoolID,
			&item.SchoolName,
			&item.ReportTitle,
			&item.ReportBody,
			&item.ReportLink,
			&attachmentsJSON,
			&item.Status,
			&item.SubmittedBy,
			&submittedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.SubmittedAt = submittedAt
		item.AttachmentURLs = parseJSONStringArray(attachmentsJSON)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) ListRankingAssessments(ctx context.Context, year int) ([]GaspolAssessment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id,
		       ga.year,
		       ga.month,
		       ga.school_id,
		       ds.nama_sekolah,
		       ga.assignment_id,
		       ga.assessor_user_id,
		       ga.assessor_role,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username) AS assessor_name,
		       ga.stage,
		       ga.score_monthly,
		       ga.score_field,
		       ga.score_presentation,
		       ga.score_total,
		       ga.notes,
		       ga.metadata,
		       ga.is_validated,
		       ga.validated_by,
		       ga.validated_at,
		       ga.created_at,
		       ga.updated_at
		FROM gaspol_assessments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		 AND p.is_active = TRUE
		JOIN data_sekolah ds ON ds.id = ga.school_id
		JOIN users u ON u.id = ga.assessor_user_id
		WHERE ga.year = $1
		  AND (
			(ga.stage = 'MONTHLY' AND ga.month BETWEEN 4 AND 9)
			OR ga.stage = 'FIELD'
			OR ga.stage = 'PRESENTATION'
		  )
		ORDER BY ga.school_id ASC, ga.month ASC, ga.updated_at DESC, ga.created_at DESC
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGaspolAssessments(rows)
}

func (r GaspolRepository) ReplaceRankingSnapshots(ctx context.Context, year int, month int, stage string, rows []GaspolRankingRow, userID int) error {
	tx, err := r.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		DELETE FROM gaspol_ranking_snapshots
		WHERE year = $1
		  AND month = $2
		  AND stage = $3
	`, year, month, stage); err != nil {
		return err
	}

	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO gaspol_ranking_snapshots (
				year,
				month,
				stage,
				school_id,
				rank_position,
				score_monthly,
				score_field,
				score_presentation,
				score_total,
				snapshot_payload,
				published_by,
				published_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, NOW())
		`, year, month, stage, row.SchoolID, row.Rank, row.Scores.ScoreMonthly, row.Scores.ScoreField, row.Scores.ScorePresentation, row.Scores.ScoreTotal, string(payload), userID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r GaspolRepository) ListRankingSnapshots(ctx context.Context, year int, month int, stage string) ([]GaspolRankingRow, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT rank_position,
		       year,
		       month,
		       stage,
		       school_id,
		       snapshot_payload
		FROM gaspol_ranking_snapshots
		WHERE year = $1
		  AND month = $2
		  AND stage = $3
		ORDER BY rank_position ASC, school_id ASC
	`, year, month, stage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolRankingRow, 0)
	for rows.Next() {
		var (
			row     GaspolRankingRow
			payload []byte
		)
		if err := rows.Scan(&row.Rank, &row.Year, &row.Month, &row.Stage, &row.SchoolID, &payload); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &row); err != nil {
				return nil, err
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r GaspolRepository) GetLatestRankingSnapshotMeta(ctx context.Context, year int) (int, string, *time.Time, error) {
	var (
		month       int
		stage       string
		publishedAt *time.Time
	)
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT month, stage, published_at
		FROM gaspol_ranking_snapshots
		WHERE year = $1
		ORDER BY month DESC, published_at DESC NULLS LAST, id DESC
		LIMIT 1
	`, year).Scan(&month, &stage, &publishedAt)
	if err != nil {
		return 0, "", nil, err
	}
	return month, stage, publishedAt, nil
}

func (r GaspolRepository) ListRankingSchoolMeta(ctx context.Context, year int) ([]GaspolRankingRow, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ds.id,
		       COALESCE(ds.npsn_sekolah, '') AS npsn,
		       ds.nama_sekolah,
		       COALESCE(UPPER(TRIM(ds.bentuk_pendidikan)), '') AS school_level,
		       COALESCE(UPPER(TRIM(ds.status_sekolah)), '') AS school_status,
		       COALESCE(ga.supervisor_name_snapshot, '') AS supervisor_name_snapshot
		FROM gaspol_school_participants p
		JOIN data_sekolah ds ON ds.id = p.school_id
		LEFT JOIN LATERAL (
			SELECT supervisor_name_snapshot
			FROM gaspol_assignments
			WHERE year = $1
			  AND school_id = p.school_id
			ORDER BY month DESC, updated_at DESC
			LIMIT 1
		) ga ON TRUE
		WHERE p.year = $1
		  AND p.is_year_configuration = FALSE
		  AND p.is_active = TRUE
		ORDER BY ds.nama_sekolah ASC
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolRankingRow, 0)
	for rows.Next() {
		var row GaspolRankingRow
		if err := rows.Scan(&row.SchoolID, &row.NPSN, &row.SchoolName, &row.SchoolLevel, &row.SchoolStatus, &row.SupervisorNameSnapshot); err != nil {
			return nil, err
		}
		row.Year = year
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r GaspolRepository) LogAudit(ctx context.Context, year *int, actorUserID int, actorRole string, action string, entityType string, entityID *int, schoolID *int, payload interface{}) error {
	if strings.TrimSpace(action) == "" {
		return errors.New("missing action")
	}
	rawPayload := []byte("{}")
	if payload != nil {
		marshaled, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		rawPayload = marshaled
	}
	_, err := r.DB.Pool.Exec(ctx, `
		INSERT INTO gaspol_audit_logs (
			year,
			actor_user_id,
			actor_role,
			action,
			entity_type,
			entity_id,
			school_id,
			payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
	`, year, actorUserID, actorRole, action, entityType, entityID, schoolID, string(rawPayload))
	return err
}

func (r GaspolRepository) FindSchoolByUser(ctx context.Context, userID int) (*int, string, error) {
	var (
		schoolID *int
		name     string
	)
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT u.school_id, COALESCE(ds.nama_sekolah, '')
		FROM users u
		LEFT JOIN data_sekolah ds ON ds.id = u.school_id
		WHERE u.id = $1
	`, userID).Scan(&schoolID, &name)
	if err != nil {
		return nil, "", err
	}
	return schoolID, name, nil
}

func (r GaspolRepository) CountPendingValidations(ctx context.Context, year int) (int, error) {
	var count int
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM gaspol_stage_assessments
		WHERE year = $1
		  AND workflow_status <> 'FINAL'
	`, year).Scan(&count)
	return count, err
}

func (r GaspolRepository) CountDistinctRoles(ctx context.Context, year int, role string) (int, error) {
	var count int
	err := r.DB.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id)
		FROM gaspol_user_roles
		WHERE year = $1::integer
		  AND role = $2::gaspol_role_type
		  AND is_active = TRUE
	`, year, role).Scan(&count)
	return count, err
}

func (r GaspolRepository) GetAssessmentValidatorScope(ctx context.Context, assessmentIDs []int, validatorUserID int) ([]int, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT ga.id
		FROM gaspol_assessments ga
		JOIN gaspol_school_participants p
		  ON p.year = ga.year
		 AND p.school_id = ga.school_id
		 AND p.is_year_configuration = FALSE
		WHERE ga.id = ANY($1)
		  AND p.validator_user_id = $2
	`, assessmentIDs, validatorUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]int, 0)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (r GaspolRepository) GetAssessmentAuditSnapshots(ctx context.Context, assessmentIDs []int) ([]GaspolAssessment, error) {
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT id, year, month, school_id, score_monthly, metadata
		FROM gaspol_assessments
		WHERE id = ANY($1)
	`, assessmentIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]GaspolAssessment, 0, len(assessmentIDs))
	for rows.Next() {
		var item GaspolAssessment
		var metadata []byte
		if err := rows.Scan(&item.ID, &item.Year, &item.Month, &item.SchoolID, &item.ScoreMonthly, &metadata); err != nil {
			return nil, err
		}
		item.Stage = GaspolStageMonthly
		applyGaspolAssessmentValidationMetadata(&item, metadata)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r GaspolRepository) GetSchoolName(ctx context.Context, schoolID int) (string, error) {
	var name string
	err := r.DB.Pool.QueryRow(ctx, `SELECT nama_sekolah FROM data_sekolah WHERE id = $1`, schoolID).Scan(&name)
	return name, err
}

func (r GaspolRepository) GetUserIDByUsername(ctx context.Context, username string) (*int, error) {
	var userID int
	err := r.DB.Pool.QueryRow(ctx, `SELECT id FROM users WHERE username = $1`, username).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &userID, nil
}

func (r GaspolRepository) BuildUsernameCandidate(base string, suffix int) string {
	normalized := strings.ToLower(strings.TrimSpace(base))
	normalized = strings.NewReplacer(" ", "-", "_", "-", ".", "-", "/", "-", "\\", "-").Replace(normalized)
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		normalized = "gaspol"
	}
	if suffix > 0 {
		return fmt.Sprintf("%s-%d", normalized, suffix)
	}
	return normalized
}

func (r GaspolRepository) ListAuditLogs(ctx context.Context, year int, limit int) ([]GaspolAuditLogRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.DB.Pool.Query(ctx, `
		SELECT gal.id,
		       gal.year,
		       gal.actor_user_id,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), u.username, '') AS actor_name,
		       gal.actor_role,
		       gal.action,
		       gal.entity_type,
		       gal.entity_id,
		       gal.school_id,
		       COALESCE(ds.nama_sekolah, '') AS school_name,
		       gal.payload::text,
		       gal.created_at
		FROM gaspol_audit_logs gal
		LEFT JOIN users u ON u.id = gal.actor_user_id
		LEFT JOIN data_sekolah ds ON ds.id = gal.school_id
		WHERE ($1 = 0 OR gal.year = $1 OR gal.year IS NULL)
		ORDER BY gal.created_at DESC, gal.id DESC
		LIMIT $2
	`, year, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GaspolAuditLogRow, 0)
	for rows.Next() {
		var item GaspolAuditLogRow
		if err := rows.Scan(
			&item.ID,
			&item.Year,
			&item.ActorUserID,
			&item.ActorName,
			&item.ActorRole,
			&item.Action,
			&item.EntityType,
			&item.EntityID,
			&item.SchoolID,
			&item.SchoolName,
			&item.Payload,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
