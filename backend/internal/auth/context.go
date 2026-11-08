package auth

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/db"
)

const contextKey = "user"

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

type User struct {
	ID                  int
	Role                string
	SchoolID            *int
	NamaInstansi        string
	TokenVersion        int
	AllowedLevels       []string
	SupervisedSchoolIDs []int
}

type SchoolScope struct {
	All bool
	IDs []int
}

type MultiSchoolScope struct {
	Role      string
	All       bool
	SchoolIDs []int
}

func SetCurrentUser(c *gin.Context, user User) {
	c.Set(contextKey, user)
}

func GetCurrentUser(c *gin.Context) (User, error) {
	value, ok := c.Get(contextKey)
	if !ok {
		return User{}, ErrUnauthorized
	}
	user, ok := value.(User)
	if !ok {
		return User{}, ErrUnauthorized
	}
	return user, nil
}

func GetSchoolScope(user User, requestedSchoolID *int) (*int, error) {
	return GetSingleSchoolScopeV2(user, requestedSchoolID)
}

func GetMultiSchoolScope(ctx context.Context, database *db.DB, user User) (MultiSchoolScope, error) {
	role := NormalizeRole(user.Role)
	if role == "" {
		return MultiSchoolScope{}, ErrForbidden
	}

	switch role {
	case RoleAdminMaster:
		return MultiSchoolScope{
			Role: role,
			All:  true,
		}, nil
	case RoleAdminCabdin:
		allowed := NormalizeAllowedLevels(user.AllowedLevels)
		if len(allowed) == 0 {
			return MultiSchoolScope{
				Role:      role,
				SchoolIDs: []int{},
			}, nil
		}
		var rawIDs []int32
		if err := database.Pool.QueryRow(ctx, `
			SELECT COALESCE(array_agg(id ORDER BY nama_sekolah), '{}'::int[])
			FROM data_sekolah
			WHERE UPPER(TRIM(COALESCE(bentuk_pendidikan, ''))) = ANY($1)
		`, allowed).Scan(&rawIDs); err != nil {
			return MultiSchoolScope{}, err
		}
		schoolIDs := make([]int, 0, len(rawIDs))
		for _, id := range rawIDs {
			if id > 0 {
				schoolIDs = append(schoolIDs, int(id))
			}
		}
		return MultiSchoolScope{
			Role:      role,
			SchoolIDs: schoolIDs,
		}, nil
	case RolePengawas:
		scopeIDs := make([]int, 0, len(user.SupervisedSchoolIDs))
		for _, schoolID := range user.SupervisedSchoolIDs {
			if schoolID > 0 {
				scopeIDs = append(scopeIDs, schoolID)
			}
		}
		return MultiSchoolScope{
			Role:      role,
			SchoolIDs: scopeIDs,
		}, nil
	case RoleSchoolAdmin:
		if user.SchoolID == nil || *user.SchoolID <= 0 {
			return MultiSchoolScope{}, ErrForbidden
		}
		return MultiSchoolScope{
			Role:      role,
			SchoolIDs: []int{*user.SchoolID},
		}, nil
	default:
		return MultiSchoolScope{}, ErrForbidden
	}
}

func EnsureSchoolInScope(scope MultiSchoolScope, schoolID int) error {
	if schoolID <= 0 {
		return ErrForbidden
	}
	if scope.All {
		return nil
	}
	if containsSchoolID(scope.SchoolIDs, schoolID) {
		return nil
	}
	return ErrForbidden
}

func GetSingleSchoolScopeV2(user User, requestedSchoolID *int) (*int, error) {
	scope, err := GetSchoolScopeV2(user, requestedSchoolID)
	if err != nil {
		return nil, ErrForbidden
	}
	if scope.All {
		return nil, nil
	}
	if len(scope.IDs) != 1 {
		return nil, ErrForbidden
	}
	schoolID := scope.IDs[0]
	return &schoolID, nil
}

func GetSchoolScopeV2(user User, requestedSchoolID *int) (SchoolScope, error) {
	switch NormalizeRole(user.Role) {
	case RoleSchoolAdmin:
		if user.SchoolID == nil {
			return SchoolScope{}, ErrForbidden
		}
		if requestedSchoolID != nil && *requestedSchoolID != *user.SchoolID {
			return SchoolScope{}, ErrForbidden
		}
		return SchoolScope{IDs: []int{*user.SchoolID}}, nil
	case RoleAdminCabdin, RoleAdminMaster:
		if requestedSchoolID == nil {
			return SchoolScope{All: true}, nil
		}
		if *requestedSchoolID <= 0 {
			return SchoolScope{}, ErrForbidden
		}
		return SchoolScope{IDs: []int{*requestedSchoolID}}, nil
	case RolePengawas:
		if len(user.SupervisedSchoolIDs) == 0 {
			return SchoolScope{}, ErrForbidden
		}
		if requestedSchoolID != nil {
			if *requestedSchoolID <= 0 || !containsSchoolID(user.SupervisedSchoolIDs, *requestedSchoolID) {
				return SchoolScope{}, ErrForbidden
			}
			return SchoolScope{IDs: []int{*requestedSchoolID}}, nil
		}
		return SchoolScope{IDs: append([]int(nil), user.SupervisedSchoolIDs...)}, nil
	default:
		return SchoolScope{}, ErrForbidden
	}
}

func containsSchoolID(ids []int, target int) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
