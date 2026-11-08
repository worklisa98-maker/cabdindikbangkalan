package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/parsers"
	"cabdin-bangkalan-backend/internal/ptchoices"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type SiswaHandler struct {
	DB *db.DB
}

const siswaTemplateMismatchMessage = "Header template tidak sesuai. Silakan download template resmi dan isi sesuai kolom."

type finalizeSiswaRequest struct {
	SchoolID   *int   `json:"school_id"`
	NISN       string `json:"nisn"`
	TahunLulus int    `json:"tahun_lulus"`
	OfferID    int64  `json:"offer_id"`
}

type cancelSiswaRequest struct {
	SchoolID   *int   `json:"school_id"`
	NISN       string `json:"nisn"`
	TahunLulus int    `json:"tahun_lulus"`
}

type upsertEligibleSNBPRequest struct {
	SchoolID      *int   `json:"school_id"`
	TahunLulus    int    `json:"tahun_lulus"`
	EligibleCount int    `json:"eligible_count"`
	Note          string `json:"note"`
}

type upsertSupervisorSnapshotRequest struct {
	SchoolID       int    `json:"school_id"`
	TahunLulus     int    `json:"tahun_lulus"`
	SNBPCount      int    `json:"snbp_count"`
	SNBTCount      int    `json:"snbt_count"`
	MandiriCount   int    `json:"mandiri_count"`
	KedinasanCount int    `json:"kedinasan_count"`
	LainnyaCount   int    `json:"lainnya_count"`
	Note           string `json:"note"`
}

type studentFinalRequestSubmitRequest struct {
	StudentID   int64  `json:"student_id"`
	RequestType string `json:"request_type"`
	Reason      string `json:"reason"`
	SchoolNote  string `json:"school_note"`
	SupportLink string `json:"support_link"`
}

type studentFinalRequestProcessRequest struct {
	Action        string `json:"action"`
	ProcessorNote string `json:"processor_note"`
}

type studentFinalDelegatesRequest struct {
	UserIDs []int `json:"user_ids"`
}

type validationFinalizeBulkRequest struct {
	SchoolID    *int     `json:"school_id"`
	FinalStatus string   `json:"final_status"`
	Slot        *int     `json:"slot"`
	TahunLulus  int      `json:"tahun_lulus"`
	NISNList    []string `json:"nisn_list"`
	StudentIDs  []int64  `json:"student_ids"`
}

type cohortListResponse struct {
	ID         int64  `json:"id"`
	SchoolID   int    `json:"school_id"`
	SchoolName string `json:"school_name"`
	NISN       string `json:"nisn"`
	NamaSiswa  string `json:"namaSiswa"`
	Rombel     string `json:"rombel"`
	TahunLulus int    `json:"tahunLulus"`
	Status     string `json:"status"`
}

type applicationListResponse struct {
	ID                  int64      `json:"id"`
	NISN                string     `json:"nisn"`
	NamaSiswa           string     `json:"namaSiswa"`
	Rombel              string     `json:"rombel"`
	JalurPenerimaan     string     `json:"jalurPenerimaan"`
	NamaPerguruanTinggi string     `json:"namaPerguruanTinggi"`
	Status              string     `json:"status"`
	FinalStatus         string     `json:"final_status"`
	FinalizedAt         *time.Time `json:"finalized_at"`
}

