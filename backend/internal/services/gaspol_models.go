package services

import "time"

const (
	GaspolRoleSchoolEntry = "SCHOOL_ENTRY"
	GaspolRoleSupervisor  = "SUPERVISOR"
	GaspolRoleValidator   = "VALIDATOR"
	GaspolRoleJury        = "JURY"
	GaspolRoleAdminMaster = "ADMIN_MASTER"

	GaspolStageMonthly      = "MONTHLY"
	GaspolStageField        = "FIELD"
	GaspolStageField1       = "FIELD_STAGE_1"
	GaspolStageField2       = "FIELD_STAGE_2"
	GaspolStagePresentation = "PRESENTATION"
	GaspolStageNomination   = "NOT_FAILED_NOMINATION"
	GaspolStageAppeal       = "BOTTOM5_APPEAL"
	GaspolStageFinal        = "FINAL"

	GaspolAssignmentManual         = "MANUAL"
	GaspolAssignmentRandom         = "RANDOM"
	GaspolAssignmentRandomFallback = "RANDOM_FALLBACK"
	GaspolAssignmentUnassigned     = "UNASSIGNED"

	GaspolSubmissionDraft     = "DRAFT"
	GaspolSubmissionSubmitted = "SUBMITTED"

	GaspolValidationAgree  = "AGREE"
	GaspolValidationObject = "OBJECT"

	GaspolMonthlyStatusValidated = "VALIDATED"
	GaspolMonthlyStatusWaiting   = "WAITING"
	GaspolMonthlyStatusZeroFinal = "ZERO_FINAL"
	GaspolMonthlyStatusNotYetDue = "NOT_YET_DUE"
)

var gaspolMutableSchoolRoles = map[string]struct{}{
	GaspolRoleSchoolEntry: {},
}

var gaspolAssignableRoles = map[string]struct{}{
	GaspolRoleSchoolEntry: {},
	GaspolRoleSupervisor:  {},
	GaspolRoleValidator:   {},
	GaspolRoleJury:        {},
	GaspolRoleAdminMaster: {},
}

type GaspolSchoolParticipant struct {
	ID                int        `json:"id"`
	Year              int        `json:"year"`
	SchoolID          *int       `json:"schoolId"`
	NPSN              string     `json:"npsn"`
	SchoolName        string     `json:"schoolName"`
	SchoolLevel       string     `json:"schoolLevel"`
	SchoolStatus      string     `json:"schoolStatus"`
	IsYearConfig      bool       `json:"isYearConfig"`
	IsActive          bool       `json:"isActive"`
	ValidatorUserID   *int       `json:"validatorUserId"`
	ValidatorName     string     `json:"validatorName"`
	SchoolEntryUserID *int       `json:"schoolEntryUserId"`
	SchoolEntryName   string     `json:"schoolEntryName"`
	Notes             string     `json:"notes"`
	CreatedAt         *time.Time `json:"createdAt,omitempty"`
	UpdatedAt         *time.Time `json:"updatedAt,omitempty"`
}

