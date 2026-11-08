package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

func requireAdminPTUser(c *gin.Context) (auth.User, bool) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return auth.User{}, false
	}
	switch auth.NormalizeRole(user.Role) {
	case auth.RoleAdminCabdin, auth.RoleAdminMaster:
		return user, true
	default:
		utils.RespondForbidden(c)
		return auth.User{}, false
	}
}

func (h SiswaHandler) ImportAdminPTOffers(c *gin.Context) {
	user, ok := requireAdminPTUser(c)
	if !ok {
		return
	}
	buffer, filename, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}
	headers, err := parsers.ExtractHeaderRow(buffer, "")
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if !parsers.HeadersMatchExact(headers, parsers.AdminPTOfferTemplateHeaders) {
		respondSiswaHeaderMismatch(c, parsers.AdminPTOfferTemplateHeaders, headers)
		return
	}
	parsed, err := parsers.ParseAdminPTOffers(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}
	result, err := (services.SiswaService{DB: h.DB}).ImportAdminPTOffers(c, user, filename, parsed.Rows)
	if errors.Is(err, auth.ErrForbidden) {
		utils.RespondForbidden(c)
		return
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	payload := gin.H{
		"totalRows":     result.TotalRows,
		"accepted":      result.Accepted,
		"rejected":      result.Rejected,
		"importBatchId": result.ImportBatchID,
		"rejections":    result.Rejections,
	}
	if result.RejectedReportID != "" {
		payload["rejectedExcelUrl"] = "/api/siswa/import-reports/" + result.RejectedReportID + "/rejected.xlsx"
		payload["rejectedExcelFilename"] = result.RejectedFilename
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) ExportAdminPTOffersTemplate(c *gin.Context) {
	user, ok := requireAdminPTUser(c)
	if !ok {
		return
	}
	scope, err := auth.GetMultiSchoolScope(c, h.DB, user)
	if err != nil {
		utils.RespondForbidden(c)
		return
	}
	query := `SELECT nama_sekolah FROM data_sekolah`
	args := []any{}
	if !scope.All {
		if len(scope.SchoolIDs) == 0 {
			utils.RespondForbidden(c)
			return
		}
		query += ` WHERE id = ANY($1)`
		args = append(args, scope.SchoolIDs)
	}
	query += ` ORDER BY nama_sekolah`
	rows, err := h.DB.Pool.Query(c, query, args...)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	schools := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			utils.RespondInternalServerError(c)
			return
		}
		schools = append(schools, strings.TrimSpace(name))
	}
	rows.Close()
	campuses, err := loadCampusListFromDB(c, h.DB)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	sheet := "Pengumuman_PT_Admin"
	file.SetSheetName("Sheet1", sheet)
	if err := setHeaderRow(file, sheet, parsers.AdminPTOfferTemplateHeaders); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addStringLookupNamedRange(file, "LOOKUP_SEKOLAH", "nama_sekolah", "SCHOOL_LIST", schools); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addNamedRangeValidation(file, sheet, parsers.AdminPTOfferTemplateHeaders, "Nama_Sekolah", "SCHOOL_LIST"); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addCampusLookupNamedRange(file, "LOOKUP_KAMPUS", campuses); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addNamedRangeValidation(file, sheet, parsers.AdminPTOfferTemplateHeaders, "Nama_PT", "KAMPUS_LIST"); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidation(file, sheet, parsers.AdminPTOfferTemplateHeaders, "Jalur", []string{"SNBP", "SNBT", "SNBM (Mandiri)", "Kedinasan", "Lainnya"}); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidation(file, sheet, parsers.AdminPTOfferTemplateHeaders, "Status_Pengumuman", []string{"DITERIMA", "DITOLAK"}); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	applyTemplateSheetLayout(file, sheet, parsers.AdminPTOfferTemplateHeaders)
	_ = addTemplateInfoSheet(file, "Status_Pengumuman wajib diisi per baris dengan DITERIMA atau DITOLAK. Nilai kosong atau selain kedua status tersebut akan masuk file rejected.")
	_ = addTemplateInfoSheet(file, "NISN opsional. Nama Sekolah dan Nama PT wajib dipilih dari referensi resmi pada sheet tersembunyi.")
	bufferOut, err := file.WriteToBuffer()
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", `attachment; filename="template-pengumuman-pt-admin.xlsx"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", bufferOut.Bytes())
}

func addStringLookupNamedRange(file *excelize.File, sheet, header, rangeName string, values []string) error {
	if len(values) == 0 {
		return nil
	}
	if _, err := file.NewSheet(sheet); err != nil {
		return err
	}
	lastRow, err := populateLookupSheet(file, sheet, header, values)
	if err != nil {
		return err
	}
	if err := file.SetSheetVisible(sheet, false); err != nil {
		return err
	}
	return file.SetDefinedName(&excelize.DefinedName{Name: rangeName, RefersTo: fmt.Sprintf("%s!$A$2:$A$%d", sheet, lastRow)})
}

func (h SiswaHandler) CreateAdminPTOffer(c *gin.Context) {
	user, ok := requireAdminPTUser(c)
	if !ok {
		return
	}
	var input services.AdminPTOfferInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	item, err := (services.SiswaService{DB: h.DB}).CreateAdminPTOffer(c, user, input)
	switch {
	case errors.Is(err, auth.ErrForbidden):
		utils.RespondForbidden(c)
	case err != nil:
		utils.RespondBadRequest(c, err.Error())
	default:
		utils.RespondData(c, http.StatusCreated, item)
	}
}

func (h SiswaHandler) ListAdminPTOffers(c *gin.Context) {
	user, ok := requireAdminPTUser(c)
	if !ok {
		return
	}
	year, _ := strconv.Atoi(strings.TrimSpace(c.Query("year")))
	limit, _ := strconv.Atoi(strings.TrimSpace(c.Query("limit")))
	items, err := (services.SiswaService{DB: h.DB}).ListAdminPTOffers(c, user, year, limit)
	if errors.Is(err, auth.ErrForbidden) {
		utils.RespondForbidden(c)
		return
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, items)
}

func (h SiswaHandler) UpdateAdminPTOffer(c *gin.Context) {
	user, ok := requireAdminPTUser(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		utils.RespondBadRequest(c, "ID tidak valid.")
		return
	}
	var input services.AdminPTOfferInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	item, err := (services.SiswaService{DB: h.DB}).UpdateAdminPTOffer(c, user, id, input)
	if errors.Is(err, auth.ErrForbidden) {
		utils.RespondForbidden(c)
		return
	}
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}
	utils.RespondData(c, http.StatusOK, item)
}

func (h SiswaHandler) ListPTOfficialCoverage(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	year, _ := strconv.Atoi(strings.TrimSpace(c.Query("year")))
	items, err := (services.SiswaService{DB: h.DB}).ListPTOfficialCoverage(c, user, year)
	if errors.Is(err, auth.ErrForbidden) {
		utils.RespondForbidden(c)
		return
	}
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, items)
}

func (h SiswaHandler) UpsertPTOfficialCount(c *gin.Context) {
	user, ok := requireAdminPTUser(c)
	if !ok {
		return
	}
	var input services.PTOfficialCountInput
	if err := c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	item, err := (services.SiswaService{DB: h.DB}).UpsertPTOfficialCount(c, user, input)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}
	utils.RespondData(c, http.StatusOK, item)
}