func (h SiswaHandler) ImportCohorts(c *gin.Context) {
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

	buffer, filename, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	uploadService := services.UploadService{DB: h.DB}
	storageKey := fmt.Sprintf("inline/%d/cohorts/%d", schoolID, time.Now().UnixNano())
	uploadID, _, err := uploadService.Create(c, user, "COHORTS", time.Now().Year(), nil, storageKey, filename)
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
	markFailed := func(err error) {
		_ = uploadService.MarkFailed(c, uploadID, err)
	}

	foundHeaders, ok, err := parsers.ValidateExactHeaders(buffer, "", parsers.SiswaCohortTemplateHeaders)
	if err != nil {
		markFailed(err)
		utils.RespondInternalServerError(c)
		return
	}
	if !ok {
		markFailed(errors.New("template mismatch"))
		respondSiswaHeaderMismatch(c, parsers.SiswaCohortTemplateHeaders, foundHeaders)
		return
	}
	parsed, err := parsers.ParseSiswaCohorts(buffer)
	if err != nil {
		markFailed(err)
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		markFailed(errors.New("missing columns"))
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportCohorts(c, schoolID, uploadID, parsed.Rows)
	if err != nil {
		markFailed(err)
		utils.RespondInternalServerError(c)
		return
	}
	if err := uploadService.MarkSuccess(c, uploadID); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportApplications(c *gin.Context) {
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

	buffer, filename, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	uploadService := services.UploadService{DB: h.DB}
	storageKey := fmt.Sprintf("inline/%d/applications/%d", schoolID, time.Now().UnixNano())
	uploadID, _, err := uploadService.Create(c, user, "APPLICATIONS", time.Now().Year(), nil, storageKey, filename)
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
	markFailed := func(err error) {
		_ = uploadService.MarkFailed(c, uploadID, err)
	}

	foundHeaders, ok, err := parsers.ValidateExactHeaders(buffer, "", parsers.SiswaApplicationTemplateHeaders)
	if err != nil {
		markFailed(err)
		utils.RespondInternalServerError(c)
		return
	}
	if len(foundHeaders) == 0 {
		if err := uploadService.MarkSuccess(c, uploadID); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(services.SiswaImportResult{}))
		return
	}
	if !ok {
		markFailed(errors.New("template mismatch"))
		respondSiswaHeaderMismatch(c, parsers.SiswaApplicationTemplateHeaders, foundHeaders)
		return
	}
	parsed, err := parsers.ParseSiswaApplications(buffer)
	if err != nil {
		markFailed(err)
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		markFailed(errors.New("missing columns"))
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}
	if len(parsed.Rows) == 0 {
		if err := uploadService.MarkSuccess(c, uploadID); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(services.SiswaImportResult{}))
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportApplications(c, schoolID, uploadID, parsed.Rows)
	if err != nil {
		markFailed(err)
		utils.RespondInternalServerError(c)
		return
	}
	if err := uploadService.MarkSuccess(c, uploadID); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportAcceptanceOffers(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if user.Role != "SCHOOL_ADMIN" {
		logging.Warnf("acceptance_offers.import", "forbidden", "path=%s user_id=%d role=%s", c.Request.URL.Path, user.ID, strings.TrimSpace(user.Role))
		utils.RespondForbidden(c)
		return
	}

	scope := siswaResolveScope(c, user, true)
	if scope == nil {
		logging.Warnf("acceptance_offers.import", "scope_missing", "path=%s user_id=%d role=%s school_id=%v", c.Request.URL.Path, user.ID, strings.TrimSpace(user.Role), user.SchoolID)
		return
	}
	schoolID := *scope

	requestStartedAt := time.Now()
	requestOutcome := "failed"
	requestStage := "init"
	defer func() {
		if requestOutcome == "success" {
			_, _ = h.DB.Pool.Exec(c, `
				UPDATE outcome_pt_offers
				SET school_confirmed_by=$1
				WHERE school_id=$2 AND school_confirmed_at IS NOT NULL AND school_confirmed_by IS NULL
			`, user.ID, schoolID)
		}
		logging.Infof(
			"acceptance_offers.import",
			"finish",
			"path=%s user_id=%d role=%s school_id=%d outcome=%s stage=%s duration_ms=%d",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			requestOutcome,
			requestStage,
			time.Since(requestStartedAt).Milliseconds(),
		)
	}()

	buffer, filename, err := readExcelUpload(c)
	if err != nil {
		requestStage = "read_upload"
		logging.Warnf(
			"acceptance_offers.import",
			"bad_request",
			"path=%s user_id=%d role=%s school_id=%d stage=%s err=%v",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			requestStage,
			err,
		)
		utils.RespondBadRequest(c, err.Error())
		return
	}
	logging.Infof(
		"acceptance_offers.import",
		"start",
		"path=%s user_id=%d role=%s school_id=%d file_name=%q file_size_bytes=%d",
		c.Request.URL.Path,
		user.ID,
		strings.TrimSpace(user.Role),
		schoolID,
		filename,
		len(buffer),
	)

	requestStage = "extract_headers"
	foundHeaders, err := parsers.ExtractHeaderRow(buffer, "")
	if err != nil {
		logging.Errorf(
			"acceptance_offers.import",
			"extract_headers",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q err=%v",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			err,
		)
		utils.RespondInternalServerError(c)
		return
	}
	switch {
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaPTOfferCompactTemplateHeaders):
		requestStage = "parse_compact_template"
		parsed, parseErr := parsers.ParseSiswaAcceptanceOfferCompact(buffer)
		if parseErr != nil {
			logging.Warnf(
				"acceptance_offers.import",
				"parse_failed",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=compact err=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				parseErr,
			)
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			logging.Warnf(
				"acceptance_offers.import",
				"missing_columns",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=compact missing_columns=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				parsed.MissingColumns,
			)
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}

		service := services.SiswaService{DB: h.DB}
		requestStage = "service_import_acceptance_offer_compact_announcements"
		result, importErr := service.ImportAcceptanceOfferCompactAnnouncements(c, schoolID, parsed.Rows)
		if importErr != nil {
			logging.Errorf(
				"acceptance_offers.import",
				"service_error",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=compact service=ImportAcceptanceOfferCompactAnnouncements err=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				importErr,
			)
			utils.RespondInternalServerError(c)
			return
		}
		requestOutcome = "success"
		requestStage = "response_success"
		logging.Infof(
			"acceptance_offers.import",
			"success",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=compact imported=%d rejected=%d",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			result.Accepted,
			len(result.Rejections),
		)
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case siswaPTOfferHeadersMatch(foundHeaders):
		requestStage = "parse_official_template"
		parsed, parseErr := parsers.ParseSiswaAcceptanceOffers(buffer)
		if parseErr != nil {
			logging.Warnf(
				"acceptance_offers.import",
				"parse_failed",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=official err=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				parseErr,
			)
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			logging.Warnf(
				"acceptance_offers.import",
				"missing_columns",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=official missing_columns=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				parsed.MissingColumns,
			)
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		logging.Infof(
			"acceptance_offers.import",
			"parsed_file",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=official rows=%d",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			len(parsed.Rows),
		)

		service := services.SiswaService{DB: h.DB}
		requestStage = "service_import_acceptance_offer_announcements"
		logging.Infof(
			"acceptance_offers.import",
			"service_start",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=official rows=%d service=ImportAcceptanceOfferAnnouncements",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			len(parsed.Rows),
		)
		result, importErr := service.ImportAcceptanceOfferAnnouncements(c, schoolID, parsed.Rows)
		if importErr != nil {
			logging.Errorf(
				"acceptance_offers.import",
				"service_error",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=official service=ImportAcceptanceOfferAnnouncements err=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				importErr,
			)
			utils.RespondInternalServerError(c)
			return
		}
		requestOutcome = "success"
		requestStage = "response_success"
		logging.Infof(
			"acceptance_offers.import",
			"success",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=official imported=%d rejected=%d",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			result.Accepted,
			len(result.Rejections),
		)
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaPTOfferVerticalLegacyTemplateHeaders):
		requestStage = "parse_vertical_legacy_template"
		parsed, parseErr := parsers.ParseSiswaAcceptanceOfferVerticalLegacy(buffer)
		if parseErr != nil {
			logging.Warnf(
				"acceptance_offers.import",
				"parse_failed",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=vertical_legacy err=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				parseErr,
			)
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			logging.Warnf(
				"acceptance_offers.import",
				"missing_columns",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=vertical_legacy missing_columns=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				parsed.MissingColumns,
			)
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}

		groupedRows := make([]parsers.SiswaAcceptanceOfferAnnouncementRow, 0, len(parsed.Rows))
		for _, legacyRow := range parsed.Rows {
			groupedRows = append(groupedRows, parsers.SiswaAcceptanceOfferAnnouncementRow{
				RowNumber:         legacyRow.RowNumber,
				GroupID:           legacyRow.EntryID,
				NISN:              legacyRow.NISN,
				NamaSiswa:         legacyRow.NamaSiswa,
				Sekolah:           legacyRow.Sekolah,
				Jalur:             legacyRow.Jalur,
				Pilihan1NamaPT:    legacyRow.NamaPT,
				Pilihan1Prodi:     legacyRow.Prodi,
				HasilPengumuman:   legacyRow.HasilPengumuman,
				Pilihan:           legacyRow.Pilihan,
				TanggalPengumuman: legacyRow.TanggalPengumuman,
				Catatan:           legacyRow.Catatan,
			})
		}
		logging.Infof(
			"acceptance_offers.import",
			"parsed_file",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=vertical_legacy rows=%d",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			len(groupedRows),
		)

		service := services.SiswaService{DB: h.DB}
		requestStage = "service_import_acceptance_offer_announcements"
		logging.Infof(
			"acceptance_offers.import",
			"service_start",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=vertical_legacy rows=%d service=ImportAcceptanceOfferAnnouncements",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			len(groupedRows),
		)
		result, importErr := service.ImportAcceptanceOfferAnnouncements(c, schoolID, groupedRows)
		if importErr != nil {
			logging.Errorf(
				"acceptance_offers.import",
				"service_error",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=vertical_legacy service=ImportAcceptanceOfferAnnouncements err=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				importErr,
			)
			utils.RespondInternalServerError(c)
			return
		}
		requestOutcome = "success"
		requestStage = "response_success"
		logging.Infof(
			"acceptance_offers.import",
			"success",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=vertical_legacy imported=%d rejected=%d",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			result.Accepted,
			len(result.Rejections),
		)
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaPTOfferChoiceLegacyTemplateHeaders):
		requestStage = "parse_choice_legacy_template"
		parsed, parseErr := parsers.ParseSiswaAcceptanceOfferChoices(buffer)
		if parseErr != nil {
			logging.Warnf(
				"acceptance_offers.import",
				"parse_failed",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=choice_legacy err=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				parseErr,
			)
			utils.RespondBadRequest(c, "Gagal membaca file Excel.")
			return
		}
		if len(parsed.MissingColumns) > 0 {
			logging.Warnf(
				"acceptance_offers.import",
				"missing_columns",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=choice_legacy missing_columns=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				parsed.MissingColumns,
			)
			utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
			return
		}
		logging.Infof(
			"acceptance_offers.import",
			"parsed_file",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=choice_legacy rows=%d",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			len(parsed.Rows),
		)

		service := services.SiswaService{DB: h.DB}
		requestStage = "service_import_acceptance_offer_choices"
		logging.Infof(
			"acceptance_offers.import",
			"service_start",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=choice_legacy rows=%d service=ImportAcceptanceOfferChoices",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			len(parsed.Rows),
		)
		result, importErr := service.ImportAcceptanceOfferChoices(c, schoolID, parsed.Rows)
		if importErr != nil {
			logging.Errorf(
				"acceptance_offers.import",
				"service_error",
				"path=%s user_id=%d role=%s school_id=%d file_name=%q template=choice_legacy service=ImportAcceptanceOfferChoices err=%v",
				c.Request.URL.Path,
				user.ID,
				strings.TrimSpace(user.Role),
				schoolID,
				filename,
				importErr,
			)
			utils.RespondInternalServerError(c)
			return
		}
		requestOutcome = "success"
		requestStage = "response_success"
		logging.Infof(
			"acceptance_offers.import",
			"success",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q template=choice_legacy imported=%d rejected=%d",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			result.Accepted,
			len(result.Rejections),
		)
		utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
		return
	case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaPTOfferLegacyTemplateHeaders):
		requestStage = "legacy_template_unsupported"
		logging.Warnf(
			"acceptance_offers.import",
			"legacy_template_unsupported",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
		)
		utils.RespondBadRequest(c, "Template lama pengumuman PT tidak lagi didukung. Silakan unduh template terbaru.")
		return
	default:
		requestStage = "template_mismatch"
		logging.Warnf(
			"acceptance_offers.import",
			"template_mismatch",
			"path=%s user_id=%d role=%s school_id=%d file_name=%q found_headers=%v",
			c.Request.URL.Path,
			user.ID,
			strings.TrimSpace(user.Role),
			schoolID,
			filename,
			foundHeaders,
		)
		respondSiswaHeaderMismatch(c, parsers.SiswaPTOfferTemplateHeaders, foundHeaders)
		return
	}

	requestStage = "template_unknown"
	logging.Warnf(
		"acceptance_offers.import",
		"template_unknown",
		"path=%s user_id=%d role=%s school_id=%d file_name=%q",
		c.Request.URL.Path,
		user.ID,
		strings.TrimSpace(user.Role),
		schoolID,
		filename,
	)
	utils.RespondBadRequest(c, "Template pengumuman PT tidak dikenali. Silakan unduh template terbaru.")
}

