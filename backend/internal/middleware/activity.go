package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

func ActivityMiddleware(database *db.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := auth.GetCurrentUser(c)
		if err == nil {
			_, _ = database.Pool.Exec(c, `
                UPDATE users
                SET last_seen_at = $1
                WHERE id = $2
            `, time.Now().UTC(), user.ID)
		}
		c.Next()
	}
}