type GaspolUserRoleAssignment struct {
	ID          int    `json:"id"`
	Year        int    `json:"year"`
	UserID      int    `json:"userId"`
	Role        string `json:"role"`
	SchoolID    *int   `json:"schoolId,omitempty"`
	IsActive    bool   `json:"isActive"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	BaseRole    string `json:"baseRole"`
}

type GaspolAssignment struct {
	ID                     int        `json:"id"`
	Year                   int        `json:"year"`
	Month                  int        `json:"month"`
	SchoolID               int        `json:"schoolId"`
	SchoolName             string     `json:"schoolName"`
	NPSN                   string     `json:"npsn"`
	SupervisorUserID       *int       `json:"supervisorUserId"`
	SupervisorName         string     `json:"supervisorName"`
	SupervisorNameSnapshot string     `json:"supervisorNameSnapshot"`
	Method                 string     `json:"method"`
	CreatedAt              *time.Time `json:"createdAt,omitempty"`
	UpdatedAt              *time.Time `json:"updatedAt,omitempty"`
}

type GaspolSchoolSubmission struct {
	ID               int        `json:"id"`
	Year             int        `json:"year"`
	Month            int        `json:"month"`
	SchoolID         int        `json:"schoolId"`
	SchoolName       string     `json:"schoolName"`
	ReportTitle      string     `json:"reportTitle"`
	ReportBody       string     `json:"reportBody"`
	ReportLink       string     `json:"reportLink"`
	AttachmentURLs   []string   `json:"attachmentUrls"`
	Status           string     `json:"status"`
	DisplayStatus    string     `json:"displayStatus"`
	AssessmentStatus string     `json:"assessmentStatus"`
	SubmittedBy      *int       `json:"submittedBy,omitempty"`
	SubmittedAt      *time.Time `json:"submittedAt,omitempty"`
	UpdatedAt        *time.Time `json:"updatedAt,omitempty"`
}

type GaspolAssessment struct {
	ID                 int        `json:"id"`
	Year               int        `json:"year"`
	Month              int        `json:"month"`
	SchoolID           int        `json:"schoolId"`
	SchoolName         string     `json:"schoolName"`
	AssignmentID       *int       `json:"assignmentId,omitempty"`
	AssessorUserID     int        `json:"assessorUserId"`
	AssessorRole       string     `json:"assessorRole"`
	AssessorName       string     `json:"assessorName"`
	Stage              string     `json:"stage"`
	ScoreMonthly       float64    `json:"scoreMonthly"`
	ScoreField         float64    `json:"scoreField"`
	ScorePresentation  float64    `json:"scorePresentation"`
	ScoreTotal         float64    `json:"scoreTotal"`
	Notes              string     `json:"notes"`
	IsValidated        bool       `json:"isValidated"`
	ValidatedBy        *int       `json:"validatedBy,omitempty"`
	ValidatedAt        *time.Time `json:"validatedAt,omitempty"`
	ValidationDecision string     `json:"validationDecision,omitempty"`
	SupervisorScore    *float64   `json:"supervisorScore,omitempty"`
	ValidatorScore     *float64   `json:"validatorScore,omitempty"`
	FinalMonthlyScore  *float64   `json:"finalMonthlyScore,omitempty"`
	ValidatorNotes     string     `json:"validatorNotes,omitempty"`
	CreatedAt          *time.Time `json:"createdAt,omitempty"`
	UpdatedAt          *time.Time `json:"updatedAt,omitempty"`
	ReviewAssessmentID *int64     `json:"reviewAssessmentId,omitempty"`
	WorkflowStatus     string     `json:"workflowStatus,omitempty"`
}

type GaspolMonthlyComponent struct {
	Month          int     `json:"month"`
	Label          string  `json:"label"`
	Weight         float64 `json:"weight"`
	Score          float64 `json:"score"`
	EffectiveScore float64 `json:"effectiveScore"`
	Status         string  `json:"status"`
	StatusLabel    string  `json:"statusLabel"`
	StatusDetail   string  `json:"statusDetail,omitempty"`
	IsFinal        bool    `json:"isFinal"`
}

type GaspolRankingStatusSummary struct {
	IsLive              bool   `json:"isLive"`
	HasWaiting          bool   `json:"hasWaiting"`
	HasNotYetDue        bool   `json:"hasNotYetDue"`
	WaitingComponents   int    `json:"waitingComponents"`
	NotYetDueComponents int    `json:"notYetDueComponents"`
	ZeroFinalComponents int    `json:"zeroFinalComponents"`
	ValidatedComponents int    `json:"validatedComponents"`
	TopTenLockReady     bool   `json:"topTenLockReady"`
	RankingMessage      string `json:"rankingMessage"`
	TopTenLockMessage   string `json:"topTenLockMessage"`
}

type GaspolScoreBreakdown struct {
	ScoreMonthly      float64                   `json:"scoreMonthly"`
	ScoreField        float64                   `json:"scoreField"`
	ScoreFieldStage1  float64                   `json:"scoreFieldStage1"`
	ScoreFieldStage2  float64                   `json:"scoreFieldStage2"`
	ScorePresentation float64                   `json:"scorePresentation"`
	ScoreNomination   float64                   `json:"scoreNomination"`
	ScoreAppeal       float64                   `json:"scoreAppeal"`
	ScoreTotal        float64                   `json:"scoreTotal"`
	MaximumScore      float64                   `json:"maximumScore"`
	CurrentMaxScore   float64                   `json:"currentMaxScore"`
	Contributions     []GaspolScoreContribution `json:"contributions"`
}

type GaspolScoreContribution struct {
	Key           string   `json:"key"`
	Label         string   `json:"label"`
	Score         *float64 `json:"score"`
	WeightedScore *float64 `json:"weightedScore"`
	Weight        float64  `json:"weight"`
	Status        string   `json:"status"`
}

type GaspolFavoriteScore struct {
	InternalScore float64 `json:"internalScore"`
	LikeScore     float64 `json:"likeScore"`
	ActualLikes   int     `json:"actualLikes"`
	Total         float64 `json:"total"`
}

type GaspolRankingRow struct {
	MonthlyFinal           bool                     `json:"monthlyFinal"`
	Rank                   int                      `json:"rank"`
	Year                   int                      `json:"year"`
	Month                  int                      `json:"month"`
	Stage                  string                   `json:"stage"`
	SchoolID               int                      `json:"schoolId"`
	SchoolName             string                   `json:"schoolName"`
	NPSN                   string                   `json:"npsn"`
	SchoolLevel            string                   `json:"schoolLevel"`
	SchoolStatus           string                   `json:"schoolStatus"`
	SupervisorNameSnapshot string                   `json:"supervisorNameSnapshot"`
	MonthlyComponents      []GaspolMonthlyComponent `json:"monthlyComponents"`
	Scores                 GaspolScoreBreakdown     `json:"scores"`
	Cohort                 string                   `json:"cohort,omitempty"`
	Path                   string                   `json:"path,omitempty"`
	Badge                  string                   `json:"badge,omitempty"`
	VisualTone             string                   `json:"visualTone,omitempty"`
}

type GaspolSummaryCard struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type GaspolPublicDashboard struct {
	Year             int                        `json:"year"`
	Month            int                        `json:"month"`
	Stage            string                     `json:"stage"`
	CompetitionState string                     `json:"competitionState"`
	Cards            []GaspolSummaryCard        `json:"cards"`
	Leaderboard      []GaspolRankingRow         `json:"leaderboard"`
	RankingStatus    GaspolRankingStatusSummary `json:"rankingStatus"`
	BestSchool       *GaspolRankingRow          `json:"bestSchool,omitempty"`
	Awards           GaspolPublicAwards         `json:"awards"`
	LastPublished    *time.Time                 `json:"lastPublished,omitempty"`
}

type GaspolPublicAward struct {
	AwardType  string   `json:"awardType"`
	SchoolID   *int     `json:"schoolId,omitempty"`
	SchoolName string   `json:"schoolName"`
	Score      *float64 `json:"score,omitempty"`
	IsFinal    bool     `json:"isFinal"`
}

type GaspolPublicAwards struct {
	Best       GaspolPublicAward   `json:"best"`
	Innovative GaspolPublicAward   `json:"innovative"`
	Favorite   GaspolPublicAward   `json:"favorite"`
	Failed     []GaspolPublicAward `json:"failed"`
}

type GaspolStageAssignment struct {
	ID             int64      `json:"id"`
	Year           int        `json:"year"`
	Stage          string     `json:"stage"`
	SchoolID       *int       `json:"schoolId,omitempty"`
	AssessorUserID int        `json:"assessorUserId"`
	AssessorName   string     `json:"assessorName"`
	Username       string     `json:"username"`
	BaseRole       string     `json:"baseRole"`
	AssignmentRole string     `json:"assignmentRole"`
	IsActive       bool       `json:"isActive"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
	SchoolName     string     `json:"schoolName"`
	NPSN           string     `json:"npsn"`
}

