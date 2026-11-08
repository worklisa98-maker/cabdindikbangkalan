package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type SchoolHandler struct {
	DB *db.DB
}

func (h SchoolHandler) List(c *gin.Context) {
	logging.Infof("school", "list", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid school_id")
			return
		}
		requestedSchoolID = &id
	}

	service := services.SchoolService{DB: h.DB}
	result, err := service.List(c, user, services.SchoolFilters{
		SchoolID: requestedSchoolID,
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("school", "list", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]map[string]interface{}, 0, len(result))
	for _, school := range result {
		var lastUpdatePrestasi *string
		if school.LastUpdatePrestasi != nil {
			formatted := school.LastUpdatePrestasi.UTC().Format(time.RFC3339)
			lastUpdatePrestasi = &formatted
		}
		var lastUpdateSiswaPT *string
		if school.LastUpdateSiswaPT != nil {
			formatted := school.LastUpdateSiswaPT.UTC().Format(time.RFC3339)
			lastUpdateSiswaPT = &formatted
		}
		payload = append(payload, map[string]interface{}{
			"id":                  school.ID,
			"npsnSekolah":         school.NPSNSekolah,
			"namaSekolah":         school.NamaSekolah,
			"bentukPendidikan":    school.BentukPendidikan,
			"statusSekolah":       school.StatusSekolah,
			"tmtAkreditasi":       school.TMTAkreditasi,
			"akreditasi":          school.Akreditasi,
			"jumlahTotalGuru":     school.JumlahTotalGuru,
			"jumlahGuruAsn":       school.JumlahGuruAsn,
			"jumlahGuruHonorer":   school.JumlahGuruHonorer,
			"jumlahTotalTendik":   school.JumlahTotalTendik,
			"jumlahTendikAsn":     school.JumlahTendikAsn,
			"jumlahTendikHonorer": school.JumlahTendikHonorer,
			"lastUpdatePrestasi":  lastUpdatePrestasi,
			"lastUpdateSiswaPt":   lastUpdateSiswaPT,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

type SchoolUpdate struct {
	NPSNSekolah         *string `json:"npsnSekolah"`
	NamaSekolah         *string `json:"namaSekolah"`
	BentukPendidikan    *string `json:"bentukPendidikan"`
	StatusSekolah       *string `json:"statusSekolah"`
	TMTAkreditasi       *string `json:"tmtAkreditasi"`
	Akreditasi          *string `json:"akreditasi"`
	JumlahGuruAsn       *int    `json:"jumlahGuruAsn"`
	JumlahGuruHonorer   *int    `json:"jumlahGuruHonorer"`
	JumlahTendikAsn     *int    `json:"jumlahTendikAsn"`
	JumlahTendikHonorer *int    `json:"jumlahTendikHonorer"`
}

type SettingsUpdate struct {
	School *SchoolUpdatePayload `json:"school"`
	User   *UserUpdatePayload   `json:"user"`
}

type SchoolUpdatePayload struct {
	NPSNSekolah         *string `json:"npsnSekolah"`
	NamaSekolah         *string `json:"namaSekolah"`
	BentukPendidikan    *string `json:"bentukPendidikan"`
	StatusSekolah       *string `json:"statusSekolah"`
	TMTAkreditasi       *string `json:"tmtAkreditasi"`
	Akreditasi          *string `json:"akreditasi"`
	JumlahGuruAsn       *int    `json:"jumlahGuruAsn"`
	JumlahGuruHonorer   *int    `json:"jumlahGuruHonorer"`
	JumlahTendikAsn     *int    `json:"jumlahTendikAsn"`
	JumlahTendikHonorer *int    `json:"jumlahTendikHonorer"`
}

type UserUpdatePayload struct {
	Username        *string `json:"username"`
	CurrentPassword *string `json:"current_password"`
	NewPassword     *string `json:"new_password"`
}

func (h SchoolHandler) Update(c *gin.Context) {
	logging.Infof("school", "update", "event=hit")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.RespondBadRequest(c, "Invalid school id")
		return
	}

	var payload SchoolUpdate
	if err := c.ShouldBindJSON(&payload); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if payload.NPSNSekolah == nil {
		utils.RespondBadRequestMessage(c, "NPSN_REQUIRED")
		return
	}
	normalizedNPSN := utils.NormalizeNPSN(*payload.NPSNSekolah)
	if !utils.IsValidNPSN(normalizedNPSN) {
		utils.RespondBadRequestMessage(c, "NPSN_INVALID")
		return
	}
	payload.NPSNSekolah = &normalizedNPSN

	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	service := services.SchoolService{DB: h.DB}
	school, err := service.Update(c, user, id, services.SchoolUpdate{
		NPSNSekolah:         payload.NPSNSekolah,
		NamaSekolah:         payload.NamaSekolah,
		BentukPendidikan:    payload.BentukPendidikan,
		StatusSekolah:       payload.StatusSekolah,
		TMTAkreditasi:       payload.TMTAkreditasi,
		Akreditasi:          payload.Akreditasi,
		JumlahGuruAsn:       payload.JumlahGuruAsn,
		JumlahGuruHonorer:   payload.JumlahGuruHonorer,
		JumlahTendikAsn:     payload.JumlahTendikAsn,
		JumlahTendikHonorer: payload.JumlahTendikHonorer,
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("school", "update", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, map[string]interface{}{
		"id":                  school.ID,
		"npsnSekolah":         school.NPSNSekolah,
		"namaSekolah":         school.NamaSekolah,
		"bentukPendidikan":    school.BentukPendidikan,
		"statusSekolah":       school.StatusSekolah,
		"tmtAkreditasi":       school.TMTAkreditasi,
		"akreditasi":          school.Akreditasi,
		"jumlahTotalGuru":     school.JumlahTotalGuru,
		"jumlahGuruAsn":       school.JumlahGuruAsn,
		"jumlahGuruHonorer":   school.JumlahGuruHonorer,
		"jumlahTotalTendik":   school.JumlahTotalTendik,
		"jumlahTendikAsn":     school.JumlahTendikAsn,
		"jumlahTendikHonorer": school.JumlahTendikHonorer,
	})
}

func (h SchoolHandler) Settings(c *gin.Context) {
	logging.Infof("school", "settings", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "SCHOOL_ADMIN" || user.SchoolID == nil {
		utils.RespondForbidden(c)
		return
	}

	service := services.SchoolService{DB: h.DB}
	result, err := service.List(c, user, services.SchoolFilters{
		SchoolID: user.SchoolID,
	})
	if err != nil || len(result) == 0 {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		if err != nil {
			logging.Errorf("school", "settings", "err=%v", err)
		} else {
			logging.Errorf("school", "settings", "err=empty_result")
		}
		utils.RespondInternalServerError(c)
		return
	}

	authService := services.AuthService{DB: h.DB}
	userRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("school", "settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	school := result[0]
	var lastUpdatePrestasi *string
	if school.LastUpdatePrestasi != nil {
		formatted := school.LastUpdatePrestasi.UTC().Format(time.RFC3339)
		lastUpdatePrestasi = &formatted
	}
	var lastUpdateSiswaPT *string
	if school.LastUpdateSiswaPT != nil {
		formatted := school.LastUpdateSiswaPT.UTC().Format(time.RFC3339)
		lastUpdateSiswaPT = &formatted
	}
	utils.RespondData(c, http.StatusOK, map[string]interface{}{
		"school": map[string]interface{}{
			"id":                  school.ID,
			"npsnSekolah":         school.NPSNSekolah,
			"namaSekolah":         school.NamaSekolah,
			"bentukPendidikan":    school.BentukPendidikan,
			"statusSekolah":       school.StatusSekolah,
			"tmtAkreditasi":       school.TMTAkreditasi,
			"akreditasi":          school.Akreditasi,
			"jumlahTotalGuru":     school.JumlahTotalGuru,
			"jumlahGuruAsn":       school.JumlahGuruAsn,
			"jumlahGuruHonorer":   school.JumlahGuruHonorer,
			"jumlahTotalTendik":   school.JumlahTotalTendik,
			"jumlahTendikAsn":     school.JumlahTendikAsn,
			"jumlahTendikHonorer": school.JumlahTendikHonorer,
			"lastUpdatePrestasi":  lastUpdatePrestasi,
			"lastUpdateSiswaPt":   lastUpdateSiswaPT,
		},
		"user": map[string]interface{}{
			"id":       userRecord.ID,
			"username": userRecord.Username,
		},
	})
}

func (h SchoolHandler) UpdateSettings(c *gin.Context) {
	logging.Infof("school", "update_settings", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "SCHOOL_ADMIN" || user.SchoolID == nil {
		utils.RespondForbidden(c)
		return
	}

	var payload SettingsUpdate
	if err := c.ShouldBindJSON(&payload); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}

	authService := services.AuthService{DB: h.DB}
	userRecord, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("school", "update_settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	var schoolUpdate *services.SchoolUpdate
	if payload.School != nil {
		if payload.School.NPSNSekolah == nil {
			utils.RespondBadRequestMessage(c, "NPSN_REQUIRED")
			return
		}
		normalizedNPSN := utils.NormalizeNPSN(*payload.School.NPSNSekolah)
		if !utils.IsValidNPSN(normalizedNPSN) {
			utils.RespondBadRequestMessage(c, "NPSN_INVALID")
			return
		}
		payload.School.NPSNSekolah = &normalizedNPSN
		schoolUpdate = &services.SchoolUpdate{
			NPSNSekolah:         payload.School.NPSNSekolah,
			NamaSekolah:         payload.School.NamaSekolah,
			BentukPendidikan:    payload.School.BentukPendidikan,
			StatusSekolah:       payload.School.StatusSekolah,
			TMTAkreditasi:       payload.School.TMTAkreditasi,
			Akreditasi:          payload.School.Akreditasi,
			JumlahGuruAsn:       payload.School.JumlahGuruAsn,
			JumlahGuruHonorer:   payload.School.JumlahGuruHonorer,
			JumlahTendikAsn:     payload.School.JumlahTendikAsn,
			JumlahTendikHonorer: payload.School.JumlahTendikHonorer,
		}
	}

	var userUpdate *services.UserAccountUpdate
	var newUsername string
	var newPassword string
	usernameChanged := false
	passwordChanged := false
	if payload.User != nil {
		if payload.User.Username != nil {
			newUsername = strings.TrimSpace(*payload.User.Username)
			if newUsername == "" {
				utils.RespondBadRequestMessage(c, "USERNAME_REQUIRED")
				return
			}
			if newUsername != userRecord.Username {
				usernameChanged = true
			}
		}
		if payload.User.NewPassword != nil {
			newPassword = strings.TrimSpace(*payload.User.NewPassword)
			if newPassword != "" {
				passwordChanged = true
			}
		}
	}

	if usernameChanged || passwordChanged {
		currentPassword := ""
		if payload.User != nil && payload.User.CurrentPassword != nil {
			currentPassword = strings.TrimSpace(*payload.User.CurrentPassword)
		}
		if currentPassword == "" {
			utils.RespondBadRequestMessage(c, "PASSWORD_LAMA_REQUIRED")
			return
		}

		now := time.Now()
		if !settingsRateLimiter.Allow(user.ID, now) {
			utils.RespondTooManyRequests(c)
			return
		}

		if !utils.VerifyPassword(currentPassword, userRecord.PasswordHash) {
			settingsRateLimiter.RecordFailure(user.ID, now)
			utils.RespondBadRequestMessage(c, "PASSWORD_LAMA_SALAH")
			return
		}
		settingsRateLimiter.Reset(user.ID)
	}

	if usernameChanged {
		exists, err := authService.UsernameExists(c, newUsername, &user.ID)
		if err != nil {
			logging.Errorf("school", "update_settings", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		if exists {
			utils.RespondBadRequestMessage(c, "USERNAME_TAK_TERSEDIA")
			return
		}
		userUpdate = &services.UserAccountUpdate{UserID: user.ID, Username: &newUsername}
	}

	if passwordChanged {
		if len(newPassword) < 8 {
			utils.RespondBadRequestMessage(c, "PASSWORD_MIN_8")
			return
		}
		hash, err := utils.HashPassword(newPassword)
		if err != nil {
			logging.Errorf("school", "update_settings", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
		if userUpdate == nil {
			userUpdate = &services.UserAccountUpdate{UserID: user.ID}
		}
		userUpdate.PasswordHash = &hash
	}

	settingsService := services.SettingsService{DB: h.DB}
	if err := settingsService.UpdateSettings(c, user, schoolUpdate, userUpdate); err != nil {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		logging.Errorf("school", "update_settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	schoolService := services.SchoolService{DB: h.DB}
	result, err := schoolService.List(c, user, services.SchoolFilters{SchoolID: user.SchoolID})
	if err != nil || len(result) == 0 {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		if err != nil {
			logging.Errorf("school", "update_settings", "err=%v", err)
		} else {
			logging.Errorf("school", "update_settings", "err=empty_result")
		}
		utils.RespondInternalServerError(c)
		return
	}

	userRecord, err = authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("school", "update_settings", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	school := result[0]
	utils.RespondData(c, http.StatusOK, map[string]interface{}{
		"school": map[string]interface{}{
			"id":                  school.ID,
			"npsnSekolah":         school.NPSNSekolah,
			"namaSekolah":         school.NamaSekolah,
			"bentukPendidikan":    school.BentukPendidikan,
			"statusSekolah":       school.StatusSekolah,
			"tmtAkreditasi":       school.TMTAkreditasi,
			"akreditasi":          school.Akreditasi,
			"jumlahTotalGuru":     school.JumlahTotalGuru,
			"jumlahGuruAsn":       school.JumlahGuruAsn,
			"jumlahGuruHonorer":   school.JumlahGuruHonorer,
			"jumlahTotalTendik":   school.JumlahTotalTendik,
			"jumlahTendikAsn":     school.JumlahTendikAsn,
			"jumlahTendikHonorer": school.JumlahTendikHonorer,
		},
		"user": map[string]interface{}{
			"id":       userRecord.ID,
			"username": userRecord.Username,
		},
	})
}
