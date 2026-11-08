package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

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
	startedAt := time.Now()
	var (
		username               string
		outcome                = "unknown"
		lookupDuration         time.Duration
		passwordVerifyDuration time.Duration
	)
	defer func() {
		totalDuration := time.Since(startedAt)
		status := c.Writer.Status()
		if status == 0 {
			status = http.StatusOK
		}

		logging.Infof(
			"auth",
			"login",
			"event=login_timing username=%q outcome=%s status=%d lookup_ms=%d password_verify_ms=%d total_ms=%d",
			username,
			outcome,
			status,
			durationMilliseconds(lookupDuration),
			durationMilliseconds(passwordVerifyDuration),
			durationMilliseconds(totalDuration),
		)

		slowStages := make([]string, 0, 3)
		if lookupDuration >= loginLookupSlowThreshold {
			slowStages = append(slowStages, "user_lookup")
		}
		if passwordVerifyDuration >= loginPasswordVerifyThreshold {
			slowStages = append(slowStages, "password_verify")
		}
		if totalDuration >= loginSlowThreshold {
			slowStages = append(slowStages, "total")
		}
		if len(slowStages) == 0 {
			return
		}

		logging.Warnf(
			"auth",
			"login",
			"event=slow_login username=%q outcome=%s status=%d slow_stage=%q lookup_ms=%d password_verify_ms=%d total_ms=%d total_threshold_ms=%d",
			username,
			outcome,
			status,
			strings.Join(slowStages, ","),
			durationMilliseconds(lookupDuration),
			durationMilliseconds(passwordVerifyDuration),
			durationMilliseconds(totalDuration),
			durationMilliseconds(loginSlowThreshold),
		)
		logPoolStatsIfAvailable(h.DB, logging.Warnf, "auth", "login", "event=slow_login_pool_stats username=%q", username)
	}()

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		outcome = "validation_error"
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	username = strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		outcome = "validation_error"
		utils.RespondBadRequest(c, "Username and password are required")
		return
	}

	ctx, cancel := requestContextWithTimeout(c, authRequestTimeout)
	defer cancel()

	authService := services.AuthService{DB: h.DB}
	lookupStartedAt := time.Now()
	record, err := authService.GetUserByUsername(ctx, username)
	lookupDuration = time.Since(lookupStartedAt)
	if err != nil {
		if isAuthRecordNotFound(err) {
			outcome = "invalid_credentials"
			utils.RespondUnauthorized(c)
			return
		}
		if isTimeoutError(err) {
			outcome = "timeout"
		} else if isDependencyUnavailableError(err) {
			outcome = "dependency_unavailable"
		} else {
			outcome = "internal_error"
		}
		if handleOperationalError(c, h.DB, "auth", "login", err) {
			return
		}
		logging.Errorf("auth", "login", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	passwordVerifyStartedAt := time.Now()
	if !utils.VerifyPassword(req.Password, record.PasswordHash) {
		passwordVerifyDuration = time.Since(passwordVerifyStartedAt)
		outcome = "invalid_credentials"
		utils.RespondUnauthorized(c)
		return
	}
	passwordVerifyDuration = time.Since(passwordVerifyStartedAt)

	token, err := utils.GenerateToken(
		h.JWTSecret,
		record.ID,
		record.Role,
		record.SchoolID,
		derefString(record.NamaInstansi),
		record.TokenVersion,
	)
	if err != nil {
		outcome = "internal_error"
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
	outcome = "success"

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
	ctx, cancel := requestContextWithTimeout(c, authRequestTimeout)
	defer cancel()

	record, err := authService.GetUserByID(ctx, user.ID)
	if err != nil {
		if isAuthRecordNotFound(err) {
			utils.RespondUnauthorized(c)
			return
		}
		if handleOperationalError(c, h.DB, "auth", "me", err) {
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
	ctx, cancel := requestContextWithTimeout(c, authRequestTimeout)
	defer cancel()

	header := c.GetHeader("Authorization")
	if header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			if claims, err := utils.ParseToken(h.JWTSecret, parts[1]); err == nil {
				if _, err := h.DB.Pool.Exec(ctx, `
                    UPDATE users
                    SET last_seen_at = NULL
                    WHERE id = $1
                `, claims.UserID); err != nil {
					logging.Warnf("auth", "logout", "event=last_seen_update_failed err=%v", err)
				}
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

func isAuthRecordNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}