func (h SiswaHandler) ExportAcceptanceOffersTemplate(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
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

	scope := siswaResolveScope(c, user, false)
	if scope == nil {
		if c.Writer.Written() {
			return
		}
		err := errors.New("school scope is unavailable")
		logAcceptanceOfferTemplateExportFailure(user, scope, year, "resolve_scope", err)
		if user.Role == "SCHOOL_ADMIN" {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondBadRequest(c, "school_id wajib diisi.")
		return
	}

	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := "Pengumuman_Diterima_PT"
	file.SetSheetName("Sheet1", sheet)

	headers := parsers.SiswaPTOfferCompactTemplateHeaders
	if err := setHeaderRow(file, sheet, headers); err != nil {
		logAcceptanceOfferTemplateExportFailure(user, scope, year, "set_header_row", err)
		utils.RespondInternalServerError(c)
		return
	}
	campuses, err := loadCampusListFromDB(c.Request.Context(), h.DB)
	if err != nil {
		logAcceptanceOfferTemplateExportFailure(user, scope, year, "load_campus_list", err)
		utils.RespondInternalServerError(c)
		return
	}
	if len(campuses) > 0 {
		if err := addCampusLookupNamedRange(file, "LOOKUP_KAMPUS", campuses); err != nil {
			logAcceptanceOfferTemplateExportFailure(user, scope, year, "add_campus_lookup", err)
			utils.RespondInternalServerError(c)
			return
		}
		if err := addNamedRangeValidation(file, sheet, headers, "Nama_PT", "KAMPUS_LIST"); err != nil {
			logAcceptanceOfferTemplateExportFailure(user, scope, year, "add_campus_validation", err)
			utils.RespondInternalServerError(c)
			return
		}
	}
	if err := addFixedListValidation(file, sheet, headers, "Jenjang_Jurusan", []string{"D1", "D3", "D4", "S1", "Tidak Diisi"}); err != nil {
		logAcceptanceOfferTemplateExportFailure(user, scope, year, "add_jenjang_validation", err)
		utils.RespondInternalServerError(c)
		return
	}
	if err := addFixedListValidation(file, sheet, headers, "Jalur", []string{"SNBP", "SNBT", "SNBM (Mandiri)", "Kedinasan", "Lainnya"}); err != nil {
		logAcceptanceOfferTemplateExportFailure(user, scope, year, "add_jalur_validation", err)
		utils.RespondInternalServerError(c)
		return
	}
	applyTemplateSheetLayout(file, sheet, headers)
	_ = addTemplateInfoSheet(file, "Setiap baris mewakili satu penerimaan PT. NPSN dan Nama Sekolah diambil dari akun login. Semua baris pada template ringkas dianggap DITERIMA.")

	buffer, err := file.WriteToBuffer()
	if err != nil {
		logAcceptanceOfferTemplateExportFailure(user, scope, year, "write_buffer", err)
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "attachment; filename=\"template-pengumuman-diterima-pt.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h SiswaHandler) AcceptanceOfferAnalytics(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "siswa", "acceptance_offer_analytics", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
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

	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.AcceptanceOfferAnalytics(ctx, user, services.AcceptanceOfferAnalyticsFilters{
		SchoolID:         requestedSchoolID,
		Jenjang:          strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah:    strings.TrimSpace(c.Query("status_sekolah")),
		Year:             parsePositiveInt(strings.TrimSpace(c.Query("year"))),
		Path:             strings.TrimSpace(c.Query("jalur")),
		Search:           strings.TrimSpace(c.Query("q")),
		ValidationStatus: strings.TrimSpace(c.Query("validation_status")),
		Page:             page,
		PageSize:         pageSize,
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			if handleOperationalError(c, h.DB, "siswa", "acceptance_offer_analytics", err) {
				return
			}
			respondSafeAnalyticsFallback(c, "siswa", "acceptance_offer_analytics", "Acceptance offer analytics is temporarily unavailable.", emptyAcceptanceOfferAnalyticsPayload(page, pageSize), err)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) AcceptanceOfferSchoolSummary(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "siswa", "acceptance_offer_school_summary", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if !services.CanAccessAcceptanceOfferSchoolSummary(user.Role) {
		utils.RespondForbidden(c)
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}

	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.AcceptanceOfferSchoolSummary(ctx, user, services.AcceptanceOfferSchoolSummaryFilters{
		SchoolID:      requestedSchoolID,
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
		Year:          parsePositiveInt(strings.TrimSpace(c.Query("year"))),
		Path:          strings.TrimSpace(c.Query("jalur")),
		Search:        strings.TrimSpace(c.Query("q")),
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			if handleOperationalError(c, h.DB, "siswa", "acceptance_offer_school_summary", err) {
				return
			}
			respondSafeAnalyticsFallback(
				c,
				"siswa",
				"acceptance_offer_school_summary",
				"Acceptance offer school summary is temporarily unavailable.",
				services.AcceptanceOfferSchoolSummaryPayload{Items: []services.AcceptanceOfferSchoolSummaryItem{}},
				err,
			)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) ExportAcceptanceOfferSchoolSummary(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if !services.CanAccessAcceptanceOfferSchoolSummary(user.Role) {
		utils.RespondForbidden(c)
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}

	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	payload, err := service.AcceptanceOfferSchoolSummary(ctx, user, services.AcceptanceOfferSchoolSummaryFilters{
		SchoolID:      requestedSchoolID,
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
		Year:          parsePositiveInt(strings.TrimSpace(c.Query("year"))),
		Path:          strings.TrimSpace(c.Query("jalur")),
		Search:        strings.TrimSpace(c.Query("q")),
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			if handleOperationalError(c, h.DB, "siswa", "acceptance_offer_school_summary_export", err) {
				return
			}
			utils.RespondInternalServerError(c)
		}
		return
	}

	file := excelize.NewFile()
	defer func() { _ = file.Close() }()

	sheet := "Rekap_Pengumuman_PT"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Nama Sekolah",
		"Jenjang",
		"Status Sekolah",
		"Upload Data Siswa Akhir",
		"Pendaftaran SNBP",
		"Penerimaan SNBP",
		"Pendaftaran SNBT",
		"Penerimaan SNBT",
		"Pendaftaran Mandiri",
		"Penerimaan Mandiri",
		"Pendaftaran Kedinasan",
		"Penerimaan Kedinasan",
		"Pendaftaran Lainnya",
		"Penerimaan Lainnya",
	}
	if err := setHeaderRow(file, sheet, headers); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	for index, item := range payload.Items {
		cell, cellErr := excelize.CoordinatesToCellName(1, index+2)
		if cellErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		rowValues := []interface{}{
			item.Sekolah,
			item.Jenjang,
			item.StatusSekolah,
			item.UploadDataSiswaAkhir,
			item.PendaftaranSNBP,
			item.PenerimaanSNBP,
			item.PendaftaranSNBT,
			item.PenerimaanSNBT,
			item.PendaftaranMandiri,
			item.PenerimaanMandiri,
			item.PendaftaranKedinasan,
			item.PenerimaanKedinasan,
			item.PendaftaranLainnya,
			item.PenerimaanLainnya,
		}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
	}

	applyTemplateSheetLayout(file, sheet, headers)
	for idx := range headers {
		col, colErr := excelize.ColumnNumberToName(idx + 1)
		if colErr != nil {
			continue
		}
		width := 16.0
		if idx == 0 {
			width = 32
		}
		if idx == 1 || idx == 2 {
			width = 18
		}
		_ = file.SetColWidth(sheet, col, col, width)
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "attachment; filename=\"rekap-pengumuman-pt-per-sekolah.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h SiswaHandler) EligibleSNBP(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}
	yearRaw := strings.TrimSpace(c.Query("year"))
	if yearRaw == "" {
		yearRaw = strings.TrimSpace(c.Query("tahun_lulus"))
	}
	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()
	payload, err := service.ListEligibleSNBP(ctx, user, services.EligibleSNBPFilters{
		SchoolID:      requestedSchoolID,
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
		Year:          parsePositiveInt(yearRaw),
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			if handleOperationalError(c, h.DB, "siswa", "eligible_snbp", err) {
				return
			}
			respondSafeListFallback(c, "siswa", "eligible_snbp", "Eligible SNBP is temporarily unavailable.", services.EligibleSNBPPayload{Items: []services.EligibleSNBPItem{}}, err)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) UpsertEligibleSNBP(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	var req upsertEligibleSNBPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, authRequestTimeout)
	defer cancel()
	err := service.UpsertEligibleSNBP(ctx, user, services.UpsertEligibleSNBPInput{
		SchoolID:      req.SchoolID,
		TahunLulus:    req.TahunLulus,
		EligibleCount: req.EligibleCount,
		Note:          req.Note,
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		case services.ErrInvalidOutcome:
			utils.RespondBadRequest(c, "Data eligible SNBP tidak valid.")
		default:
			if handleOperationalError(c, h.DB, "siswa", "eligible_snbp_upsert", err) {
				return
			}
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"ok": true})
}

func (h SiswaHandler) SupervisorPTComparison(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	if !services.CanAccessAcceptanceOfferSchoolSummary(user.Role) {
		utils.RespondForbidden(c)
		return
	}
	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}
	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()
	payload, err := service.SupervisorPTComparison(ctx, user, services.SupervisorComparisonFilters{
		SchoolID:      requestedSchoolID,
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
		Year:          parsePositiveInt(strings.TrimSpace(c.Query("year"))),
		Path:          strings.TrimSpace(c.Query("jalur")),
		Search:        strings.TrimSpace(c.Query("q")),
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			if handleOperationalError(c, h.DB, "siswa", "supervisor_pt_comparison", err) {
				return
			}
			respondSafeAnalyticsFallback(c, "siswa", "supervisor_pt_comparison", "Supervisor comparison is temporarily unavailable.", services.SupervisorComparisonPayload{Items: []services.SupervisorComparisonItem{}}, err)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) UpsertSupervisorPTSnapshot(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	var req upsertSupervisorSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, authRequestTimeout)
	defer cancel()
	err := service.UpsertSupervisorPTSnapshot(ctx, user, services.UpsertSupervisorSnapshotInput{
		SchoolID:       req.SchoolID,
		TahunLulus:     req.TahunLulus,
		SNBPCount:      req.SNBPCount,
		SNBTCount:      req.SNBTCount,
		MandiriCount:   req.MandiriCount,
		KedinasanCount: req.KedinasanCount,
		LainnyaCount:   req.LainnyaCount,
		Note:           req.Note,
	})
	if err != nil {
		if errors.Is(err, services.ErrSupervisorSnapshotRoleForbidden) {
			utils.RespondError(c, http.StatusForbidden, "Role tidak diizinkan menyimpan rekap pengawas.")
			return
		}
		if errors.Is(err, services.ErrSupervisorSnapshotScopeForbidden) {
			utils.RespondError(c, http.StatusForbidden, "Sekolah tidak termasuk binaan pengawas atau tidak berada dalam scope pengguna.")
			return
		}
		switch err {
		case auth.ErrForbidden:
			utils.RespondError(c, http.StatusForbidden, "Tidak diizinkan menyimpan rekap pengawas untuk sekolah ini.")
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		case services.ErrInvalidOutcome:
			utils.RespondBadRequest(c, "school_id, tahun_lulus, atau jumlah rekap pengawas tidak valid.")
		default:
			if handleOperationalError(c, h.DB, "siswa", "supervisor_pt_snapshot_upsert", err) {
				return
			}
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"ok": true})
}

func (h SiswaHandler) StudentFinalAccess(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	service := services.SiswaService{DB: h.DB}
	payload, err := service.StudentFinalAccess(c, user)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h SiswaHandler) ListStudentFinalRequests(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(strings.TrimSpace(c.Query("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("pageSize")))
	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListStudentFinalRequests(c, user, services.StudentFinalRequestFilters{
		SchoolID:    requestedSchoolID,
		RequestType: strings.TrimSpace(c.Query("type")),
		Status:      strings.TrimSpace(c.Query("status")),
		Page:        page,
		PageSize:    pageSize,
		Search:      strings.TrimSpace(c.Query("q")),
	})
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
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	utils.RespondData(c, http.StatusOK, gin.H{"data": rows, "meta": gin.H{"page": page, "pageSize": pageSize, "total": total}})
}

func (h SiswaHandler) SubmitStudentFinalRequest(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	var req studentFinalRequestSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	service := services.SiswaService{DB: h.DB}
	item, err := service.SubmitStudentFinalRequest(c, user, services.StudentFinalRequestInput{
		StudentID: req.StudentID, RequestType: req.RequestType, Reason: req.Reason, SchoolNote: req.SchoolNote, SupportLink: req.SupportLink,
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case services.ErrStudentFinalRequestValidation:
			utils.RespondBadRequestMessage(c, "Alasan/jenis koreksi dan catatan sekolah wajib diisi.")
		case services.ErrStudentFinalDriveLink:
			utils.RespondBadRequestMessage(c, "Link surat pernyataan wajib berupa link file Google Drive, bukan folder.")
		case services.ErrStudentFinalRequestNotFound:
			utils.RespondNotFound(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, item)
}

func (h SiswaHandler) ProcessStudentFinalRequest(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		utils.RespondBadRequest(c, "Invalid request id")
		return
	}
	var req studentFinalRequestProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	service := services.SiswaService{DB: h.DB}
	item, err := service.ProcessStudentFinalRequest(c, user, services.StudentFinalProcessInput{ID: id, Action: req.Action, ProcessorNote: req.ProcessorNote})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case services.ErrStudentFinalRequestValidation:
			utils.RespondBadRequest(c, "Aksi pengajuan tidak valid.")
		case services.ErrStudentFinalRequestNotFound:
			utils.RespondNotFound(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}
	utils.RespondData(c, http.StatusOK, item)
}

func (h SiswaHandler) ListStudentFinalDelegates(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	service := services.SiswaService{DB: h.DB}
	items, err := service.ListStudentFinalDelegates(c, user)
	if err != nil {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"items": items})
}

func (h SiswaHandler) SetStudentFinalDelegates(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	var req studentFinalDelegatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}
	service := services.SiswaService{DB: h.DB}
	if err := service.SetStudentFinalDelegates(c, user, req.UserIDs); err != nil {
		if err == auth.ErrForbidden {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"ok": true})
}

func (h SiswaHandler) ExportAcceptedAcceptanceOffers(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}
	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}
	search := strings.TrimSpace(c.Query("q"))
	if search == "" {
		search = strings.TrimSpace(c.Query("search"))
	}
	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()
	rows, err := service.AcceptedOfferExportRows(ctx, user, services.AcceptedOfferExportFilters{
		SchoolID:      requestedSchoolID,
		Jenjang:       strings.TrimSpace(c.Query("jenjang")),
		StatusSekolah: strings.TrimSpace(c.Query("status_sekolah")),
		Year:          parsePositiveInt(strings.TrimSpace(c.Query("year"))),
		Path:          strings.TrimSpace(c.Query("jalur")),
		Search:        search,
	})
	if err != nil {
		switch err {
		case auth.ErrForbidden:
			utils.RespondForbidden(c)
		case auth.ErrUnauthorized:
			utils.RespondUnauthorized(c)
		default:
			if handleOperationalError(c, h.DB, "siswa", "acceptance_offer_accepted_export", err) {
				return
			}
			utils.RespondInternalServerError(c)
		}
		return
	}
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	sheet := "Detail_Diumumkan"
	file.SetSheetName("Sheet1", sheet)
	headers := []string{"Nama Sekolah", "Jenjang", "Status Sekolah", "NISN", "Nama Siswa", "Rombel", "Tahun Lulus", "Jalur", "Nama PT", "Prodi/Jurusan", "Tahun Diterima", "Tanggal Pengumuman"}
	if err := setHeaderRow(file, sheet, headers); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	for index, item := range rows {
		cell, cellErr := excelize.CoordinatesToCellName(1, index+2)
		if cellErr != nil {
			utils.RespondInternalServerError(c)
			return
		}
		tahunDiterima := interface{}("")
		if item.TahunDiterima != nil {
			tahunDiterima = *item.TahunDiterima
		}
		tanggal := ""
		if item.TanggalPengumuman != nil {
			tanggal = item.TanggalPengumuman.Format("2006-01-02")
		}
		rowValues := []interface{}{item.Sekolah, item.Jenjang, item.StatusSekolah, item.NISN, item.NamaSiswa, item.Rombel, item.TahunLulus, item.Jalur, item.NamaPT, item.Prodi, tahunDiterima, tanggal}
		if err := file.SetSheetRow(sheet, cell, &rowValues); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
	}
	applyTemplateSheetLayout(file, sheet, headers)
	buffer, err := file.WriteToBuffer()
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "attachment; filename=\"detail-siswa-diumumkan-pt.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer.Bytes())
}

func (h SiswaHandler) ImportUKK(c *gin.Context) {
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
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}
	schoolID := *scope

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}
	parsed, err := parsers.ParseSiswaUKK(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportUKK(c, schoolID, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportHistoricalFinal(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, true)
	if scope == nil {
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}
	schoolID := *scope

	settings, _, ok := h.ensureLegacyImportEnabled(c, user, schoolID)
	if !ok {
		return
	}

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}
	parsed, err := parsers.ParseSiswaHistoricalFinals(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	service := services.SiswaService{DB: h.DB}
	result, err := service.ImportHistoricalFinals(c, schoolID, user.ID, settings.MaxYear, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ImportLegacyWork(c *gin.Context) {
	h.importLegacyFinalSimple(c, "WORK")
}

func (h SiswaHandler) ImportLegacyEntrepreneur(c *gin.Context) {
	h.importLegacyFinalSimple(c, "ENTREPRENEUR")
}

func (h SiswaHandler) ImportLegacyUndefined(c *gin.Context) {
	h.importLegacyFinalSimple(c, "UNDEFINED")
}

func (h SiswaHandler) importLegacyFinalSimple(c *gin.Context, finalType string) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, true)
	if scope == nil {
		if user.Role == "ADMIN" || user.Role == "ADMIN_MASTER" {
			utils.RespondBadRequest(c, "school_id wajib diisi.")
		}
		return
	}
	schoolID := *scope

	settings, _, ok := h.ensureLegacyImportEnabled(c, user, schoolID)
	if !ok {
		return
	}

	buffer, _, err := readExcelUpload(c)
	if err != nil {
		utils.RespondBadRequest(c, err.Error())
		return
	}

	service := services.SiswaService{DB: h.DB}
	foundHeaders, err := parsers.ExtractHeaderRow(buffer, "")
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	switch finalType {
	case "WORK":
		switch {
		case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacyWorkTemplateHeaders):
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
		case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacySimpleTemplateHeaders):
		default:
			respondSiswaHeaderMismatch(c, parsers.SiswaLegacyWorkTemplateHeaders, foundHeaders)
			return
		}
	case "ENTREPRENEUR":
		switch {
		case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacyEntrepreneurTemplateHeaders):
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
		case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacySimpleTemplateHeaders):
		default:
			respondSiswaHeaderMismatch(c, parsers.SiswaLegacyEntrepreneurTemplateHeaders, foundHeaders)
			return
		}
	case "UNDEFINED":
		switch {
		case parsers.HeadersMatchExact(foundHeaders, parsers.SiswaLegacyUndefinedTemplateHeaders):
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
		default:
			respondSiswaHeaderMismatch(c, parsers.SiswaLegacyUndefinedTemplateHeaders, foundHeaders)
			return
		}
	}

	parsed, err := parsers.ParseSiswaLegacyFinalSimple(buffer)
	if err != nil {
		utils.RespondBadRequest(c, "Gagal membaca file Excel.")
		return
	}
	if len(parsed.MissingColumns) > 0 {
		utils.RespondBadRequest(c, gin.H{"missing_columns": parsed.MissingColumns})
		return
	}

	result, err := service.ImportLegacyFinals(c, schoolID, user.ID, finalType, settings.MaxYear, parsed.Rows)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, siswaImportResponsePayload(result))
}

func (h SiswaHandler) ListCohorts(c *gin.Context) {
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
	includePlanned := false
	if raw := strings.TrimSpace(c.Query("includePlanned")); raw != "" {
		includePlanned = strings.EqualFold(raw, "true") || raw == "1"
	}
	if includePlanned && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
		includePlanned = false
	}

	filters := services.SiswaCohortListFilters{
		SchoolID:       requestedSchoolID,
		Page:           page,
		PageSize:       pageSize,
		Search:         strings.TrimSpace(c.Query("q")),
		Year:           year,
		IncludePlanned: includePlanned,
	}

	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListCohorts(c, user, filters)
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

	payload := make([]cohortListResponse, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, cohortListResponse{
			ID:         row.ID,
			SchoolID:   row.SchoolID,
			SchoolName: row.SchoolName,
			NISN:       row.NISN,
			NamaSiswa:  row.NamaSiswa,
			Rombel:     row.Rombel,
			TahunLulus: row.TahunLulus,
			Status:     row.Status,
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

func (h SiswaHandler) ListApplications(c *gin.Context) {
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

	filters := services.SiswaApplicationListFilters{
		SchoolID: requestedSchoolID,
		Page:     page,
		PageSize: pageSize,
		Search:   strings.TrimSpace(c.Query("q")),
		Year:     year,
	}

	service := services.SiswaService{DB: h.DB}
	rows, total, err := service.ListApplications(c, user, filters)
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

	payload := make([]applicationListResponse, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, applicationListResponse{
			ID:                  row.ID,
			NISN:                row.NISN,
			NamaSiswa:           row.NamaSiswa,
			Rombel:              row.Rombel,
			JalurPenerimaan:     row.JalurPenerimaan,
			NamaPerguruanTinggi: row.NamaPerguruanTinggi,
			Status:              row.Status,
			FinalStatus:         row.FinalStatus,
			FinalizedAt:         row.FinalizedAt,
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

func (h SiswaHandler) ValidationPending(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
		return
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

	service := services.SiswaService{DB: h.DB}
	result, err := service.ValidationPending(c, user, scope, year)
	if err != nil {
		if isMissingRelation(err) {
			if user.Role == "SCHOOL_ADMIN" {
				utils.RespondData(c, http.StatusOK, services.SiswaPendingValidationResponse{
					BelumRencana:         []services.SiswaPendingValidationItem{},
					FinalkanPilihanSiswa: []services.SiswaPendingValidationItem{},
					MultiJalur:           []services.SiswaPendingValidationItem{},
					BelumFinal:           []services.SiswaPendingValidationItem{},
					DiterimaMultiJalur:   []services.SiswaPendingValidationItem{},
				})
				return
			}
			utils.RespondData(c, http.StatusOK, []services.SiswaPendingSchoolSummary{})
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	utils.RespondData(c, http.StatusOK, result)
}

func (h SiswaHandler) ValidationSummary(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	requestedSchoolID, ok := siswaRequestedSchoolIDFromQuery(c)
	if !ok {
		return
	}

	multiScope, scopeErr := auth.GetMultiSchoolScope(c, h.DB, user)
	if scopeErr != nil {
		if errors.Is(scopeErr, auth.ErrForbidden) {
			utils.RespondForbidden(c)
		} else {
			utils.RespondInternalServerError(c)
		}
		return
	}

	scope := auth.SchoolScope{}
	if requestedSchoolID != nil {
		if err := auth.EnsureSchoolInScope(multiScope, *requestedSchoolID); err != nil {
			utils.RespondForbidden(c)
			return
		}
		scope = auth.SchoolScope{IDs: []int{*requestedSchoolID}}
	} else if multiScope.All {
		scope = auth.SchoolScope{All: true}
	} else {
		scope = auth.SchoolScope{IDs: append([]int(nil), multiScope.SchoolIDs...)}
	}
	if !scope.All && len(scope.IDs) == 0 {
		utils.RespondData(c, http.StatusOK, services.SiswaValidationSummary{})
		return
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

	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	result, err := service.ValidationSummaryScope(ctx, user, scope, year)
	if err != nil {
		if handleOperationalError(c, h.DB, "siswa", "validation_summary", err) {
			return
		}
		if isMissingRelation(err) {
			respondSafeSummaryFallback(c, "siswa", "validation_summary", "Siswa validation summary is temporarily unavailable.", emptyValidationSummary(), err)
			return
		}
		respondSafeSummaryFallback(c, "siswa", "validation_summary", "Siswa validation summary is temporarily unavailable.", emptyValidationSummary(), err)
		return
	}
	utils.RespondData(c, http.StatusOK, result)
}

func (h SiswaHandler) ValidationFinalizeBulk(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	var req validationFinalizeBulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	status := strings.ToUpper(strings.TrimSpace(req.FinalStatus))
	if status == "" {
		utils.RespondBadRequest(c, "final_status wajib diisi.")
		return
	}

	if status == "WIRAUSAHA" {
		if req.Slot == nil || *req.Slot < 1 || *req.Slot > 3 {
			utils.RespondBadRequest(c, "slot wajib diisi (1-3) untuk WIRAUSAHA.")
			return
		}
	} else if status == "PT" {
		if req.Slot != nil && !ptchoices.IsValidChoiceSlot(*req.Slot) {
			utils.RespondBadRequest(c, "slot tidak valid.")
			return
		}
	} else {
		if req.Slot != nil {
			utils.RespondBadRequest(c, "slot harus null untuk KERJA/BELUM.")
			return
		}
	}

	targetCount := len(req.NISNList)
	if len(req.StudentIDs) > 0 {
		targetCount = len(req.StudentIDs)
	}
	if targetCount == 0 {
		utils.RespondBadRequest(c, "nisn_list atau student_ids wajib diisi.")
		return
	}
	if targetCount > 500 {
		utils.RespondBadRequest(c, "Maksimal 500 siswa per request.")
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

	service := services.SiswaService{DB: h.DB}
	result, err := service.ValidationFinalizeBulk(c, user, services.ValidationFinalizeBulkInput{
		SchoolID:    *scope,
		FinalStatus: status,
		Slot:        req.Slot,
		Year:        req.TahunLulus,
		NISNList:    req.NISNList,
		StudentIDs:  req.StudentIDs,
		ChangedBy:   user.ID,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidOutcome):
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

	utils.RespondData(c, http.StatusOK, result)
}

func (h SiswaHandler) Finalize(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	var req finalizeSiswaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return
	}

	requestedSchoolID := req.SchoolID
	scope, err := auth.GetSingleSchoolScopeV2(user, requestedSchoolID)
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
	if nisn == "" || req.TahunLulus <= 0 || req.OfferID <= 0 {
		utils.RespondBadRequest(c, "nisn, tahun_lulus, dan offer_id wajib diisi.")
		return
	}

	service := services.SiswaService{DB: h.DB}
	if err := service.FinalizeHigherEd(c, *scope, nisn, req.TahunLulus, req.OfferID); err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
		case errors.Is(err, services.ErrSiswaOfferNotFound):
			utils.RespondBadRequest(c, "Offer tidak valid untuk siswa tersebut.")
		case errors.Is(err, services.ErrSiswaFinalized):
			utils.RespondBadRequest(c, "Data sudah final dan terkunci.")
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"nisn":        nisn,
		"tahun_lulus": req.TahunLulus,
		"offer_id":    req.OfferID,
		"final_type":  "HIGHER_ED",
	})
}

func (h SiswaHandler) CancelFinal(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	schoolID, nisn, tahunLulus, ok := h.resolveCancelRequest(c, user)
	if !ok {
		return
	}

	if err := ensureGraduationYearOpen(c, h.DB, schoolID, tahunLulus); err != nil {
		if errors.Is(err, services.ErrYearClosed) {
			utils.RespondBadRequest(c, graduationYearClosedMessage(tahunLulus))
			return
		}
		utils.RespondInternalServerError(c)
		return
	}

	service := services.SiswaService{DB: h.DB}
	if err := service.CancelOutcomeFinalization(c, user, schoolID, nisn, tahunLulus); err != nil {
		switch {
		case errors.Is(err, services.ErrSiswaNotFound):
			utils.RespondBadRequest(c, "Data final siswa tidak ditemukan.")
		case errors.Is(err, services.ErrOutcomeReadOnly):
			utils.RespondBadRequest(c, "Finalisasi alumni impor tidak dapat dibatalkan.")
		case errors.Is(err, auth.ErrForbidden):
			utils.RespondForbidden(c)
		default:
			utils.RespondInternalServerError(c)
		}
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"status":      "ok",
		"nisn":        nisn,
		"tahun_lulus": tahunLulus,
	})
}

func (h SiswaHandler) CancelAcceptanceOffers(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	schoolID, nisn, tahunLulus, ok := h.resolveCancelRequest(c, user)
	if !ok {
		return
	}

	if err := ensureGraduationYearOpen(c, h.DB, schoolID, tahunLulus); err != nil {
		if errors.Is(err, services.ErrYearClosed) {
			utils.RespondBadRequest(c, graduationYearClosedMessage(tahunLulus))
			return
		}
		utils.RespondInternalServerError(c)
		return
	}

	finalized, err := h.isStudentFinalized(c, schoolID, nisn, tahunLulus)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if finalized {
		utils.RespondBadRequest(c, "Siswa sudah final. Batalkan final terlebih dahulu.")
		return
	}

	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer func() { _ = tx.Rollback(c) }()

	cohortID, err := h.lookupCohortID(c, tx, schoolID, nisn, tahunLulus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
			return
		}
		utils.RespondInternalServerError(c)
		return
	}
	var affectedNormalizedName string
	_ = tx.QueryRow(c, `
		SELECT COALESCE(NULLIF(normalized_student_name,''),LOWER(REGEXP_REPLACE(TRIM(COALESCE(student_name,'')),'\s+',' ','g')))
		FROM outcome_pt_offers
		WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3
		ORDER BY updated_at DESC,id DESC LIMIT 1
	`, schoolID, tahunLulus, nisn).Scan(&affectedNormalizedName)

	if _, err := tx.Exec(c, `
		DELETE FROM student_higher_ed_acceptance_offers
		WHERE school_id = $1
		  AND cohort_id = $2
	`, schoolID, cohortID); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if _, err := tx.Exec(c, `
		DELETE FROM outcome_pt_offers
		WHERE school_id = $1
		  AND tahun_lulus = $2
		  AND nisn = $3
	`, schoolID, tahunLulus, nisn); err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if affectedNormalizedName != "" {
		service := services.SiswaService{DB: h.DB}
		if err := service.ReconcilePTIdentityTx(c, tx, schoolID, tahunLulus, affectedNormalizedName); err != nil {
			utils.RespondInternalServerError(c)
			return
		}
	}

	if err := tx.Commit(c); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"status":      "ok",
		"nisn":        nisn,
		"tahun_lulus": tahunLulus,
	})
}

func (h SiswaHandler) CancelApplications(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	schoolID, nisn, tahunLulus, ok := h.resolveCancelRequest(c, user)
	if !ok {
		return
	}

	if err := ensureGraduationYearOpen(c, h.DB, schoolID, tahunLulus); err != nil {
		if errors.Is(err, services.ErrYearClosed) {
			utils.RespondBadRequest(c, graduationYearClosedMessage(tahunLulus))
			return
		}
		utils.RespondInternalServerError(c)
		return
	}

	finalized, err := h.isStudentFinalized(c, schoolID, nisn, tahunLulus)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if finalized {
		utils.RespondBadRequest(c, "Siswa sudah final. Batalkan final terlebih dahulu.")
		return
	}

	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer func() { _ = tx.Rollback(c) }()

	cohortID, err := h.lookupCohortID(c, tx, schoolID, nisn, tahunLulus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
			return
		}
		utils.RespondInternalServerError(c)
		return
	}

	if _, err := tx.Exec(c, `
		DELETE FROM student_higher_ed_applications
		WHERE school_id = $1
		  AND cohort_id = $2
	`, schoolID, cohortID); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	if err := tx.Commit(c); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"status":      "ok",
		"nisn":        nisn,
		"tahun_lulus": tahunLulus,
	})
}

func (h SiswaHandler) CancelWirausaha(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	schoolID, nisn, tahunLulus, ok := h.resolveCancelRequest(c, user)
	if !ok {
		return
	}

	if err := ensureGraduationYearOpen(c, h.DB, schoolID, tahunLulus); err != nil {
		if errors.Is(err, services.ErrYearClosed) {
			utils.RespondBadRequest(c, graduationYearClosedMessage(tahunLulus))
			return
		}
		utils.RespondInternalServerError(c)
		return
	}

	finalized, err := h.isStudentFinalized(c, schoolID, nisn, tahunLulus)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	if finalized {
		utils.RespondBadRequest(c, "Siswa sudah final. Batalkan final terlebih dahulu.")
		return
	}

	tx, err := h.DB.Pool.Begin(c)
	if err != nil {
		utils.RespondInternalServerError(c)
		return
	}
	defer func() { _ = tx.Rollback(c) }()

	cohortID, err := h.lookupCohortID(c, tx, schoolID, nisn, tahunLulus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondBadRequest(c, "Data siswa tidak ditemukan.")
			return
		}
		utils.RespondInternalServerError(c)
		return
	}

	if _, err := tx.Exec(c, `
		DELETE FROM student_entrepreneur_plans
		WHERE school_id = $1
		  AND cohort_id = $2
	`, schoolID, cohortID); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	if err := tx.Commit(c); err != nil {
		utils.RespondInternalServerError(c)
		return
	}

	utils.RespondData(c, http.StatusOK, gin.H{
		"status":      "ok",
		"nisn":        nisn,
		"tahun_lulus": tahunLulus,
	})
}

func (h SiswaHandler) AnalyticsHigherEd(c *gin.Context) {
	obs := beginEndpointObservation(c, h.DB, "siswa", "analytics_higher_ed", "analytics", analyticsSlowThreshold)
	defer obs.Finish()
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	scope := siswaResolveScope(c, user, false)
	if scope == nil && user.Role != "ADMIN" && user.Role != "ADMIN_MASTER" {
		return
	}

	service := services.SiswaService{DB: h.DB}
	ctx, cancel := requestContextWithTimeout(c, analyticsRequestTimeout)
	defer cancel()

	result, err := service.AnalyticsHigherEd(ctx, user, scope)
	if err != nil {
		if handleOperationalError(c, h.DB, "siswa", "analytics_higher_ed", err) {
			return
		}
		respondSafeListFallback(c, "siswa", "analytics_higher_ed", "Higher education analytics is temporarily unavailable.", emptyHigherEdAnalytics(), err)
		return
	}
	utils.RespondData(c, http.StatusOK, result)
}

func (h SiswaHandler) DownloadRejectedImportReport(c *gin.Context) {
	user, ok := siswaAuthorizedUser(c)
	if !ok {
		return
	}

	reportID := strings.TrimSpace(c.Param("id"))
	if reportID == "" {
		utils.RespondBadRequest(c, "Invalid report id")
		return
	}

	service := services.SiswaService{DB: h.DB}
	schoolID, filename, content, err := service.GetImportReport(c, reportID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondBadRequest(c, "Laporan tidak ditemukan.")
		} else {
			utils.RespondInternalServerError(c)
		}
		return
	}

	scope, scopeErr := auth.GetSingleSchoolScopeV2(user, nil)
	if scopeErr != nil {
		if errors.Is(scopeErr, auth.ErrForbidden) {
			utils.RespondForbidden(c)
			return
		}
		utils.RespondUnauthorized(c)
		return
	}
	if scope != nil && schoolID != *scope {
		utils.RespondForbidden(c)
		return
	}

	if filename == "" {
		filename = "rejected.xlsx"
	}
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func siswaImportResponsePayload(result services.SiswaImportResult) gin.H {
	payload := gin.H{
		"total_rows": result.TotalRows,
		"accepted":   result.Accepted,
		"rejected":   result.Rejected,
	}
	if result.RejectedReportID != "" {
		payload["rejected_excel_url"] = "/api/siswa/import-reports/" + result.RejectedReportID + "/rejected.xlsx"
		payload["rejected_excel_filename"] = result.RejectedFilename
	}
	if len(result.Rejections) > 0 {
		items := make([]gin.H, 0, len(result.Rejections))
		for _, rejection := range result.Rejections {
			items = append(items, gin.H{
				"row_number": rejection.RowNumber,
				"reason":     rejection.Reason,
			})
		}
		payload["rejections"] = items
	}
	return payload
}

func siswaAuthorizedUser(c *gin.Context) (auth.User, bool) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return auth.User{}, false
	}
	role := strings.ToUpper(strings.TrimSpace(user.Role))
	if role != "ADMIN" && role != "ADMIN_CABDIN" && role != "SCHOOL_ADMIN" && role != "ADMIN_MASTER" && role != "PENGAWAS" {
		utils.RespondForbidden(c)
		return auth.User{}, false
	}
	return user, true
}

