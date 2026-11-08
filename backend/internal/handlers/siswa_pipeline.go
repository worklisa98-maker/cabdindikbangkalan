package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type ptPipelineRowPayload struct {
	ID          int64      `json:"id"`
	SchoolID    int        `json:"school_id"`
	SchoolName  string     `json:"school_name"`
	NISN        string     `json:"nisn"`
	NamaSiswa   string     `json:"namaSiswa"`
	Rombel      string     `json:"rombel"`
	TahunLulus  int        `json:"tahunLulus"`
	Status      string     `json:"status"`
	FinalStatus string     `json:"final_status"`
	FinalPath   string     `json:"final_path"`
	FinalizedAt *time.Time `json:"finalized_at"`
	YearClosed  bool       `json:"year_closed"`
	FilledPaths []string   `json:"filled_paths"`
}

type ptDetailChoicePayload struct {
	PilihanKe    int    `json:"pilihan_ke"`
	Jalur        string `json:"jalur"`
	KampusID     *int   `json:"kampus_id,omitempty"`
	Kampus       string `json:"kampus"`
	Prodi        string `json:"prodi"`
	JenjangProdi string `json:"jenjang_prodi"`
}

type ptKedinasanPayload struct {
	Jenis           string `json:"jenis"`
	Matra           string `json:"matra,omitempty"`
	JenisPendidikan string `json:"jenis_pendidikan,omitempty"`
}

type ptDetailResponse struct {
	Student   gin.H                   `json:"student"`
	Status    string                  `json:"status"`
	Choices   []ptDetailChoicePayload `json:"choices"`
	Kedinasan *ptKedinasanPayload     `json:"kedinasan,omitempty"`
}

type ptUpdateRequest struct {
	SchoolID   *int                    `json:"school_id"`
	NISN       string                  `json:"nisn"`
	TahunLulus int                     `json:"tahun_lulus"`
	Jalur      string                  `json:"jalur"`
	Choices    []ptDetailChoicePayload `json:"choices"`
	Kedinasan  *ptKedinasanPayload     `json:"kedinasan,omitempty"`
}

type ptCancelPathRequest struct {
	SchoolID   *int   `json:"school_id"`
	NISN       string `json:"nisn"`
	TahunLulus int    `json:"tahun_lulus"`
	Jalur      string `json:"jalur"`
}

type wirausahaListRowPayload struct {
	ID          int64      `json:"id"`
	SchoolID    int        `json:"school_id"`
	SchoolName  string     `json:"school_name"`
	NISN        string     `json:"nisn"`
	NamaSiswa   string     `json:"namaSiswa"`
	Rombel      string     `json:"rombel"`
	TahunLulus  int        `json:"tahunLulus"`
	Status      string     `json:"status"`
	PlanCount   int        `json:"plan_count"`
	FinalStatus string     `json:"final_status"`
	FinalPath   string     `json:"final_path"`
	FinalizedAt *time.Time `json:"finalized_at"`
	YearClosed  bool       `json:"year_closed"`
}

type wirausahaDetailPlanPayload struct {
	RencanaKe      int    `json:"rencana_ke"`
	NamaUsaha      string `json:"nama_usaha"`
	KategoriUsaha  string `json:"kategori_usaha"`
	StatusKesiapan string `json:"status_kesiapan"`
	RencanaMulai   string `json:"rencana_mulai"`
	ModalAwal      string `json:"modal_awal"`
	SumberModal    string `json:"sumber_modal"`
	Lokasi         string `json:"lokasi"`
	Platform       string `json:"platform"`
	Catatan        string `json:"catatan"`
}

type wirausahaDetailResponse struct {
	Student gin.H                        `json:"student"`
	Status  string                       `json:"status"`
	Plans   []wirausahaDetailPlanPayload `json:"plans"`
}

type wirausahaUpdateRequest struct {
	SchoolID   *int                         `json:"school_id"`
	NISN       string                       `json:"nisn"`
	TahunLulus int                          `json:"tahun_lulus"`
	Plans      []wirausahaDetailPlanPayload `json:"plans"`
}

