package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type ukkAssessorPayload struct {
	Nama     string `json:"nama"`
	Instansi string `json:"instansi"`
}

type ukkListRowPayload struct {
	NISN        string               `json:"nisn"`
	Nama        string               `json:"nama"`
	Rombel      string               `json:"rombel"`
	TahunLulus  int                  `json:"tahun_lulus"`
	SkemaUKK    *string              `json:"skema_ukk"`
	NilaiAkhir  *float64             `json:"nilai_akhir"`
	Predikat    *string              `json:"predikat"`
	Asesor      []ukkAssessorPayload `json:"asesor"`
	FinalStatus string               `json:"final_status"`
	FinalizedAt *time.Time           `json:"finalized_at"`
}

type ukkUpdateRequest struct {
	NISN       string               `json:"nisn"`
	TahunLulus int                  `json:"tahun_lulus"`
	SkemaUKK   string               `json:"skema_ukk"`
	NilaiAkhir *float64             `json:"nilai_akhir"`
	Predikat   string               `json:"predikat"`
	Asesor     []ukkAssessorPayload `json:"asesor"`
}

func (h SiswaHandler) ListUKK(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" || user.SchoolID == nil {
		utils.RespondForbidden(c)
		return
	}

	rawYear := strings.TrimSpace(c.Query("tahun_lulus"))
	if rawYear == "" {
		utils.RespondBadRequest(c, "tahun_lulus wajib diisi.")
		return
	}
	year, err := strconv.Atoi(rawYear)
	if err != nil || year <= 0 {
		utils.RespondBadRequest(c, "Invalid tahun_lulus")
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ListUKK(c, *user.SchoolID, year)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	rows := make([]ukkListRowPayload, 0, len(result.Rows))
	for _, row := range result.Rows {
		assessors := make([]ukkAssessorPayload, 0, len(row.Assessors))
		for _, assessor := range row.Assessors {
			assessors = append(assessors, ukkAssessorPayload{
				Nama:     assessor.Nama,
				Instansi: assessor.Instansi,
			})
		}
		rows = append(rows, ukkListRowPayload{
			NISN:        row.NISN,
			Nama:        row.NamaSiswa,
			Rombel:      row.Rombel,
			TahunLulus:  row.TahunLulus,
			SkemaUKK:    row.SkemaUKK,
			NilaiAkhir:  row.NilaiAkhir,
			Predikat:    row.Predikat,
			Asesor:      assessors,
			FinalStatus: row.FinalStatus,
			FinalizedAt: row.FinalizedAt,
		})
	}

	logging.Infof(
		"siswa_ukk",
		"list",
		"school_id=%d tahun_lulus=%d total=%d complete=%d missing=%d",
		*user.SchoolID,
		year,
		result.TotalStudents,
		result.CompleteCount,
		result.MissingCount,
	)

	utils.RespondData(c, http.StatusOK, gin.H{
		"total_students": result.TotalStudents,
		"complete_count": result.CompleteCount,
		"missing_count":  result.MissingCount,
		"year_closed":    result.YearClosed,
		"rows":           rows,
	})
}

func (h SiswaHandler) DownloadUKKTemplate(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" || user.SchoolID == nil {
		utils.RespondForbidden(c)
		return
	}

	rawYear := strings.TrimSpace(c.Query("tahun_lulus"))
	if rawYear == "" {
		utils.RespondBadRequest(c, "tahun_lulus wajib diisi.")
		return
	}
	year, err := strconv.Atoi(rawYear)
	if err != nil || year <= 0 {
		utils.RespondBadRequest(c, "Invalid tahun_lulus")
		return
	}

	missingOnly := strings.EqualFold(strings.TrimSpace(c.Query("missing_only")), "true")

	service := services.SiswaService{DB: h.DB}
	result, err := service.ListUKK(c, *user.SchoolID, year)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	rows := result.Rows
	if missingOnly {
		filtered := make([]services.UkkListRow, 0, len(result.Rows))
		for _, row := range result.Rows {
			if row.IsMissing {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}

	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	sheet := "UKK"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{
		"NISN",
		"Nama",
		"Rombel",
		"Tahun Lulus",
		"Skema UKK",
		"Nilai Akhir (NS)",
		"Predikat",
		"Asesor 1 Nama",
		"Asesor 1 Instansi",
		"Asesor 2 Nama",
		"Asesor 2 Instansi",
		"Nomor Sertifikat",
	}
	if err := setHeaderRow(file, sheet, headers); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	for i, row := range rows {
		rowValues := []interface{}{
			row.NISN,
			row.NamaSiswa,
			row.Rombel,
			row.TahunLulus,
			"",
			"",
			"",
			"",
			"",
			"",
			"",
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

	buffer, err := writeToBuffer(file)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	filename := fmt.Sprintf("ukk_template_%d.xlsx", year)
	if missingOnly {
		filename = fmt.Sprintf("ukk_missing_%d.xlsx", year)
	}
	logging.Infof(
		"siswa_ukk",
		"template",
		"school_id=%d tahun_lulus=%d missing_only=%t rows=%d missing=%d",
		*user.SchoolID,
		year,
		missingOnly,
		len(rows),
		result.MissingCount,
	)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, xlsxContentType, buffer)
}

func (h SiswaHandler) UpdateUKK(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" || user.SchoolID == nil {
		utils.RespondForbidden(c)
		return
	}

	var req ukkUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	if strings.TrimSpace(req.NISN) == "" || req.TahunLulus <= 0 {
		utils.RespondBadRequest(c, "nisn dan tahun_lulus wajib diisi.")
		return
	}

	assessors := make([]services.UkkAssessor, 0, len(req.Asesor))
	for _, assessor := range req.Asesor {
		assessors = append(assessors, services.UkkAssessor{
			Nama:     strings.TrimSpace(assessor.Nama),
			Instansi: strings.TrimSpace(assessor.Instansi),
		})
	}

	service := services.SiswaService{DB: h.DB}
	if err := service.UpsertUKK(c, *user.SchoolID, strings.TrimSpace(req.NISN), req.TahunLulus, services.UkkManualPayload{
		SkemaUKK:   strings.TrimSpace(req.SkemaUKK),
		NilaiAkhir: req.NilaiAkhir,
		Predikat:   strings.TrimSpace(req.Predikat),
		Assessors:  assessors,
	}); err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
		case errors.Is(err, services.ErrSiswaFinalized):
			utils.RespondBadRequest(c, "Data sudah final dan terkunci.")
		case errors.Is(err, services.ErrYearClosed):
			utils.RespondBadRequest(c, "Tahun sudah tutup buku. Hanya finalisasi yang diperbolehkan.")
		case errors.Is(err, services.ErrInvalidPlan):
			utils.RespondBadRequest(c, "Payload tidak valid.")
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{"status": "ok"})
}