func siswaRequestedSchoolIDFromQuery(c *gin.Context) (*int, bool) {
	raw := strings.TrimSpace(c.Query("school_id"))
	if raw == "" {
		return nil, true
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		utils.RespondBadRequest(c, "Invalid school_id")
		return nil, false
	}
	return &id, true
}

func siswaResolveScope(c *gin.Context, user auth.User, includeForm bool) *int {
	var requestedSchoolID *int
	if raw := strings.TrimSpace(c.Query("school_id")); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil || id <= 0 {
			utils.RespondBadRequest(c, "Invalid school_id")
			return nil
		}
		requestedSchoolID = &id
	} else if includeForm {
		if raw := strings.TrimSpace(c.PostForm("school_id")); raw != "" {
			id, err := strconv.Atoi(raw)
			if err != nil || id <= 0 {
				utils.RespondBadRequest(c, "Invalid school_id")
				return nil
			}
			requestedSchoolID = &id
		}
	}

	scope, err := auth.GetSingleSchoolScopeV2(user, requestedSchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
		} else {
			utils.RespondUnauthorized(c)
		}
		return nil
	}
	return scope
}

func (h SiswaHandler) fetchSchoolLevel(c *gin.Context, schoolID int) (string, error) {
	var level string
	if err := h.DB.Pool.QueryRow(c, `
		SELECT COALESCE(UPPER(TRIM(bentuk_pendidikan)), '')
		FROM data_sekolah
		WHERE id = $1
	`, schoolID).Scan(&level); err != nil {
		return "", err
	}
	if strings.TrimSpace(level) == "" {
		return "", errors.New("LEVEL_NOT_SET")
	}
	return level, nil
}

