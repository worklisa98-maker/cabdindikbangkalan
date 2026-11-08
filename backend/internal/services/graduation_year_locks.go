package services

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

type GraduationYearLock struct {
	SchoolID   *int       `json:"school_id,omitempty"`
	TahunLulus int        `json:"tahun_lulus"`
	IsClosed   bool       `json:"is_closed"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
	ClosedBy   *int       `json:"closed_by,omitempty"`
}

type GraduationYearLockSummary struct {
	SchoolID      *int       `json:"school_id,omitempty"`
	TahunLulus    int        `json:"tahun_lulus"`
	IsClosed      bool       `json:"is_closed"`
	Locked        bool       `json:"locked"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
	ClosedBy      *int       `json:"closed_by,omitempty"`
	TotalSchools  int        `json:"total_schools"`
	LockedSchools int        `json:"locked_schools"`
}

func newGraduationYearLockSummary(tahun int) *GraduationYearLockSummary {
	return &GraduationYearLockSummary{
		TahunLulus:    tahun,
		IsClosed:      false,
		Locked:        false,
		TotalSchools:  0,
		LockedSchools: 0,
	}
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

func (s SiswaService) GetGraduationYearLockSummary(
	ctx context.Context,
	scope auth.MultiSchoolScope,
	requestedSchoolID *int,
	tahun int,
) (*GraduationYearLockSummary, error) {
	if requestedSchoolID != nil || scope.Role == auth.RoleSchoolAdmin {
		return s.getGraduationYearLockSingleSummary(ctx, scope, requestedSchoolID, tahun)
	}
	return s.getGraduationYearLockAggregateSummary(ctx, scope, tahun)
}

func (s SiswaService) getGraduationYearLockSingleSummary(
	ctx context.Context,
	scope auth.MultiSchoolScope,
	requestedSchoolID *int,
	tahun int,
) (*GraduationYearLockSummary, error) {
	summary := newGraduationYearLockSummary(tahun)

	var schoolID *int
	if requestedSchoolID != nil {
		resolved := *requestedSchoolID
		schoolID = &resolved
	} else if len(scope.SchoolIDs) > 0 {
		resolved := scope.SchoolIDs[0]
		schoolID = &resolved
	}
	if schoolID == nil || *schoolID <= 0 {
		return summary, nil
	}

	lock, err := s.GetGraduationYearLock(ctx, schoolID, tahun)
	if err != nil {
		return nil, err
	}

	resolved := *schoolID
	summary.SchoolID = &resolved
	summary.ClosedAt = lock.ClosedAt
	summary.ClosedBy = lock.ClosedBy
	summary.TotalSchools = 1
	if lock.IsClosed {
		summary.LockedSchools = 1
	}
	summary.IsClosed = lock.IsClosed
	summary.Locked = lock.IsClosed
	return summary, nil
}

func (s SiswaService) getGraduationYearLockAggregateSummary(
	ctx context.Context,
	scope auth.MultiSchoolScope,
	tahun int,
) (*GraduationYearLockSummary, error) {
	summary := newGraduationYearLockSummary(tahun)
	if tahun <= 0 {
		return summary, nil
	}

	var totalSchools int
	if scope.All {
		ok, _, err := tablesExist(ctx, s.DB, "data_sekolah")
		if err != nil {
			return nil, err
		}
		if !ok {
			return summary, nil
		}
		if err := s.DB.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM data_sekolah`).Scan(&totalSchools); err != nil {
			return nil, err
		}
	} else {
		scopeIDs := normalizeLockScopeSchoolIDs(scope.SchoolIDs)
		totalSchools = len(scopeIDs)
	}

	summary.TotalSchools = totalSchools
	if totalSchools == 0 {
		return summary, nil
	}

	ok, _, err := tablesExist(ctx, s.DB, "graduation_year_locks")
	if err != nil {
		return nil, err
	}
	if !ok {
		return summary, nil
	}

	var globalClosed bool
	if err := s.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM graduation_year_locks
			WHERE school_id IS NULL
			  AND tahun_lulus = $1
			  AND is_closed = TRUE
		)
	`, tahun).Scan(&globalClosed); err != nil {
		return nil, err
	}
	if globalClosed {
		summary.LockedSchools = totalSchools
		summary.IsClosed = true
		summary.Locked = true
		return summary, nil
	}

	var lockedSchools int
	if scope.All {
		if err := s.DB.Pool.QueryRow(ctx, `
			SELECT COUNT(DISTINCT gyl.school_id)
			FROM graduation_year_locks gyl
			JOIN data_sekolah ds ON ds.id = gyl.school_id
			WHERE gyl.tahun_lulus = $1
			  AND gyl.is_closed = TRUE
		`, tahun).Scan(&lockedSchools); err != nil {
			return nil, err
		}
	} else {
		scopeIDs := normalizeLockScopeSchoolIDs(scope.SchoolIDs)
		int32IDs := make([]int32, 0, len(scopeIDs))
		for _, id := range scopeIDs {
			int32IDs = append(int32IDs, int32(id))
		}
		if err := s.DB.Pool.QueryRow(ctx, `
			SELECT COUNT(DISTINCT school_id)
			FROM graduation_year_locks
			WHERE tahun_lulus = $1
			  AND is_closed = TRUE
			  AND school_id = ANY($2::int[])
		`, tahun, int32IDs).Scan(&lockedSchools); err != nil {
			return nil, err
		}
	}

	if lockedSchools > totalSchools {
		lockedSchools = totalSchools
	}
	summary.LockedSchools = lockedSchools
	summary.IsClosed = lockedSchools == totalSchools
	summary.Locked = summary.IsClosed
	return summary, nil
}

func normalizeLockScopeSchoolIDs(ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
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

func (s SiswaService) EnsureYearNotClosed(ctx context.Context, schoolID int, tahun int) error {
	return s.ensureYearOpen(ctx, schoolID, tahun)
}
