package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type HardiknasHandler struct {
	DB              *db.DB
	StorageLocalDir string
	Clock           services.SystemClock
	FrontendOrigin  string
}

func (h HardiknasHandler) service() services.HardiknasService {
	return services.HardiknasService{
		DB:              h.DB,
		StorageLocalDir: h.StorageLocalDir,
		Clock:           h.Clock,
	}
}

func (h HardiknasHandler) List(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	payload, err := h.service().ListForUser(c, user)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) Summary(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	payload, err := h.service().SummaryForUser(c, user)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) PublicList(c *gin.Context) {
	payload, err := h.service().ListPublic(c)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) PublicSummary(c *gin.Context) {
	payload, err := h.service().SummaryPublic(c)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) PublicWinners(c *gin.Context) {
	payload, err := h.service().WinnersPublic(c)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) Winners(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	payload, err := h.service().WinnersForUser(c, user)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) SaveWinners(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.HardiknasSaveWinnersInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, services.ErrHardiknasInvalidPayload.Error())
		return
	}
	payload, err := h.service().SaveWinners(c, user, input)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) ListTable(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	payload, err := h.service().ListTableForUser(c, user, readHardiknasTableFilters(c))
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) PublicListTable(c *gin.Context) {
	payload, err := h.service().ListTablePublic(c, readHardiknasTableFilters(c))
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) PublicDetail(c *gin.Context) {
	entryID, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || entryID <= 0 {
		utils.RespondNotFound(c)
		return
	}
	payload, err := h.service().PublicDetail(c, entryID)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) SuggestSchools(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	payload, err := h.service().SuggestSchoolsForUser(c, user, strings.TrimSpace(c.Query("keyword")), readHardiknasSuggestionLimit(c))
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) PublicSuggestSchools(c *gin.Context) {
	payload, err := h.service().SuggestSchoolsPublic(c, strings.TrimSpace(c.Query("keyword")), readHardiknasSuggestionLimit(c))
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) Export(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	rows, err := h.service().ExportRowsForUser(c, user, readHardiknasTableFilters(c))
	if err != nil {
		handleHardiknasError(c, err)
		return
	}

	file := excelize.NewFile()
	sheet := "Hardiknas"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Judul Poster",
		"Link Preview",
		"Link Surat Pernyataan",
		"NISN",
		"Nama Siswa",
		"Rombel",
		"Nama Sekolah",
	}
	for index, header := range headers {
		cell, cellErr := excelize.CoordinatesToCellName(index+1, 1)
		if cellErr != nil {
			handleHardiknasError(c, cellErr)
			return
		}
		if err := file.SetCellValue(sheet, cell, header); err != nil {
			handleHardiknasError(c, err)
			return
		}
	}

	for index, row := range rows {
		rowValues := []interface{}{
			row.JudulPoster,
			buildHardiknasPreviewURL(row.ID, h.FrontendOrigin),
			row.LinkSuratPernyataanOrisinalitas,
			row.NISN,
			row.NamaSiswa,
			row.Rombel,
			row.NamaSekolah,
		}
		cell, cellErr := excelize.CoordinatesToCellName(1, index+2)
		if cellErr != nil {
			handleHardiknasError(c, cellErr)
			return
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			handleHardiknasError(c, err)
			return
		}
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		handleHardiknasError(c, err)
		return
	}

	filename := fmt.Sprintf("lomba-hardiknas-%s.xlsx", h.Clock.RealNow().Format("20060102-1504"))
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h HardiknasHandler) Create(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	input, err := readHardiknasCreateInput(c, false)
	if err != nil {
		utils.RespondBadRequestMessage(c, services.ErrHardiknasInvalidPayload.Error())
		return
	}
	payload, err := h.service().Create(c, user, input)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) PublicCreate(c *gin.Context) {
	input, err := readHardiknasCreateInput(c, true)
	if err != nil {
		if errors.Is(err, services.ErrHardiknasInvalidSchool) {
			handleHardiknasError(c, err)
			return
		}
		utils.RespondBadRequestMessage(c, services.ErrHardiknasInvalidPayload.Error())
		return
	}
	payload, err := h.service().CreatePublic(c, input)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h HardiknasHandler) Delete(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	entryID, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || entryID <= 0 {
		utils.RespondBadRequestMessage(c, services.ErrHardiknasInvalidPayload.Error())
		return
	}

	if err := h.service().Delete(c, user, entryID); err != nil {
		handleHardiknasError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"id": entryID, "deleted": true})
}