func (h SiswaHandler) resolveCancelRequest(c *gin.Context, user auth.User) (int, string, int, bool) {
	var req cancelSiswaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequest(c, "Payload tidak valid.")
		return 0, "", 0, false
	}

	scope, err := auth.GetSingleSchoolScopeV2(user, req.SchoolID)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			utils.RespondForbidden(c)
		} else {
			utils.RespondUnauthorized(c)
		}
		return 0, "", 0, false
	}
	if scope == nil {
		utils.RespondBadRequest(c, "school_id wajib diisi.")
		return 0, "", 0, false
	}

	nisn := strings.TrimSpace(req.NISN)
	if nisn == "" || req.TahunLulus <= 0 {
		utils.RespondBadRequest(c, "nisn dan tahun_lulus wajib diisi.")
		return 0, "", 0, false
	}

	return *scope, nisn, req.TahunLulus, true
}

func (h SiswaHandler) lookupCohortID(c *gin.Context, tx pgx.Tx, schoolID int, nisn string, tahunLulus int) (int64, error) {
	var cohortID int64
	if err := tx.QueryRow(c, `
		SELECT id
		FROM student_cohorts
		WHERE school_id = $1
		  AND nisn = $2
		  AND tahun_lulus = $3
	`, schoolID, nisn, tahunLulus).Scan(&cohortID); err != nil {
		return 0, err
	}
	return cohortID, nil
}

