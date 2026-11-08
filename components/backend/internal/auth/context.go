package auth

import (
	"errors"

	"github.com/gin-gonic/gin"
)

const contextKey = "user"

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
)

type User struct {
	ID           int
	Role         string
	SchoolID     *int
	NamaInstansi string
	TokenVersion int
	AllowedLevels []string
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
	switch user.Role {
	case "SCHOOL_ADMIN":
		if user.SchoolID == nil {
			return nil, ErrForbidden
		}
		if requestedSchoolID != nil && *requestedSchoolID != *user.SchoolID {
			return nil, ErrForbidden
		}
		return user.SchoolID, nil
	case "ADMIN", "ADMIN_MASTER":
		if requestedSchoolID != nil && *requestedSchoolID == 0 {
			return nil, ErrForbidden
		}
		return requestedSchoolID, nil
	default:
		return nil, ErrForbidden
	}
}