type GaspolAssignmentSchoolCandidate struct {
	SchoolID   int      `json:"schoolId"`
	NPSN       string   `json:"npsn"`
	SchoolName string   `json:"schoolName"`
	Cohorts    []string `json:"cohorts"`
}

type GaspolStageTask struct {
	AssignmentID   int64    `json:"assignmentId"`
	Stage          string   `json:"stage"`
	AssignmentRole string   `json:"assignmentRole"`
	SchoolID       int      `json:"schoolId"`
	SchoolName     string   `json:"schoolName"`
	NPSN           string   `json:"npsn"`
	AssessmentID   *int64   `json:"assessmentId,omitempty"`
	RawScore       *float64 `json:"rawScore,omitempty"`
	WorkflowStatus string   `json:"workflowStatus,omitempty"`
	Notes          string   `json:"notes"`
	ReturnNotes    string   `json:"returnNotes"`
}

type GaspolSchoolDashboard struct {
	MonthlyFinal       bool                      `json:"monthlyFinal"`
	Year               int                       `json:"year"`
	Month              int                       `json:"month"`
	Stage              string                    `json:"stage"`
	SchoolID           int                       `json:"schoolId"`
	SchoolName         string                    `json:"schoolName"`
	IsParticipant      bool                      `json:"isParticipant"`
	CanEdit            bool                      `json:"canEdit"`
	ActionsDisabled    bool                      `json:"actionsDisabled"`
	DisabledReason     string                    `json:"disabledReason"`
	AssignedSupervisor string                    `json:"assignedSupervisor"`
	AssignedValidator  string                    `json:"assignedValidator"`
	EffectiveTime      *time.Time                `json:"effectiveTime,omitempty"`
	LockedMonths       []int                     `json:"lockedMonths"`
	ScoredMonths       []int                     `json:"scoredMonths"`
	LatestSubmission   *GaspolSchoolSubmission   `json:"latestSubmission,omitempty"`
	Scores             GaspolScoreBreakdown      `json:"scores"`
	Ranking            *GaspolRankingRow         `json:"ranking,omitempty"`
	MonthlyComponents  []GaspolMonthlyComponent  `json:"monthlyComponents"`
	MonthlyStatuses    []GaspolSchoolMonthStatus `json:"monthlyStatuses"`
	History            []GaspolSchoolSubmission  `json:"history"`
	Cohorts            []string                  `json:"cohorts"`
	CompetitionNotice  string                    `json:"competitionNotice,omitempty"`
}