func (h SiswaHandler) isStudentFinalized(c *gin.Context, schoolID int, nisn string, tahunLulus int) (bool, error) {
	var finalized bool
	if err := h.DB.Pool.QueryRow(c, `
		SELECT EXISTS (
			SELECT 1
			FROM student_outcomes o
			WHERE o.school_id = $1
			  AND o.nisn = $2
			  AND o.tahun_lulus = $3
			  AND o.finalized_at IS NOT NULL
		)
		OR (
			to_regclass('public.student_final_outcomes') IS NOT NULL
			AND to_regclass('public.student_cohorts') IS NOT NULL
			AND EXISTS (
				SELECT 1
				FROM student_final_outcomes f
				JOIN student_cohorts c
				  ON c.school_id = f.school_id
				 AND c.id = f.cohort_id
				WHERE c.school_id = $1
				  AND c.nisn = $2
				  AND c.tahun_lulus = $3
				  AND f.finalized_at IS NOT NULL
			)
		)
	`, schoolID, nisn, tahunLulus).Scan(&finalized); err != nil {
		return false, err
	}
	return finalized, nil
}

func ensureGraduationYearOpen(c *gin.Context, database *db.DB, schoolID int, tahunLulus int) error {
	service := services.SiswaService{DB: database}
	return service.EnsureYearNotClosed(c, schoolID, tahunLulus)
}

