package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type AuthHandler struct {
	DB        *db.DB
	JWTSecret string
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userResponse struct {
	ID            int      `json:"id"`
	Username      string   `json:"username"`
	Role          string   `json:"role"`
	SchoolID      *int     `json:"schoolId"`
	NPSNInstansi  string   `json:"npsnInstansi,omitempty"`
	NamaInstansi  string   `json:"namaInstansi,omitempty"`
	AllowedLevels []string `json:"allowedLevels,omitempty"`
	SchoolLevel   string   `json:"school_level,omitempty"`
}

func (h AuthHandler) Login(c *gin.Context) {
	logging.Infof("auth", "login", "event=hit")
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	if req.Username == "" || req.Password == "" {
		utils.RespondBadRequest(c, "Username and password are required")
		return
	}

	authService := services.AuthService{DB: h.DB}
	record, err := authService.GetUserByUsername(c, req.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			utils.RespondUnauthorized(c)
			return
		}
		logging.Errorf("auth", "login", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	if !utils.VerifyPassword(req.Password, record.PasswordHash) {
		utils.RespondUnauthorized(c)
		return
	}

	token, err := utils.GenerateToken(
		h.JWTSecret,
		record.ID,
		record.Role,
		record.SchoolID,
		derefString(record.NamaInstansi),
		record.TokenVersion,
	)
	if err != nil {
		logging.Errorf("auth", "login", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	user := userResponse{
		ID:            record.ID,
		Username:      record.Username,
		Role:          record.Role,
		SchoolID:      record.SchoolID,
		NPSNInstansi:  derefString(record.NPSNInstansi),
		NamaInstansi:  derefString(record.NamaInstansi),
		AllowedLevels: record.AllowedLevels,
		SchoolLevel:   normalizeSchoolLevel(record.SchoolLevel),
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token": token,
			"user":  user,
		},
	})
}

func (h AuthHandler) Me(c *gin.Context) {
	logging.Infof("auth", "me", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	authService := services.AuthService{DB: h.DB}
	record, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			utils.RespondUnauthorized(c)
			return
		}
		logging.Errorf("auth", "me", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, userResponse{
		ID:            record.ID,
		Username:      record.Username,
		Role:          record.Role,
		SchoolID:      record.SchoolID,
		NPSNInstansi:  derefString(record.NPSNInstansi),
		NamaInstansi:  derefString(record.NamaInstansi),
		AllowedLevels: record.AllowedLevels,
		SchoolLevel:   normalizeSchoolLevel(record.SchoolLevel),
	})
}

func (h AuthHandler) Logout(c *gin.Context) {
	logging.Infof("auth", "logout", "event=hit")
	header := c.GetHeader("Authorization")
	if header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			if claims, err := utils.ParseToken(h.JWTSecret, parts[1]); err == nil {
				_, _ = h.DB.Pool.Exec(c, `
                    UPDATE users
                    SET last_seen_at = NULL
                    WHERE id = $1
                `, claims.UserID)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "logout success",
	})
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeSchoolLevel(value *string) string {
	if value == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(*value))
}
