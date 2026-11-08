package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

func AuthMiddleware(secret string, database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		token := ""
		if header != "" {
			parts := strings.SplitN(header, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				token = strings.TrimSpace(parts[1])
			}
		}
		if token == "" {

			if cookieToken, err := c.Cookie("cabdin_token"); err == nil {
				token = strings.TrimSpace(cookieToken)
			}
		}
		if token == "" {
			logging.Warnf("auth", "unauthorized", "path=%s reason=missing_token", c.Request.URL.Path)

			utils.RespondUnauthorized(c)
			c.Abort()
			return
		}

		claims, err := utils.ParseToken(secret, token)
		if err != nil {
			logging.Warnf("auth", "unauthorized", "path=%s reason=invalid_token err=%v", c.Request.URL.Path, err)

			utils.RespondUnauthorized(c)
			c.Abort()
			return
		}

		authService := services.AuthService{DB: database}
		record, err := authService.GetUserByID(c, claims.UserID)
		if err != nil {
			logging.Errorf("auth", "unauthorized", "path=%s reason=user_lookup_failed err=%v", c.Request.URL.Path, err)

			utils.RespondUnauthorized(c)
			c.Abort()
			return
		}
		if claims.TokenVersion != record.TokenVersion {
			logging.Warnf("auth", "unauthorized", "path=%s reason=token_revoked user_id=%d role=%s", c.Request.URL.Path, claims.UserID, claims.Role)

			c.JSON(http.StatusUnauthorized, utils.APIResponse{Error: "TOKEN_REVOKED"})
			c.Abort()
			return
		}

		normalizedRecordRole := auth.NormalizeRole(record.Role)
		supervisedSchoolIDs := []int{}
		if normalizedRecordRole == auth.RolePengawas {
			var supervisedSchoolIDsRaw []int32
			if err := database.Pool.QueryRow(c, `
				SELECT COALESCE(array_agg(id ORDER BY nama_sekolah), '{}'::int[])
				FROM data_sekolah
				WHERE supervisor_user_id = $1
			`, record.ID).Scan(&supervisedSchoolIDsRaw); err != nil {
				logging.Errorf("auth", "unauthorized", "path=%s reason=supervised_school_lookup_failed user_id=%d err=%v", c.Request.URL.Path, record.ID, err)
				utils.RespondInternalServerError(c)
				c.Abort()
				return
			}
			supervisedSchoolIDs = make([]int, 0, len(supervisedSchoolIDsRaw))
			for _, schoolID := range supervisedSchoolIDsRaw {
				supervisedSchoolIDs = append(supervisedSchoolIDs, int(schoolID))
			}
		}

		currentUser := auth.User{
			ID:                  record.ID,
			Role:                normalizedRecordRole,
			SchoolID:            record.SchoolID,
			NamaInstansi:        derefString(record.NamaInstansi),
			TokenVersion:        record.TokenVersion,
			AllowedLevels:       record.AllowedLevels,
			SupervisedSchoolIDs: supervisedSchoolIDs,
		}
		auth.SetCurrentUser(c, currentUser)
		if normalizedRecordRole == "GASPOL_VOTER" && !isAllowedGaspolVoterRequest(c.Request.Method, c.Request.URL.Path) {
			utils.RespondForbidden(c)
			c.Abort()
			return
		}
		if auth.IsPengawas(currentUser) && isMutatingMethod(c.Request.Method) && !isAllowedPengawasMutationRequest(c.Request.Method, c.Request.URL.Path) {
			utils.RespondForbidden(c)
			c.Abort()
			return
		}
		if auth.IsGaspolSchoolUser(currentUser) && !isAllowedGaspolSchoolUserRequest(c.Request.Method, c.Request.URL.Path) {
			logGaspolSchoolUserDeniedRequest(c, currentUser)
			utils.RespondForbidden(c)
			c.Abort()
			return
		}
		if auth.IsMGMPUser(currentUser) && !isAllowedMGMPUserRequest(c.Request.Method, c.Request.URL.Path) {
			logging.Warnf("authz", "deny", "[AUTHZ] deny role=%s method=%s path=%s user_id=%d", auth.RoleMGMPUser, c.Request.Method, normalizeRequestPath(c.Request.URL.Path), currentUser.ID)
			utils.RespondForbidden(c)
			c.Abort()
			return
		}
		c.Next()
	}
}

