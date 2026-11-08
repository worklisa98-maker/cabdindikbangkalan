package services

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/db"
)

type GraduationYearLock struct {
	SchoolID   *int       `json:"school_id,omitempty"`
	TahunLulus int        `json:"tahun_lulus"`
	IsClosed   bool       `json:"is_closed"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
	ClosedBy   *int       `json:"closed_by,omitempty"`
}

func IsYearClosed(ctx context.Context, database *db.DB, schoolID int, tahun int) (bool, error) {
	if tahun <= 0 {
		return false, nil
	}
	ok, _, err := tablesExist(ctx, database, "graduation_year_locks")
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	query := `
		SELECT 1
		FROM graduation_year_locks
		WHERE tahun_lulus = $1
		  AND is_closed = TRUE
	`
	args := []interface{}{tahun}
	if schoolID > 0 {
		query += " AND (school_id = $2 OR school_id IS NULL)"
		args = append(args, schoolID)
	} else {
		query += " AND school_id IS NULL"
	}
	query += " LIMIT 1"

	var exists int
	if err := database.Pool.QueryRow(ctx, query, args...).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s SiswaService) IsYearClosed(ctx context.Context, schoolID int, tahun int) (bool, error) {
	return IsYearClosed(ctx, s.DB, schoolID, tahun)
}

func (s SiswaService) GetGraduationYearLock(ctx context.Context, schoolID *int, tahun int) (*GraduationYearLock, error) {
	lock := &GraduationYearLock{
		SchoolID:   schoolID,
		TahunLulus: tahun,
		IsClosed:   false,
	}
	if tahun <= 0 {
		return lock, nil
	}
	ok, _, err := tablesExist(ctx, s.DB, "graduation_year_locks")
	if err != nil {
		return nil, err
	}
	if !ok {
		return lock, nil
	}

	query := `
		SELECT school_id, tahun_lulus, is_closed, closed_at, closed_by
		FROM graduation_year_locks
		WHERE tahun_lulus = $1
	`
	args := []interface{}{tahun}
	if schoolID != nil {
		query += " AND (school_id = $2 OR school_id IS NULL)"
		args = append(args, *schoolID)
	} else {
		query += " AND school_id IS NULL"
	}
	query += " ORDER BY (school_id IS NULL) ASC, closed_at DESC NULLS LAST, id DESC LIMIT 1"

	var storedSchoolID *int
	var closedAt *time.Time
	var closedBy *int
	var isClosed bool
	var storedYear int
	if err := s.DB.Pool.QueryRow(ctx, query, args...).Scan(&storedSchoolID, &storedYear, &isClosed, &closedAt, &closedBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return lock, nil
		}
		return nil, err
	}

	lock.SchoolID = storedSchoolID
	lock.TahunLulus = storedYear
	lock.IsClosed = isClosed
	lock.ClosedAt = closedAt
	lock.ClosedBy = closedBy
	return lock, nil
}

func (s SiswaService) CloseGraduationYear(ctx context.Context, schoolID *int, tahun int, userID int) (*GraduationYearLock, error) {
	if tahun <= 0 {
		return nil, errors.New("tahun_lulus wajib diisi")
	}
	ok, _, err := tablesExist(ctx, s.DB, "graduation_year_locks")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("graduation_year_locks not available")
	}

	lock := &GraduationYearLock{
		SchoolID:   schoolID,
		TahunLulus: tahun,
	}

	if schoolID == nil {
		var id int64
		err := s.DB.Pool.QueryRow(ctx, `
			SELECT id
			FROM graduation_year_locks
			WHERE school_id IS NULL AND tahun_lulus = $1
			ORDER BY closed_at DESC NULLS LAST, id DESC
			LIMIT 1
		`, tahun).Scan(&id)
		if err == nil {
			if err := s.DB.Pool.QueryRow(ctx, `
				UPDATE graduation_year_locks
				SET is_closed = TRUE,
				    closed_at = COALESCE(closed_at, NOW()),
				    closed_by = COALESCE(closed_by, $2),
				    updated_at = NOW()
				WHERE id = $1
				RETURNING is_closed, closed_at, closed_by
			`, id, userID).Scan(&lock.IsClosed, &lock.ClosedAt, &lock.ClosedBy); err != nil {
				return nil, err
			}
			return lock, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}

		if err := s.DB.Pool.QueryRow(ctx, `
			INSERT INTO graduation_year_locks (
				school_id, tahun_lulus, is_closed, closed_at, closed_by
			) VALUES (NULL, $1, TRUE, NOW(), $2)
			RETURNING is_closed, closed_at, closed_by
		`, tahun, userID).Scan(&lock.IsClosed, &lock.ClosedAt, &lock.ClosedBy); err != nil {
			return nil, err
		}
		return lock, nil
	}

	if err := s.DB.Pool.QueryRow(ctx, `
		INSERT INTO graduation_year_locks (
			school_id, tahun_lulus, is_closed, closed_at, closed_by
		) VALUES ($1, $2, TRUE, NOW(), $3)
		ON CONFLICT (school_id, tahun_lulus)
		DO UPDATE SET
			is_closed = TRUE,
			closed_at = COALESCE(graduation_year_locks.closed_at, EXCLUDED.closed_at),
			closed_by = COALESCE(graduation_year_locks.closed_by, EXCLUDED.closed_by),
			updated_at = NOW()
		RETURNING is_closed, closed_at, closed_by
	`, *schoolID, tahun, userID).Scan(&lock.IsClosed, &lock.ClosedAt, &lock.ClosedBy); err != nil {
		return nil, err
	}
	return lock, nil
}

func (s SiswaService) ensureYearOpen(ctx context.Context, schoolID int, tahun int) error {
	closed, err := s.IsYearClosed(ctx, schoolID, tahun)
	if err != nil {
		return err
	}
	if closed {
		return ErrYearClosed
	}
	return nil
}