type GaspolSupervisorDashboard struct {
	Year                   int                        `json:"year"`
	Month                  int                        `json:"month"`
	AssignmentCount        int                        `json:"assignmentCount"`
	ValidatedCount         int                        `json:"validatedCount"`
	PendingCount           int                        `json:"pendingCount"`
	MissingSubmissionCount int                        `json:"missingSubmissionCount"`
	Assignments            []GaspolAssignment         `json:"assignments"`
	LatestAssessments      []GaspolAssessment         `json:"latestAssessments"`
	MissingSubmissions     []GaspolMonthlyWorkflowRow `json:"missingSubmissions"`
	PendingReviews         []GaspolMonthlyWorkflowRow `json:"pendingReviews"`
	CompletedReviews       []GaspolMonthlyWorkflowRow `json:"completedReviews"`
	IsKoorGaspol           bool                       `json:"isKoorGaspol"`
	KoorPendingAssessments []GaspolStageAssessmentRow `json:"koorPendingAssessments"`
	StageTasks             []GaspolStageTask          `json:"stageTasks"`
}

type GaspolValidatorDashboard struct {
	Year                 int                        `json:"year"`
	Month                int                        `json:"month"`
	PendingCount         int                        `json:"pendingCount"`
	ValidatedCount       int                        `json:"validatedCount"`
	ParticipantCount     int                        `json:"participantCount"`
	BlockedCount         int                        `json:"blockedCount"`
	PendingAssessments   []GaspolAssessment         `json:"pendingAssessments"`
	ValidatedAssessments []GaspolAssessment         `json:"validatedAssessments"`
	NotReadyRows         []GaspolMonthlyWorkflowRow `json:"notReadyRows"`
	PendingRows          []GaspolMonthlyWorkflowRow `json:"pendingRows"`
	ValidatedRows        []GaspolMonthlyWorkflowRow `json:"validatedRows"`
	Leaderboard          []GaspolRankingRow         `json:"leaderboard"`
	ValidationQueue      []GaspolStageAssessmentRow `json:"validationQueue"`
}