func (h HardiknasHandler) PublicMediaOriginal(c *gin.Context) {
	h.publicMedia(c, false)
}

func (h HardiknasHandler) PublicMediaThumbnail(c *gin.Context) {
	h.publicMedia(c, true)
}

func (h HardiknasHandler) publicMedia(c *gin.Context, thumbnail bool) {
	entryID, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || entryID <= 0 {
		utils.RespondNotFound(c)
		return
	}
	result, err := h.service().PublicMedia(c, entryID, thumbnail)
	if err != nil {
		handleHardiknasError(c, err)
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.Header("Content-Disposition", "inline; filename=\""+result.Filename+"\"")
	c.Data(http.StatusOK, result.ContentType, result.Content)
}

func (h HardiknasHandler) StorageDiagnostics(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if !auth.IsAdminMaster(user) {
		utils.RespondForbidden(c)
		return
	}
	utils.RespondData(c, http.StatusOK, h.service().StorageDiagnostics())
}

func readHardiknasTableFilters(c *gin.Context) services.HardiknasTableFilters {
	return services.HardiknasTableFilters{
		SchoolKeyword: strings.TrimSpace(c.Query("school")),
		PosterTitle:   strings.TrimSpace(c.Query("title")),
		StudentName:   strings.TrimSpace(c.Query("student")),
	}
}

func readHardiknasSuggestionLimit(c *gin.Context) int {
	limit, err := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "10")))
	if err != nil || limit <= 0 {
		return 10
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func buildHardiknasPreviewURL(entryID int, frontendOrigin string) string {
	path := fmt.Sprintf("/lomba-hardiknas/karya/%d", entryID)
	base := strings.TrimRight(strings.TrimSpace(frontendOrigin), "/")
	if base == "" {
		return path
	}
	return base + path
}

func readHardiknasCreateInput(c *gin.Context, includeSchoolID bool) (services.HardiknasCreateInput, error) {
	fileHeader, err := c.FormFile("hasil_karya")
	if err != nil {
		return services.HardiknasCreateInput{}, err
	}
	file, err := fileHeader.Open()
	if err != nil {
		return services.HardiknasCreateInput{}, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return services.HardiknasCreateInput{}, err
	}

	input := services.HardiknasCreateInput{
		NISN:                            c.PostForm("nisn"),
		NamaSiswa:                       c.PostForm("nama_siswa"),
		Rombel:                          c.PostForm("rombel"),
		LinkSuratPernyataanOrisinalitas: c.PostForm("link_surat_pernyataan_orisinalitas"),
		JudulPoster:                     c.PostForm("judul_poster"),
		OriginalFilename:                fileHeader.Filename,
		FileBytes:                       content,
	}
	if includeSchoolID {
		schoolID, parseErr := strconv.Atoi(strings.TrimSpace(c.PostForm("school_id")))
		if parseErr != nil || schoolID <= 0 {
			return services.HardiknasCreateInput{}, services.ErrHardiknasInvalidSchool
		}
		input.SchoolID = &schoolID
	}
	return input, nil
}

func handleHardiknasError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, auth.ErrUnauthorized):
		utils.RespondUnauthorized(c)
	case errors.Is(err, auth.ErrForbidden):
		utils.RespondForbidden(c)
	case errors.Is(err, pgx.ErrNoRows):
		utils.RespondNotFound(c)
	case errors.Is(err, services.ErrHardiknasInvalidPayload),
		errors.Is(err, services.ErrHardiknasInvalidSchool),
		errors.Is(err, services.ErrHardiknasInvalidDriveLink),
		errors.Is(err, services.ErrHardiknasInvalidImage),
		errors.Is(err, services.ErrHardiknasInvalidImageType),
		errors.Is(err, services.ErrHardiknasFileTooLarge),
		errors.Is(err, services.ErrHardiknasCreateClosed),
		errors.Is(err, services.ErrHardiknasDeleteClosed),
		errors.Is(err, services.ErrHardiknasActionNotAllowed):
		utils.RespondBadRequestMessage(c, err.Error())
	case errors.Is(err, services.ErrHardiknasFileNotAvailable):
		utils.RespondError(c, http.StatusNotFound, services.ErrHardiknasFileNotAvailable.Error())
	default:
		logging.Errorf("hardiknas", "handler", "err=%v", err)
		utils.RespondInternalServerError(c)
	}
}