func graduationYearClosedMessage(tahunLulus int) string {
	return fmt.Sprintf("Tahun lulusan %d sudah ditutup. Tidak dapat melakukan perubahan.", tahunLulus)
}

func (h SiswaHandler) ensureLegacyImportEnabled(c *gin.Context, user auth.User, schoolID int) (services.LegacyImportSettings, string, bool) {
	level, err := h.fetchSchoolLevel(c, schoolID)
	if err != nil {
		utils.RespondBadRequest(c, "Invalid school level")
		return services.LegacyImportSettings{}, "", false
	}

	var cabdinUserID *int
	if user.Role == "ADMIN" {
		cabdinUserID = &user.ID
	}
	legacyService := services.LegacyImportService{DB: h.DB}
	enabled, settings, source, err := legacyService.EffectiveLegacyEnabled(c, level, cabdinUserID, time.Now())
	if err != nil {
		utils.RespondInternalServerError(c)
		return services.LegacyImportSettings{}, "", false
	}
	if !enabled {
		c.JSON(http.StatusForbidden, utils.APIResponse{
			Error:   "FORBIDDEN",
			Message: "LEGACY_IMPORT_DISABLED",
			Details: gin.H{
				"level":  level,
				"source": source,
			},
		})
		return settings, level, false
	}
	return settings, level, true
}