type GaspolAccessContext struct {
	Year int    `json:"year"`
	View string `json:"view"`
}

type GaspolJuryDashboard struct {
	Year              int                `json:"year"`
	Month             int                `json:"month"`
	Stage             string             `json:"stage"`
	CandidateCount    int                `json:"candidateCount"`
	Candidates        []GaspolRankingRow `json:"candidates"`
	LatestAssessments []GaspolAssessment `json:"latestAssessments"`
	StageTasks        []GaspolStageTask  `json:"stageTasks"`
}

type GaspolAdminDashboard struct {
	TopTenReady                 bool                       `json:"topTenReady"`
	Year                        int                        `json:"year"`
	Month                       int                        `json:"month"`
	Stage                       string                     `json:"stage"`
	IsYearActive                bool                       `json:"isYearActive"`
	ParticipantCount            int                        `json:"participantCount"`
	SupervisorCount             int                        `json:"supervisorCount"`
	ValidatorCount              int                        `json:"validatorCount"`
	JuryCount                   int                        `json:"juryCount"`
	PendingValidation           int                        `json:"pendingValidation"`
	UnassignedCabdinAssessments int                        `json:"unassignedCabdinAssessments"`
	RankingStatus               GaspolRankingStatusSummary `json:"rankingStatus"`
	TopTen                      []GaspolRankingRow         `json:"topTen"`
	TopFive                     []GaspolRankingRow         `json:"topFive"`
}

type GaspolSchoolMonitoringRow struct {
	SchoolID           int                     `json:"schoolId"`
	SchoolName         string                  `json:"schoolName"`
	NPSN               string                  `json:"npsn"`
	SchoolLevel        string                  `json:"schoolLevel"`
	SchoolStatus       string                  `json:"schoolStatus"`
	IsParticipant      bool                    `json:"isParticipant"`
	AssignedSupervisor string                  `json:"assignedSupervisor"`
	AssignedValidator  string                  `json:"assignedValidator"`
	LastSubmission     *GaspolSchoolSubmission `json:"lastSubmission,omitempty"`
	Scores             GaspolScoreBreakdown    `json:"scores"`
}

type GaspolSchoolMonthStatus struct {
	Year             int      `json:"year"`
	Month            int      `json:"month"`
	ReportTitle      string   `json:"reportTitle"`
	ReportLink       string   `json:"reportLink"`
	DisplayStatus    string   `json:"displayStatus"`
	AssessmentStatus string   `json:"assessmentStatus"`
	FinalScore       *float64 `json:"finalScore,omitempty"`
}