func (h SiswaHandler) ListApplicationsPipeline(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))
	if pageSize <= 0 {
		pageSize = 10
	}
	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}
	includeFinalized := false
	if raw := strings.TrimSpace(c.Query("includeFinalized")); raw != "" {
		includeFinalized = strings.EqualFold(raw, "true") || raw == "1"
	}
	if includeFinalized && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
		includeFinalized = false
	}

	filters := services.PtPipelineListFilters{
		SchoolID:         requestedSchoolID,
		Page:             page,
		PageSize:         pageSize,
		Search:           strings.TrimSpace(c.Query("q")),
		Year:             year,
		IncludeFinalized: includeFinalized,
	}

	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListPtPipeline(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]ptPipelineRowPayload, 0, len(rows))
	for _, row := range rows {
		filledPaths := row.FilledPaths
		if filledPaths == nil {
			filledPaths = []string{}
		}
		payload = append(payload, ptPipelineRowPayload{
			ID:          row.ID,
			SchoolID:    row.SchoolID,
			SchoolName:  row.SchoolName,
			NISN:        row.NISN,
			NamaSiswa:   row.NamaSiswa,
			Rombel:      row.Rombel,
			TahunLulus:  row.TahunLulus,
			Status:      row.Status,
			FinalStatus: row.FinalStatus,
			FinalPath:   row.FinalPath,
			FinalizedAt: row.FinalizedAt,
			YearClosed:  row.YearClosed,
			FilledPaths: filledPaths,
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

func (h SiswaHandler) GetApplicationsDetail(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil {
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}

	nisn := strings.TrimSpace(c.Query("nisn"))
	tahunRaw := strings.TrimSpace(c.Query("tahun_lulus"))
	if nisn == "" || tahunRaw == "" {
		utils.RespondBadRequest(c, "nisn dan tahun_lulus wajib diisi.")
		return
	}
	tahunLulus, convErr := strconv.Atoi(tahunRaw)
	if convErr != nil || tahunLulus <= 0 {
		utils.RespondBadRequest(c, "Invalid tahun_lulus")
		return
	}

	service := services.SiswaService{DB: h.DB}
	cohort, choices, kedinasan, status, finalizedAt, err := service.GetPtApplications(c, user, *scope, nisn, tahunLulus)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	choicePayload := make([]ptDetailChoicePayload, 0, len(choices))
	for _, choice := range choices {
		choicePayload = append(choicePayload, ptDetailChoicePayload{
			PilihanKe:    choice.PilihanKe,
			Jalur:        choice.Jalur,
			KampusID:     choice.KampusID,
			Kampus:       choice.Kampus,
			Prodi:        choice.Prodi,
			JenjangProdi: choice.JenjangProdi,
		})
	}

	var kedinasanPayload *ptKedinasanPayload
	if kedinasan != nil {
		kedinasanPayload = &ptKedinasanPayload{
			Jenis:           kedinasan.Jenis,
			Matra:           kedinasan.Matra,
			JenisPendidikan: kedinasan.JenisPendidikan,
		}
	}

	utils.RespondData(c, http.StatusOK, ptDetailResponse{
		Student: gin.H{
			"id":           cohort.ID,
			"nisn":         cohort.NISN,
			"namaSiswa":    cohort.NamaSiswa,
			"rombel":       cohort.Rombel,
			"tahunLulus":   cohort.TahunLulus,
			"finalized_at": finalizedAt,
		},
		Status:    status,
		Choices:   choicePayload,
		Kedinasan: kedinasanPayload,
	})
}

func (h SiswaHandler) UpdateApplications(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	var req ptUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	scope, err := auth.GetSingleSchoolScopeV2(user, req.SchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondUnauthorized(c)
		return
	}
	if scope == nil {
		utils.RespondBadRequest(c, "school_id wajib diisi.")
		return
	}

	if strings.TrimSpace(req.NISN) == "" || req.TahunLulus <= 0 {
		utils.RespondBadRequest(c, "nisn dan tahun_lulus wajib diisi.")
		return
	}
	if strings.TrimSpace(req.Jalur) == "" {
		utils.RespondBadRequest(c, "jalur wajib diisi.")
		return
	}

	choices := make([]services.PtApplicationChoice, 0, len(req.Choices))
	for _, choice := range req.Choices {
		choices = append(choices, services.PtApplicationChoice{
			PilihanKe:    choice.PilihanKe,
			Jalur:        choice.Jalur,
			KampusID:     choice.KampusID,
			Kampus:       choice.Kampus,
			Prodi:        choice.Prodi,
			JenjangProdi: choice.JenjangProdi,
		})
	}

	var kedinasan *services.PtKedinasanPlan
	if req.Kedinasan != nil {
		kedinasan = &services.PtKedinasanPlan{
			Jenis:           req.Kedinasan.Jenis,
			Matra:           req.Kedinasan.Matra,
			JenisPendidikan: req.Kedinasan.JenisPendidikan,
		}
	}

	service := services.SiswaService{DB: h.DB}
	if err := service.UpsertPtApplications(
		c,
		user,
		*scope,
		strings.TrimSpace(req.NISN),
		req.TahunLulus,
		req.Jalur,
		choices,
		kedinasan,
	); err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotInRoster):
			utils.RespondBadRequest(c, "Siswa belum ada di roster tahun tersebut")
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
		case errors.Is(err, services.ErrSiswaFinalized):
			utils.RespondBadRequest(c, "Data sudah final dan terkunci.")
		case errors.Is(err, services.ErrYearClosed):
			utils.RespondBadRequest(c, "Tahun sudah tutup buku. Hanya finalisasi yang diperbolehkan.")
		case errors.Is(err, services.ErrRefKampusNotFound):
			utils.RespondBadRequest(c, "Kampus tidak ditemukan di referensi")
		case errors.Is(err, services.ErrSNBPEligibleRequired):
			utils.RespondBadRequestMessage(c, services.SNBPEligibleRequiredMessage(req.TahunLulus))
		case errors.Is(err, services.ErrInvalidPlan):
			utils.RespondBadRequest(c, "Payload tidak valid.")
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{"status": "ok"})
}

func (h SiswaHandler) CancelApplicationsPath(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	var req ptCancelPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	scope, err := auth.GetSingleSchoolScopeV2(user, req.SchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondUnauthorized(c)
		return
	}
	if scope == nil {
		utils.RespondBadRequest(c, "school_id wajib diisi.")
		return
	}

	nisn := strings.TrimSpace(req.NISN)
	if nisn == "" || req.TahunLulus <= 0 {
		utils.RespondBadRequest(c, "nisn dan tahun_lulus wajib diisi.")
		return
	}

	upperJalur := strings.ToUpper(strings.TrimSpace(req.Jalur))
	switch upperJalur {
	case "SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA":
	default:
		utils.RespondBadRequest(c, "jalur wajib salah satu dari SNBP, SNBT, MANDIRI, KEDINASAN, atau LAINNYA.")
		return
	}

	service := services.SiswaService{DB: h.DB}
	if err := service.CancelPtApplicationsByJalur(c, *scope, nisn, req.TahunLulus, upperJalur); err != nil {
		log.Printf("[CancelApplicationsPath] school_id=%d nisn=%s tahun=%d jalur=%s err=%v", *scope, nisn, req.TahunLulus, upperJalur, err)
		switch {
		case errors.Is(err, services.ErrYearClosed):
			utils.RespondBadRequest(c, graduationYearClosedMessage(req.TahunLulus))
		case errors.Is(err, services.ErrSiswaFinalized):
			utils.RespondBadRequest(c, "Siswa sudah final. Batalkan final terlebih dahulu.")
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Siswa tidak ditemukan.")
		case errors.Is(err, services.ErrInvalidPlan):
			utils.RespondBadRequest(c, "Payload tidak valid.")
		default:
			utils.RespondInternalServerError(c)
			/*msg := fmt.Sprintf("InternalError: %v", err)
			log.Printf("[CancelApplicationsPath][DEBUG] %s", msg)
			utils.RespondError(c, http.StatusInternalServerError, msg)*/
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{"ok": true})
}

func (h SiswaHandler) ExportApplicationsTemplate(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil {
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}

	year := 0
	if raw := strings.TrimSpace(c.Query("tahun_lulus")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil || parsed <= 0 {
			utils.RespondBadRequest(c, "Invalid tahun_lulus")
			return
		}
		year = parsed
	}

	if year == 0 {
		_ = h.DB.Pool.QueryRow(c, `
            SELECT COALESCE(MAX(tahun_lulus), 0)
            FROM class12_students
            WHERE school_id = $1
        `, *scope).Scan(&year)
	}
	if year == 0 {
		year = time.Now().Year()
	}

	rows, err := h.DB.Pool.Query(c, `
        SELECT c.nisn,
               c.nama_siswa,
               c.rombel,
               c.tahun_lulus
        FROM class12_students c
        WHERE c.school_id = $1
          AND c.tahun_lulus = $2
        ORDER BY c.nama_siswa ASC
    `, *scope, year)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer rows.Close()

	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	sheet := "Pendaftaran_PT"
	file.SetSheetName("Sheet1", sheet)

	headers := parsers.SiswaApplicationTemplateHeaders
	if err := setHeaderRow(file, sheet, headers); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidationRange(
		file,
		sheet,
		"E2:E2000",
		[]string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"},
	); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	jenjangOptions := []string{"D1", "D2", "D3", "D4", "S1", "S2", "S3", "UNKNOWN"}
	if err := addFixedListValidationRange(file, sheet, "H2:H2000", jenjangOptions); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidationRange(file, sheet, "K2:K2000", jenjangOptions); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidationRange(file, sheet, "N2:N2000", jenjangOptions); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidationRange(file, sheet, "Q2:Q2000", jenjangOptions); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	service := services.SiswaService{DB: h.DB}
	campusItems, err := service.ListAllRefKampus(c)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if _, err := file.NewSheet("Referensi Kampus"); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	campusNames := make([]string, 0, len(campusItems))
	for _, item := range campusItems {
		campusNames = append(campusNames, item.Nama)
	}
	lastRow, err := populateLookupSheet(file, "Referensi Kampus", "nama_kampus", campusNames)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if len(campusNames) > 0 {
		refersTo := fmt.Sprintf("'Referensi Kampus'!$A$2:$A$%d", lastRow)
		if err := file.SetDefinedName(&excelize.DefinedName{Name: "KAMPUS_LIST", RefersTo: refersTo}); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addNamedRangeValidationRange(file, sheet, "F2:F2000", "KAMPUS_LIST"); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addNamedRangeValidationRange(file, sheet, "I2:I2000", "KAMPUS_LIST"); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addNamedRangeValidationRange(file, sheet, "L2:L2000", "KAMPUS_LIST"); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addNamedRangeValidationRange(file, sheet, "O2:O2000", "KAMPUS_LIST"); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
	}

	rowIndex := 2
	for rows.Next() {
		var nisn, nama, rombel string
		var tahun int
		if scanErr := rows.Scan(&nisn, &nama, &rombel, &tahun); scanErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		rowValues := []interface{}{nisn, nama, rombel, tahun, "", "", "", "", "", "", "", "", "", "", "", "", ""}
		cell, cellErr := excelize.CoordinatesToCellName(1, rowIndex)
		if cellErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		rowIndex++
	}

	_ = addTemplateInfoSheet(file, "Template pendaftaran PT selalu menyediakan 4 slot pilihan. Batas aktif per jalur dan tahun lulus tetap divalidasi saat import.")

	buffer, err := file.WriteToBuffer()
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	filename := fmt.Sprintf("template-pt-%d.xlsx", year)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h SiswaHandler) ListWirausahaPlans(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))
	if pageSize <= 0 {
		pageSize = 10
	}
	year := 0
	if raw := strings.TrimSpace(c.Query("year")); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			utils.RespondBadRequest(c, "Invalid year")
			return
		}
		year = parsed
	}
	includeFinalized := false
	if raw := strings.TrimSpace(c.Query("includeFinalized")); raw != "" {
		includeFinalized = strings.EqualFold(raw, "true") || raw == "1"
	}
	if includeFinalized && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
		includeFinalized = false
	}

	filters := services.WirausahaPlanListFilters{
		SchoolID:         requestedSchoolID,
		Page:             page,
		PageSize:         pageSize,
		Search:           strings.TrimSpace(c.Query("q")),
		Year:             year,
		IncludeFinalized: includeFinalized,
	}

	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListWirausahaPlans(c, user, filters)
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	payload := make([]wirausahaListRowPayload, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, wirausahaListRowPayload{
			ID:          row.ID,
			SchoolID:    row.SchoolID,
			SchoolName:  row.SchoolName,
			NISN:        row.NISN,
			NamaSiswa:   row.NamaSiswa,
			Rombel:      row.Rombel,
			TahunLulus:  row.TahunLulus,
			Status:      row.Status,
			PlanCount:   row.PlanCount,
			FinalStatus: row.FinalStatus,
			FinalPath:   row.FinalPath,
			FinalizedAt: row.FinalizedAt,
			YearClosed:  row.YearClosed,
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

func (h SiswaHandler) GetWirausahaPlans(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil {
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}

	nisn := strings.TrimSpace(c.Query("nisn"))
	tahunRaw := strings.TrimSpace(c.Query("tahun_lulus"))
	if nisn == "" || tahunRaw == "" {
		utils.RespondBadRequest(c, "nisn dan tahun_lulus wajib diisi.")
		return
	}
	tahunLulus, convErr := strconv.Atoi(tahunRaw)
	if convErr != nil || tahunLulus <= 0 {
		utils.RespondBadRequest(c, "Invalid tahun_lulus")
		return
	}

	service := services.SiswaService{DB: h.DB}
	cohort, plans, status, finalizedAt, err := service.GetWirausahaPlans(c, user, *scope, nisn, tahunLulus)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	planPayload := make([]wirausahaDetailPlanPayload, 0, len(plans))
	for _, plan := range plans {
		planPayload = append(planPayload, wirausahaDetailPlanPayload{
			RencanaKe:      plan.RencanaKe,
			NamaUsaha:      plan.NamaUsaha,
			KategoriUsaha:  plan.KategoriUsaha,
			StatusKesiapan: plan.StatusKesiapan,
			RencanaMulai:   plan.RencanaMulai,
			ModalAwal:      plan.ModalAwal,
			SumberModal:    plan.SumberModal,
			Lokasi:         plan.Lokasi,
			Platform:       plan.Platform,
			Catatan:        plan.Catatan,
		})
	}

	utils.RespondData(c, http.StatusOK, wirausahaDetailResponse{
		Student: gin.H{
			"id":           cohort.ID,
			"nisn":         cohort.NISN,
			"namaSiswa":    cohort.NamaSiswa,
			"rombel":       cohort.Rombel,
			"tahunLulus":   cohort.TahunLulus,
			"finalized_at": finalizedAt,
		},
		Status: status,
		Plans:  planPayload,
	})
}

func (h SiswaHandler) UpdateWirausahaPlans(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	var req wirausahaUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	scope, err := auth.GetSingleSchoolScopeV2(user, req.SchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondUnauthorized(c)
		return
	}
	if scope == nil {
		utils.RespondBadRequest(c, "school_id wajib diisi.")
		return
	}

	if strings.TrimSpace(req.NISN) == "" || req.TahunLulus <= 0 {
		utils.RespondBadRequest(c, "nisn dan tahun_lulus wajib diisi.")
		return
	}

	plans := make([]services.WirausahaPlanPayload, 0, len(req.Plans))
	for _, plan := range req.Plans {
		plans = append(plans, services.WirausahaPlanPayload{
			RencanaKe:      plan.RencanaKe,
			NamaUsaha:      plan.NamaUsaha,
			KategoriUsaha:  plan.KategoriUsaha,
			StatusKesiapan: plan.StatusKesiapan,
			RencanaMulai:   plan.RencanaMulai,
			ModalAwal:      plan.ModalAwal,
			SumberModal:    plan.SumberModal,
			Lokasi:         plan.Lokasi,
			Platform:       plan.Platform,
			Catatan:        plan.Catatan,
		})
	}

	service := services.SiswaService{DB: h.DB}
	if err := service.UpsertWirausahaPlans(c, user, *scope, strings.TrimSpace(req.NISN), req.TahunLulus, plans); err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
		case errors.Is(err, services.ErrSiswaFinalized):
			utils.RespondBadRequest(c, "Data sudah final dan terkunci.")
		case errors.Is(err, services.ErrYearClosed):
			utils.RespondBadRequest(c, "Tahun sudah tutup buku. Hanya finalisasi yang diperbolehkan.")
		case errors.Is(err, services.ErrInvalidPlan):
			utils.RespondBadRequest(c, "Payload tidak valid.")
		case err == auth.ErrForbidden:
			utils.RespondForbidden(c)
		case err == auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{"status": "ok"})
}

func (h SiswaHandler) ImportWirausahaPlans(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	scope := siswaResolveScope(c, user, true)
	if scope == nil {
		return
	}
	schoolID := *scope

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	foundHeaders, ok, err := parsers.ValidateExactHeaders(buffer, "", parsers.SiswaWirausahaTemplateHeaders)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if !ok {
		respondSiswaHeaderMismatch(c, parsers.SiswaWirausahaTemplateHeaders, foundHeaders)
		return
	}

	parsed, err := parsers.ParseSiswaWirausaha(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportWirausahaPlans(c, schoolID, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ExportWirausahaTemplate(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	if strings.EqualFold(strings.TrimSpace(c.Query("mode")), "final") {
		scope := siswaResolveScope(c, user, true)
		if scope == nil {
			return
		}
		year := 0
		if raw := strings.TrimSpace(c.Query("year")); raw != "" {
			parsedYear, convErr := strconv.Atoi(raw)
			if convErr != nil || parsedYear <= 0 {
				utils.RespondBadRequest(c, "Invalid year")
				return
			}
			year = parsedYear
		}

		const maxExportRows = 50000
		service := services.SiswaService{DB: h.DB}
		rows, total, err := service.ExportOutcomeEntrepreneurTemplateRows(c, user, services.SiswaPlanListFilters{
			SchoolID: scope,
			Year:     year,
		}, maxExportRows)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrExportLimitExceeded):
				utils.RespondBadRequest(c, fmt.Sprintf("Export melebihi batas %d baris (total %d). Gunakan filter untuk memperkecil data.", maxExportRows, total))
			case err == auth.ErrForbidden:
				utils.RespondForbidden(c)
			case err == auth.ErrUnauthorized:
				utils.RespondUnauthorized(c)
			default:
				utils.RespondInternalServerError(c)
			}
			return
		}

		file := excelize.NewFile()
		defer func() { _ = file.Close() }()
		sheet := "Final_Wirausaha"
		file.SetSheetName("Sheet1", sheet)
		headers := parsers.SiswaOutcomeEntrepreneurTemplateHeaders
		if err := setHeaderRow(file, sheet, headers); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		if err := addFixedListValidation(file, sheet, headers, "Pilihan_Final", []string{"1", "2"}); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		for i, row := range rows {
			rowValues := []interface{}{
				row.TahunLulus,
				row.NISN,
				row.NamaSiswa,
				row.Rombel,
				row.Pilihan1NamaUsaha,
				row.Pilihan1Kategori,
				row.Pilihan1ProdukJasa,
				row.Pilihan1LokasiUsaha,
				row.Pilihan2NamaUsaha,
				row.Pilihan2Kategori,
				row.Pilihan2ProdukJasa,
				row.Pilihan2LokasiUsaha,
				"",
			}
			cell, cellErr := excelize.CoordinatesToCellName(1, i+2)
			if cellErr != nil {
				utils.RespondInternalServerError(c)
				return
			}
			if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
				utils.RespondInternalServerError(c)
				return
			}
		}
		applyTemplateSheetLayout(file, sheet, headers)
		_ = addTemplateInfoSheet(file, "Isi kolom Pilihan_Final dengan 1 atau 2 sesuai usaha yang menjadi hasil final. Kosongkan jika belum ada pilihan final.")

		buffer, err := file.WriteToBuffer()
		if err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=\"template-final-wirausaha.xlsx\"")
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
		return
	}

	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	sheet := "Rencana_Wirausaha"
	file.SetSheetName("Sheet1", sheet)

	headers := parsers.SiswaWirausahaTemplateHeaders
	if err := setHeaderRow(file, sheet, headers); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidationRange(file, sheet, "C2:C2000", []string{"1", "2"}); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	applyTemplateSheetLayout(file, sheet, headers)

	buffer, err := file.WriteToBuffer()
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=\"template-wirausaha.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}
