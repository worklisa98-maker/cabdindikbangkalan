package middleware

import (
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

		auth.SetCurrentUser(c, auth.User{
			ID:            record.ID,
			Role:          record.Role,
			SchoolID:      record.SchoolID,
			NamaInstansi:  derefString(record.NamaInstansi),
			TokenVersion:  record.TokenVersion,
			AllowedLevels: record.AllowedLevels,
		})
		c.Next()
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
