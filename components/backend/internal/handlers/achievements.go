package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type AchievementHandler struct {
	DB *db.DB
}

type achievementListResponse struct {
	IDPrestasi       string `json:"idPrestasi"`
	SchoolID         int    `json:"schoolId"`
	NamaSekolah      string `json:"namaSekolah"`
	NamaLomba        string `json:"namaLomba"`
	TingkatLomba     string `json:"tingkatLomba"`
	KategoriPrestasi string `json:"kategoriPrestasi"`
	Peringkat        string `json:"peringkatPrestasi"`
	NamaPeserta      string `json:"namaPeserta"`
	JenisLomba       string `json:"jenisLomba"`
	TahunLomba       string `json:"tahunLomba"`
}

type achievementUpdateRequest struct {
	Password string `json:"password"`
	Payload  struct {
		NamaLomba        string `json:"namaLomba"`
		TingkatLomba     string `json:"tingkatLomba"`
		KategoriPrestasi string `json:"kategoriPrestasi"`
		Peringkat        string `json:"peringkatPrestasi"`
		NamaPeserta      string `json:"namaPeserta"`
		JenisLomba       string `json:"jenisLomba"`
	} `json:"payload"`
}

type achievementAdminSummaryResponse struct {
	RekapJenjangTingkat []services.AchievementJenjangTingkat `json:"rekapJenjangTingkat"`
	DetailSekolah       []services.AchievementSchoolDetail   `json:"detailSekolah"`
}

func (h AchievementHandler) List(c *gin.Context) {
	logging.Infof("achievement", "list", "event=hit")
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

	filters := services.AchievementFilters{
		SchoolID: requestedSchoolID,
		Jenjang:  strings.TrimSpace(c.Query("jenjang")),
		Tahun:    strings.TrimSpace(c.Query("tahun")),
		Bentuk:   strings.TrimSpace(c.Query("bentuk")),
	}

	service := services.AchievementService{DB: h.DB}
	result, err := service.List(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("achievement", "list", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]map[string]interface{}, 0, len(result))
	for _, item := range result {
		payload = append(payload, map[string]interface{}{
			"idPrestasi":        item.IDPrestasi,
			"schoolId":          item.SchoolID,
			"namaSekolah":       item.NamaSekolah,
			"jenisLomba":        item.JenisLomba,
			"kategoriPrestasi":  item.KategoriPrestasi,
			"jenjangPendidikan": item.Jenjang,
			"tingkatLomba":      item.TingkatLomba,
			"tahunLomba":        item.TahunLomba,
			"namaLomba":         item.NamaLomba,
			"peringkatPrestasi": item.Peringkat,
		})
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AchievementHandler) ListTable(c *gin.Context) {
	logging.Infof("achievement", "list_table", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}

	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("sekolahId")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid sekolahId")
			return
		}
		requestedSchoolID = &id
	}

	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))
	if pageSize <= 0 {
		pageSize = 20
	}

	filters := services.AchievementListFilters{
		SchoolID:   requestedSchoolID,
		Year:       strings.TrimSpace(c.Query("year")),
		Tingkat:    strings.TrimSpace(c.Query("tingkat")),
		JenisPeserta: strings.TrimSpace(c.Query("jenis_peserta")),
		Bentuk:     strings.TrimSpace(c.Query("bentuk")),
		Search:     strings.TrimSpace(c.Query("q")),
		Page:       page,
		PageSize:   pageSize,
	}

	service := services.AchievementService{DB: h.DB}
	rows, total, err := service.ListTable(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("achievement", "list_table", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]achievementListResponse, 0, len(rows))
	for _, item := range rows {
		payload = append(payload, achievementListResponse{
			IDPrestasi:       item.IDPrestasi,
			SchoolID:         item.SchoolID,
			NamaSekolah:      item.NamaSekolah,
			NamaLomba:        item.NamaLomba,
			TingkatLomba:     item.TingkatLomba,
			KategoriPrestasi: item.KategoriPrestasi,
			Peringkat:        item.Peringkat,
			NamaPeserta:      item.NamaPeserta,
			JenisLomba:       item.JenisLomba,
			TahunLomba:       item.TahunLomba,
		})
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"data": payload,
		"meta": gin.H{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
	})
}

