package handlers

import (
	"bytes"
	"errors"
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
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/utils"
)

type GaspolHandler struct {
	DB                  *db.DB
	MasterEncryptionKey []byte
}

type gaspolParticipantRequest struct {
	Year      int   `json:"year"`
	SchoolIDs []int `json:"schoolIds"`
}

type gaspolRoleAssignRequest struct {
	Year    int   `json:"year"`
	UserIDs []int `json:"userIds"`
}

type gaspolValidatorAssignRequest struct {
	Year            int `json:"year"`
	SchoolID        int `json:"schoolId"`
	ValidatorUserID int `json:"validatorUserId"`
}

type gaspolValidatorAssignAllRequest struct {
	Year            int `json:"year"`
	ValidatorUserID int `json:"validatorUserId"`
}

type gaspolManualAssignmentRequest struct {
	Year             int `json:"year"`
	Month            int `json:"month"`
	SchoolID         int `json:"schoolId"`
	SupervisorUserID int `json:"supervisorUserId"`
}

type gaspolGenerateUsersRequest struct {
	Year      int   `json:"year"`
	SchoolIDs []int `json:"schoolIds"`
}

type gaspolResetPasswordRequest struct {
	UserID      int    `json:"userId"`
	NewPassword string `json:"newPassword"`
}

type gaspolSubmissionRequest struct {
	Year           int      `json:"year"`
	Month          int      `json:"month"`
	ReportTitle    string   `json:"reportTitle"`
	ReportBody     string   `json:"reportBody"`
	ReportLink     string   `json:"reportLink"`
	AttachmentURLs []string `json:"attachmentUrls"`
	Status         string   `json:"status"`
}

type gaspolCancelSubmitRequest struct {
	Year     int    `json:"year"`
	Month    int    `json:"month"`
	SchoolID int    `json:"schoolId"`
	Reason   string `json:"reason"`
}

type gaspolAssessmentRequest struct {
	Year     int     `json:"year"`
	Month    int     `json:"month"`
	SchoolID int     `json:"schoolId"`
	Score    float64 `json:"score"`
	Notes    string  `json:"notes"`
}

type gaspolValidationRequest struct {
	Year          int      `json:"year"`
	AssessmentID  int      `json:"assessmentId"`
	AssessmentIDs []int    `json:"assessmentIds"`
	Decision      string   `json:"decision"`
	Score         *float64 `json:"score"`
	Notes         string   `json:"notes"`
}

