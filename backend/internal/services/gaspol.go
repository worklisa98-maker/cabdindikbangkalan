package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/config"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/utils"
)

type GaspolService struct {
	DB                  *db.DB
	Repo                GaspolRepository
	MasterEncryptionKey []byte
	Clock               SystemClock
}

func NewGaspolService(database *db.DB, masterEncryptionKey []byte) GaspolService {
	return GaspolService{
		DB:                  database,
		Repo:                GaspolRepository{DB: database},
		MasterEncryptionKey: masterEncryptionKey,
		Clock:               NewSystemClock(database, config.Load()),
	}
}

type GaspolParticipantUpdateInput struct {
	Year      int
	SchoolIDs []int
}

type GaspolGenerateUsersInput struct {
	Year      int
	SchoolIDs []int
}

type GaspolAssignmentRoleInput struct {
	Year    int
	UserIDs []int
}

type GaspolManualAssignmentInput struct {
	Year             int
	Month            int
	SchoolID         int
	SupervisorUserID int
}

type GaspolValidatorAssignInput struct {
	Year            int
	SchoolID        int
	ValidatorUserID int
}

type GaspolBulkValidatorAssignInput struct {
	Year            int
	ValidatorUserID int
}

type GaspolResetPasswordInput struct {
	UserID      int
	NewPassword string
}

type GaspolSubmissionInput struct {
	Year           int
	Month          int
	ReportTitle    string
	ReportBody     string
	ReportLink     string
	AttachmentURLs []string
	Status         string
}

type GaspolCancelSubmitInput struct {
	Year     int    `json:"year"`
	Month    int    `json:"month"`
	SchoolID int    `json:"schoolId"`
	Reason   string `json:"reason"`
}

type GaspolAssessmentInput struct {
	Year     int
	Month    int
	SchoolID int
	Score    float64
	Notes    string
}

type GaspolValidationInput struct {
	Year          int
	AssessmentID  int
	AssessmentIDs []int
	Decision      string
	Score         *float64
	Notes         string
}

type GaspolJuryAccountInput struct {
	Year        int
	Username    string
	DisplayName string
}

type GaspolRoleStatusInput struct {
	Year     int
	UserID   int
	Role     string
	IsActive bool
}

func normalizeGaspolYear(year int) int {
	if year > 0 {
		return year
	}
	return time.Now().Year()
}

func normalizeGaspolMonth(month int) int {
	if month >= 1 && month <= 12 {
		return month
	}
	return int(time.Now().Month())
}

func normalizeGaspolStage(stage string, month int) string {
	value := strings.ToUpper(strings.TrimSpace(stage))
	if value == GaspolStageMonthly || value == GaspolStageField || value == GaspolStagePresentation || value == GaspolStageFinal {
		return value
	}
	switch month {
	case 10:
		return GaspolStageField
	case 11:
		return GaspolStagePresentation
	default:
		return GaspolStageMonthly
	}
}

func gaspolStageMonth(stage string) int {
	switch normalizeGaspolStage(stage, 0) {
	case GaspolStageField:
		return 10
	case GaspolStagePresentation, GaspolStageFinal:
		return 11
	default:
		return 9
	}
}

func validateGaspolRole(role string) error {
	normalized := strings.ToUpper(strings.TrimSpace(role))
	if _, ok := gaspolAssignableRoles[normalized]; !ok {
		return errors.New("INVALID_ROLE")
	}
	return nil
}

func gaspolAssignableBaseRoles(role string) []string {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case GaspolRoleSupervisor:
		return []string{auth.RolePengawas}
	case GaspolRoleValidator:
		return []string{auth.RoleAdminCabdin}
	case GaspolRoleJury:
		return []string{}
	case GaspolRoleSchoolEntry:
		return []string{auth.RoleSchoolAdmin, auth.RoleGaspolSchoolUser}
	case GaspolRoleAdminMaster:
		return []string{auth.RoleAdminMaster}
	default:
		return []string{}
	}
}

func gaspolHasAssignableBaseRole(baseRole string, role string) bool {
	normalizedBaseRole := auth.NormalizeRole(strings.ToUpper(strings.TrimSpace(baseRole)))
	for _, allowedRole := range gaspolAssignableBaseRoles(role) {
		if normalizedBaseRole == auth.NormalizeRole(allowedRole) {
			return true
		}
	}
	return false
}