func (h AchievementHandler) Export(c *gin.Context) {
	logging.Infof("achievement", "export", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) && user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("sekolahId")); raw != "" {
		id, convErr := strconv.Atoi(raw)
		if convErr != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid sekolahId")
			return
		}
		requestedSchoolID = &id
	}

	filters := services.AchievementExportFilters{
		SchoolID:     requestedSchoolID,
		Year:         strings.TrimSpace(c.Query("year")),
		Tingkat:      strings.TrimSpace(c.Query("tingkat")),
		JenisPeserta: strings.TrimSpace(c.Query("jenis_peserta")),
		Bentuk:       strings.TrimSpace(c.Query("bentuk")),
		Search:       strings.TrimSpace(c.Query("q")),
	}

	const maxExportRows = 50000
	service := services.AchievementService{DB: h.DB}
	rows, total, err := service.ExportRows(c, user, filters, maxExportRows)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrExportLimitExceeded):
			utils.RespondBadRequest(c, fmt.Sprintf("Export melebihi batas %d baris (total %d). Gunakan filter untuk memperkecil data.", maxExportRows, total))
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("achievement", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	file := excelize.NewFile()
	sheet := "Prestasi"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Nama_Sekolah",
		"Nama_Lomba",
		"Tingkat_Lomba",
		"Kategori_Prestasi",
		"Peringkat_Prestasi",
		"Nama_Peserta",
		"Tahun_Lomba",
		"Mode_Pelaksanaan",
		"Penyelenggaraan_Lomba",
		"Jenis_Peserta",
		"Jenis_Lomba",
	}

	for i, header := range headers {
		cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
		if cellErr != nil {
			logging.Errorf("achievement", "export", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			logging.Errorf("achievement", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	for i, row := range rows {
		rowValues := []interface{}{
			row.NamaSekolah,
			row.NamaLomba,
			row.TingkatLomba,
			row.KategoriPrestasi,
			row.PeringkatPrestasi,
			row.NamaPeserta,
			row.TahunLomba,
			row.ModePelaksanaan,
			row.Penyelenggaraan,
			row.JenisPeserta,
			row.JenisLomba,
		}
		cell, cellErr := excelize.CoordinatesToCellName(1, i+2)
		if cellErr != nil {
			logging.Errorf("achievement", "export", "err=%v", cellErr)
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			logging.Errorf("achievement", "export", "err=%v", err)
			utils.RespondInternalServerError(c)
			return
		}
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		logging.Errorf("achievement", "export", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\"prestasi.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h AchievementHandler) Update(c *gin.Context) {
	logging.Infof("achievement", "update", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		utils.RespondBadRequest(c, "Invalid id")
		return
	}

	var req achievementUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Invalid payload")
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		utils.RespondBadRequestMessage(c, "PASSWORD_REQUIRED")
		return
	}

	authService := services.AuthService{DB: h.DB}
	record, err := authService.GetUserByID(c, user.ID)
	if err != nil {
		logging.Errorf("achievement", "update", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if !utils.VerifyPassword(req.Password, record.PasswordHash) {
		utils.RespondBadRequestMessage(c, "PASSWORD_SALAH")
		return
	}
	if user.SchoolID == nil {
		utils.RespondForbidden(c)
		return
	}

	result, err := h.DB.Pool.Exec(c, `
        UPDATE prestasi_siswa
        SET nama_lomba = $1,
            tingkat_lomba = $2,
            kategori_prestasi = $3,
            peringkat_prestasi = $4,
            nama_peserta = $5,
            jenis_lomba = $6,
            updated_at = NOW()
        WHERE id_prestasi = $7
          AND school_id = $8
    `,
		strings.TrimSpace(req.Payload.NamaLomba),
		strings.TrimSpace(req.Payload.TingkatLomba),
		strings.TrimSpace(req.Payload.KategoriPrestasi),
		strings.TrimSpace(req.Payload.Peringkat),
		strings.TrimSpace(req.Payload.NamaPeserta),
		strings.TrimSpace(req.Payload.JenisLomba),
		id,
		*user.SchoolID,
	)
	if err != nil {
		logging.Errorf("achievement", "update", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	if result.RowsAffected() == 0 {
		utils.RespondForbidden(c)
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{"id": id})
}

func (h AchievementHandler) Analytics(c *gin.Context) {
	logging.Infof("achievement", "analytics", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		logging.Warnf("achievement", "analytics_reject", "reason=unauthorized path=%s", c.Request.URL.Path)
		utils.RespondUnauthorized(c)
		return
	}
	role := strings.ToUpper(strings.TrimSpace(user.Role))
	if role != "ADMIN" && role != "SCHOOL_ADMIN" && role != "ADMIN_MASTER" {
		logging.Warnf("achievement", "analytics_reject", "reason=forbidden path=%s role=%s", c.Request.URL.Path, role)
		utils.RespondForbidden(c)
		return
	}
	logging.Debugf("achievement", "analytics_request", "path=%s role=%s school_id=%v query_school_id=%s", c.Request.URL.Path, role, user.SchoolID, strings.TrimSpace(c.Query("school_id")))

	yearParam := strings.TrimSpace(c.Query("year"))
	if yearParam != "" && !strings.EqualFold(yearParam, "ALL") {
		if _, convErr := strconv.Atoi(yearParam); convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
	}

	var requestedSchoolID *int
	if role == "ADMIN" || role == "ADMIN_MASTER" {
		if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
			id, convErr := strconv.Atoi(raw)
			if convErr != nil || id <= 0 {
				utils.RespondBadRequest(c, "Invalid school_id")
				return
			}
			requestedSchoolID = &id
		}
	}

	filters := services.AchievementAnalyticsSummaryFilters{
		Role:       role,
		SchoolID:   requestedSchoolID,
		Year:       yearParam,
		Jenjang:    strings.TrimSpace(c.Query("jenjang")),
		Status:     strings.TrimSpace(c.Query("status")),
		JenisLomba: strings.TrimSpace(c.Query("jenis_lomba")),
	}
	if role == "ADMIN" {
		filters.AllowedLevels = auth.NormalizeAllowedLevels(user.AllowedLevels)
		if filters.AllowedLevels == nil {
			filters.AllowedLevels = []string{}
		}
	}
	if role == "SCHOOL_ADMIN" {
		filters.SchoolID = user.SchoolID
	}

	service := services.AchievementAnalyticsSummaryService{DB: h.DB}
	payload, err := service.Get(c, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			logging.Warnf("achievement", "analytics_reject", "reason=forbidden path=%s role=%s", c.Request.URL.Path, role)
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			logging.Warnf("achievement", "analytics_reject", "reason=unauthorized path=%s", c.Request.URL.Path)
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("achievement", "analytics", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h AchievementHandler) AdminSummary(c *gin.Context) {
	logging.Infof("achievement", "admin_summary", "event=hit")
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdmin(user) && user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	var requestedSchoolID *int
	if auth.IsAdmin(user) {
		if raw := strings.TrimSpace(c.Query("schoolId")); raw != "" {
			id, convErr := strconv.Atoi(raw)
			if convErr != nil || id <= 0 {
				utils.RespondBadRequest(c, "Invalid schoolId")
				return
			}
			requestedSchoolID = &id
		}
	}

	filters := services.AchievementAdminSummaryFilters{
		SchoolID: requestedSchoolID,
		Year:     strings.TrimSpace(c.Query("year")),
		Kategori: strings.TrimSpace(c.Query("kategori")),
		Bentuk:   strings.TrimSpace(c.Query("bentuk")),
		Status:   strings.TrimSpace(c.Query("status")),
	}

	service := services.AchievementService{DB: h.DB}
	payload, err := service.AdminSummary(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			logging.Errorf("achievement", "admin_summary", "err=%v", err)
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, achievementAdminSummaryResponse{
		RekapJenjangTingkat: payload.RekapJenjangTingkat,
		DetailSekolah:       payload.DetailSekolah,
	})
}