type GaspolMonthlyWorkflowRow struct {
	Month              int      `json:"month"`
	SchoolID           int      `json:"schoolId"`
	SchoolName         string   `json:"schoolName"`
	ReportLink         string   `json:"reportLink"`
	SubmissionStatus   string   `json:"submissionStatus"`
	ProcessStatus      string   `json:"processStatus"`
	SupervisorName     string   `json:"supervisorName"`
	ValidatorName      string   `json:"validatorName"`
	AssessmentID       *int     `json:"assessmentId,omitempty"`
	StageAssessmentID  *int64   `json:"stageAssessmentId,omitempty"`
	WorkflowStatus     string   `json:"workflowStatus,omitempty"`
	SupervisorNotes    string   `json:"supervisorNotes,omitempty"`
	ReturnNotes        string   `json:"returnNotes,omitempty"`
	SupervisorScore    *float64 `json:"supervisorScore,omitempty"`
	ValidatorScore     *float64 `json:"validatorScore,omitempty"`
	FinalMonthlyScore  *float64 `json:"finalMonthlyScore,omitempty"`
	ValidationDecision string   `json:"validationDecision,omitempty"`
}

type GaspolSupervisorMonitoringRow struct {
	UserID          int                `json:"userId"`
	Username        string             `json:"username"`
	DisplayName     string             `json:"displayName"`
	BaseRole        string             `json:"baseRole"`
	AssignmentCount int                `json:"assignmentCount"`
	CompletedCount  int                `json:"completedCount"`
	Schools         []GaspolAssignment `json:"schools"`
}

type GaspolSchoolMonitoringExportRow struct {
	NPSN       string `json:"npsn"`
	SchoolName string `json:"schoolName"`
	Month      int    `json:"month"`
	Mengisi    string `json:"mengisi"`
}

type GaspolSupervisorMonitoringExportRow struct {
	SupervisorName string `json:"supervisorName"`
	SchoolName     string `json:"schoolName"`
	Month          int    `json:"month"`
	Penilaian      string `json:"penilaian"`
}

type GaspolAssignmentGenerateResult struct {
	Year                int                `json:"year"`
	Month               int                `json:"month"`
	Assignments         []GaspolAssignment `json:"assignments"`
	UnassignedSchoolIDs []int              `json:"unassignedSchoolIds"`
}

type GaspolAssignmentPreviewRow struct {
	SupervisorUserID   int      `json:"supervisorUserId"`
	SupervisorName     string   `json:"supervisorName"`
	PreviousMonthCount int      `json:"previousMonthCount"`
	CurrentMonthCount  int      `json:"currentMonthCount"`
	HistoricalCount    int      `json:"historicalCount"`
	RunningTotal       int      `json:"runningTotal"`
	SchoolNames        []string `json:"schoolNames"`
	NewCount           int      `json:"newCount"`
	ManualCount        int      `json:"manualCount"`
	LockedCount        int      `json:"lockedCount"`
}

type GaspolAssignmentPreview struct {
	PreviewID           string                       `json:"previewId"`
	Year                int                          `json:"year"`
	Month               int                          `json:"month"`
	Rows                []GaspolAssignmentPreviewRow `json:"rows"`
	Assignments         []GaspolAssignmentPlan       `json:"assignments"`
	LockedAssignments   []GaspolAssignment           `json:"lockedAssignments"`
	UnassignedSchoolIDs []int                        `json:"unassignedSchoolIds"`
}

type GaspolExportBundle struct {
	Summary                    GaspolAdminDashboard                  `json:"summary"`
	SchoolMonitoring           []GaspolSchoolMonitoringRow           `json:"schoolMonitoring"`
	SupervisorMonitoring       []GaspolSupervisorMonitoringRow       `json:"supervisorMonitoring"`
	SchoolMonitoringExportRows []GaspolSchoolMonitoringExportRow     `json:"schoolMonitoringExportRows"`
	SupervisorExportRows       []GaspolSupervisorMonitoringExportRow `json:"supervisorExportRows"`
	Leaderboard                []GaspolRankingRow                    `json:"leaderboard"`
}

type GaspolParticipantUpsertResult struct {
	Year         int                       `json:"year"`
	IsYearActive bool                      `json:"isYearActive"`
	Participants []GaspolSchoolParticipant `json:"participants"`
}

