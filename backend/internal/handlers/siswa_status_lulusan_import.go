package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type statusLulusanHeaderOption struct {
	Label   string   `json:"label"`
	Headers []string `json:"headers"`
}

func (h SiswaHandler) ImportStatusLulusan(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		utils.RespondForbidden(c)
		return
	}

	status, ok := normalizeStatusLulusanImport(c.Query("status"))
	if !ok {
		utils.RespondBadRequest(c, "status wajib diisi dan harus PT/KERJA/WIRAUSAHA/BELUM.")
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

	foundHeaders, err := parsers.ExtractHeaderRow(buffer, "")
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	service := services.SiswaService{DB: h.DB}
	switch {
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaOutcomeTemplateHeaders):
		parsed, parseErr := parsers.ParseSiswaOutcomes(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportOutcomes(c, user, schoolID, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaOutcomeWorkTemplateHeaders):
		if status != "KERJA" {
			respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
			return
		}
		parsed, parseErr := parsers.ParseSiswaOutcomeWork(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportOutcomeWorkRows(c, user, schoolID, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaOutcomeEntrepreneurTemplateHeaders):
		if status != "WIRAUSAHA" {
			respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
			return
		}
		parsed, parseErr := parsers.ParseSiswaOutcomeEntrepreneur(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportOutcomeEntrepreneurRowsV2(c, user, schoolID, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaOutcomeEntrepreneurCheckboxLegacyTemplateHeaders):
		if status != "WIRAUSAHA" {
			respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
			return
		}
		parsed, parseErr := parsers.ParseSiswaOutcomeEntrepreneurCheckboxLegacy(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportOutcomeEntrepreneurRowsV2(c, user, schoolID, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaOutcomeUndefinedTemplateHeaders):
		if status != "BELUM" {
			respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
			return
		}
		parsed, parseErr := parsers.ParseSiswaOutcomeUndefined(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportOutcomeUndefinedRows(c, user, schoolID, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacyHigherEdTemplateHeaders):
		if status != "PT" {
			respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
			return
		}
		settings, _, legacyOK := h.ensureLegacyImportEnabled(c, user, schoolID)
		if !legacyOK {
			return
		}
		parsed, parseErr := parsers.ParseSiswaHistoricalFinals(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportHistoricalFinals(c, schoolID, user.ID, settings.MaxYear, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacyWorkTemplateHeaders):
		if status != "KERJA" {
			respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
			return
		}
		settings, _, legacyOK := h.ensureLegacyImportEnabled(c, user, schoolID)
		if !legacyOK {
			return
		}
		parsed, parseErr := parsers.ParseSiswaLegacyWork(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportLegacyWorkDetailed(c, schoolID, user.ID, settings.MaxYear, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacyEntrepreneurTemplateHeaders):
		if status != "WIRAUSAHA" {
			respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
			return
		}
		settings, _, legacyOK := h.ensureLegacyImportEnabled(c, user, schoolID)
		if !legacyOK {
			return
		}
		parsed, parseErr := parsers.ParseSiswaLegacyEntrepreneur(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportLegacyEntrepreneurDetailed(c, schoolID, user.ID, settings.MaxYear, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacyUndefinedTemplateHeaders):
		if status != "BELUM" {
			respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
			return
		}
		settings, _, legacyOK := h.ensureLegacyImportEnabled(c, user, schoolID)
		if !legacyOK {
			return
		}
		parsed, parseErr := parsers.ParseSiswaLegacyUndefined(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportLegacyUndefinedDetailed(c, schoolID, user.ID, settings.MaxYear, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacySimpleTemplateHeaders):
		finalType, mapped := mapLegacySimpleFinalType(status)
		if !mapped {
			respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
			return
		}
		settings, _, legacyOK := h.ensureLegacyImportEnabled(c, user, schoolID)
		if !legacyOK {
			return
		}
		parsed, parseErr := parsers.ParseSiswaLegacyFinalSimple(buffer)
		if parseErr != nil {
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		result, importErr := service.ImportLegacyFinals(c, schoolID, user.ID, finalType, settings.MaxYear, parsed.Rows)
		if importErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	default:
		respondStatusLulusanTemplateMismatch(c, status, foundHeaders)
		return
	}
}

func normalizeStatusLulusanImport(raw string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "PT":
		return "PT", true
	case "KERJA":
		return "KERJA", true
	case "WIRAUSAHA":
		return "WIRAUSAHA", true
	case "BELUM":
		return "BELUM", true
	default:
		return "", false
	}
}

func mapLegacySimpleFinalType(status string) (string, bool) {
	switch status {
	case "KERJA":
		return "WORK", true
	case "WIRAUSAHA":
		return "ENTREPRENEUR", true
	case "BELUM":
		return "UNDEFINED", true
	default:
		return "", false
	}
}

func statusLulusanExpectedHeaders(status string) []statusLulusanHeaderOption {
	options := []statusLulusanHeaderOption{}
	switch status {
	case "PT":
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Final Siswa Akhir",
			Headers: parsers.SiswaOutcomeTemplateHeaders,
		})
	case "KERJA":
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Final Siswa Akhir",
			Headers: parsers.SiswaOutcomeWorkTemplateHeaders,
		})
	case "WIRAUSAHA":
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Final Siswa Akhir",
			Headers: parsers.SiswaOutcomeEntrepreneurTemplateHeaders,
		})
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Final Siswa Akhir (Legacy Checkbox)",
			Headers: parsers.SiswaOutcomeEntrepreneurCheckboxLegacyTemplateHeaders,
		})
	case "BELUM":
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Final Siswa Akhir",
			Headers: parsers.SiswaOutcomeUndefinedTemplateHeaders,
		})
	}
	if status != "PT" {
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Final Siswa Akhir (Legacy Lama)",
			Headers: parsers.SiswaOutcomeTemplateHeaders,
		})
	}
	if status == "PT" {
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Siswa Alumni",
			Headers: parsers.SiswaLegacyHigherEdTemplateHeaders,
		})
		return options
	}
	switch status {
	case "KERJA":
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Siswa Alumni",
			Headers: parsers.SiswaLegacyWorkTemplateHeaders,
		})
	case "WIRAUSAHA":
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Siswa Alumni",
			Headers: parsers.SiswaLegacyEntrepreneurTemplateHeaders,
		})
	case "BELUM":
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Siswa Alumni",
			Headers: parsers.SiswaLegacyUndefinedTemplateHeaders,
		})
	}
	if status != "PT" {
		options = append(options, statusLulusanHeaderOption{
			Label:   "Template Siswa Alumni (Legacy Lama)",
			Headers: parsers.SiswaLegacySimpleTemplateHeaders,
		})
	}
	return options
}

func respondStatusLulusanTemplateMismatch(c *gin.Context, status string, found []string) {
	options := statusLulusanExpectedHeaders(status)
	primary := []string{}
	if len(options) > 0 {
		primary = options[0].Headers
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusBadRequest, utils.APIResponse{
		Error:   "TEMPLATE_MISMATCH",
		Message: siswaTemplateMismatchMessage,
		Details: gin.H{
			"expected_headers":          primary,
			"expected_header_templates": options,
			"found_headers":             found,
		},
	})
}
