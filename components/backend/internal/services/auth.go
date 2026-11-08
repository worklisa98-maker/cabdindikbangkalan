package services

import (
	"context"
	"time"

	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
)

type AuthService struct {
	DB *db.DB
}

type UserRecord struct {
	ID                int
	Username          string
	PasswordHash      string
	Role              string
	SchoolID          *int
	NPSNInstansi      *string
	NamaInstansi      *string
	AllowedLevels     []string
	SchoolLevel       *string
	LastSeenAt        *time.Time
	TokenVersion      int
	PasswordUpdatedAt *time.Time
}

func (s AuthService) GetUserByUsername(ctx context.Context, username string) (UserRecord, error) {
	row := s.DB.Pool.QueryRow(ctx, `
        SELECT u.id, u.username, u.password_hash, u.role, u.school_id, u.npsn_instansi, u.nama_instansi,
               u.allowed_levels, u.last_seen_at, u.token_version, u.password_updated_at,
               ds.bentuk_pendidikan
        FROM users u
        LEFT JOIN data_sekolah ds ON ds.id = u.school_id
        WHERE u.username = $1
    `, username)

	var record UserRecord
	err := row.Scan(
		&record.ID,
		&record.Username,
		&record.PasswordHash,
		&record.Role,
		&record.SchoolID,
		&record.NPSNInstansi,
		&record.NamaInstansi,
		&record.AllowedLevels,
		&record.LastSeenAt,
		&record.TokenVersion,
		&record.PasswordUpdatedAt,
		&record.SchoolLevel,
	)
	if err != nil {
		logging.Errorf("auth", "query", "err=%v", err)
	}
	return record, err
}

func (s AuthService) GetUserByID(ctx context.Context, userID int) (UserRecord, error) {
	row := s.DB.Pool.QueryRow(ctx, `
        SELECT u.id, u.username, u.role, u.school_id, u.npsn_instansi, u.nama_instansi,
               u.allowed_levels, u.password_hash, u.last_seen_at, u.token_version, u.password_updated_at,
               ds.bentuk_pendidikan
        FROM users u
        LEFT JOIN data_sekolah ds ON ds.id = u.school_id
        WHERE u.id = $1
    `, userID)

	var record UserRecord
	err := row.Scan(
		&record.ID,
		&record.Username,
		&record.Role,
		&record.SchoolID,
		&record.NPSNInstansi,
		&record.NamaInstansi,
		&record.AllowedLevels,
		&record.PasswordHash,
		&record.LastSeenAt,
		&record.TokenVersion,
		&record.PasswordUpdatedAt,
		&record.SchoolLevel,
	)
	if err != nil {
		logging.Errorf("auth", "query", "err=%v", err)
	}
	return record, err
}

func (s AuthService) UsernameExists(ctx context.Context, username string, excludeID *int) (bool, error) {
	if excludeID == nil {
		var exists bool
		err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists)
		if err != nil {
			logging.Errorf("auth", "query", "err=%v", err)
		}
		return exists, err
	}

	var exists bool
	err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 AND id <> $2)`, username, *excludeID).Scan(&exists)
	if err != nil {
		logging.Errorf("auth", "query", "err=%v", err)
	}
	return exists, err
}