type GaspolGeneratedCredential struct {
	UserID      int    `json:"userId"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	SchoolID    *int   `json:"schoolId,omitempty"`
	SchoolName  string `json:"schoolName"`
	DisplayName string `json:"displayName"`
}

type GaspolGenerateUsersResult struct {
	Year    int                         `json:"year"`
	Created []GaspolGeneratedCredential `json:"created"`
	Skipped []GaspolGeneratedCredential `json:"skipped"`
}

type GaspolEmptyState struct {
	Items []string `json:"items"`
}

type GaspolSchoolUserRow struct {
	SchoolID                  int     `json:"schoolId"`
	NPSN                      string  `json:"npsn"`
	SchoolName                string  `json:"schoolName"`
	SchoolLevel               string  `json:"schoolLevel"`
	SchoolStatus              string  `json:"schoolStatus"`
	IsParticipant             bool    `json:"isParticipant"`
	UserID                    *int    `json:"userId,omitempty"`
	Username                  string  `json:"username"`
	AccountStatus             string  `json:"accountStatus"`
	RoleIsActive              bool    `json:"roleIsActive"`
	DisplayName               string  `json:"displayName"`
	InitialPassword           string  `json:"-"`
	InitialPasswordCiphertext *string `json:"-"`
	InitialPasswordNonce      *string `json:"-"`
	InitialPasswordEncrypted  *string `json:"-"`
}

type GaspolBulkValidatorAssignResult struct {
	Year            int `json:"year"`
	ValidatorUserID int `json:"validatorUserId"`
	UpdatedCount    int `json:"updatedCount"`
}

type GaspolRoleCandidate struct {
	RoleAssignmentID    *int   `json:"roleAssignmentId,omitempty"`
	UserID              int    `json:"userId"`
	Username            string `json:"username"`
	DisplayName         string `json:"displayName"`
	BaseRole            string `json:"baseRole"`
	IsAssigned          bool   `json:"isAssigned"`
	IsActive            bool   `json:"isActive"`
	AssignedSchoolCount int    `json:"assignedSchoolCount"`
	AssignmentCount     int    `json:"assignmentCount"`
}

type GaspolSchoolMonitoringStatusRow struct {
	SchoolID     int    `json:"schoolId"`
	NPSN         string `json:"npsn"`
	SchoolName   string `json:"schoolName"`
	SchoolLevel  string `json:"schoolLevel"`
	SchoolStatus string `json:"schoolStatus"`
	Month        int    `json:"month"`
	Mengisi      string `json:"mengisi"`
}

type GaspolSupervisorMonitoringStatusRow struct {
	AssignmentID      int    `json:"assignmentId"`
	SchoolID          int    `json:"schoolId"`
	NPSN              string `json:"npsn"`
	SchoolName        string `json:"schoolName"`
	Month             int    `json:"month"`
	SupervisorUserID  *int   `json:"supervisorUserId,omitempty"`
	SupervisorName    string `json:"supervisorName"`
	Method            string `json:"method"`
	Penilaian         string `json:"penilaian"`
	HasSubmittedScore bool   `json:"hasSubmittedScore"`
}

type GaspolAuditLogRow struct {
	ID          int        `json:"id"`
	Year        *int       `json:"year,omitempty"`
	ActorUserID *int       `json:"actorUserId,omitempty"`
	ActorName   string     `json:"actorName"`
	ActorRole   string     `json:"actorRole"`
	Action      string     `json:"action"`
	EntityType  string     `json:"entityType"`
	EntityID    *int       `json:"entityId,omitempty"`
	SchoolID    *int       `json:"schoolId,omitempty"`
	SchoolName  string     `json:"schoolName"`
	Payload     string     `json:"payload"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
}