func readExcelUpload(c *gin.Context) ([]byte, string, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return nil, "", errors.New("file Excel wajib diunggah")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, "", errors.New("gagal membuka file Excel")
	}
	defer file.Close()
	buffer, err := io.ReadAll(file)
	if err != nil {
		return nil, "", errors.New("gagal membaca file Excel")
	}
	return buffer, fileHeader.Filename, nil
}

func respondSiswaHeaderMismatch(c *gin.Context, expected, found []string) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusBadRequest, utils.APIResponse{
		Error:   "TEMPLATE_MISMATCH",
		Message: siswaTemplateMismatchMessage,
		Details: gin.H{
			"expected_headers": expected,
			"found_headers":    found,
		},
	})
}

func logAcceptanceOfferTemplateExportFailure(user auth.User, scope *int, year int, stage string, err error) {
	if err == nil {
		return
	}
	scopeValue := "nil"
	if scope != nil {
		scopeValue = strconv.Itoa(*scope)
	}
	logging.Errorf(
		"siswa",
		"export_acceptance_offer_template",
		"event=failed stage=%s role=%s school_scope=%s year=%d err=%v",
		stage,
		user.Role,
		scopeValue,
		year,
		err,
	)
}

func siswaPTOfferHeadersMatch(foundHeaders []string) bool {
	return parsers.HeadersMatchExact(foundHeaders, parsers.SiswaPTOfferCompactTemplateHeaders) ||
		parsers.HeadersMatchExact(foundHeaders, parsers.SiswaPTOfferTemplateHeaders) ||
		parsers.HeadersMatchExact(foundHeaders, parsers.SiswaPTOfferTemplateHeadersLegacyProdi)
}