func dedupeIntSlice(values []int) []int {
	if len(values) == 0 {
		return []int{}
	}
	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (s GaspolService) ensureAdminMaster(user auth.User) error {
	if user.Role != auth.RoleAdminMaster {
		return auth.ErrForbidden
	}
	return nil
}

func (s GaspolService) ensureSchoolReader(ctx context.Context, user auth.User, year int, schoolID int) (bool, error) {
	if user.SchoolID == nil || *user.SchoolID != schoolID {
		return false, auth.ErrForbidden
	}
	canEdit, err := s.Repo.UserHasRole(ctx, year, user.ID, GaspolRoleSchoolEntry, &schoolID)
	if err != nil {
		return false, err
	}
	return canEdit, nil
}

func (s GaspolService) ensureGaspolRole(ctx context.Context, user auth.User, year int, role string, schoolID *int) error {
	if role == GaspolRoleAdminMaster && user.Role == auth.RoleAdminMaster {
		return nil
	}
	ok, err := s.Repo.UserHasRole(ctx, year, user.ID, role, schoolID)
	if err != nil {
		return err
	}
	if !ok {
		return auth.ErrForbidden
	}
	return nil
}

func (s GaspolService) ensureAssignableGaspolUser(ctx context.Context, userID int, gaspolRole string) (gaspolUserMeta, error) {
	meta, err := s.Repo.GetUserMeta(ctx, userID)
	if err != nil {
		return gaspolUserMeta{}, err
	}
	if meta.SchoolID != nil && gaspolRole != GaspolRoleSchoolEntry {
		return gaspolUserMeta{}, errors.New("USER_NOT_ASSIGNABLE")
	}
	if !gaspolHasAssignableBaseRole(meta.BaseRole, gaspolRole) {
		return gaspolUserMeta{}, errors.New("USER_NOT_ASSIGNABLE")
	}
	return meta, nil
}

func (s GaspolService) ensureReusableJuryAccount(ctx context.Context, userID int) (gaspolUserMeta, error) {
	meta, err := s.Repo.GetUserMeta(ctx, userID)
	if err != nil {
		return gaspolUserMeta{}, err
	}
	if meta.SchoolID != nil {
		return gaspolUserMeta{}, errors.New("USER_NOT_ASSIGNABLE")
	}
	if auth.NormalizeRole(meta.BaseRole) != auth.NormalizeRole(auth.RoleSchoolAdmin) {
		return gaspolUserMeta{}, errors.New("USER_NOT_ASSIGNABLE")
	}
	return meta, nil
}

func (s GaspolService) AccessContext(ctx context.Context, user auth.User, year int) (GaspolAccessContext, error) {
	year = normalizeGaspolYear(year)
	switch auth.NormalizeRole(user.Role) {
	case auth.RoleAdminMaster:
		return GaspolAccessContext{
			Year: year,
			View: GaspolRoleAdminMaster,
		}, nil
	case auth.RoleGaspolSchoolUser:
		if user.SchoolID == nil {
			break
		}
		ok, err := s.Repo.UserHasRole(ctx, year, user.ID, GaspolRoleSchoolEntry, user.SchoolID)
		if err != nil {
			return GaspolAccessContext{}, err
		}
		if ok {
			return GaspolAccessContext{Year: year, View: GaspolRoleSchoolEntry}, nil
		}
		return GaspolAccessContext{Year: year, View: "DISABLED"}, nil
	case auth.RolePengawas:
		isKoor, err := s.isKoorGaspol(ctx, year, user.ID)
		if err != nil {
			return GaspolAccessContext{}, err
		}
		if isKoor {
			return GaspolAccessContext{Year: year, View: GaspolRoleSupervisor}, nil
		}
		ok, err := s.Repo.UserHasRole(ctx, year, user.ID, GaspolRoleSupervisor, nil)
		if err != nil {
			return GaspolAccessContext{}, err
		}
		if ok {
			return GaspolAccessContext{Year: year, View: GaspolRoleSupervisor}, nil
		}
		return GaspolAccessContext{Year: year, View: "DISABLED"}, nil
	}

	var hasStageAssignment bool
	if err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$2 AND is_active=TRUE AND school_id IS NOT NULL)`, year, user.ID).Scan(&hasStageAssignment); err != nil {
		return GaspolAccessContext{}, err
	}
	if hasStageAssignment {
		return GaspolAccessContext{Year: year, View: GaspolRoleJury}, nil
	}

	checks := []struct {
		view     string
		role     string
		schoolID *int
	}{
		{view: GaspolRoleSchoolEntry, role: GaspolRoleSchoolEntry, schoolID: user.SchoolID},
		{view: GaspolRoleSupervisor, role: GaspolRoleSupervisor},
		{view: GaspolRoleValidator, role: GaspolRoleValidator},
		{view: GaspolRoleJury, role: GaspolRoleJury},
	}

	for _, item := range checks {
		ok, err := s.Repo.UserHasRole(ctx, year, user.ID, item.role, item.schoolID)
		if err != nil {
			return GaspolAccessContext{}, err
		}
		if ok {
			return GaspolAccessContext{
				Year: year,
				View: item.view,
			}, nil
		}
	}

	return GaspolAccessContext{
		Year: year,
		View: "DISABLED",
	}, nil
}

type gaspolInitialPasswordRecord struct {
	Ciphertext *string
	Nonce      *string
	Encrypted  *string
}

func (s GaspolService) initialPasswordSupport(ctx context.Context) (InitialPasswordSupport, error) {
	return AdminService{DB: s.DB}.InitialPasswordSupport(ctx)
}

func (s GaspolService) buildInitialPasswordRecord(support InitialPasswordSupport, password string) (gaspolInitialPasswordRecord, error) {
	if strings.TrimSpace(password) == "" {
		return gaspolInitialPasswordRecord{}, nil
	}
	if support.V2 {
		ciphertext, nonce, err := utils.EncryptInitialPassword(s.MasterEncryptionKey, password)
		if err != nil {
			return gaspolInitialPasswordRecord{}, err
		}
		return gaspolInitialPasswordRecord{
			Ciphertext: &ciphertext,
			Nonce:      &nonce,
		}, nil
	}
	if support.V1 {
		encrypted, err := utils.EncryptString(s.MasterEncryptionKey, password)
		if err != nil {
			return gaspolInitialPasswordRecord{}, err
		}
		return gaspolInitialPasswordRecord{
			Encrypted: &encrypted,
		}, nil
	}
	return gaspolInitialPasswordRecord{}, nil
}

func (s GaspolService) decryptInitialPassword(row GaspolSchoolUserRow) (string, error) {
	if row.InitialPasswordCiphertext != nil && row.InitialPasswordNonce != nil {
		return utils.DecryptInitialPassword(s.MasterEncryptionKey, *row.InitialPasswordCiphertext, *row.InitialPasswordNonce)
	}
	if row.InitialPasswordEncrypted != nil {
		return utils.DecryptString(s.MasterEncryptionKey, *row.InitialPasswordEncrypted)
	}
	return "", nil
}

func (s GaspolService) listSchoolUsers(ctx context.Context, year int) ([]GaspolSchoolUserRow, error) {
	support, err := s.initialPasswordSupport(ctx)
	if err != nil {
		return nil, err
	}
	return s.Repo.ListSchoolUsers(ctx, year, support)
}

func (s GaspolService) buildGaspolLiveScoringInputs(ctx context.Context, year int, schoolIDs []int, effectiveNow time.Time) (map[int]GaspolScoringInput, GaspolRankingStatusSummary, error) {
	submissions, err := s.Repo.ListRankingMonthlySubmissions(ctx, year)
	if err != nil {
		return nil, GaspolRankingStatusSummary{}, err
	}
	assessments, err := s.Repo.ListRankingAssessments(ctx, year)
	if err != nil {
		return nil, GaspolRankingStatusSummary{}, err
	}

	submissionsBySchool := make(map[int][]GaspolSchoolSubmission)
	for _, item := range submissions {
		submissionsBySchool[item.SchoolID] = append(submissionsBySchool[item.SchoolID], item)
	}
	assessmentsBySchool := make(map[int][]GaspolAssessment)
	for _, item := range assessments {
		assessmentsBySchool[item.SchoolID] = append(assessmentsBySchool[item.SchoolID], item)
	}

	scoreMap := make(map[int]GaspolScoringInput)
	allSchoolIDs := make(map[int]struct{}, len(submissionsBySchool)+len(assessmentsBySchool)+len(schoolIDs))
	for _, schoolID := range schoolIDs {
		if schoolID > 0 {
			allSchoolIDs[schoolID] = struct{}{}
		}
	}
	for schoolID := range submissionsBySchool {
		allSchoolIDs[schoolID] = struct{}{}
	}
	for schoolID := range assessmentsBySchool {
		allSchoolIDs[schoolID] = struct{}{}
	}
	for schoolID := range allSchoolIDs {
		schoolSubmissions := submissionsBySchool[schoolID]
		schoolAssessments := assessmentsBySchool[schoolID]
		fieldScore, hasField := latestValidatedAssessmentScore(schoolAssessments, GaspolStageField)
		presentationScore, hasPresentation := latestValidatedAssessmentScore(schoolAssessments, GaspolStagePresentation)
		scoreMap[schoolID] = GaspolScoringInput{
			MonthlyComponents: buildGaspolMonthlyComponents(
				year,
				buildMonthlySubmissionMap(schoolSubmissions),
				buildMonthlyAssessmentMap(schoolAssessments),
				effectiveNow,
			),
			FieldScore:        fieldScore,
			FieldStage1Score:  fieldScore,
			PresentationScore: presentationScore,
			HasFieldStage1:    hasField,
			HasPresentation:   hasPresentation,
		}
	}

	monthly, err := s.Repo.ListMonthlyCompetitionComponents(ctx, year, 0, effectiveNow)
	if err != nil {
		return nil, GaspolRankingStatusSummary{}, err
	}
	activeScores := make(map[int]GaspolScoringInput, len(schoolIDs))
	for _, id := range schoolIDs {
		input := scoreMap[id]
		input.MonthlyComponents = monthly[id]
		activeScores[id] = input
	}
	return activeScores, buildGaspolRankingStatusSummary(activeScores), nil
}

func (s GaspolService) buildRanking(ctx context.Context, year int, month int, stage string) ([]GaspolRankingRow, GaspolRankingStatusSummary, error) {
	stage = normalizeGaspolStage(stage, month)
	if month == 0 {
		month = gaspolStageMonth(stage)
	}

	baseRows, err := s.Repo.ListRankingSchoolMeta(ctx, year)
	if err != nil {
		return nil, GaspolRankingStatusSummary{}, err
	}
	if len(baseRows) == 0 {
		return []GaspolRankingRow{}, GaspolRankingStatusSummary{
			IsLive:            true,
			TopTenLockReady:   false,
			RankingMessage:    "Belum ada peserta aktif untuk disusun ke ranking.",
			TopTenLockMessage: "Top 10 resmi belum bisa dikunci karena belum ada peserta aktif.",
		}, nil
	}
	schoolIDs := make([]int, 0, len(baseRows))
	for _, row := range baseRows {
		schoolIDs = append(schoolIDs, row.SchoolID)
	}

	effectiveNow, err := s.Clock.Now(ctx)
	if err != nil {
		return nil, GaspolRankingStatusSummary{}, err
	}

	scoreMap, rankingStatus, err := s.buildGaspolLiveScoringInputs(ctx, year, schoolIDs, effectiveNow)
	if err != nil {
		return nil, GaspolRankingStatusSummary{}, err
	}

	return rankGaspolCompetitionRows(baseRows, scoreMap, month, stage), rankingStatus, nil
}

func rankGaspolCompetitionRows(baseRows []GaspolRankingRow, scoreMap map[int]GaspolScoringInput, month int, stage string) []GaspolRankingRow {
	for index := range baseRows {
		input := scoreMap[baseRows[index].SchoolID]
		baseRows[index].Month = month
		baseRows[index].Stage = stage
		baseRows[index].MonthlyComponents = append([]GaspolMonthlyComponent(nil), input.MonthlyComponents...)
		baseRows[index].MonthlyFinal = gaspolMonthlySeriesIsFinal(input.MonthlyComponents)
		baseRows[index].Scores = ComputeGaspolStageScore(stage, input)
		baseRows[index].Path = "MAIN"
	}

	monthlyRows := append([]GaspolRankingRow(nil), baseRows...)
	for i := range monthlyRows {
		monthlyRows[i].Scores = ComputeGaspolStageScore(GaspolStageMonthly, scoreMap[monthlyRows[i].SchoolID])
	}
	monthlyRanking := RankGaspolRows(monthlyRows)
	switch stage {
	case GaspolStageField:
		return RankGaspolRows(filterGaspolRankingRowsBySchoolIDs(baseRows, schoolIDsFromRanking(selectGaspolReadyTopTen(monthlyRanking))))
	case GaspolStagePresentation, GaspolStageFinal:
		fieldRows := make([]GaspolRankingRow, 0, len(baseRows))
		for index := range baseRows {
			row := baseRows[index]
			row.Stage = GaspolStageField
			row.Month = gaspolStageMonth(GaspolStageField)
			row.Scores = ComputeGaspolStageScore(GaspolStageField, scoreMap[row.SchoolID])
			fieldRows = append(fieldRows, row)
		}
		topFiveIDs := schoolIDsFromRanking(SelectGaspolTopFive(RankGaspolRows(filterGaspolRankingRowsBySchoolIDs(fieldRows, schoolIDsFromRanking(selectGaspolReadyTopTen(monthlyRanking))))))
		return RankGaspolRows(filterGaspolRankingRowsBySchoolIDs(baseRows, topFiveIDs))
	default:
		return monthlyRanking
	}
}

func (s GaspolService) refreshRankingSnapshot(ctx context.Context, year int, month int, stage string, actorID int) ([]GaspolRankingRow, error) {
	rows, _, err := s.buildRanking(ctx, year, month, stage)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.ReplaceRankingSnapshots(ctx, year, month, stage, rows, actorID); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s GaspolService) refreshRankingCascade(ctx context.Context, year int, actorID int) error {
	stages := []string{GaspolStageMonthly, GaspolStageField, GaspolStagePresentation}
	for _, stage := range stages {
		if _, err := s.refreshRankingSnapshot(ctx, year, gaspolStageMonth(stage), stage, actorID); err != nil {
			return err
		}
	}
	return nil
}

func (s GaspolService) getLeaderboard(ctx context.Context, year int, month int, stage string) ([]GaspolRankingRow, GaspolRankingStatusSummary, *time.Time, error) {
	rows, rankingStatus, err := s.buildRanking(ctx, year, month, stage)
	if err != nil {
		return nil, GaspolRankingStatusSummary{}, nil, err
	}
	return rows, rankingStatus, nil, nil
}

func (s GaspolService) PublicDashboard(ctx context.Context, year int) (GaspolPublicDashboard, error) {
	year = normalizeGaspolYear(year)
	month := gaspolStageMonth(GaspolStageMonthly)
	stage := GaspolStageMonthly
	var publishedAt *time.Time
	competitionState := "MONTHLY_ACTIVE"

	participants, err := s.Repo.ListParticipants(ctx, year)
	if err != nil {
		return GaspolPublicDashboard{}, err
	}
	leaderboard, rankingStatus, rankingPublishedAt, err := s.getLeaderboard(ctx, year, month, stage)
	if err != nil {
		return GaspolPublicDashboard{}, err
	}
	if rankingPublishedAt != nil {
		publishedAt = rankingPublishedAt
	}
	// Public output switches only on immutable, explicitly locked snapshots.
	// Pending Koor/Cabdin values never leak through this branch.
	if config, configErr := s.CompetitionConfig(ctx, year); configErr == nil {
		competitionState = config.State
		if lockedRows, lockedStage, lockedErr := s.publicPublishedRanking(ctx, year, config); lockedErr != nil {
			return GaspolPublicDashboard{}, lockedErr
		} else if len(lockedRows) > 0 {
			leaderboard = lockedRows
			stage = lockedStage
			month = gaspolStageMonth(lockedStage)
			rankingStatus.IsLive = false
			rankingStatus.RankingMessage = "Hasil resmi berdasarkan snapshot penetapan yang telah dikunci."
		}
	}

	bestSchool := (*GaspolRankingRow)(nil)
	if len(leaderboard) > 0 {
		copyRow := leaderboard[0]
		bestSchool = &copyRow
	}
	awards, err := s.PublicAwards(ctx, year, bestSchool)
	if err != nil {
		return GaspolPublicDashboard{}, err
	}

	pendingCount, err := s.Repo.CountPendingValidations(ctx, year)
	if err != nil {
		return GaspolPublicDashboard{}, err
	}

	return GaspolPublicDashboard{
		Year:             year,
		Month:            month,
		Stage:            stage,
		CompetitionState: competitionState,
		Cards: []GaspolSummaryCard{
			{Label: "participants", Value: len(participants)},
			{Label: "leaderboardRows", Value: len(leaderboard)},
			{Label: "pendingValidation", Value: pendingCount},
		},
		Leaderboard:   ensureGaspolRankingRows(leaderboard),
		RankingStatus: rankingStatus,
		BestSchool:    bestSchool,
		Awards:        awards,
		LastPublished: publishedAt,
	}, nil
}

func ensureGaspolRankingRows(rows []GaspolRankingRow) []GaspolRankingRow {
	if rows == nil {
		return []GaspolRankingRow{}
	}
	return rows
}

func schoolIDsFromRanking(rows []GaspolRankingRow) map[int]struct{} {
	ids := make(map[int]struct{}, len(rows))
	for _, row := range rows {
		ids[row.SchoolID] = struct{}{}
	}
	return ids
}

func filterGaspolRankingRowsBySchoolIDs(rows []GaspolRankingRow, allowed map[int]struct{}) []GaspolRankingRow {
	if len(rows) == 0 || len(allowed) == 0 {
		return []GaspolRankingRow{}
	}
	filtered := make([]GaspolRankingRow, 0, len(rows))
	for _, row := range rows {
		if _, ok := allowed[row.SchoolID]; ok {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func ensureGaspolAssignments(rows []GaspolAssignment) []GaspolAssignment {
	if rows == nil {
		return []GaspolAssignment{}
	}
	return rows
}

func ensureGaspolAssessments(rows []GaspolAssessment) []GaspolAssessment {
	if rows == nil {
		return []GaspolAssessment{}
	}
	return rows
}

func ensureGaspolSubmissions(rows []GaspolSchoolSubmission) []GaspolSchoolSubmission {
	if rows == nil {
		return []GaspolSchoolSubmission{}
	}
	return rows
}

func ensureGaspolSchoolMonthStatuses(rows []GaspolSchoolMonthStatus) []GaspolSchoolMonthStatus {
	if rows == nil {
		return []GaspolSchoolMonthStatus{}
	}
	return rows
}

func ensureGaspolWorkflowRows(rows []GaspolMonthlyWorkflowRow) []GaspolMonthlyWorkflowRow {
	if rows == nil {
		return []GaspolMonthlyWorkflowRow{}
	}
	return rows
}

func containsGaspolMonth(months []int, month int) bool {
	for _, item := range months {
		if item == month {
			return true
		}
	}
	return false
}

func gaspolSchoolSubmissionDisplayStatus(submission GaspolSchoolSubmission, lockedMonths []int, scoredMonths []int) string {
	switch {
	case containsGaspolMonth(scoredMonths, submission.Month):
		return "Sudah Dinilai"
	case strings.EqualFold(strings.TrimSpace(submission.Status), GaspolSubmissionSubmitted):
		return "Terkirim ke Penilai"
	case containsGaspolMonth(lockedMonths, submission.Month):
		return "Terkunci"
	default:
		return "Belum Dikirim"
	}
}

func gaspolSchoolMonthDisplayStatus(month int, status string, lockedMonths []int, scoredMonths []int) string {
	submission := GaspolSchoolSubmission{
		Month:  month,
		Status: status,
	}
	return gaspolSchoolSubmissionDisplayStatus(submission, lockedMonths, scoredMonths)
}

func gaspolValidationDecisionLabel(decision string) string {
	switch strings.ToUpper(strings.TrimSpace(decision)) {
	case GaspolValidationAgree:
		return "Setuju"
	case GaspolValidationObject:
		return "Keberatan"
	default:
		return ""
	}
}

func gaspolSchoolAssessmentStatus(submission *GaspolSchoolSubmission, assessment *GaspolAssessment, locked bool) string {
	switch {
	case assessment != nil && assessment.IsValidated && assessment.FinalMonthlyScore != nil:
		return fmt.Sprintf("Nilai akhir: %.2f", *assessment.FinalMonthlyScore)
	case assessment != nil:
		return "Belum divalidasi cabdin"
	case submission != nil && strings.EqualFold(strings.TrimSpace(submission.Status), GaspolSubmissionSubmitted):
		return "Belum dinilai pengawas"
	case locked:
		return "Periode terkunci"
	default:
		return "Belum dikirim ke penilai"
	}
}

func gaspolWorkflowDecisionStatus(assessment *GaspolAssessment) string {
	if assessment == nil || !assessment.IsValidated {
		return ""
	}
	label := gaspolValidationDecisionLabel(assessment.ValidationDecision)
	if label == "" {
		return "Setuju"
	}
	return label
}

func gaspolWorkflowFinalScore(assessment *GaspolAssessment) *float64 {
	if assessment == nil || !assessment.IsValidated || assessment.FinalMonthlyScore == nil {
		return nil
	}
	score := *assessment.FinalMonthlyScore
	return &score
}

func ensureGaspolSchoolMonitoringExportRows(rows []GaspolSchoolMonitoringExportRow) []GaspolSchoolMonitoringExportRow {
	if rows == nil {
		return []GaspolSchoolMonitoringExportRow{}
	}
	return rows
}

func ensureGaspolSupervisorExportRows(rows []GaspolSupervisorMonitoringExportRow) []GaspolSupervisorMonitoringExportRow {
	if rows == nil {
		return []GaspolSupervisorMonitoringExportRow{}
	}
	return rows
}

func leaderboardStageOrDefault(rows []GaspolRankingRow) string {
	if len(rows) == 0 || strings.TrimSpace(rows[0].Stage) == "" {
		return GaspolStageMonthly
	}
	return rows[0].Stage
}

func leaderboardMonthOrDefault(rows []GaspolRankingRow) int {
	stage := leaderboardStageOrDefault(rows)
	if len(rows) == 0 || rows[0].Month == 0 {
		return gaspolStageMonth(stage)
	}
	return rows[0].Month
}

func gaspolSubmissionUpdatedAt(item GaspolSchoolSubmission) time.Time {
	if item.UpdatedAt != nil {
		return *item.UpdatedAt
	}
	if item.SubmittedAt != nil {
		return *item.SubmittedAt
	}
	return time.Time{}
}

func buildMonthlySubmissionMap(rows []GaspolSchoolSubmission) map[int]GaspolSchoolSubmission {
	result := make(map[int]GaspolSchoolSubmission, len(rows))
	for _, item := range rows {
		existing, ok := result[item.Month]
		if !ok || gaspolSubmissionUpdatedAt(item).After(gaspolSubmissionUpdatedAt(existing)) {
			result[item.Month] = item
		}
	}
	return result
}

func gaspolAssessmentUpdatedAt(item GaspolAssessment) time.Time {
	if item.UpdatedAt != nil {
		return *item.UpdatedAt
	}
	if item.ValidatedAt != nil {
		return *item.ValidatedAt
	}
	if item.CreatedAt != nil {
		return *item.CreatedAt
	}
	return time.Time{}
}

func buildMonthlyAssessmentMap(rows []GaspolAssessment) map[int]GaspolAssessment {
	result := map[int]GaspolAssessment{}
	for _, item := range rows {
		if item.Stage != GaspolStageMonthly {
			continue
		}
		existing, ok := result[item.Month]
		if !ok {
			result[item.Month] = item
			continue
		}
		if gaspolAssessmentUpdatedAt(item).After(gaspolAssessmentUpdatedAt(existing)) {
			result[item.Month] = item
		}
	}
	return result
}

func latestValidatedAssessmentScore(rows []GaspolAssessment, stage string) (float64, bool) {
	var (
		selected GaspolAssessment
		found    bool
	)
	for _, item := range rows {
		if item.Stage != stage || !item.IsValidated {
			continue
		}
		if !found || gaspolAssessmentUpdatedAt(item).After(gaspolAssessmentUpdatedAt(selected)) {
			selected = item
			found = true
		}
	}
	if !found {
		return 0, false
	}
	switch stage {
	case GaspolStageField:
		return sanitizeGaspolScore(selected.ScoreField), true
	case GaspolStagePresentation:
		return sanitizeGaspolScore(selected.ScorePresentation), true
	default:
		return 0, false
	}
}

func upsertGaspolValidatedMonthlyComponent(components []GaspolMonthlyComponent, month int, score float64) []GaspolMonthlyComponent {
	definitions := gaspolMonthlyComponentDefinitions()
	result := make([]GaspolMonthlyComponent, 0, len(definitions))
	found := false
	for _, item := range definitions {
		component := item
		component.Status = GaspolMonthlyStatusNotYetDue
		component.StatusLabel = gaspolMonthlyComponentStatusLabel(component.Status)
		component.StatusDetail = "Komponen belum final."
		component.IsFinal = gaspolMonthlyComponentStatusIsFinal(component.Status)
		for _, existing := range components {
			if existing.Month == item.Month {
				component = existing
				break
			}
		}
		if component.Month == month {
			component.Score = sanitizeGaspolScore(score)
			component.EffectiveScore = roundGaspolScore(component.Score * component.Weight)
			component.Status = GaspolMonthlyStatusValidated
			component.StatusLabel = gaspolMonthlyComponentStatusLabel(component.Status)
			component.StatusDetail = "Nilai tervalidasi masuk ke ranking."
			component.IsFinal = true
			found = true
		}
		result = append(result, component)
	}
	if found {
		return result
	}
	return components
}

func buildGaspolMonthlyComponents(
	year int,
	submissionByMonth map[int]GaspolSchoolSubmission,
	assessmentByMonth map[int]GaspolAssessment,
	effectiveNow time.Time,
) []GaspolMonthlyComponent {
	now := normalizeJakartaTime(effectiveNow)
	components := make([]GaspolMonthlyComponent, 0, len(gaspolMonthlyComponentDefinitions()))
	for _, definition := range gaspolMonthlyComponentDefinitions() {
		component := definition
		component.Score = 0
		component.EffectiveScore = 0
		component.Status = GaspolMonthlyStatusNotYetDue
		component.StatusLabel = gaspolMonthlyComponentStatusLabel(component.Status)
		component.StatusDetail = "Deadline belum lewat."

		submission, hasSubmission := submissionByMonth[definition.Month]
		assessment, hasAssessment := assessmentByMonth[definition.Month]
		isSubmitted := hasSubmission && (submission.SubmittedAt != nil || strings.EqualFold(strings.TrimSpace(submission.Status), GaspolSubmissionSubmitted))

		switch {
		case hasAssessment && assessment.IsValidated:
			score := assessment.ScoreMonthly
			if assessment.FinalMonthlyScore != nil {
				score = *assessment.FinalMonthlyScore
			}
			component.Score = sanitizeGaspolScore(score)
			component.Status = GaspolMonthlyStatusValidated
			component.StatusLabel = gaspolMonthlyComponentStatusLabel(component.Status)
			component.StatusDetail = "Nilai tervalidasi masuk ke ranking."
		case isSubmitted:
			component.Status = GaspolMonthlyStatusWaiting
			component.StatusLabel = gaspolMonthlyComponentStatusLabel(component.Status)
			if hasAssessment {
				component.StatusDetail = "Laporan sudah dinilai tetapi masih menunggu validasi Cabdin."
			} else {
				component.StatusDetail = "Laporan sudah terkirim dan masih menunggu penilaian pengawas."
			}
		case !hasAssessment && definition.Month <= 6 && now.After(GaspolDeadlineForMonthInLocation(year, definition.Month, JakartaLocation())):
			component.Status = GaspolMonthlyStatusZeroFinal
			component.StatusLabel = gaspolMonthlyComponentStatusLabel(component.Status)
			component.StatusDetail = "Sekolah tidak mengirim sampai deadline, nilai 0 final."
		case hasAssessment || now.After(GaspolDeadlineForMonthInLocation(year, definition.Month, JakartaLocation())):
			component.Status = GaspolMonthlyStatusWaiting
			component.StatusLabel = gaspolMonthlyComponentStatusLabel(component.Status)
			component.StatusDetail = "Menunggu penilaian dan finalisasi pengawas."
		default:
			component.Status = GaspolMonthlyStatusNotYetDue
			component.StatusLabel = gaspolMonthlyComponentStatusLabel(component.Status)
			component.StatusDetail = "Komponen belum sampai deadline."
		}

		component.EffectiveScore = roundGaspolScore(component.Score * component.Weight)
		component.IsFinal = gaspolMonthlyComponentStatusIsFinal(component.Status)
		components = append(components, component)
	}
	return components
}

func buildGaspolRankingStatusSummary(rows map[int]GaspolScoringInput) GaspolRankingStatusSummary {
	summary := GaspolRankingStatusSummary{
		IsLive: true,
	}
	for _, item := range rows {
		for _, component := range item.MonthlyComponents {
			switch component.Status {
			case GaspolMonthlyStatusValidated:
				summary.ValidatedComponents++
			case GaspolMonthlyStatusZeroFinal:
				summary.ZeroFinalComponents++
			case GaspolMonthlyStatusWaiting:
				summary.WaitingComponents++
				summary.HasWaiting = true
			case GaspolMonthlyStatusNotYetDue:
				summary.NotYetDueComponents++
				summary.HasNotYetDue = true
			}
		}
	}

	summary.TopTenLockReady = len(rows) > 0
	for _, item := range rows {
		summary.TopTenLockReady = summary.TopTenLockReady && gaspolMonthlySeriesIsFinal(item.MonthlyComponents)
	}
	switch {
	case summary.HasWaiting && summary.HasNotYetDue:
		summary.RankingMessage = "Ranking masih sementara karena masih ada komponen menunggu penilaian/validasi dan ada komponen yang belum jatuh tempo."
	case summary.HasWaiting:
		summary.RankingMessage = "Ranking masih sementara karena masih ada komponen menunggu penilaian/validasi."
	case summary.HasNotYetDue:
		summary.RankingMessage = "Ranking masih sementara karena masih ada komponen yang belum jatuh tempo."
	default:
		summary.RankingMessage = "Semua komponen April–September sudah final."
	}
	if summary.TopTenLockReady {
		summary.TopTenLockMessage = "Top 10 resmi sudah bisa dikunci karena semua komponen April–September sudah final."
	} else {
		summary.TopTenLockMessage = "Top 10 resmi belum bisa dikunci sampai semua komponen April–September berstatus tervalidasi atau 0 final."
	}
	return summary
}

func buildSchoolMonthStatuses(
	year int,
	submissionByMonth map[int]GaspolSchoolSubmission,
	assessmentByMonth map[int]GaspolAssessment,
	lockedMonths []int,
	scoredMonths []int,
) []GaspolSchoolMonthStatus {
	months := []int{4, 5, 6, 7, 8, 9, 10, 11}
	rows := make([]GaspolSchoolMonthStatus, 0, len(months))
	for _, month := range months {
		submission, hasSubmission := submissionByMonth[month]
		assessment, hasAssessment := assessmentByMonth[month]
		locked := containsGaspolMonth(lockedMonths, month)
		displayStatus := gaspolSchoolMonthDisplayStatus(month, submission.Status, lockedMonths, scoredMonths)
		assessmentStatus := gaspolSchoolAssessmentStatus(
			func() *GaspolSchoolSubmission {
				if !hasSubmission {
					return nil
				}
				copySubmission := submission
				return &copySubmission
			}(),
			func() *GaspolAssessment {
				if !hasAssessment {
					return nil
				}
				copyAssessment := assessment
				return &copyAssessment
			}(),
			locked,
		)
		var finalScore *float64
		if hasAssessment && assessment.FinalMonthlyScore != nil {
			score := *assessment.FinalMonthlyScore
			finalScore = &score
		}
		rows = append(rows, GaspolSchoolMonthStatus{
			Year:             year,
			Month:            month,
			ReportTitle:      submission.ReportTitle,
			ReportLink:       submission.ReportLink,
			DisplayStatus:    displayStatus,
			AssessmentStatus: assessmentStatus,
			FinalScore:       finalScore,
		})
	}
	return rows
}

func buildSupervisorWorkflowRows(
	assignments []GaspolAssignment,
	submissions []GaspolSchoolSubmission,
	assessments []GaspolAssessment,
	stageAssessments []GaspolStageAssessmentRow,
) ([]GaspolMonthlyWorkflowRow, []GaspolMonthlyWorkflowRow, []GaspolMonthlyWorkflowRow) {
	submissionBySchoolID := make(map[int]GaspolSchoolSubmission, len(submissions))
	for _, item := range submissions {
		submissionBySchoolID[item.SchoolID] = item
	}
	assessmentBySchoolID := make(map[int]GaspolAssessment, len(assessments))
	for _, item := range assessments {
		if item.Stage != GaspolStageMonthly {
			continue
		}
		assessmentBySchoolID[item.SchoolID] = item
	}
	stageAssessmentBySchoolID := make(map[int]GaspolStageAssessmentRow, len(stageAssessments))
	for _, item := range stageAssessments {
		if item.Stage == GaspolStageMonthly {
			stageAssessmentBySchoolID[item.SchoolID] = item
		}
	}

	missing := make([]GaspolMonthlyWorkflowRow, 0)
	pending := make([]GaspolMonthlyWorkflowRow, 0)
	completed := make([]GaspolMonthlyWorkflowRow, 0)
	for _, assignment := range assignments {
		submission, hasSubmission := submissionBySchoolID[assignment.SchoolID]
		assessment, hasAssessment := assessmentBySchoolID[assignment.SchoolID]
		stageAssessment, hasStageAssessment := stageAssessmentBySchoolID[assignment.SchoolID]
		row := GaspolMonthlyWorkflowRow{
			Month:            assignment.Month,
			SchoolID:         assignment.SchoolID,
			SchoolName:       assignment.SchoolName,
			SupervisorName:   assignment.SupervisorNameSnapshot,
			ReportLink:       "",
			SubmissionStatus: "Belum Mengirim Laporan",
			ProcessStatus:    "Belum mengirim laporan",
		}
		if hasSubmission {
			row.ReportLink = strings.TrimSpace(submission.ReportLink)
		}
		if hasSubmission && strings.EqualFold(strings.TrimSpace(submission.Status), GaspolSubmissionSubmitted) {
			row.SubmissionStatus = "Terkirim ke Penilai"
			row.ProcessStatus = "Siap dinilai"
		}
		if hasStageAssessment {
			stageAssessmentID := stageAssessment.ID
			row.StageAssessmentID = &stageAssessmentID
			row.WorkflowStatus = stageAssessment.WorkflowStatus
			row.SupervisorNotes = stageAssessment.Notes
			row.ReturnNotes = stageAssessment.ReviewerNotes
			score := stageAssessment.RawScore
			row.SupervisorScore = &score
			if hasAssessment {
				row.AssessmentID = &assessment.ID
			}
			row.SubmissionStatus = "Terkirim ke Penilai"
			switch stageAssessment.WorkflowStatus {
			case GaspolWorkflowReturnedKoor:
				row.ProcessStatus = "Dikembalikan Koor - perlu perbaikan"
				pending = append(pending, row)
			case GaspolWorkflowReturnedCabdin:
				row.ProcessStatus = "Dikembalikan Cabdin - perlu perbaikan"
				pending = append(pending, row)
			case GaspolWorkflowWaitingKoor:
				row.ProcessStatus = "Menunggu validasi Koor GASPOL"
				completed = append(completed, row)
			case GaspolWorkflowWaitingCabdin:
				row.ProcessStatus = "Menunggu validasi Cabdin"
				completed = append(completed, row)
			case GaspolWorkflowFinal:
				row.ProcessStatus = "Sudah divalidasi Cabdin"
				if stageAssessment.FinalScore != nil {
					finalScore := *stageAssessment.FinalScore
					row.FinalMonthlyScore = &finalScore
				}
				completed = append(completed, row)
			default:
				row.ProcessStatus = "Status workflow tidak dikenali"
				pending = append(pending, row)
			}
			continue
		}
		// Compatibility fallback for a legacy row which has not yet been
		// backfilled. Existing v2 rows always take precedence above.
		if hasAssessment {
			row.AssessmentID = &assessment.ID
			row.SubmissionStatus = "Terkirim ke Penilai"
			if assessment.IsValidated {
				row.ProcessStatus = "Sudah divalidasi Cabdin"
				row.FinalMonthlyScore = gaspolWorkflowFinalScore(&assessment)
				row.ValidationDecision = gaspolWorkflowDecisionStatus(&assessment)
				if assessment.ValidatorScore != nil {
					validatorScore := *assessment.ValidatorScore
					row.ValidatorScore = &validatorScore
				}
			} else {
				row.ProcessStatus = "Menunggu validasi Koor GASPOL"
			}
			score := assessment.ScoreMonthly
			if assessment.SupervisorScore != nil {
				score = *assessment.SupervisorScore
			}
			row.SupervisorScore = &score
			completed = append(completed, row)
			continue
		}
		if hasSubmission && strings.EqualFold(strings.TrimSpace(submission.Status), GaspolSubmissionSubmitted) {
			pending = append(pending, row)
			continue
		}
		// July and August are hybrid, while September is direct assessment: a
		// missing school submission must never block the assigned supervisor.
		if assignment.Month == 7 || assignment.Month == 8 || assignment.Month == 9 {
			if assignment.Month == 9 {
				row.SubmissionStatus = "Penilaian langsung"
			} else {
				row.SubmissionStatus = "Laporan opsional"
			}
			row.ProcessStatus = "Siap dinilai tanpa laporan sekolah"
			pending = append(pending, row)
			continue
		}
		missing = append(missing, row)
	}
	return missing, pending, completed
}

func buildValidatorWorkflowRows(
	month int,
	participants []GaspolSchoolParticipant,
	assignments []GaspolAssignment,
	submissions []GaspolSchoolSubmission,
	assessments []GaspolAssessment,
) ([]GaspolMonthlyWorkflowRow, []GaspolMonthlyWorkflowRow, []GaspolMonthlyWorkflowRow) {
	assignmentBySchoolID := make(map[int]GaspolAssignment, len(assignments))
	for _, item := range assignments {
		assignmentBySchoolID[item.SchoolID] = item
	}
	submissionBySchoolID := make(map[int]GaspolSchoolSubmission, len(submissions))
	for _, item := range submissions {
		submissionBySchoolID[item.SchoolID] = item
	}
	assessmentBySchoolID := make(map[int]GaspolAssessment, len(assessments))
	for _, item := range assessments {
		if item.Stage != GaspolStageMonthly {
			continue
		}
		assessmentBySchoolID[item.SchoolID] = item
	}

	blocked := make([]GaspolMonthlyWorkflowRow, 0)
	pending := make([]GaspolMonthlyWorkflowRow, 0)
	validated := make([]GaspolMonthlyWorkflowRow, 0)
	for _, participant := range participants {
		if participant.SchoolID == nil {
			continue
		}
		schoolID := *participant.SchoolID
		assignment := assignmentBySchoolID[schoolID]
		submission, hasSubmission := submissionBySchoolID[schoolID]
		assessment, hasAssessment := assessmentBySchoolID[schoolID]
		row := GaspolMonthlyWorkflowRow{
			Month:            month,
			SchoolID:         schoolID,
			SchoolName:       participant.SchoolName,
			ReportLink:       "",
			SupervisorName:   assignment.SupervisorNameSnapshot,
			ValidatorName:    participant.ValidatorName,
			SubmissionStatus: "Belum Dikirim",
		}
		if hasSubmission {
			row.ReportLink = strings.TrimSpace(submission.ReportLink)
		}

		if month <= 6 && (!hasSubmission || !strings.EqualFold(strings.TrimSpace(submission.Status), GaspolSubmissionSubmitted)) {
			row.ProcessStatus = "Sekolah belum mengerjakan"
			blocked = append(blocked, row)
			continue
		}

		if hasSubmission && strings.EqualFold(strings.TrimSpace(submission.Status), GaspolSubmissionSubmitted) {
			row.SubmissionStatus = "Terkirim ke Penilai"
		} else if month == 9 {
			row.SubmissionStatus = "Penilaian langsung"
		} else {
			row.SubmissionStatus = "Laporan opsional"
		}
		if !hasAssessment {
			row.ProcessStatus = "Belum dinilai pengawas"
			blocked = append(blocked, row)
			continue
		}

		row.AssessmentID = &assessment.ID
		if assessment.ReviewAssessmentID != nil {
			reviewID := int(*assessment.ReviewAssessmentID)
			row.AssessmentID = &reviewID
		}
		if assessment.SupervisorScore != nil {
			score := *assessment.SupervisorScore
			row.SupervisorScore = &score
		} else {
			score := assessment.ScoreMonthly
			row.SupervisorScore = &score
		}
		if assessment.ValidatorScore != nil {
			score := *assessment.ValidatorScore
			row.ValidatorScore = &score
		}
		row.FinalMonthlyScore = gaspolWorkflowFinalScore(&assessment)
		row.ValidationDecision = gaspolWorkflowDecisionStatus(&assessment)

		if assessment.IsValidated || assessment.WorkflowStatus == GaspolWorkflowFinal {
			row.ProcessStatus = "Sudah divalidasi"
			validated = append(validated, row)
			continue
		}
		if assessment.WorkflowStatus == GaspolWorkflowWaitingCabdin {
			row.ProcessStatus = "Menunggu Validator Cabdin"
			pending = append(pending, row)
			continue
		}
		row.ProcessStatus = "Menunggu persetujuan Koor GASPOL"
		blocked = append(blocked, row)
	}
	return blocked, pending, validated
}

func (s GaspolService) eligibleJurySchoolIDs(ctx context.Context, year int, stage string) (map[int]struct{}, error) {
	switch normalizeGaspolStage(stage, 0) {
	case GaspolStageField:
		rows, _, err := s.buildRanking(ctx, year, gaspolStageMonth(GaspolStageMonthly), GaspolStageMonthly)
		if err != nil {
			return nil, err
		}
		return schoolIDsFromRanking(selectGaspolReadyTopTen(rows)), nil
	case GaspolStagePresentation:
		rows, _, err := s.buildRanking(ctx, year, gaspolStageMonth(GaspolStageField), GaspolStageField)
		if err != nil {
			return nil, err
		}
		return schoolIDsFromRanking(SelectGaspolTopFive(rows)), nil
	default:
		return map[int]struct{}{}, nil
	}
}

func (s GaspolService) SchoolDashboard(ctx context.Context, user auth.User, year int) (GaspolSchoolDashboard, error) {
	year = normalizeGaspolYear(year)
	if user.SchoolID == nil || *user.SchoolID <= 0 {
		return GaspolSchoolDashboard{}, auth.ErrForbidden
	}
	effectiveNow, err := s.Clock.Now(ctx)
	if err != nil {
		return GaspolSchoolDashboard{}, err
	}
	schoolID := *user.SchoolID
	canEdit, err := s.ensureSchoolReader(ctx, user, year, schoolID)
	if err != nil {
		return GaspolSchoolDashboard{}, err
	}

	participant, err := s.Repo.GetParticipantBySchool(ctx, year, schoolID)
	isParticipant := true
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return GaspolSchoolDashboard{}, err
		}
		name, nameErr := s.Repo.GetSchoolName(ctx, schoolID)
		if nameErr != nil {
			return GaspolSchoolDashboard{}, nameErr
		}
		participant = GaspolSchoolParticipant{
			SchoolName: name,
			IsActive:   false,
		}
		isParticipant = false
	}

	submissions, err := s.Repo.ListSchoolSubmissions(ctx, year, schoolID)
	if err != nil {
		return GaspolSchoolDashboard{}, err
	}
	assessments, err := s.Repo.ListAssessmentsBySchool(ctx, year, schoolID)
	if err != nil {
		return GaspolSchoolDashboard{}, err
	}
	leaderboard, _, _, err := s.getLeaderboard(ctx, year, gaspolStageMonth(GaspolStageMonthly), GaspolStageMonthly)
	if err != nil {
		return GaspolSchoolDashboard{}, err
	}

	scoredMonthSet := map[int]struct{}{}
	monthlyAssessmentByMonth := buildMonthlyAssessmentMap(assessments)
	submissionByMonth := buildMonthlySubmissionMap(submissions)
	monthly, err := s.Repo.ListMonthlyCompetitionComponents(ctx, year, schoolID, effectiveNow)
	if err != nil {
		return GaspolSchoolDashboard{}, err
	}
	monthlyComponents := monthly[schoolID]
	for _, item := range assessments {
		if !item.IsValidated {
			continue
		}
		switch item.Stage {
		case GaspolStageMonthly:
			if isGaspolMonthlyComponentMonth(item.Month) {
				scoredMonthSet[item.Month] = struct{}{}
			}
		}
	}
	fieldScore, hasFieldScore := latestValidatedAssessmentScore(assessments, GaspolStageField)
	presentationScore, hasPresentationScore := latestValidatedAssessmentScore(assessments, GaspolStagePresentation)

	var ranking *GaspolRankingRow
	assignedSupervisor := ""
	for index := range leaderboard {
		if leaderboard[index].SchoolID == schoolID {
			copyRow := leaderboard[index]
			ranking = &copyRow
			assignedSupervisor = copyRow.SupervisorNameSnapshot
			break
		}
	}

	lockedMonths := make([]int, 0, 8)
	for month := 4; month <= 11; month++ {
		if IsGaspolMonthlyEntryLocked(year, month, effectiveNow) {
			lockedMonths = append(lockedMonths, month)
		}
	}
	scoredMonths := make([]int, 0, len(scoredMonthSet))
	for month := 4; month <= 11; month++ {
		if _, ok := scoredMonthSet[month]; ok {
			scoredMonths = append(scoredMonths, month)
		}
	}
	for index := range submissions {
		submissions[index].DisplayStatus = gaspolSchoolSubmissionDisplayStatus(
			submissions[index],
			lockedMonths,
			scoredMonths,
		)
		assessment, ok := monthlyAssessmentByMonth[submissions[index].Month]
		submissions[index].AssessmentStatus = gaspolSchoolAssessmentStatus(
			&submissions[index],
			func() *GaspolAssessment {
				if !ok {
					return nil
				}
				copyAssessment := assessment
				return &copyAssessment
			}(),
			containsGaspolMonth(lockedMonths, submissions[index].Month),
		)
	}
	monthlyStatuses := buildSchoolMonthStatuses(year, submissionByMonth, monthlyAssessmentByMonth, lockedMonths, scoredMonths)
	var latestSubmission *GaspolSchoolSubmission
	if len(submissions) > 0 {
		copySubmission := submissions[0]
		latestSubmission = &copySubmission
	}

	disabledReason := ""
	if !isParticipant || !participant.IsActive {
		disabledReason = "SCHOOL_NOT_PARTICIPANT"
	} else if !canEdit {
		disabledReason = "READ_ONLY"
	}
	cohorts, competitionNotice, err := s.SchoolCompetitionCohorts(ctx, year, schoolID)
	if err != nil {
		return GaspolSchoolDashboard{}, err
	}

	return GaspolSchoolDashboard{
		MonthlyFinal:       gaspolMonthlySeriesIsFinal(monthlyComponents),
		Year:               year,
		Month:              leaderboardMonthOrDefault(leaderboard),
		Stage:              leaderboardStageOrDefault(leaderboard),
		SchoolID:           schoolID,
		SchoolName:         participant.SchoolName,
		IsParticipant:      isParticipant && participant.IsActive,
		CanEdit:            canEdit && isParticipant && participant.IsActive,
		ActionsDisabled:    !canEdit || !isParticipant || !participant.IsActive,
		DisabledReason:     disabledReason,
		AssignedSupervisor: assignedSupervisor,
		AssignedValidator:  participant.ValidatorName,
		EffectiveTime:      pointerToTime(effectiveNow),
		LockedMonths:       lockedMonths,
		ScoredMonths:       scoredMonths,
		LatestSubmission:   latestSubmission,
		Scores: ComputeGaspolScore(GaspolScoringInput{
			MonthlyComponents: monthlyComponents,
			FieldScore:        fieldScore,
			PresentationScore: presentationScore,
			HasFieldStage1:    hasFieldScore,
			HasPresentation:   hasPresentationScore,
		}),
		Ranking:           ranking,
		MonthlyComponents: append([]GaspolMonthlyComponent(nil), monthlyComponents...),
		MonthlyStatuses:   ensureGaspolSchoolMonthStatuses(monthlyStatuses),
		History:           ensureGaspolSubmissions(submissions),
		Cohorts:           cohorts,
		CompetitionNotice: competitionNotice,
	}, nil
}

func (s GaspolService) UpsertSchoolSubmission(ctx context.Context, user auth.User, input GaspolSubmissionInput) (GaspolSchoolSubmission, error) {
	year := normalizeGaspolYear(input.Year)
	month := normalizeGaspolMonth(input.Month)
	if month < 4 || month > 8 {
		return GaspolSchoolSubmission{}, errors.New("INVALID_MONTH")
	}
	if user.SchoolID == nil || *user.SchoolID <= 0 {
		return GaspolSchoolSubmission{}, auth.ErrForbidden
	}
	if err := s.ensureGaspolRole(ctx, user, year, GaspolRoleSchoolEntry, user.SchoolID); err != nil {
		return GaspolSchoolSubmission{}, err
	}
	participant, err := s.Repo.GetParticipantBySchool(ctx, year, *user.SchoolID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GaspolSchoolSubmission{}, errors.New("SCHOOL_NOT_PARTICIPANT")
		}
		return GaspolSchoolSubmission{}, err
	}
	if !participant.IsActive {
		return GaspolSchoolSubmission{}, errors.New("SCHOOL_NOT_PARTICIPANT")
	}
	effectiveNow, err := s.Clock.Now(ctx)
	if err != nil {
		return GaspolSchoolSubmission{}, err
	}
	if IsGaspolMonthlyEntryLocked(year, month, effectiveNow) {
		return GaspolSchoolSubmission{}, errors.New("MONTHLY_ENTRY_LOCKED")
	}

	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status == "" {
		status = GaspolSubmissionDraft
	}
	if status != GaspolSubmissionDraft && status != GaspolSubmissionSubmitted {
		return GaspolSchoolSubmission{}, errors.New("INVALID_STATUS")
	}
	if status == GaspolSubmissionSubmitted {
		if strings.TrimSpace(input.ReportTitle) == "" ||
			strings.TrimSpace(input.ReportBody) == "" ||
			strings.TrimSpace(input.ReportLink) == "" {
			return GaspolSchoolSubmission{}, errors.New("INCOMPLETE_SUBMISSION")
		}
		parsedLink, parseErr := url.ParseRequestURI(strings.TrimSpace(input.ReportLink))
		if parseErr != nil || strings.TrimSpace(parsedLink.Scheme) == "" || strings.TrimSpace(parsedLink.Host) == "" {
			return GaspolSchoolSubmission{}, errors.New("INVALID_REPORT_LINK")
		}
	}
	existingSubmission, err := s.Repo.GetSchoolSubmission(ctx, year, month, *user.SchoolID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return GaspolSchoolSubmission{}, err
	}
	if err == nil && strings.EqualFold(strings.TrimSpace(existingSubmission.Status), GaspolSubmissionSubmitted) {
		return GaspolSchoolSubmission{}, errors.New("SUBMISSION_LOCKED")
	}

	item, err := s.Repo.UpsertSubmission(ctx, GaspolSchoolSubmission{
		Year:           year,
		Month:          month,
		SchoolID:       *user.SchoolID,
		ReportTitle:    strings.TrimSpace(input.ReportTitle),
		ReportBody:     strings.TrimSpace(input.ReportBody),
		ReportLink:     strings.TrimSpace(input.ReportLink),
		AttachmentURLs: input.AttachmentURLs,
		Status:         status,
	}, user.ID)
	if err != nil {
		return GaspolSchoolSubmission{}, err
	}
	lockedMonths := make([]int, 0, 8)
	for currentMonth := 4; currentMonth <= 11; currentMonth++ {
		if IsGaspolMonthlyEntryLocked(year, currentMonth, effectiveNow) {
			lockedMonths = append(lockedMonths, currentMonth)
		}
	}
	item.DisplayStatus = gaspolSchoolMonthDisplayStatus(item.Month, item.Status, lockedMonths, []int{})
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "UPSERT_SUBMISSION", "gaspol_school_submissions", &item.ID, user.SchoolID, input)
	return item, nil
}

func (s GaspolService) CancelSubmitSchoolSubmission(ctx context.Context, user auth.User, input GaspolCancelSubmitInput) (GaspolSchoolSubmission, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolSchoolSubmission{}, err
	}
	year := normalizeGaspolYear(input.Year)
	month := normalizeGaspolMonth(input.Month)
	if month < 4 || month > 11 {
		return GaspolSchoolSubmission{}, errors.New("INVALID_MONTH")
	}
	if input.SchoolID <= 0 {
		return GaspolSchoolSubmission{}, errors.New("INVALID_SCHOOL_ID")
	}

	submission, err := s.Repo.GetSchoolSubmission(ctx, year, month, input.SchoolID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GaspolSchoolSubmission{}, errors.New("SUBMISSION_NOT_SUBMITTED")
		}
		return GaspolSchoolSubmission{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(submission.Status), GaspolSubmissionSubmitted) {
		return GaspolSchoolSubmission{}, errors.New("SUBMISSION_NOT_SUBMITTED")
	}

	effectiveNow, err := s.Clock.Now(ctx)
	if err != nil {
		return GaspolSchoolSubmission{}, err
	}
	if IsGaspolMonthlyEntryLocked(year, month, effectiveNow) {
		return GaspolSchoolSubmission{}, errors.New("SUBMISSION_DEADLINE_PASSED")
	}

	assessed, err := s.Repo.HasSubmittedSupervisorAssessment(ctx, year, month, input.SchoolID)
	if err != nil {
		return GaspolSchoolSubmission{}, err
	}
	if assessed {
		return GaspolSchoolSubmission{}, errors.New("SUBMISSION_ALREADY_ASSESSED")
	}

	item, err := s.Repo.CancelSubmitSubmission(ctx, year, month, input.SchoolID)
	if err != nil {
		return GaspolSchoolSubmission{}, err
	}
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "CANCEL_SUBMIT_SUBMISSION", "gaspol_school_submissions", &item.ID, &input.SchoolID, map[string]any{
		"year":     year,
		"month":    month,
		"schoolId": input.SchoolID,
		"reason":   strings.TrimSpace(input.Reason),
	})
	return item, nil
}

func (s GaspolService) SupervisorDashboard(ctx context.Context, user auth.User, year int, month int) (GaspolSupervisorDashboard, error) {
	year = normalizeGaspolYear(year)
	month = normalizeGaspolMonth(month)
	if month < 4 || month > 9 {
		month = 9
	}
	isKoor, err := s.isKoorGaspol(ctx, year, user.ID)
	if err != nil {
		return GaspolSupervisorDashboard{}, err
	}
	stageTasks, err := s.Repo.ListStageTasksByAssessor(ctx, year, user.ID)
	if err != nil {
		return GaspolSupervisorDashboard{}, err
	}
	if roleErr := s.ensureGaspolRole(ctx, user, year, GaspolRoleSupervisor, nil); roleErr != nil && !isKoor && len(stageTasks) == 0 {
		return GaspolSupervisorDashboard{}, roleErr
	}
	assignments, err := s.Repo.ListAssignmentsBySupervisor(ctx, year, month, user.ID)
	if err != nil {
		return GaspolSupervisorDashboard{}, err
	}
	submissions, err := s.Repo.ListSchoolSubmissionsByMonth(ctx, year, month)
	if err != nil {
		return GaspolSupervisorDashboard{}, err
	}
	assessments, err := s.Repo.ListSupervisorAssessments(ctx, year, month, user.ID)
	if err != nil {
		return GaspolSupervisorDashboard{}, err
	}
	stageAssessments, err := s.Repo.ListSupervisorMonthlyStageAssessments(ctx, year, month, user.ID)
	if err != nil {
		return GaspolSupervisorDashboard{}, err
	}
	missingRows, pendingRows, completedRows := buildSupervisorWorkflowRows(assignments, submissions, assessments, stageAssessments)
	koorPending := []GaspolStageAssessmentRow{}
	if isKoor {
		koorPending, err = s.ListStageAssessmentQueue(ctx, year, GaspolWorkflowWaitingKoor, nil)
		if err != nil {
			return GaspolSupervisorDashboard{}, err
		}
	}
	return GaspolSupervisorDashboard{
		Year:                   year,
		Month:                  month,
		AssignmentCount:        len(assignments),
		ValidatedCount:         len(completedRows),
		PendingCount:           len(pendingRows),
		MissingSubmissionCount: len(missingRows),
		Assignments:            ensureGaspolAssignments(assignments),
		LatestAssessments:      ensureGaspolAssessments(assessments),
		MissingSubmissions:     ensureGaspolWorkflowRows(missingRows),
		PendingReviews:         ensureGaspolWorkflowRows(pendingRows),
		CompletedReviews:       ensureGaspolWorkflowRows(completedRows),
		IsKoorGaspol:           isKoor,
		KoorPendingAssessments: koorPending,
		StageTasks:             stageTasks,
	}, nil
}

func (s GaspolService) UpsertSupervisorAssessment(ctx context.Context, user auth.User, input GaspolAssessmentInput) (GaspolAssessment, error) {
	year := normalizeGaspolYear(input.Year)
	month := normalizeGaspolMonth(input.Month)
	if month < 4 || month > 9 {
		return GaspolAssessment{}, errors.New("INVALID_MONTH")
	}
	if err := s.ensureGaspolRole(ctx, user, year, GaspolRoleSupervisor, nil); err != nil {
		return GaspolAssessment{}, err
	}
	if month == 9 {
		config, configErr := s.CompetitionConfig(ctx, year)
		if configErr != nil {
			return GaspolAssessment{}, configErr
		}
		now, clockErr := s.Clock.Now(ctx)
		if clockErr != nil {
			return GaspolAssessment{}, clockErr
		}
		if now.After(config.SeptemberField1Deadline) {
			return GaspolAssessment{}, errors.New("ASSESSMENT_DEADLINE_PASSED")
		}
	}

	assignments, err := s.Repo.ListAssignmentsBySupervisor(ctx, year, month, user.ID)
	if err != nil {
		return GaspolAssessment{}, err
	}
	var assignmentID *int
	allowed := false
	for _, item := range assignments {
		if item.SchoolID == input.SchoolID {
			allowed = true
			assignmentID = &item.ID
			break
		}
	}
	if !allowed {
		return GaspolAssessment{}, auth.ErrForbidden
	}
	// Mirror the authorized legacy monthly assignment into the v2 assignment
	// table so SaveStageAssessment can enforce its normal ownership check.
	// This is idempotent and does not change assignments for any other stage.
	_, err = s.DB.Pool.Exec(ctx, `
		INSERT INTO gaspol_stage_assignments(
		 year,stage,school_id,assessor_user_id,assignment_role,is_active,assigned_by
		) VALUES($1,'MONTHLY',$2,$3,'SUPERVISOR_MONTHLY',TRUE,$3)
		ON CONFLICT(year,stage,school_id,assessor_user_id) DO UPDATE SET
		 assignment_role='SUPERVISOR_MONTHLY',is_active=TRUE,updated_at=NOW()
	`, year, input.SchoolID, user.ID)
	if err != nil {
		return GaspolAssessment{}, err
	}

	scoreMonthly, scoreField, scorePresentation := ScoreValueForStage(GaspolStageMonthly, input.Score)
	notes := strings.TrimSpace(input.Notes)
	var existingStageID int64
	var existingLegacyID *int
	var existingWorkflow string
	stageErr := s.DB.Pool.QueryRow(ctx, `
		SELECT id,legacy_assessment_id,workflow_status
		FROM gaspol_stage_assessments
		WHERE year=$1 AND stage='MONTHLY' AND month=$2 AND school_id=$3 AND assessor_user_id=$4
	`, year, month, input.SchoolID, user.ID).Scan(&existingStageID, &existingLegacyID, &existingWorkflow)
	if stageErr != nil && !errors.Is(stageErr, pgx.ErrNoRows) {
		return GaspolAssessment{}, stageErr
	}
	resubmitting := stageErr == nil
	if resubmitting && existingWorkflow != GaspolWorkflowReturnedKoor && existingWorkflow != GaspolWorkflowReturnedCabdin {
		return GaspolAssessment{}, errors.New("ASSESSMENT_ALREADY_SUBMITTED")
	}

	var item GaspolAssessment
	if !resubmitting {
		item, err = s.Repo.CreateSupervisorAssessment(ctx, GaspolAssessment{
			Year:              year,
			Month:             month,
			SchoolID:          input.SchoolID,
			AssignmentID:      assignmentID,
			AssessorUserID:    user.ID,
			AssessorRole:      GaspolRoleSupervisor,
			Stage:             GaspolStageMonthly,
			ScoreMonthly:      scoreMonthly,
			ScoreField:        scoreField,
			ScorePresentation: scorePresentation,
			ScoreTotal:        scoreMonthly,
			Notes:             notes,
		})
		if err != nil {
			return GaspolAssessment{}, err
		}
		existingLegacyID = &item.ID
	} else if existingLegacyID == nil {
		// A v2 monthly row can exist without a legacy link when it came through
		// the generic stage endpoint. Create the compatibility row once.
		item, err = s.Repo.CreateSupervisorAssessment(ctx, GaspolAssessment{
			Year: year, Month: month, SchoolID: input.SchoolID, AssignmentID: assignmentID,
			AssessorUserID: user.ID, AssessorRole: GaspolRoleSupervisor, Stage: GaspolStageMonthly,
			ScoreMonthly: scoreMonthly, ScoreField: scoreField, ScorePresentation: scorePresentation,
			ScoreTotal: scoreMonthly, Notes: notes,
		})
		if err != nil {
			return GaspolAssessment{}, err
		}
		existingLegacyID = &item.ID
	}

	stageRow, err := s.SaveStageAssessment(ctx, user, GaspolStageAssessmentInput{
		Year: year, Stage: GaspolStageMonthly, Month: month, SchoolID: input.SchoolID,
		Score: input.Score, Rubric: map[string]float64{}, Notes: notes,
	})
	if err != nil {
		return GaspolAssessment{}, err
	}
	if existingLegacyID != nil {
		_, err = s.DB.Pool.Exec(ctx, `
			UPDATE gaspol_stage_assessments SET legacy_assessment_id=$2
			WHERE id=$1 AND (legacy_assessment_id IS NULL OR legacy_assessment_id=$2)
		`, stageRow.ID, *existingLegacyID)
		if err != nil {
			return GaspolAssessment{}, err
		}
		_, err = s.DB.Pool.Exec(ctx, `
			UPDATE gaspol_assessments SET assignment_id=$2,score_monthly=$3,score_total=$3,
			 notes=$4,is_validated=FALSE,validated_by=NULL,validated_at=NULL,
			 workflow_status='WAITING_KOOR',updated_at=NOW()
			WHERE id=$1
		`, *existingLegacyID, assignmentID, scoreMonthly, notes)
		if err != nil {
			return GaspolAssessment{}, err
		}
	}
	item.ID = func() int {
		if existingLegacyID != nil {
			return *existingLegacyID
		}
		return 0
	}()
	item.Year, item.Month, item.SchoolID = year, month, input.SchoolID
	item.AssignmentID, item.AssessorUserID, item.AssessorRole = assignmentID, user.ID, GaspolRoleSupervisor
	item.Stage, item.ScoreMonthly, item.ScoreTotal, item.Notes = GaspolStageMonthly, scoreMonthly, scoreMonthly, notes
	item.WorkflowStatus = stageRow.WorkflowStatus
	item.ReviewAssessmentID = &stageRow.ID
	return item, nil
}

func (s GaspolService) UpsertJuryAssessment(ctx context.Context, user auth.User, stage string, input GaspolAssessmentInput) (GaspolAssessment, error) {
	year := normalizeGaspolYear(input.Year)
	month := normalizeGaspolMonth(input.Month)
	normalizedStage := normalizeGaspolStage(stage, month)
	if normalizedStage != GaspolStageField && normalizedStage != GaspolStagePresentation {
		return GaspolAssessment{}, errors.New("INVALID_STAGE")
	}
	if normalizedStage == GaspolStageField && month != 10 {
		return GaspolAssessment{}, errors.New("INVALID_MONTH")
	}
	if normalizedStage == GaspolStagePresentation && month != 11 {
		return GaspolAssessment{}, errors.New("INVALID_MONTH")
	}
	if err := s.ensureGaspolRole(ctx, user, year, GaspolRoleJury, nil); err != nil {
		return GaspolAssessment{}, err
	}
	assignmentStage := GaspolStageField2
	assignmentRole := "FIELD_STAGE_2_ASSESSOR"
	if normalizedStage == GaspolStagePresentation {
		assignmentStage = GaspolStagePresentation
		assignmentRole = "PRESENTATION_ASSESSOR"
	}
	var assigned bool
	if err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND stage=$2 AND school_id=$3 AND assessor_user_id=$4 AND assignment_role=$5 AND is_active=TRUE)`, year, assignmentStage, input.SchoolID, user.ID, assignmentRole).Scan(&assigned); err != nil {
		return GaspolAssessment{}, err
	}
	if !assigned {
		return GaspolAssessment{}, auth.ErrForbidden
	}
	eligibleSchoolIDs, err := s.eligibleJurySchoolIDs(ctx, year, normalizedStage)
	if err != nil {
		return GaspolAssessment{}, err
	}
	if _, ok := eligibleSchoolIDs[input.SchoolID]; !ok {
		return GaspolAssessment{}, errors.New("SCHOOL_NOT_ELIGIBLE")
	}
	var alreadyFinal bool
	if err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM gaspol_stage_assessments
		WHERE year=$1 AND stage=$2 AND month=$3 AND school_id=$4
		 AND assessor_user_id=$5 AND workflow_status='FINAL'
	)`, year, assignmentStage, month, input.SchoolID, user.ID).Scan(&alreadyFinal); err != nil {
		return GaspolAssessment{}, err
	}
	if alreadyFinal {
		return GaspolAssessment{}, errors.New("ASSESSMENT_ALREADY_SUBMITTED")
	}

	scoreMonthly, scoreField, scorePresentation := ScoreValueForStage(normalizedStage, input.Score)
	stageScore := scoreField
	if normalizedStage == GaspolStagePresentation {
		stageScore = scorePresentation
	}

	item, err := s.Repo.UpsertAssessment(ctx, GaspolAssessment{
		Year:              year,
		Month:             month,
		SchoolID:          input.SchoolID,
		AssessorUserID:    user.ID,
		AssessorRole:      GaspolRoleJury,
		Stage:             normalizedStage,
		ScoreMonthly:      scoreMonthly,
		ScoreField:        scoreField,
		ScorePresentation: scorePresentation,
		ScoreTotal:        stageScore,
		Notes:             strings.TrimSpace(input.Notes),
	})
	if err != nil {
		return GaspolAssessment{}, err
	}
	stageRow, err := s.SaveStageAssessment(ctx, user, GaspolStageAssessmentInput{
		Year: year, Stage: assignmentStage, Month: month, SchoolID: input.SchoolID,
		Score: input.Score, Rubric: map[string]float64{}, Notes: input.Notes,
	})
	if err != nil {
		return GaspolAssessment{}, err
	}
	if _, err = s.DB.Pool.Exec(ctx, `UPDATE gaspol_stage_assessments SET legacy_assessment_id=$2 WHERE id=$1 AND legacy_assessment_id IS NULL`, stageRow.ID, item.ID); err != nil {
		return GaspolAssessment{}, err
	}
	if stageRow.WorkflowStatus == GaspolWorkflowFinal && stageRow.FinalScore != nil {
		_, err = s.DB.Pool.Exec(ctx, `
			UPDATE gaspol_assessments legacy SET
			 is_validated=TRUE,validated_at=NOW(),workflow_status='FINAL',
			 score_field=CASE WHEN $2='FIELD_STAGE_2' THEN $3 ELSE score_field END,
			 score_presentation=CASE WHEN $2='PRESENTATION' THEN $3 ELSE score_presentation END,
			 score_total=$3,updated_at=NOW()
			FROM gaspol_stage_assessments sa
			WHERE sa.year=$1 AND sa.stage=$2 AND sa.school_id=$4 AND sa.month=$5
			 AND sa.workflow_status='FINAL' AND legacy.id=sa.legacy_assessment_id
		`, year, assignmentStage, *stageRow.FinalScore, input.SchoolID, month)
		if err != nil {
			return GaspolAssessment{}, err
		}
	}
	schoolID := input.SchoolID
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "UPSERT_"+normalizedStage+"_ASSESSMENT", "gaspol_assessments", &item.ID, &schoolID, input)
	return item, nil
}

func (s GaspolService) ValidatorDashboard(ctx context.Context, user auth.User, year int, month int) (GaspolValidatorDashboard, error) {
	year = normalizeGaspolYear(year)
	month = normalizeGaspolMonth(month)
	if month < 4 || month > 9 {
		month = 9
	}
	if err := s.ensureGaspolRole(ctx, user, year, GaspolRoleValidator, nil); err != nil {
		return GaspolValidatorDashboard{}, err
	}
	validationQueue, err := s.ListStageAssessmentQueue(ctx, year, GaspolWorkflowWaitingCabdin, &user.ID)
	if err != nil {
		return GaspolValidatorDashboard{}, err
	}
	leaderboard, _, _, err := s.getLeaderboard(ctx, year, month, "")
	if err != nil {
		return GaspolValidatorDashboard{}, err
	}
	participants, err := s.Repo.ListParticipants(ctx, year)
	if err != nil {
		return GaspolValidatorDashboard{}, err
	}
	filteredRanking := make([]GaspolRankingRow, 0)
	allowedSchoolIDs := map[int]struct{}{}
	scopeParticipants := make([]GaspolSchoolParticipant, 0)
	for _, item := range participants {
		if item.ValidatorUserID != nil && *item.ValidatorUserID == user.ID && item.SchoolID != nil {
			allowedSchoolIDs[*item.SchoolID] = struct{}{}
			scopeParticipants = append(scopeParticipants, item)
		}
	}
	for _, row := range leaderboard {
		if _, ok := allowedSchoolIDs[row.SchoolID]; ok {
			filteredRanking = append(filteredRanking, row)
		}
	}
	assignments, err := s.Repo.ListAssignmentsByMonth(ctx, year, month)
	if err != nil {
		return GaspolValidatorDashboard{}, err
	}
	submissions, err := s.Repo.ListSchoolSubmissionsByMonth(ctx, year, month)
	if err != nil {
		return GaspolValidatorDashboard{}, err
	}
	assessments, err := s.Repo.ListStageAssessments(ctx, year, month, GaspolStageMonthly)
	if err != nil {
		return GaspolValidatorDashboard{}, err
	}
	for index := range assessments {
		var reviewID int64
		var workflowStatus string
		reviewErr := s.DB.Pool.QueryRow(ctx, `SELECT id,workflow_status FROM gaspol_stage_assessments WHERE legacy_assessment_id=$1`, assessments[index].ID).Scan(&reviewID, &workflowStatus)
		if reviewErr == nil {
			assessments[index].ReviewAssessmentID = &reviewID
			assessments[index].WorkflowStatus = workflowStatus
		} else if !errors.Is(reviewErr, pgx.ErrNoRows) {
			return GaspolValidatorDashboard{}, reviewErr
		}
	}
	filteredAssessments := make([]GaspolAssessment, 0)
	for _, item := range assessments {
		if item.AssessorRole != GaspolRoleSupervisor {
			continue
		}
		if _, ok := allowedSchoolIDs[item.SchoolID]; ok {
			filteredAssessments = append(filteredAssessments, item)
		}
	}
	blockedRows, pendingRows, validatedRows := buildValidatorWorkflowRows(month, scopeParticipants, assignments, submissions, filteredAssessments)
	pendingAssessments := make([]GaspolAssessment, 0, len(filteredAssessments))
	validatedAssessments := make([]GaspolAssessment, 0, len(filteredAssessments))
	for _, item := range filteredAssessments {
		if item.IsValidated {
			validatedAssessments = append(validatedAssessments, item)
			continue
		}
		pendingAssessments = append(pendingAssessments, item)
	}

	return GaspolValidatorDashboard{
		Year:                 year,
		Month:                month,
		PendingCount:         len(pendingRows),
		ValidatedCount:       len(validatedRows),
		ParticipantCount:     len(allowedSchoolIDs),
		BlockedCount:         len(blockedRows),
		PendingAssessments:   ensureGaspolAssessments(pendingAssessments),
		ValidatedAssessments: ensureGaspolAssessments(validatedAssessments),
		NotReadyRows:         ensureGaspolWorkflowRows(blockedRows),
		PendingRows:          ensureGaspolWorkflowRows(pendingRows),
		ValidatedRows:        ensureGaspolWorkflowRows(validatedRows),
		Leaderboard:          ensureGaspolRankingRows(filteredRanking),
		ValidationQueue:      validationQueue,
	}, nil
}

func (s GaspolService) JuryDashboard(ctx context.Context, user auth.User, year int, month int, stage string) (GaspolJuryDashboard, error) {
	year = normalizeGaspolYear(year)
	stage = normalizeGaspolStage(stage, month)
	month = gaspolStageMonth(stage)
	if stage != GaspolStageField && stage != GaspolStagePresentation {
		stage = GaspolStageField
		month = gaspolStageMonth(stage)
	}
	stageTasks, err := s.Repo.ListStageTasksByAssessor(ctx, year, user.ID)
	if err != nil {
		return GaspolJuryDashboard{}, err
	}
	if roleErr := s.ensureGaspolRole(ctx, user, year, GaspolRoleJury, nil); roleErr != nil && len(stageTasks) == 0 {
		return GaspolJuryDashboard{}, roleErr
	}

	candidates, _, err := s.buildRanking(ctx, year, month, stage)
	if err != nil {
		return GaspolJuryDashboard{}, err
	}
	assignmentStage := GaspolStageField2
	if stage == GaspolStagePresentation {
		assignmentStage = GaspolStagePresentation
	}
	allowedSchoolIDs := map[int]bool{}
	for _, task := range stageTasks {
		if task.Stage == assignmentStage {
			allowedSchoolIDs[task.SchoolID] = true
		}
	}
	filteredCandidates := make([]GaspolRankingRow, 0, len(candidates))
	for _, candidate := range candidates {
		if allowedSchoolIDs[candidate.SchoolID] {
			filteredCandidates = append(filteredCandidates, candidate)
		}
	}
	candidates = filteredCandidates
	assessments, err := s.Repo.ListJuryAssessmentsByUser(ctx, year, user.ID)
	if err != nil {
		return GaspolJuryDashboard{}, err
	}

	filteredAssessments := make([]GaspolAssessment, 0, len(assessments))
	for _, item := range assessments {
		if item.Stage == stage {
			filteredAssessments = append(filteredAssessments, item)
		}
	}

	return GaspolJuryDashboard{
		Year:              year,
		Month:             month,
		Stage:             stage,
		CandidateCount:    len(candidates),
		Candidates:        ensureGaspolRankingRows(candidates),
		LatestAssessments: ensureGaspolAssessments(filteredAssessments),
		StageTasks:        stageTasks,
	}, nil
}

func (s GaspolService) ValidateAssessments(ctx context.Context, user auth.User, input GaspolValidationInput) ([]GaspolRankingRow, error) {
	// The legacy endpoint performed one-step Cabdin finalization. Pending legacy
	// and all new scores must use ReviewStageAssessment (KOOR then CABDIN).
	return nil, errors.New("TWO_LEVEL_VALIDATION_REQUIRED")
	/*
		year := normalizeGaspolYear(input.Year)
		assessmentIDs := append([]int{}, input.AssessmentIDs...)
		if input.AssessmentID > 0 {
			assessmentIDs = append(assessmentIDs, input.AssessmentID)
		}
		assessmentIDs = dedupeIntSlice(assessmentIDs)
		if len(assessmentIDs) == 0 {
			return []GaspolRankingRow{}, nil
		}
		if err := s.ensureGaspolRole(ctx, user, year, GaspolRoleValidator, nil); err != nil {
			return nil, err
		}
		decision := strings.ToUpper(strings.TrimSpace(input.Decision))
		if decision == "" {
			decision = GaspolValidationAgree
		}
		if decision != GaspolValidationAgree && decision != GaspolValidationObject {
			return nil, errors.New("INVALID_PAYLOAD")
		}
		if decision == GaspolValidationObject {
			if len(assessmentIDs) != 1 || input.Score == nil {
				return nil, errors.New("INVALID_PAYLOAD")
			}
			score := sanitizeGaspolScore(*input.Score)
			input.Score = &score
		}

		allowedIDs, err := s.Repo.GetAssessmentValidatorScope(ctx, assessmentIDs, user.ID)
		if err != nil {
			return nil, err
		}
		if len(allowedIDs) == 0 {
			return []GaspolRankingRow{}, nil
		}
		auditSnapshots, err := s.Repo.GetAssessmentAuditSnapshots(ctx, allowedIDs)
		if err != nil {
			return nil, err
		}

		if len(allowedIDs) == 1 {
			if err := s.Repo.ApplyValidatorDecision(ctx, allowedIDs[0], user.ID, decision, input.Score, input.Notes); err != nil {
				return nil, err
			}
		} else {
			if decision != GaspolValidationAgree || input.Score != nil || strings.TrimSpace(input.Notes) != "" {
				return nil, errors.New("INVALID_PAYLOAD")
			}
			if err := s.Repo.ValidateAssessments(ctx, allowedIDs, user.ID); err != nil {
				return nil, err
			}
		}

		if err := s.refreshRankingCascade(ctx, year, user.ID); err != nil {
			return nil, err
		}
		rows, err := s.Leaderboard(ctx, year, 0, "")
		if err != nil {
			return nil, err
		}
		for _, snapshot := range auditSnapshots {
			beforeScore := snapshot.ScoreMonthly
			if snapshot.SupervisorScore != nil {
				beforeScore = *snapshot.SupervisorScore
			}
			afterScore := beforeScore
			if decision == GaspolValidationObject && input.Score != nil {
				afterScore = *input.Score
			}
			assessmentID, schoolID := snapshot.ID, snapshot.SchoolID
			_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "VALIDATE_ASSESSMENT", "gaspol_assessments", &assessmentID, &schoolID, map[string]any{
				"year": year, "month": snapshot.Month, "schoolId": schoolID, "decision": decision,
				"beforeScore": beforeScore, "afterScore": afterScore, "validatorScore": input.Score, "notes": strings.TrimSpace(input.Notes),
			})
		}
		return rows, nil
	*/
}

func (s GaspolService) AdminDashboard(ctx context.Context, user auth.User, year int) (GaspolAdminDashboard, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolAdminDashboard{}, err
	}
	year = normalizeGaspolYear(year)
	isActive, err := s.Repo.IsYearActive(ctx, year)
	if err != nil {
		return GaspolAdminDashboard{}, err
	}
	participants, err := s.Repo.ListParticipants(ctx, year)
	if err != nil {
		return GaspolAdminDashboard{}, err
	}
	supervisorCount, err := s.Repo.CountDistinctRoles(ctx, year, GaspolRoleSupervisor)
	if err != nil {
		return GaspolAdminDashboard{}, err
	}
	validatorCount, err := s.Repo.CountDistinctRoles(ctx, year, GaspolRoleValidator)
	if err != nil {
		return GaspolAdminDashboard{}, err
	}
	juryCount, err := s.Repo.CountDistinctRoles(ctx, year, GaspolRoleJury)
	if err != nil {
		return GaspolAdminDashboard{}, err
	}
	pendingValidation, err := s.Repo.CountPendingValidations(ctx, year)
	if err != nil {
		return GaspolAdminDashboard{}, err
	}
	unassignedCabdin, err := s.CountUnassignedCabdinAssessments(ctx, year)
	if err != nil {
		return GaspolAdminDashboard{}, err
	}
	latestMonth := gaspolStageMonth(GaspolStageMonthly)
	latestStage := GaspolStageMonthly

	topTen, rankingStatus, err := s.buildRanking(ctx, year, gaspolStageMonth(GaspolStageMonthly), GaspolStageMonthly)
	if err != nil {
		return GaspolAdminDashboard{}, err
	}
	topFive, _, err := s.buildRanking(ctx, year, gaspolStageMonth(GaspolStageField), GaspolStageField)
	if err != nil {
		for _, item := range []struct {
			kind, stage string
			target      *[]GaspolRankingRow
		}{{"TOP_10", GaspolStageField1, &topTen}, {"TOP_5", GaspolStageField2, &topFive}} {
			official, e := gaspolOfficialCandidates(ctx, s.DB.Pool, year, item.kind, false)
			if e != nil {
				return GaspolAdminDashboard{}, e
			}
			if len(official) > 0 {
				snapshot := []GaspolRankingRow{}
				for _, c := range official {
					snapshot = append(snapshot, GaspolRankingRow{Rank: c.Rank, Year: year, Stage: item.stage, SchoolID: c.SchoolID, SchoolName: c.SchoolName, Cohort: item.kind, MonthlyFinal: true, MonthlyComponents: completeMonthlyComponentsForRanking(c.MonthlyScore), Scores: gaspolCompetitionScore(c, "MAIN", gaspolCandidateCompleteness{HasField1: true, HasField2: item.kind == "TOP_5"})})
				}
				*item.target = snapshot
			}
		}
		return GaspolAdminDashboard{}, err
	}
	return GaspolAdminDashboard{
		TopTenReady:                 gaspolTopTenReady(topTen),
		Year:                        year,
		Month:                       latestMonth,
		Stage:                       latestStage,
		IsYearActive:                isActive,
		ParticipantCount:            len(participants),
		SupervisorCount:             supervisorCount,
		ValidatorCount:              validatorCount,
		JuryCount:                   juryCount,
		PendingValidation:           pendingValidation,
		UnassignedCabdinAssessments: unassignedCabdin,
		RankingStatus:               rankingStatus,
		TopTen:                      ensureGaspolRankingRows(SelectGaspolTopTen(topTen)),
		TopFive:                     ensureGaspolRankingRows(SelectGaspolTopFive(topFive)),
	}, nil
}

func (s GaspolService) AdminSettingsOverview(ctx context.Context, user auth.User, year int) (GaspolAdminSettingsOverview, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	year = normalizeGaspolYear(year)

	summary, err := s.AdminDashboard(ctx, user, year)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	participants, err := s.Repo.ListAllSchoolsWithParticipantState(ctx, year)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	schoolUsers, err := s.listSchoolUsers(ctx, year)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	supervisorCandidates, err := s.Repo.ListAssignableUsers(ctx, year, GaspolRoleSupervisor)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	validatorCandidates, err := s.Repo.ListAssignableUsers(ctx, year, GaspolRoleValidator)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	koorCandidates := append([]GaspolRoleCandidate(nil), supervisorCandidates...)
	koorAssignments, err := s.Repo.ListStageAssignments(ctx, year, "KOOR_GASPOL")
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	stageAssignments, err := s.Repo.ListStageAssignments(ctx, year, "")
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	stageSchoolCandidates, err := s.Repo.ListStageSchoolCandidates(ctx, year)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	internalCandidates, err := s.Repo.ListInternalStageCandidates(ctx, year)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	juryAccounts, err := s.Repo.ListAssignedRoleUsers(ctx, year, GaspolRoleJury)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	schoolMonitoring, err := s.buildDetailedSchoolMonitoringRows(ctx, year)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	supervisorMonitoring, err := s.buildDetailedSupervisorMonitoringRows(ctx, year)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}
	auditLogs, err := s.Repo.ListAuditLogs(ctx, year, 200)
	if err != nil {
		return GaspolAdminSettingsOverview{}, err
	}

	supervisorAssignmentCount := map[int]int{}
	for _, row := range supervisorMonitoring {
		if row.SupervisorUserID != nil {
			supervisorAssignmentCount[*row.SupervisorUserID]++
		}
	}
	validatorSchoolCount := map[int]int{}
	for _, item := range participants {
		if item.IsActive && item.ValidatorUserID != nil {
			validatorSchoolCount[*item.ValidatorUserID]++
		}
	}
	for index := range supervisorCandidates {
		supervisorCandidates[index].AssignmentCount = supervisorAssignmentCount[supervisorCandidates[index].UserID]
	}
	for index := range validatorCandidates {
		validatorCandidates[index].AssignedSchoolCount = validatorSchoolCount[validatorCandidates[index].UserID]
	}

	return GaspolAdminSettingsOverview{
		Year:                  year,
		Summary:               summary,
		Participants:          participants,
		SchoolUsers:           schoolUsers,
		SupervisorCandidates:  supervisorCandidates,
		ValidatorCandidates:   validatorCandidates,
		KoorCandidates:        koorCandidates,
		KoorAssignments:       koorAssignments,
		StageAssignments:      stageAssignments,
		StageSchoolCandidates: stageSchoolCandidates,
		InternalCandidates:    internalCandidates,
		JuryAccounts:          juryAccounts,
		SchoolMonitoring:      schoolMonitoring,
		SupervisorMonitoring:  supervisorMonitoring,
		AuditLogs:             auditLogs,
	}, nil
}

func (s GaspolService) buildDetailedSchoolMonitoringRows(ctx context.Context, year int) ([]GaspolSchoolMonitoringStatusRow, error) {
	participants, err := s.Repo.ListParticipants(ctx, year)
	if err != nil {
		return nil, err
	}
	effectiveNow, err := s.Clock.Now(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]GaspolSchoolMonitoringStatusRow, 0)
	for _, participant := range participants {
		if !participant.IsActive || participant.SchoolID == nil {
			continue
		}
		submissions, err := s.Repo.ListSchoolSubmissions(ctx, year, *participant.SchoolID)
		if err != nil {
			return nil, err
		}
		submittedByMonth := map[int]bool{}
		for _, submission := range submissions {
			if submission.Status == GaspolSubmissionSubmitted {
				submittedByMonth[submission.Month] = true
			}
		}
		for month := 4; month <= 11; month++ {
			mengisi := "Tidak"
			if submittedByMonth[month] {
				mengisi = "Sudah"
			} else if CanEditGaspolMonthlyEntry(year, month, effectiveNow) {
				mengisi = "Belum"
			}
			rows = append(rows, GaspolSchoolMonitoringStatusRow{
				SchoolID:     *participant.SchoolID,
				NPSN:         participant.NPSN,
				SchoolName:   participant.SchoolName,
				SchoolLevel:  participant.SchoolLevel,
				SchoolStatus: participant.SchoolStatus,
				Month:        month,
				Mengisi:      mengisi,
			})
		}
	}
	return rows, nil
}

func (s GaspolService) buildDetailedSupervisorMonitoringRows(ctx context.Context, year int) ([]GaspolSupervisorMonitoringStatusRow, error) {
	rows := make([]GaspolSupervisorMonitoringStatusRow, 0)
	for month := 4; month <= 9; month++ {
		assignments, err := s.Repo.ListAssignments(ctx, year, month)
		if err != nil {
			return nil, err
		}
		assessments, err := s.Repo.ListStageAssessments(ctx, year, month, GaspolStageMonthly)
		if err != nil {
			return nil, err
		}
		submittedBySchoolID := map[int]bool{}
		for _, assessment := range assessments {
			submittedBySchoolID[assessment.SchoolID] = true
		}
		for _, assignment := range assignments {
			supervisorName := strings.TrimSpace(assignment.SupervisorNameSnapshot)
			if supervisorName == "" {
				supervisorName = "Belum ditugaskan"
			}
			hasSubmitted := submittedBySchoolID[assignment.SchoolID]
			rows = append(rows, GaspolSupervisorMonitoringStatusRow{
				AssignmentID:      assignment.ID,
				SchoolID:          assignment.SchoolID,
				NPSN:              assignment.NPSN,
				SchoolName:        assignment.SchoolName,
				Month:             assignment.Month,
				SupervisorUserID:  assignment.SupervisorUserID,
				SupervisorName:    supervisorName,
				Method:            assignment.Method,
				Penilaian:         map[bool]string{true: "Sudah", false: "Belum"}[hasSubmitted],
				HasSubmittedScore: hasSubmitted,
			})
		}
	}
	return rows, nil
}

func (s GaspolService) ActivateYear(ctx context.Context, user auth.User, year int) (GaspolParticipantUpsertResult, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolParticipantUpsertResult{}, err
	}
	year = normalizeGaspolYear(year)
	if err := s.Repo.EnsureYearActive(ctx, year, user.ID); err != nil {
		return GaspolParticipantUpsertResult{}, err
	}
	participants, err := s.Repo.ListParticipants(ctx, year)
	if err != nil {
		return GaspolParticipantUpsertResult{}, err
	}
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "ACTIVATE_YEAR", "gaspol_school_participants", nil, nil, map[string]int{"year": year})
	return GaspolParticipantUpsertResult{
		Year:         year,
		IsYearActive: true,
		Participants: participants,
	}, nil
}

func (s GaspolService) SaveParticipants(ctx context.Context, user auth.User, input GaspolParticipantUpdateInput) (GaspolParticipantUpsertResult, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolParticipantUpsertResult{}, err
	}
	year := normalizeGaspolYear(input.Year)
	schoolIDs := dedupeIntSlice(input.SchoolIDs)
	if err := s.Repo.UpsertParticipants(ctx, year, schoolIDs, user.ID); err != nil {
		return GaspolParticipantUpsertResult{}, err
	}
	participants, err := s.Repo.ListParticipants(ctx, year)
	if err != nil {
		return GaspolParticipantUpsertResult{}, err
	}
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "UPSERT_PARTICIPANTS", "gaspol_school_participants", nil, nil, schoolIDs)
	return GaspolParticipantUpsertResult{
		Year:         year,
		IsYearActive: true,
		Participants: participants,
	}, nil
}

func (s GaspolService) GenerateSchoolUsers(ctx context.Context, user auth.User, input GaspolGenerateUsersInput) (GaspolGenerateUsersResult, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolGenerateUsersResult{}, err
	}
	year := normalizeGaspolYear(input.Year)
	participants, err := s.Repo.ListParticipants(ctx, year)
	if err != nil {
		return GaspolGenerateUsersResult{}, err
	}
	support, err := s.initialPasswordSupport(ctx)
	if err != nil {
		return GaspolGenerateUsersResult{}, err
	}

	filterIDs := dedupeIntSlice(input.SchoolIDs)
	filter := map[int]struct{}{}
	for _, schoolID := range filterIDs {
		filter[schoolID] = struct{}{}
	}

	result := GaspolGenerateUsersResult{
		Year:    year,
		Created: []GaspolGeneratedCredential{},
		Skipped: []GaspolGeneratedCredential{},
	}

	for _, participant := range participants {
		if !participant.IsActive || participant.SchoolID == nil {
			continue
		}
		if len(filter) > 0 {
			if _, ok := filter[*participant.SchoolID]; !ok {
				continue
			}
		}
		if participant.SchoolEntryUserID != nil {
			existing, err := s.Repo.GetUser(ctx, *participant.SchoolEntryUserID)
			if err == nil {
				existing.Password = ""
				result.Skipped = append(result.Skipped, existing)
				continue
			}
		}

		password, err := utils.GenerateStrongPassword(10)
		if err != nil {
			return GaspolGenerateUsersResult{}, err
		}
		passwordHash, err := utils.HashPassword(password)
		if err != nil {
			return GaspolGenerateUsersResult{}, err
		}
		initialPassword, err := s.buildInitialPasswordRecord(support, password)
		if err != nil {
			return GaspolGenerateUsersResult{}, err
		}

		base := participant.NPSN
		if base == "" {
			base = fmt.Sprintf("school-%d", *participant.SchoolID)
		}
		base = "gaspol-" + base
		var createdUserID int
		for suffix := 0; suffix < 20; suffix++ {
			username := s.Repo.BuildUsernameCandidate(base, suffix)
			existingID, err := s.Repo.GetUserIDByUsername(ctx, username)
			if err != nil {
				return GaspolGenerateUsersResult{}, err
			}
			if existingID != nil {
				continue
			}
			createdUserID, err = s.Repo.CreateGaspolSchoolUser(ctx, username, passwordHash, auth.RoleGaspolSchoolUser, participant.SchoolID, "GASPOL "+participant.SchoolName, initialPassword)
			if err == nil {
				break
			}
			if suffix == 19 {
				return GaspolGenerateUsersResult{}, err
			}
		}
		if createdUserID == 0 {
			continue
		}
		if _, err := s.Repo.UpsertUserRole(ctx, year, createdUserID, GaspolRoleSchoolEntry, participant.SchoolID, user.ID); err != nil {
			return GaspolGenerateUsersResult{}, err
		}
		if err := s.Repo.SetParticipantSchoolEntryUser(ctx, year, *participant.SchoolID, createdUserID); err != nil {
			return GaspolGenerateUsersResult{}, err
		}
		created, err := s.Repo.GetUser(ctx, createdUserID)
		if err != nil {
			return GaspolGenerateUsersResult{}, err
		}
		created.Password = password
		result.Created = append(result.Created, created)
	}

	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "GENERATE_SCHOOL_USERS", "users", nil, nil, result)
	return result, nil
}

func (s GaspolService) ResetGaspolUserPassword(ctx context.Context, user auth.User, input GaspolResetPasswordInput) (GaspolGeneratedCredential, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolGeneratedCredential{}, err
	}
	if input.UserID <= 0 {
		return GaspolGeneratedCredential{}, errors.New("INVALID_USER_ID")
	}
	hasRole, err := s.Repo.UserHasAnyRole(ctx, input.UserID, []string{GaspolRoleSchoolEntry, GaspolRoleJury})
	if err != nil {
		return GaspolGeneratedCredential{}, err
	}
	if !hasRole {
		return GaspolGeneratedCredential{}, errors.New("USER_NOT_RESETTABLE")
	}

	password := strings.TrimSpace(input.NewPassword)
	if password == "" {
		password, err = utils.GenerateStrongPassword(10)
		if err != nil {
			return GaspolGeneratedCredential{}, err
		}
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return GaspolGeneratedCredential{}, err
	}
	if err := s.Repo.ResetUserPassword(ctx, input.UserID, hash); err != nil {
		return GaspolGeneratedCredential{}, err
	}
	meta, err := s.Repo.GetUser(ctx, input.UserID)
	if err != nil {
		return GaspolGeneratedCredential{}, err
	}
	meta.Password = password
	_ = s.Repo.LogAudit(ctx, nil, user.ID, user.Role, "RESET_GASPOL_PASSWORD", "users", &input.UserID, meta.SchoolID, nil)
	return meta, nil
}

func (s GaspolService) AssignRole(ctx context.Context, user auth.User, input GaspolAssignmentRoleInput, role string) ([]GaspolUserRoleAssignment, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return nil, err
	}
	if err := validateGaspolRole(role); err != nil {
		return nil, err
	}
	year := normalizeGaspolYear(input.Year)
	userIDs := dedupeIntSlice(input.UserIDs)
	result := make([]GaspolUserRoleAssignment, 0, len(userIDs))
	for _, userID := range userIDs {
		if _, err := s.ensureAssignableGaspolUser(ctx, userID, role); err != nil {
			return nil, err
		}
		item, err := s.Repo.UpsertUserRole(ctx, year, userID, role, nil, user.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "ASSIGN_"+role, "gaspol_user_roles", nil, nil, userIDs)
	return result, nil
}

func (s GaspolService) SetRoleStatus(ctx context.Context, user auth.User, input GaspolRoleStatusInput) (GaspolUserRoleAssignment, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolUserRoleAssignment{}, err
	}
	year := normalizeGaspolYear(input.Year)
	role := strings.ToUpper(strings.TrimSpace(input.Role))
	if input.UserID <= 0 {
		return GaspolUserRoleAssignment{}, errors.New("INVALID_USER_ID")
	}
	if role != GaspolRoleSupervisor && role != GaspolRoleValidator && role != GaspolRoleJury {
		return GaspolUserRoleAssignment{}, errors.New("INVALID_ROLE")
	}

	if input.IsActive {
		if _, err := s.ensureAssignableGaspolUser(ctx, input.UserID, role); err != nil {
			return GaspolUserRoleAssignment{}, err
		}
		item, err := s.Repo.UpsertUserRole(ctx, year, input.UserID, role, nil, user.ID)
		if err != nil {
			return GaspolUserRoleAssignment{}, err
		}
		_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "SET_ROLE_ACTIVE_"+role, "gaspol_user_roles", &item.ID, nil, input)
		return item, nil
	}

	err := s.Repo.SetUserRoleActive(ctx, year, input.UserID, role, false)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return GaspolUserRoleAssignment{}, err
	}
	meta, metaErr := s.Repo.GetUserMeta(ctx, input.UserID)
	if metaErr != nil {
		return GaspolUserRoleAssignment{}, metaErr
	}
	item := GaspolUserRoleAssignment{
		Year:        year,
		UserID:      input.UserID,
		Role:        role,
		IsActive:    false,
		Username:    meta.Username,
		DisplayName: meta.DisplayName,
		BaseRole:    meta.BaseRole,
	}
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "SET_ROLE_INACTIVE_"+role, "gaspol_user_roles", nil, nil, input)
	return item, nil
}

func (s GaspolService) AssignValidator(ctx context.Context, user auth.User, input GaspolValidatorAssignInput) (GaspolSchoolParticipant, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolSchoolParticipant{}, err
	}
	year := normalizeGaspolYear(input.Year)
	if input.SchoolID <= 0 || input.ValidatorUserID <= 0 {
		return GaspolSchoolParticipant{}, errors.New("INVALID_PAYLOAD")
	}
	if _, err := s.ensureAssignableGaspolUser(ctx, input.ValidatorUserID, GaspolRoleValidator); err != nil {
		return GaspolSchoolParticipant{}, err
	}
	if _, err := s.Repo.UpsertUserRole(ctx, year, input.ValidatorUserID, GaspolRoleValidator, nil, user.ID); err != nil {
		return GaspolSchoolParticipant{}, err
	}
	if err := s.Repo.SetParticipantValidator(ctx, year, input.SchoolID, input.ValidatorUserID); err != nil {
		return GaspolSchoolParticipant{}, err
	}
	item, err := s.Repo.GetParticipantBySchool(ctx, year, input.SchoolID)
	if err != nil {
		return GaspolSchoolParticipant{}, err
	}
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "ASSIGN_VALIDATOR", "gaspol_school_participants", &item.ID, &input.SchoolID, input)
	return item, nil
}

func (s GaspolService) AssignValidatorAll(ctx context.Context, user auth.User, input GaspolBulkValidatorAssignInput) (GaspolBulkValidatorAssignResult, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolBulkValidatorAssignResult{}, err
	}
	year := normalizeGaspolYear(input.Year)
	if input.ValidatorUserID <= 0 {
		return GaspolBulkValidatorAssignResult{}, errors.New("INVALID_PAYLOAD")
	}
	if _, err := s.ensureAssignableGaspolUser(ctx, input.ValidatorUserID, GaspolRoleValidator); err != nil {
		return GaspolBulkValidatorAssignResult{}, err
	}
	if _, err := s.Repo.UpsertUserRole(ctx, year, input.ValidatorUserID, GaspolRoleValidator, nil, user.ID); err != nil {
		return GaspolBulkValidatorAssignResult{}, err
	}
	updatedCount, err := s.Repo.SetAllParticipantValidators(ctx, year, input.ValidatorUserID)
	if err != nil {
		return GaspolBulkValidatorAssignResult{}, err
	}
	result := GaspolBulkValidatorAssignResult{
		Year:            year,
		ValidatorUserID: input.ValidatorUserID,
		UpdatedCount:    updatedCount,
	}
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "ASSIGN_VALIDATOR_ALL", "gaspol_school_participants", nil, nil, result)
	return result, nil
}

func (s GaspolService) ManualAssignment(ctx context.Context, user auth.User, input GaspolManualAssignmentInput) (GaspolAssignment, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolAssignment{}, err
	}
	year := normalizeGaspolYear(input.Year)
	month := normalizeGaspolMonth(input.Month)
	if month < 4 || month > 9 {
		return GaspolAssignment{}, errors.New("INVALID_MONTH")
	}
	if input.SchoolID <= 0 || input.SupervisorUserID <= 0 {
		return GaspolAssignment{}, errors.New("INVALID_PAYLOAD")
	}
	meta, err := s.ensureAssignableGaspolUser(ctx, input.SupervisorUserID, GaspolRoleSupervisor)
	if err != nil {
		return GaspolAssignment{}, err
	}
	locked, err := s.Repo.HasSubmittedSupervisorAssessment(ctx, year, month, input.SchoolID)
	if err != nil {
		return GaspolAssignment{}, err
	}
	if locked {
		return GaspolAssignment{}, errors.New("ASSIGNMENT_LOCKED")
	}
	if _, err := s.Repo.UpsertUserRole(ctx, year, input.SupervisorUserID, GaspolRoleSupervisor, nil, user.ID); err != nil {
		return GaspolAssignment{}, err
	}
	item, err := s.Repo.UpsertAssignment(ctx, year, month, input.SchoolID, &input.SupervisorUserID, meta.DisplayName, GaspolAssignmentManual, user.ID)
	if err != nil {
		return GaspolAssignment{}, err
	}
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "MANUAL_ASSIGNMENT", "gaspol_assignments", &item.ID, &input.SchoolID, input)
	return item, nil
}

func (s GaspolService) RandomAssignments(ctx context.Context, user auth.User, year int, month int) (GaspolAssignmentGenerateResult, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolAssignmentGenerateResult{}, err
	}
	year = normalizeGaspolYear(year)
	month = normalizeGaspolMonth(month)
	if month < 4 || month > 9 {
		return GaspolAssignmentGenerateResult{}, errors.New("INVALID_MONTH")
	}
	schools, supervisors, err := s.Repo.ListRandomAssignmentInputs(ctx, year, month)
	if err != nil {
		return GaspolAssignmentGenerateResult{}, err
	}
	existingAssignments, err := s.Repo.ListAssignments(ctx, year, month)
	if err != nil {
		return GaspolAssignmentGenerateResult{}, err
	}
	existingBySchoolID := make(map[int]GaspolAssignment, len(existingAssignments))
	for _, item := range existingAssignments {
		existingBySchoolID[item.SchoolID] = item
	}
	lockedSchools := make(map[int]bool)
	lockedLoads := make(map[int]int)
	mutableSchools := make([]GaspolAssignmentCandidate, 0, len(schools))
	for _, school := range schools {
		locked, lockErr := s.Repo.HasSubmittedSupervisorAssessment(ctx, year, month, school.SchoolID)
		if lockErr != nil {
			return GaspolAssignmentGenerateResult{}, lockErr
		}
		if locked {
			lockedSchools[school.SchoolID] = true
			if existing, exists := existingBySchoolID[school.SchoolID]; exists && existing.SupervisorUserID != nil {
				lockedLoads[*existing.SupervisorUserID]++
			}
			continue
		}
		mutableSchools = append(mutableSchools, school)
	}
	for index := range supervisors {
		supervisors[index].CurrentLoad = lockedLoads[supervisors[index].UserID]
	}
	plans := BuildGaspolRandomAssignments(year, month, mutableSchools, supervisors)
	assignments := make([]GaspolAssignment, 0, len(plans))
	unassigned := make([]int, 0)
	for schoolID := range lockedSchools {
		if existing, ok := existingBySchoolID[schoolID]; ok {
			assignments = append(assignments, existing)
			if existing.SupervisorUserID == nil {
				unassigned = append(unassigned, existing.SchoolID)
			}
		}
	}
	for _, plan := range plans {
		item, err := s.Repo.UpsertAssignment(ctx, year, month, plan.SchoolID, plan.SupervisorUserID, plan.SupervisorNameSnapshot, plan.Method, user.ID)
		if err != nil {
			return GaspolAssignmentGenerateResult{}, err
		}
		assignments = append(assignments, item)
		if plan.SupervisorUserID == nil {
			unassigned = append(unassigned, plan.SchoolID)
		}
	}
	result := GaspolAssignmentGenerateResult{
		Year:                year,
		Month:               month,
		Assignments:         ensureGaspolAssignments(assignments),
		UnassignedSchoolIDs: unassigned,
	}
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "RANDOM_ASSIGNMENT", "gaspol_assignments", nil, nil, result)
	return result, nil
}

func (s GaspolService) GenerateJuryAccount(ctx context.Context, user auth.User, input GaspolJuryAccountInput) (GaspolGeneratedCredential, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolGeneratedCredential{}, err
	}
	year := normalizeGaspolYear(input.Year)
	username := strings.TrimSpace(input.Username)
	if username == "" {
		username = "gaspol-jury"
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = "Gaspol Jury"
	}

	password, err := utils.GenerateStrongPassword(10)
	if err != nil {
		return GaspolGeneratedCredential{}, err
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return GaspolGeneratedCredential{}, err
	}

	existingID, err := s.Repo.GetUserIDByUsername(ctx, username)
	if err != nil {
		return GaspolGeneratedCredential{}, err
	}
	var userID int
	if existingID == nil {
		userID, err = s.Repo.CreateGaspolUser(ctx, username, hash, auth.RoleSchoolAdmin, nil, displayName)
		if err != nil {
			return GaspolGeneratedCredential{}, err
		}
	} else {
		userID = *existingID
		if _, err := s.ensureReusableJuryAccount(ctx, userID); err != nil {
			return GaspolGeneratedCredential{}, err
		}
		if err := s.Repo.ResetUserPassword(ctx, userID, hash); err != nil {
			return GaspolGeneratedCredential{}, err
		}
	}
	if _, err := s.Repo.UpsertUserRole(ctx, year, userID, GaspolRoleJury, nil, user.ID); err != nil {
		return GaspolGeneratedCredential{}, err
	}
	meta, err := s.Repo.GetUser(ctx, userID)
	if err != nil {
		return GaspolGeneratedCredential{}, err
	}
	meta.Password = password
	_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "GENERATE_JURY_ACCOUNT", "users", &userID, nil, map[string]string{"username": username})
	return meta, nil
}

func (s GaspolService) Leaderboard(ctx context.Context, year int, month int, stage string) ([]GaspolRankingRow, error) {
	year = normalizeGaspolYear(year)
	if month <= 0 && strings.TrimSpace(stage) == "" {
		stage = GaspolStageMonthly
		month = gaspolStageMonth(stage)
	}
	stage = normalizeGaspolStage(stage, month)
	if month <= 0 {
		month = gaspolStageMonth(stage)
	}
	rows, _, _, err := s.getLeaderboard(ctx, year, month, stage)
	if err != nil {
		return nil, err
	}
	return ensureGaspolRankingRows(rows), nil
}

func (s GaspolService) BestSchool(ctx context.Context, year int) (*GaspolRankingRow, error) {
	rows, err := s.Leaderboard(ctx, year, gaspolStageMonth(GaspolStagePresentation), GaspolStagePresentation)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		rows, err = s.Leaderboard(ctx, year, 0, "")
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	item := rows[0]
	return &item, nil
}

func (s GaspolService) MonitoringSchools(ctx context.Context, user auth.User, year int) ([]GaspolSchoolMonitoringRow, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return nil, err
	}
	year = normalizeGaspolYear(year)
	schools, err := s.Repo.ListParticipants(ctx, year)
	if err != nil {
		return nil, err
	}
	leaderboard, err := s.Leaderboard(ctx, year, 0, "")
	if err != nil {
		return nil, err
	}
	rankingMap := map[int]GaspolRankingRow{}
	for _, row := range leaderboard {
		rankingMap[row.SchoolID] = row
	}
	result := make([]GaspolSchoolMonitoringRow, 0, len(schools))
	for _, school := range schools {
		if school.SchoolID == nil || !school.IsActive {
			continue
		}
		submissions, err := s.Repo.ListSchoolSubmissions(ctx, year, *school.SchoolID)
		if err != nil {
			return nil, err
		}
		var latest *GaspolSchoolSubmission
		if len(submissions) > 0 {
			copySubmission := submissions[0]
			latest = &copySubmission
		}
		result = append(result, GaspolSchoolMonitoringRow{
			SchoolID:           *school.SchoolID,
			SchoolName:         school.SchoolName,
			NPSN:               school.NPSN,
			SchoolLevel:        school.SchoolLevel,
			SchoolStatus:       school.SchoolStatus,
			IsParticipant:      school.IsActive,
			AssignedSupervisor: rankingMap[*school.SchoolID].SupervisorNameSnapshot,
			AssignedValidator:  school.ValidatorName,
			LastSubmission:     latest,
			Scores:             rankingMap[*school.SchoolID].Scores,
		})
	}
	return result, nil
}

func (s GaspolService) MonitoringSupervisors(ctx context.Context, user auth.User, year int, month int) ([]GaspolSupervisorMonitoringRow, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return nil, err
	}
	year = normalizeGaspolYear(year)
	month = normalizeGaspolMonth(month)
	supervisors, err := s.Repo.ListRoleAssignments(ctx, year, GaspolRoleSupervisor)
	if err != nil {
		return nil, err
	}
	result := make([]GaspolSupervisorMonitoringRow, 0, len(supervisors))
	for _, supervisor := range supervisors {
		assignments, err := s.Repo.ListAssignmentsBySupervisor(ctx, year, month, supervisor.UserID)
		if err != nil {
			return nil, err
		}
		assessments, err := s.Repo.ListSupervisorAssessments(ctx, year, month, supervisor.UserID)
		if err != nil {
			return nil, err
		}
		completed := 0
		for _, item := range assessments {
			if item.Stage == GaspolStageMonthly {
				completed++
			}
		}
		result = append(result, GaspolSupervisorMonitoringRow{
			UserID:          supervisor.UserID,
			Username:        supervisor.Username,
			DisplayName:     supervisor.DisplayName,
			BaseRole:        supervisor.BaseRole,
			AssignmentCount: len(assignments),
			CompletedCount:  completed,
			Schools:         ensureGaspolAssignments(assignments),
		})
	}
	return result, nil
}

func (s GaspolService) BuildSchoolMonitoringExportRows(ctx context.Context, user auth.User, year int) ([]GaspolSchoolMonitoringExportRow, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return nil, err
	}
	year = normalizeGaspolYear(year)

	participants, err := s.Repo.ListParticipants(ctx, year)
	if err != nil {
		return nil, err
	}

	rows := make([]GaspolSchoolMonitoringExportRow, 0)
	for _, participant := range participants {
		if !participant.IsActive || participant.SchoolID == nil {
			continue
		}
		submissions, err := s.Repo.ListSchoolSubmissions(ctx, year, *participant.SchoolID)
		if err != nil {
			return nil, err
		}
		submittedByMonth := make(map[int]bool, len(submissions))
		for _, submission := range submissions {
			submittedByMonth[submission.Month] = true
		}
		for month := 4; month <= 11; month++ {
			rows = append(rows, GaspolSchoolMonitoringExportRow{
				NPSN:       participant.NPSN,
				SchoolName: participant.SchoolName,
				Month:      month,
				Mengisi:    boolToSudahTidak(submittedByMonth[month]),
			})
		}
	}
	return rows, nil
}

func (s GaspolService) BuildSupervisorMonitoringExportRows(ctx context.Context, user auth.User, year int) ([]GaspolSupervisorMonitoringExportRow, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return nil, err
	}
	year = normalizeGaspolYear(year)

	rows := make([]GaspolSupervisorMonitoringExportRow, 0)
	for month := 4; month <= 9; month++ {
		assignments, err := s.Repo.ListAssignments(ctx, year, month)
		if err != nil {
			return nil, err
		}
		assessments, err := s.Repo.ListStageAssessments(ctx, year, month, GaspolStageMonthly)
		if err != nil {
			return nil, err
		}
		assessedKeys := make(map[string]struct{}, len(assessments))
		for _, assessment := range assessments {
			key := fmt.Sprintf("%d:%d:%d", assessment.AssessorUserID, assessment.SchoolID, assessment.Month)
			assessedKeys[key] = struct{}{}
		}
		for _, assignment := range assignments {
			supervisorName := strings.TrimSpace(assignment.SupervisorNameSnapshot)
			if supervisorName == "" {
				supervisorName = "BELUM_DITUGASKAN"
			}
			status := "Belum"
			if assignment.SupervisorUserID != nil {
				key := fmt.Sprintf("%d:%d:%d", *assignment.SupervisorUserID, assignment.SchoolID, assignment.Month)
				if _, ok := assessedKeys[key]; ok {
					status = "Sudah"
				}
			}
			rows = append(rows, GaspolSupervisorMonitoringExportRow{
				SupervisorName: supervisorName,
				SchoolName:     assignment.SchoolName,
				Month:          assignment.Month,
				Penilaian:      status,
			})
		}
	}
	return rows, nil
}

func boolToSudahTidak(value bool) string {
	if value {
		return "Sudah"
	}
	return "Tidak"
}

func (s GaspolService) ExportBundle(ctx context.Context, user auth.User, year int, month int) (GaspolExportBundle, error) {
	summary, err := s.AdminDashboard(ctx, user, year)
	if err != nil {
		return GaspolExportBundle{}, err
	}
	schools, err := s.MonitoringSchools(ctx, user, year)
	if err != nil {
		return GaspolExportBundle{}, err
	}
	supervisors, err := s.MonitoringSupervisors(ctx, user, year, month)
	if err != nil {
		return GaspolExportBundle{}, err
	}
	leaderboard, err := s.Leaderboard(ctx, year, month, "")
	if err != nil {
		return GaspolExportBundle{}, err
	}
	schoolExportRows, err := s.BuildSchoolMonitoringExportRows(ctx, user, year)
	if err != nil {
		return GaspolExportBundle{}, err
	}
	supervisorExportRows, err := s.BuildSupervisorMonitoringExportRows(ctx, user, year)
	if err != nil {
		return GaspolExportBundle{}, err
	}
	return GaspolExportBundle{
		Summary:                    summary,
		SchoolMonitoring:           schools,
		SupervisorMonitoring:       supervisors,
		SchoolMonitoringExportRows: ensureGaspolSchoolMonitoringExportRows(schoolExportRows),
		SupervisorExportRows:       ensureGaspolSupervisorExportRows(supervisorExportRows),
		Leaderboard:                leaderboard,
	}, nil
}

func (s GaspolService) SchoolUserExportRows(ctx context.Context, user auth.User, year int) ([]GaspolSchoolUserRow, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return nil, err
	}
	year = normalizeGaspolYear(year)

	rows, err := s.listSchoolUsers(ctx, year)
	if err != nil {
		return nil, err
	}

	result := make([]GaspolSchoolUserRow, 0, len(rows))
	for _, row := range rows {
		if !row.IsParticipant {
			continue
		}
		initialPassword, err := s.decryptInitialPassword(row)
		if err != nil {
			return nil, err
		}
		row.InitialPassword = initialPassword
		result = append(result, row)
	}

	return result, nil
}