type GaspolAdminSettingsOverview struct {
	Year                  int                                   `json:"year"`
	Summary               GaspolAdminDashboard                  `json:"summary"`
	Participants          []GaspolSchoolParticipant             `json:"participants"`
	SchoolUsers           []GaspolSchoolUserRow                 `json:"schoolUsers"`
	SupervisorCandidates  []GaspolRoleCandidate                 `json:"supervisorCandidates"`
	ValidatorCandidates   []GaspolRoleCandidate                 `json:"validatorCandidates"`
	KoorCandidates        []GaspolRoleCandidate                 `json:"koorCandidates"`
	KoorAssignments       []GaspolStageAssignment               `json:"koorAssignments"`
	StageAssignments      []GaspolStageAssignment               `json:"stageAssignments"`
	StageSchoolCandidates []GaspolAssignmentSchoolCandidate     `json:"stageSchoolCandidates"`
	InternalCandidates    []GaspolRoleCandidate                 `json:"internalCandidates"`
	JuryAccounts          []GaspolRoleCandidate                 `json:"juryAccounts"`
	SchoolMonitoring      []GaspolSchoolMonitoringStatusRow     `json:"schoolMonitoring"`
	SupervisorMonitoring  []GaspolSupervisorMonitoringStatusRow `json:"supervisorMonitoring"`
	AuditLogs             []GaspolAuditLogRow                   `json:"auditLogs"`
}

type GaspolHistoryFilter struct {
	Tab           string
	Year          int
	Month         int
	Stage         string
	DateFrom      string
	DateTo        string
	Status        string
	SupervisorID  int
	ValidatorID   int
	Search        string
	Page          int
	PageSize      int
	AllowedLevels []string
}

type GaspolHistoryRow struct {
	ID                 int        `json:"id"`
	Year               int        `json:"year"`
	Month              int        `json:"month"`
	Stage              string     `json:"stage"`
	SchoolID           int        `json:"schoolId"`
	SchoolName         string     `json:"schoolName"`
	NPSN               string     `json:"npsn"`
	SchoolLevel        string     `json:"schoolLevel"`
	ReportTitle        string     `json:"reportTitle"`
	ReportLink         string     `json:"reportLink"`
	Status             string     `json:"status"`
	SubmittedByName    string     `json:"submittedByName"`
	SupervisorName     string     `json:"supervisorName"`
	ValidatorName      string     `json:"validatorName"`
	ActorName          string     `json:"actorName"`
	EventAt            *time.Time `json:"eventAt,omitempty"`
	SupervisorScore    *float64   `json:"supervisorScore,omitempty"`
	ValidatorScore     *float64   `json:"validatorScore,omitempty"`
	FinalScore         *float64   `json:"finalScore,omitempty"`
	Score              *float64   `json:"score,omitempty"`
	Notes              string     `json:"notes"`
	ValidatorNotes     string     `json:"validatorNotes"`
	ValidationDecision string     `json:"validationDecision"`
	Action             string     `json:"action"`
	ActorRole          string     `json:"actorRole"`
	BeforeStatus       string     `json:"beforeStatus"`
	AfterStatus        string     `json:"afterStatus"`
	AggregateScore     *float64   `json:"aggregateScore,omitempty"`
	Reference          string     `json:"reference"`
}

type GaspolHistorySummary struct {
	Submitted         int `json:"submitted"`
	Assessed          int `json:"assessed"`
	Validated         int `json:"validated"`
	PendingValidation int `json:"pendingValidation"`
	JuryAssessed      int `json:"juryAssessed"`
}

type GaspolHistoryOption struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type GaspolHistoryResponse struct {
	Items       []GaspolHistoryRow    `json:"items"`
	Summary     GaspolHistorySummary  `json:"summary"`
	Total       int                   `json:"total"`
	Page        int                   `json:"page"`
	PageSize    int                   `json:"pageSize"`
	TotalPages  int                   `json:"totalPages"`
	Supervisors []GaspolHistoryOption `json:"supervisors"`
	Validators  []GaspolHistoryOption `json:"validators"`
}

type GaspolHistoryDetail struct {
	Row      GaspolHistoryRow    `json:"row"`
	Timeline []GaspolAuditLogRow `json:"timeline"`
}