func isAllowedMGMPUserRequest(method string, path string) bool {
	normalized := normalizeRequestPath(path)
	switch {
	case normalized == "/auth/me", normalized == "/auth/logout", normalized == "/api/user/me/password":
		return true
	case strings.ToUpper(strings.TrimSpace(method)) == http.MethodGet && normalized == "/api/service-links":
		return true
	case normalized == "/api/mgmp" || strings.HasPrefix(normalized, "/api/mgmp/"):
		return true
	case strings.ToUpper(strings.TrimSpace(method)) == http.MethodGet && (normalized == "/api/system-time" || normalized == "/api/system-time/status"):
		return true
	default:
		return false
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func isAllowedGaspolVoterRequest(method, path string) bool {
	p := normalizeRequestPath(path)
	m := strings.ToUpper(method)
	switch p {
	case "/auth/me", "/auth/logout", "/api/auth/me", "/api/auth/logout":
		return true
	case "/api/user/me/password":
		return m == http.MethodPut
	case "/api/gaspol/voting":
		return m == http.MethodGet
	case "/api/gaspol/voting/vote", "/api/gaspol/voting/assistance":
		return m == http.MethodPost
	case "/api/system-time", "/api/system-time/status", "/api/service-links":
		return m == http.MethodGet
	default:
		return false
	}
}

func isMutatingMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func normalizeRequestPath(path string) string {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		return "/"
	}
	normalized = strings.TrimRight(normalized, "/")
	if normalized == "" {
		return "/"
	}
	return normalized
}

func isGaspolMutationPath(path string) bool {
	normalized := normalizeRequestPath(path)
	return normalized == "/api/gaspol" || strings.HasPrefix(normalized, "/api/gaspol/")
}

func isAllowedPengawasMutationRequest(method string, path string) bool {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	normalizedPath := normalizeRequestPath(path)
	if isGaspolMutationPath(normalizedPath) {
		return true
	}
	return normalizedMethod == http.MethodPut && normalizedPath == "/api/siswa/acceptance-offers/supervisor-snapshot"
}

func isAllowedGaspolSchoolUserPath(path string) bool {
	return isAllowedGaspolSchoolUserRequest(http.MethodGet, path)
}

func isAllowedGaspolSchoolUserRequest(method string, path string) bool {
	normalized := normalizeRequestPath(path)
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if normalizedMethod == "" {
		normalizedMethod = http.MethodGet
	}
	switch {
	case normalized == "/auth/me":
		return true
	case normalized == "/auth/logout":
		return true
	case normalized == "/api/user/me/password":
		return true
	case normalizedMethod == http.MethodGet && normalized == "/api/service-links":
		return true
	case normalizedMethod == http.MethodGet && (normalized == "/api/system-time" || normalized == "/api/system-time/status"):
		return true
	case normalized == "/api/gaspol" || strings.HasPrefix(normalized, "/api/gaspol/"):
		return true
	case normalized == "/api/public/gaspol" || strings.HasPrefix(normalized, "/api/public/gaspol/"):
		return true
	default:
		return false
	}
}

func logGaspolSchoolUserDeniedRequest(c *gin.Context, user auth.User) {
	schoolID := "null"
	if user.SchoolID != nil {
		schoolID = fmt.Sprintf("%d", *user.SchoolID)
	}
	logging.Warnf(
		"authz",
		"deny",
		"[AUTHZ] deny role=%s method=%s path=%s user_id=%d school_id=%s",
		auth.RoleGaspolSchoolUser,
		strings.ToUpper(strings.TrimSpace(c.Request.Method)),
		normalizeRequestPath(c.Request.URL.Path),
		user.ID,
		schoolID,
	)
}