type gaspolJuryAccountRequest struct {
	Year        int    `json:"year"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type gaspolRoleStatusRequest struct {
	Year     int    `json:"year"`
	UserID   int    `json:"userId"`
	Role     string `json:"role"`
	IsActive bool   `json:"isActive"`
}

func parsePositiveInt(raw string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(raw))
	if value < 0 {
		return 0
	}
	return value
}

func gaspolService(database *db.DB, masterEncryptionKey ...[]byte) services.GaspolService {
	var key []byte
	if len(masterEncryptionKey) > 0 {
		key = masterEncryptionKey[0]
	}
	return services.NewGaspolService(database, key)
}

func handleGaspolError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	switch {
	case strings.HasPrefix(err.Error(), "INCOMPLETE_ASSESSMENTS:"):
		utils.RespondBadRequestMessage(c, err.Error())
	case err.Error() == "BA_EXPORT_STALE":
		utils.RespondConflictMessage(c, "Bahan BA kedaluwarsa. Ekspor ulang dan sesuaikan BA sebelum penetapan.")
	case err.Error() == "OFFICIAL_REVISION_REQUIRED":
		utils.RespondConflictMessage(c, "Hasil resmi dilindungi; koreksi memerlukan revisi penetapan resmi.")
	case err.Error() == "JOINT_FINALIZATION_REQUIRED":
		utils.RespondBadRequestMessage(c, "Gunakan penetapan final bersama dengan satu BA gabungan.")
	case err.Error() == "ZERO_SCORE_REASON_REQUIRED":
		utils.RespondBadRequestMessage(c, "Nilai nol wajib disertai alasan pemeriksaan sesuai rubrik.")
	case err.Error() == "INCOMPLETE_OFFICIAL_ZONES" || err.Error() == "INVALID_TIE_ORDER" || err.Error() == "DUPLICATE_OFFICIAL_SCHOOL" || err.Error() == "RESULT_PREVIEW_STALE":
		utils.RespondBadRequestMessage(c, err.Error())
	case errors.Is(err, auth.ErrUnauthorized):
		utils.RespondUnauthorized(c)
	case errors.Is(err, auth.ErrForbidden):
		utils.RespondForbidden(c)
	case errors.Is(err, pgx.ErrNoRows):
		utils.RespondData(c, http.StatusOK, gin.H{})
	default:
		switch err.Error() {
		case "RUBRIC_INDICATORS_REQUIRED", "INVALID_RUBRIC", "RUBRIC_WEIGHT_MUST_TOTAL_100", "RUBRIC_SCORES_REQUIRED", "INVALID_RUBRIC_SCORE", "RUBRIC_NOT_DRAFT", "ACTIVE_RUBRIC_REQUIRED", "RUBRIC_VERSION_MISMATCH", "INDICATOR_NOTES_REQUIRED", "VOTING_ROSTER_LOCKED", "INVALID_VOTER_DENOMINATOR", "LIKE_CAPTURE_AFTER_CUTOFF", "LIKE_NOT_SUBMITTED", "MANAGED_VOTING_REQUIRED", "ASSISTANCE_CONSENT_REQUIRED", "VOTING_SESSION_MUST_BE_CLOSED", "LIKE_VERIFICATION_REQUIRED":
			utils.RespondBadRequestMessage(c, err.Error())
		case "SUBMISSION_NOT_SUBMITTED":
			utils.RespondBadRequestMessage(c, "Laporan ini tidak sedang berstatus terkirim ke penilai.")
		case "SUBMISSION_ALREADY_ASSESSED":
			utils.RespondBadRequestMessage(c, "Laporan sudah dinilai pengawas sehingga status kirim tidak dapat dibatalkan.")
		case "SUBMISSION_DEADLINE_PASSED":
			utils.RespondBadRequestMessage(c, "Laporan sudah melewati deadline sehingga status kirim tidak dapat dibatalkan.")
		case "INVALID_SCHOOL_ID":
			utils.RespondBadRequestMessage(c, "Sekolah tidak valid.")
		case "ASSESSMENT_ALREADY_SUBMITTED":
			utils.RespondConflictMessage(c, "Penilaian resmi sudah pernah dikirim dan tidak dapat diubah kembali.")
		case "INVALID_MONTH", "INVALID_STAGE", "INVALID_STATE", "INVALID_STATE_TRANSITION", "INVALID_LOCK_TYPE", "INVALID_STATUS", "INVALID_USER_ID", "INVALID_ROLE", "INVALID_PAYLOAD", "INVALID_WORKFLOW_TRANSITION", "TWO_LEVEL_VALIDATION_REQUIRED", "KOOR_VALIDATION_REQUIRED", "ASSESSMENT_DEADLINE_PASSED", "VOTER_DENOMINATOR_LOCKED", "VOTING_SESSION_CLOSED", "VOTER_NOT_ELIGIBLE", "VOTE_ALREADY_CAST", "CONTENT_URL_REQUIRED", "INVALID_CONTENT_URL", "UNSUPPORTED_CONTENT_PLATFORM", "CONTENT_LINK_LOCKED", "BA_REQUIRED", "INCOMPLETE_ASSESSMENTS", "MANUAL_TIE_RESOLUTION_REQUIRED", "REOPEN_REASON_REQUIRED", "INVALID_HISTORY_TAB", "INVALID_YEAR", "INVALID_DATE", "INVALID_DATE_RANGE", "INVALID_SEARCH", "INVALID_ID", "USER_NOT_GASPOL", "USER_NOT_RESETTABLE", "SCHOOL_NOT_PARTICIPANT", "SCHOOL_NOT_ELIGIBLE", "ASSIGNMENT_LOCKED", "USER_NOT_ASSIGNABLE", "MONTHLY_ENTRY_LOCKED", "INCOMPLETE_SUBMISSION", "INVALID_REPORT_LINK", "SUBMISSION_LOCKED", "MASTER_RETURN_STAGE_NOT_ALLOWED", "MASTER_RETURN_NOTES_REQUIRED", "TOP_10_ALREADY_LOCKED", "RESULT_ALREADY_LOCKED", "RESULT_HAS_LOCKED_DEPENDENTS":
			utils.RespondBadRequestMessage(c, err.Error())
		default:
			utils.RespondInternalServerError(c)
		}
	}
}

func (h GaspolHandler) AccessContext(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.AccessContext(c, user, parsePositiveInt(c.Query("year")))
	if err != nil {
		logging.Errorf("gaspol", "access_context", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) CompetitionConfig(c *gin.Context) {
	payload, err := gaspolService(h.DB).CompetitionConfig(c, parsePositiveInt(c.Query("year")))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) UpdateCompetitionConfig(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.GaspolCompetitionConfigInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	payload, err := gaspolService(h.DB).UpdateCompetitionConfig(c, user, input)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) AssignCompetitionStage(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.GaspolStageAssignmentInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = gaspolService(h.DB).AssignCompetitionStage(c, user, input); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"saved": true})
}

func (h GaspolHandler) BulkAssignCompetitionStage(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.GaspolBulkStageAssignmentInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	payload, err := gaspolService(h.DB).BulkAssignCompetitionStage(c, user, input)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) SaveStageAssessment(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.GaspolStageAssessmentInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	payload, err := gaspolService(h.DB).SaveStageAssessment(c, user, input)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ReviewStageAssessment(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.RespondBadRequestMessage(c, "INVALID_ID")
		return
	}
	var input services.GaspolReviewInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = gaspolService(h.DB).ReviewStageAssessment(c, user, id, c.Param("level"), input); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"reviewed": true})
}

func (h GaspolHandler) MasterReturnStageAssessment(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.RespondBadRequestMessage(c, "INVALID_ID")
		return
	}
	var input services.GaspolMasterReturnInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = gaspolService(h.DB).MasterReturnStageAssessment(c, user, id, input); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"status": services.GaspolWorkflowReturnedMaster})
}

func (h GaspolHandler) ManualLikeOverride(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.GaspolManualLikeInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = gaspolService(h.DB).RecordManualLike(c, user, input); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"saved": true})
}

func (h GaspolHandler) OpenFavoriteVoting(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input struct {
		Year             int       `json:"year"`
		VoterDenominator int       `json:"voterDenominator"`
		ClosesAt         time.Time `json:"closesAt"`
	}
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	payload, err := gaspolService(h.DB).OpenVotingSession(c, user, input.Year, input.VoterDenominator, input.ClosesAt)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) CastFavoriteVote(c *gin.Context) {
	// Retired public endpoint must never trust a username or shared QR token.
	utils.RespondUnauthorized(c)
}

func (h GaspolHandler) UploadBADocument(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	if err = c.Request.ParseMultipartForm(12 << 20); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil || len(content) == 0 || len(content) > 10<<20 {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	year := parsePositiveInt(c.PostForm("year"))
	isPublic, _ := strconv.ParseBool(c.PostForm("isPublic"))
	payload, err := gaspolService(h.DB).UploadBADocument(c, user, year, header.Filename, header.Header.Get("Content-Type"), content, isPublic)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) DownloadBADocument(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_ID")
		return
	}
	meta, content, err := gaspolService(h.DB).BADocumentContent(c, &user, id)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(meta.FileName, `"`, "")+`"`)
	c.Data(http.StatusOK, meta.ContentType, content)
}

func (h GaspolHandler) PreviewCompetitionRanking(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	payload, err := gaspolService(h.DB).PreviewCompetitionRanking(c, user, parsePositiveInt(c.Query("year")), c.Query("stage"), c.Query("cohort"))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) LockCompetitionResult(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.GaspolLockInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	payload, err := gaspolService(h.DB).LockCompetitionResult(c, user, input)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ReopenCompetitionResult(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_ID")
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = gaspolService(h.DB).ReopenCompetitionResult(c, user, id, input.Reason); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"reopened": true})
}

func (h GaspolHandler) SetFavoriteContent(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.GaspolFavoriteContentInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	// No official social provider is configured in this binary yet. The service
	// records SYNC_ERROR with a clear credential message and keeps manual audit fallback available.
	if err = gaspolService(h.DB).SetFavoriteContent(c, user, input, nil); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"saved": true})
}

func (h GaspolHandler) SaveNominationSubmission(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.GaspolNominationSubmissionInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = gaspolService(h.DB).SaveNominationSubmission(c, user, input); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"saved": true})
}

func (h GaspolHandler) ProposeInnovationAward(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input struct {
		Year int `json:"year"`
	}
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	id, err := gaspolService(h.DB).ProposeInnovationAward(c, user, input.Year)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"nominationId": id})
}

func (h GaspolHandler) DecideInnovationAward(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_ID")
		return
	}
	var input services.GaspolInnovationDecisionInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	if err = gaspolService(h.DB).DecideInnovationAward(c, user, id, input); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"decided": true})
}

func (h GaspolHandler) FinalizeBestAward(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input struct {
		Year int `json:"year"`
	}
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	schoolID, err := gaspolService(h.DB).FinalizeBestAward(c, user, input.Year)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"schoolId": schoolID, "award": "Terbaik"})
}

func (h GaspolHandler) FinalizeFavoriteAward(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var input services.GaspolFinalizeFavoriteInput
	if err = c.ShouldBindJSON(&input); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	schoolID, err := gaspolService(h.DB).FinalizeFavoriteAward(c, user, input)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"schoolId": schoolID, "award": "Ter-Favorit"})
}

func (h GaspolHandler) PublicDashboard(c *gin.Context) {
	service := gaspolService(h.DB)
	payload, err := service.PublicDashboard(c, parsePositiveInt(c.Query("year")))
	if err != nil {
		logging.Errorf("gaspol", "public_dashboard", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) PublicLeaderboard(c *gin.Context) {
	service := gaspolService(h.DB)
	payload, err := service.Leaderboard(c, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("month")), c.Query("stage"))
	if err != nil {
		logging.Errorf("gaspol", "public_leaderboard", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) PublicBestSchool(c *gin.Context) {
	service := gaspolService(h.DB)
	payload, err := service.BestSchool(c, parsePositiveInt(c.Query("year")))
	if err != nil {
		logging.Errorf("gaspol", "public_best_school", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	if payload == nil {
		utils.RespondData(c, http.StatusOK, gin.H{})
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) SchoolDashboard(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.SchoolDashboard(c, user, parsePositiveInt(c.Query("year")))
	if err != nil {
		logging.Errorf("gaspol", "school_dashboard", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) SchoolSubmissions(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.SchoolDashboard(c, user, parsePositiveInt(c.Query("year")))
	if err != nil {
		logging.Errorf("gaspol", "school_submissions", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload.History)
}

func (h GaspolHandler) UpsertSchoolSubmission(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.UpsertSchoolSubmission(c, user, services.GaspolSubmissionInput{
		Year:           req.Year,
		Month:          req.Month,
		ReportTitle:    req.ReportTitle,
		ReportBody:     req.ReportBody,
		ReportLink:     req.ReportLink,
		AttachmentURLs: req.AttachmentURLs,
		Status:         req.Status,
	})
	if err != nil {
		logging.Errorf("gaspol", "upsert_school_submission", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) CancelSubmitSchoolSubmission(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolCancelSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.CancelSubmitSchoolSubmission(c, user, services.GaspolCancelSubmitInput{
		Year:     req.Year,
		Month:    req.Month,
		SchoolID: req.SchoolID,
		Reason:   req.Reason,
	})
	if err != nil {
		logging.Errorf("gaspol", "gaspol/cancel_submit_submission", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) SupervisorDashboard(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.SupervisorDashboard(c, user, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("month")))
	if err != nil {
		logging.Errorf("gaspol", "supervisor_dashboard", "assessor_user_id=%d year=%d month=%d err=%v", user.ID, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("month")), err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) SupervisorAssignments(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.SupervisorDashboard(c, user, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("month")))
	if err != nil {
		logging.Errorf("gaspol", "supervisor_assignments", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload.Assignments)
}

func (h GaspolHandler) UpsertSupervisorAssessment(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolAssessmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.UpsertSupervisorAssessment(c, user, services.GaspolAssessmentInput{
		Year:     req.Year,
		Month:    req.Month,
		SchoolID: req.SchoolID,
		Score:    req.Score,
		Notes:    req.Notes,
	})
	if err != nil {
		logging.Errorf("gaspol", "upsert_supervisor_assessment", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ValidatorDashboard(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.ValidatorDashboard(c, user, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("month")))
	if err != nil {
		logging.Errorf("gaspol", "validator_dashboard", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ValidatorRanking(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.ValidatorDashboard(c, user, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("month")))
	if err != nil {
		logging.Errorf("gaspol", "validator_ranking", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload.Leaderboard)
}

func (h GaspolHandler) ValidateAssessments(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolValidationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.ValidateAssessments(c, user, services.GaspolValidationInput{
		Year:          req.Year,
		AssessmentID:  req.AssessmentID,
		AssessmentIDs: req.AssessmentIDs,
		Decision:      req.Decision,
		Score:         req.Score,
		Notes:         req.Notes,
	})
	if err != nil {
		logging.Errorf("gaspol", "validate_assessments", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) UpsertFieldAssessment(c *gin.Context) {
	h.upsertJuryAssessment(c, services.GaspolStageField)
}

func (h GaspolHandler) JuryDashboard(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.JuryDashboard(c, user, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("month")), c.Query("stage"))
	if err != nil {
		logging.Errorf("gaspol", "jury_dashboard", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) UpsertPresentationAssessment(c *gin.Context) {
	h.upsertJuryAssessment(c, services.GaspolStagePresentation)
}

func (h GaspolHandler) upsertJuryAssessment(c *gin.Context, stage string) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolAssessmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.UpsertJuryAssessment(c, user, stage, services.GaspolAssessmentInput{
		Year:     req.Year,
		Month:    req.Month,
		SchoolID: req.SchoolID,
		Score:    req.Score,
		Notes:    req.Notes,
	})
	if err != nil {
		logging.Errorf("gaspol", "upsert_jury_assessment", "stage=%s err=%v", stage, err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) AdminDashboard(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.AdminDashboard(c, user, parsePositiveInt(c.Query("year")))
	if err != nil {
		logging.Errorf("gaspol", "admin_dashboard", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) AdminSettingsOverview(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.AdminSettingsOverview(c, user, parsePositiveInt(c.Query("year")))
	if err != nil {
		logging.Errorf("gaspol", "admin_settings_overview", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ActivateYear(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req struct {
		Year int `json:"year"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.ActivateYear(c, user, req.Year)
	if err != nil {
		logging.Errorf("gaspol", "activate_year", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ListParticipants(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.MonitoringSchools(c, user, parsePositiveInt(c.Query("year")))
	if err != nil {
		logging.Errorf("gaspol", "list_participants", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) SaveParticipants(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolParticipantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.SaveParticipants(c, user, services.GaspolParticipantUpdateInput{
		Year:      req.Year,
		SchoolIDs: req.SchoolIDs,
	})
	if err != nil {
		logging.Errorf("gaspol", "save_participants", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) GenerateSchoolUsers(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolGenerateUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB, h.MasterEncryptionKey)
	payload, err := service.GenerateSchoolUsers(c, user, services.GaspolGenerateUsersInput{
		Year:      req.Year,
		SchoolIDs: req.SchoolIDs,
	})
	if err != nil {
		logging.Errorf("gaspol", "generate_school_users", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ExportSchoolUsersExcel(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	year := parsePositiveInt(c.Query("year"))
	service := gaspolService(h.DB, h.MasterEncryptionKey)
	rows, err := service.SchoolUserExportRows(c, user, year)
	if err != nil {
		logging.Errorf("gaspol", "export_school_users_excel", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	normalizedYear := year
	if normalizedYear <= 0 {
		normalizedYear = time.Now().Year()
	}
	content, err := buildGaspolSchoolUsersWorkbook(normalizedYear, rows)
	if err != nil {
		logging.Errorf("gaspol", "export_school_users_excel", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	filename := "gaspol-user-sekolah-" + strconv.Itoa(normalizedYear) + ".xlsx"
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func (h GaspolHandler) ResetGaspolUserPassword(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.ResetGaspolUserPassword(c, user, services.GaspolResetPasswordInput{
		UserID:      req.UserID,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		logging.Errorf("gaspol", "reset_password", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) AssignAllValidators(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolValidatorAssignAllRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.AssignValidatorAll(c, user, services.GaspolBulkValidatorAssignInput{
		Year:            req.Year,
		ValidatorUserID: req.ValidatorUserID,
	})
	if err != nil {
		logging.Errorf("gaspol", "assign_all_validators", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) AssignSupervisors(c *gin.Context) {
	h.assignGaspolRole(c, services.GaspolRoleSupervisor)
}

func (h GaspolHandler) SetRoleStatus(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolRoleStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.SetRoleStatus(c, user, services.GaspolRoleStatusInput{
		Year:     req.Year,
		UserID:   req.UserID,
		Role:     req.Role,
		IsActive: req.IsActive,
	})
	if err != nil {
		logging.Errorf("gaspol", "set_role_status", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) assignGaspolRole(c *gin.Context, role string) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolRoleAssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.AssignRole(c, user, services.GaspolAssignmentRoleInput{
		Year:    req.Year,
		UserIDs: req.UserIDs,
	}, role)
	if err != nil {
		logging.Errorf("gaspol", "assign_role", "role=%s err=%v", role, err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) AssignValidator(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolValidatorAssignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.AssignValidator(c, user, services.GaspolValidatorAssignInput{
		Year:            req.Year,
		SchoolID:        req.SchoolID,
		ValidatorUserID: req.ValidatorUserID,
	})
	if err != nil {
		logging.Errorf("gaspol", "assign_validator", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ManualAssignment(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolManualAssignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.ManualAssignment(c, user, services.GaspolManualAssignmentInput{
		Year:             req.Year,
		Month:            req.Month,
		SchoolID:         req.SchoolID,
		SupervisorUserID: req.SupervisorUserID,
	})
	if err != nil {
		logging.Errorf("gaspol", "manual_assignment", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) RandomAssignments(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.RandomAssignments(c, user, req.Year, req.Month)
	if err != nil {
		logging.Errorf("gaspol", "random_assignments", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) PreviewRandomAssignments(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	payload, err := gaspolService(h.DB).PreviewRandomAssignments(c, user, req.Year, req.Month)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ConfirmRandomAssignments(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req struct {
		PreviewID string `json:"previewId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.PreviewID) == "" {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	payload, err := gaspolService(h.DB).CommitRandomAssignmentPreview(c, user, strings.TrimSpace(req.PreviewID))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) GenerateJuryAccount(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var req gaspolJuryAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.GenerateJuryAccount(c, user, services.GaspolJuryAccountInput{
		Year:        req.Year,
		Username:    req.Username,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		logging.Errorf("gaspol", "generate_jury_account", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) MonitoringSchools(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.MonitoringSchools(c, user, parsePositiveInt(c.Query("year")))
	if err != nil {
		logging.Errorf("gaspol", "monitoring_schools", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) MonitoringSupervisors(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	service := gaspolService(h.DB)
	payload, err := service.MonitoringSupervisors(c, user, parsePositiveInt(c.Query("year")), parsePositiveInt(c.Query("month")))
	if err != nil {
		logging.Errorf("gaspol", "monitoring_supervisors", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ExportExcel(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	year := parsePositiveInt(c.Query("year"))
	month := parsePositiveInt(c.Query("month"))
	service := gaspolService(h.DB)
	payload, err := service.ExportBundle(c, user, year, month)
	if err != nil {
		logging.Errorf("gaspol", "export_excel", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	content, err := buildGaspolWorkbook(payload)
	if err != nil {
		logging.Errorf("gaspol", "export_excel", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "attachment; filename=\"gaspol-monitoring.xlsx\"")
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func gaspolHistoryFilterFromQuery(c *gin.Context) services.GaspolHistoryFilter {
	return services.GaspolHistoryFilter{
		Tab: c.Query("tab"), Year: parsePositiveInt(c.Query("year")), Month: parsePositiveInt(c.Query("month")),
		Stage:    c.Query("stage"),
		DateFrom: c.Query("dateFrom"), DateTo: c.Query("dateTo"), Status: c.Query("status"),
		SupervisorID: parsePositiveInt(c.Query("supervisorId")), ValidatorID: parsePositiveInt(c.Query("validatorId")),
		Search: c.Query("search"), Page: parsePositiveInt(c.Query("page")), PageSize: parsePositiveInt(c.Query("pageSize")),
	}
}

func (h GaspolHandler) History(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	payload, err := gaspolService(h.DB).History(c, user, gaspolHistoryFilterFromQuery(c), false)
	if err != nil {
		logging.Errorf("gaspol", "history", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) HistoryDetail(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	id := parsePositiveInt(c.Param("id"))
	payload, err := gaspolService(h.DB).HistoryDetail(c, user, c.Query("tab"), id)
	if err != nil {
		logging.Errorf("gaspol", "history_detail", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, payload)
}

func (h GaspolHandler) ExportHistoryExcel(c *gin.Context) {
	user, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	filter := gaspolHistoryFilterFromQuery(c)
	payload, err := gaspolService(h.DB).History(c, user, filter, true)
	if err != nil {
		logging.Errorf("gaspol", "history_export", "err=%v", err)
		handleGaspolError(c, err)
		return
	}
	if len(payload.Items) == 0 {
		utils.RespondBadRequestMessage(c, "Tidak ada data histori yang sesuai dengan filter.")
		return
	}
	content, err := buildGaspolHistoryWorkbook(filter.Tab, payload.Items)
	if err != nil {
		logging.Errorf("gaspol", "history_export_workbook", "err=%v", err)
		utils.RespondInternalServerError(c)
		return
	}
	location, _ := time.LoadLocation("Asia/Jakarta")
	label := map[string]string{"submissions": "Pengumpulan", "assessments": "Penilaian-Pengawas", "validations": "Validasi", "jury": "Penilaian-Juri", "events": "Aktivitas-Tahapan"}[strings.ToLower(filter.Tab)]
	if label == "" {
		label = "Pengumpulan"
	}
	filename := "Histori-GASPOL-" + label + "-" + time.Now().In(location).Format("2006-01-02") + ".xlsx"
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func buildGaspolHistoryWorkbook(tab string, items []services.GaspolHistoryRow) ([]byte, error) {
	file := excelize.NewFile()
	defer file.Close()
	sheet := "Histori GASPOL"
	file.SetSheetName("Sheet1", sheet)
	headers := []interface{}{"Tahun", "Bulan/Tahap", "Sekolah", "NPSN", "Jenjang", "Status", "Pelaku", "Role Pelaku", "Action", "Pengawas", "Validator", "Tanggal WIB", "Nilai Pengawas", "Nilai Koreksi", "Nilai Final", "Nilai Juri", "Nilai Aggregate", "Status Sebelum", "Status Sesudah", "Referensi", "Judul Laporan", "Link Laporan", "Catatan", "Catatan Validator", "Keputusan"}
	if err := file.SetSheetRow(sheet, "A1", &headers); err != nil {
		return nil, err
	}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	for i, row := range items {
		date := ""
		if row.EventAt != nil {
			date = row.EventAt.In(loc).Format("02/01/2006 15:04:05 WIB")
		}
		actorName := row.ActorName
		if actorName == "" {
			actorName = row.SubmittedByName
		}
		period := row.Stage
		if period == "" || period == "MONTHLY" {
			period = gaspolMonthLabel(row.Month)
		}
		values := []interface{}{row.Year, period, row.SchoolName, row.NPSN, row.SchoolLevel, row.Status, actorName, row.ActorRole, row.Action, row.SupervisorName, row.ValidatorName, date, historyNumber(row.SupervisorScore), historyNumber(row.ValidatorScore), historyNumber(row.FinalScore), historyNumber(row.Score), historyNumber(row.AggregateScore), row.BeforeStatus, row.AfterStatus, row.Reference, row.ReportTitle, row.ReportLink, row.Notes, row.ValidatorNotes, row.ValidationDecision}
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := file.SetSheetRow(sheet, cell, &values); err != nil {
			return nil, err
		}
	}
	style, err := file.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"15803D"}, Pattern: 1}, Alignment: &excelize.Alignment{Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	_ = file.SetCellStyle(sheet, "A1", "Y1", style)
	_ = file.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	_ = file.AutoFilter(sheet, "A1:Y1", nil)
	_ = file.SetColWidth(sheet, "A", "B", 15)
	_ = file.SetColWidth(sheet, "C", "C", 38)
	_ = file.SetColWidth(sheet, "D", "E", 16)
	_ = file.SetColWidth(sheet, "F", "L", 22)
	_ = file.SetColWidth(sheet, "M", "Q", 15)
	_ = file.SetColWidth(sheet, "R", "Y", 30)
	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func historyNumber(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func buildGaspolWorkbook(bundle services.GaspolExportBundle) ([]byte, error) {
	file := excelize.NewFile()
	file.SetSheetName("Sheet1", "Summary")
	summaryRows := [][]interface{}{
		{"Year", bundle.Summary.Year},
		{"Month", bundle.Summary.Month},
		{"Stage", bundle.Summary.Stage},
		{"Year Active", bundle.Summary.IsYearActive},
		{"Participants", bundle.Summary.ParticipantCount},
		{"Supervisors", bundle.Summary.SupervisorCount},
		{"Validators", bundle.Summary.ValidatorCount},
		{"Jury", bundle.Summary.JuryCount},
		{"Pending Validation", bundle.Summary.PendingValidation},
		{"Top 10 Ready", bundle.Summary.TopTenReady},
	}
	for index, row := range summaryRows {
		cell, _ := excelize.CoordinatesToCellName(1, index+1)
		if err := file.SetSheetRow("Summary", cell, &row); err != nil {
			return nil, err
		}
	}

	if _, err := file.NewSheet("Monitoring Sekolah"); err != nil {
		return nil, err
	}
	schoolHeaders := []interface{}{"NPSN", "Nama Sekolah", "Bulan", "Mengisi"}
	_ = file.SetSheetRow("Monitoring Sekolah", "A1", &schoolHeaders)
	for index, row := range bundle.SchoolMonitoringExportRows {
		values := []interface{}{
			row.NPSN,
			row.SchoolName,
			gaspolMonthLabel(row.Month),
			row.Mengisi,
		}
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := file.SetSheetRow("Monitoring Sekolah", cell, &values); err != nil {
			return nil, err
		}
	}

	if _, err := file.NewSheet("Monitoring Pengawas"); err != nil {
		return nil, err
	}
	supervisorHeaders := []interface{}{"Nama Pengawas", "Nama Sekolah", "Bulan", "Penilaian"}
	_ = file.SetSheetRow("Monitoring Pengawas", "A1", &supervisorHeaders)
	for index, row := range bundle.SupervisorExportRows {
		values := []interface{}{row.SupervisorName, row.SchoolName, gaspolMonthLabel(row.Month), row.Penilaian}
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := file.SetSheetRow("Monitoring Pengawas", cell, &values); err != nil {
			return nil, err
		}
	}

	if _, err := file.NewSheet("Leaderboard"); err != nil {
		return nil, err
	}
	leaderboardHeaders := []interface{}{"Rank", "School Name", "NPSN", "Level", "Status", "Supervisor Snapshot", "Score Monthly", "Score Field", "Score Presentation", "Score Total", "Monthly Final"}
	_ = file.SetSheetRow("Leaderboard", "A1", &leaderboardHeaders)
	for index, row := range bundle.Leaderboard {
		values := []interface{}{
			row.Rank,
			row.SchoolName,
			row.NPSN,
			row.SchoolLevel,
			row.SchoolStatus,
			row.SupervisorNameSnapshot,
			row.Scores.ScoreMonthly,
			row.Scores.ScoreField,
			row.Scores.ScorePresentation,
			row.Scores.ScoreTotal,
			row.MonthlyFinal,
		}
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := file.SetSheetRow("Leaderboard", cell, &values); err != nil {
			return nil, err
		}
	}
	if _, err := file.NewSheet("Nilai Bulanan"); err != nil {
		return nil, err
	}
	monthlyHeaders := []interface{}{"NPSN", "Nama Sekolah", "Bulan", "Nilai", "Status", "Final"}
	if err := file.SetSheetRow("Nilai Bulanan", "A1", &monthlyHeaders); err != nil {
		return nil, err
	}
	monthlyRow := 2
	for _, row := range bundle.Leaderboard {
		for _, component := range row.MonthlyComponents {
			var score interface{}
			if component.IsFinal {
				score = component.Score
			}
			values := []interface{}{row.NPSN, row.SchoolName, gaspolMonthLabel(component.Month), score, component.Status, component.IsFinal}
			cell, _ := excelize.CoordinatesToCellName(1, monthlyRow)
			if err := file.SetSheetRow("Nilai Bulanan", cell, &values); err != nil {
				return nil, err
			}
			monthlyRow++
		}
	}

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildGaspolSchoolUsersWorkbook(year int, rows []services.GaspolSchoolUserRow) ([]byte, error) {
	file := excelize.NewFile()
	sheet := "User Sekolah GASPOL"
	file.SetSheetName("Sheet1", sheet)

	headers := []interface{}{
		"NPSN",
		"Nama Sekolah",
		"Jenjang",
		"Status Sekolah",
		"Peserta GASPOL",
		"Username",
		"Password Awal",
		"Display Name / Nama Akun",
		"Status Akun",
		"Status Role SCHOOL_ENTRY",
		"Tahun",
	}
	if err := file.SetSheetRow(sheet, "A1", &headers); err != nil {
		return nil, err
	}

	for index, row := range rows {
		values := []interface{}{
			row.NPSN,
			row.SchoolName,
			row.SchoolLevel,
			row.SchoolStatus,
			boolToAktifTidak(row.IsParticipant),
			dashWhenEmpty(row.Username),
			row.InitialPassword,
			dashWhenEmpty(row.DisplayName),
			dashWhenEmpty(row.AccountStatus),
			schoolEntryRoleStatusLabel(row),
			year,
		}
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err := file.SetSheetRow(sheet, cell, &values); err != nil {
			return nil, err
		}
	}

	_ = file.SetColWidth(sheet, "A", "A", 16)
	_ = file.SetColWidth(sheet, "B", "B", 42)
	_ = file.SetColWidth(sheet, "C", "D", 18)
	_ = file.SetColWidth(sheet, "E", "E", 16)
	_ = file.SetColWidth(sheet, "F", "G", 24)
	_ = file.SetColWidth(sheet, "H", "H", 30)
	_ = file.SetColWidth(sheet, "I", "J", 20)
	_ = file.SetColWidth(sheet, "K", "K", 10)

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func dashWhenEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func boolToAktifTidak(value bool) string {
	if value {
		return "Aktif"
	}
	return "Tidak"
}

func schoolEntryRoleStatusLabel(row services.GaspolSchoolUserRow) string {
	if row.UserID == nil {
		return "Belum tersedia"
	}
	if row.RoleIsActive {
		return "Aktif"
	}
	return "Nonaktif"
}

func gaspolMonthLabel(month int) string {
	switch month {
	case 1:
		return "Januari"
	case 2:
		return "Februari"
	case 3:
		return "Maret"
	case 4:
		return "April"
	case 5:
		return "Mei"
	case 6:
		return "Juni"
	case 7:
		return "Juli"
	case 8:
		return "Agustus"
	case 9:
		return "September"
	case 10:
		return "Oktober"
	case 11:
		return "November"
	case 12:
		return "Desember"
	default:
		return ""
	}
}

func (h GaspolHandler) DeterminationMonitor(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	out, err := gaspolService(h.DB).DeterminationMonitor(c, u, parsePositiveInt(c.Query("year")))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, out)
}
func (h GaspolHandler) DetermineFinal(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	var in services.GaspolDeterminationInput
	if err = c.ShouldBindJSON(&in); err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_PAYLOAD")
		return
	}
	out, err := gaspolService(h.DB).DetermineFinal(c, u, in)
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, out)
}
func (h GaspolHandler) OfficialDocumentsZIP(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	out, err := gaspolService(h.DB).OfficialDocumentsZIP(c, u, parsePositiveInt(c.Query("year")))
	if err != nil {
		handleGaspolError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="gaspol-ba.zip"`)
	c.Data(http.StatusOK, "application/zip", out)
}

func (h GaspolHandler) DeleteStagedBA(c *gin.Context) {
	u, err := auth.GetCurrentUser(c)
	if err != nil {
		utils.RespondUnauthorized(c)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.RespondBadRequestMessage(c, "INVALID_ID")
		return
	}
	if err = gaspolService(h.DB).DeleteStagedBA(c, u, id); err != nil {
		handleGaspolError(c, err)
		return
	}
	utils.RespondData(c, http.StatusOK, gin.H{"ok": true})
}
