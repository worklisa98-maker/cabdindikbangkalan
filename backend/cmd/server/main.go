package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"cabdin-bangkalan-backend/internal/config"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/handlers"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/middleware"
	"cabdin-bangkalan-backend/internal/services"
	"cabdin-bangkalan-backend/internal/storage"
	"cabdin-bangkalan-backend/internal/utils"
	_ "cabdin-bangkalan-backend/migrations"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" || cfg.JWTSecret == "" {
		logging.Fatalf("server", "config", "msg=%q", "DATABASE_URL and JWT_SECRET must be set")
	}
	if cfg.MasterEncryptionKey == "" {
		logging.Fatalf("server", "config", "msg=%q", "MASTER_ENCRYPTION_KEY must be set")
	}

	ctx := context.Background()
	if shouldAutoMigrate() {
		if err := runMigrations(cfg.DatabaseURL); err != nil {
			log.Printf("[migrate] failed: %v", err)
			os.Exit(1)
		}
	}
	database, err := db.Connect(ctx, cfg.DatabaseURL, db.PoolConfig{
		MaxConns:          cfg.DBPoolMaxConns,
		MinConns:          cfg.DBPoolMinConns,
		MaxConnLifetime:   cfg.DBPoolMaxConnLifetime,
		MaxConnIdleTime:   cfg.DBPoolMaxConnIdleTime,
		HealthCheckPeriod: cfg.DBPoolHealthCheck,
	})
	if err != nil {
		logging.Fatalf("server", "db_connect", "err=%v", err)
	}
	defer database.Close()
	if err := services.CheckGaspolMasterReturnSchema(ctx, database); err != nil {
		logging.Fatalf("gaspol", "schema_check", "err=%v", err)
	}
	if err := services.CheckGaspolSchoolNoticeSchema(ctx, database); err != nil {
		logging.Fatalf("gaspol", "schema_check", "err=%v", err)
	}
	if err := services.CheckGaspolPhase234Schema(ctx, database); err != nil {
		logging.Fatalf("gaspol", "schema_check", "err=%v", err)
	}
	logging.Infof("gaspol", "schema_check", "status=ready required_migration=071")
	db.BootstrapAdminMaster(ctx, database)

	services.LogHardiknasStorageStartup(cfg.StorageLocalDir)
	storageProvider := storage.New(cfg.StorageProvider, cfg.StorageLocalDir)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	authHandler := handlers.AuthHandler{DB: database, JWTSecret: cfg.JWTSecret}
	schoolHandler := handlers.SchoolHandler{DB: database}
	achievementHandler := handlers.AchievementHandler{DB: database}
	admissionHandler := handlers.AdmissionHandler{DB: database}
	siswaHandler := handlers.SiswaHandler{DB: database}
	uploadHandler := handlers.UploadHandler{DB: database, Storage: storageProvider}
	masterKey, err := utils.DecodeMasterKey(cfg.MasterEncryptionKey)
	if err != nil {
		logging.Fatalf("server", "decode_master_key", "err=%v", err)
	}

	adminHandler := handlers.AdminHandler{DB: database, MasterEncryptionKey: masterKey}
	systemClock := services.NewSystemClock(database, cfg)
	systemTimeHandler := handlers.SystemTimeHandler{Clock: systemClock}
	templateHandler := handlers.TemplateHandler{DB: database}
	publicHandler := handlers.PublicHandler{DB: database}
	gaspolHandler := handlers.GaspolHandler{DB: database, MasterEncryptionKey: masterKey}
	dashboardHandler := handlers.DashboardHandler{DB: database}
	healthHandler := handlers.HealthHandler{DB: database}
	legacyImportHandler := handlers.LegacyImportHandler{DB: database}
	legacyMigrationHandler := handlers.LegacyMigrationHandler{DB: database}
	guideBookHandler := handlers.GuideBookHandler{DB: database, StorageLocalDir: cfg.StorageLocalDir}
	serviceInfoHandler := handlers.ServiceInfoHandler{DB: database}
	landingSettingsHandler := handlers.LandingSettingsHandler{DB: database}
	serviceLinksHandler := handlers.ServiceLinksHandler{DB: database}
	complaintHandler := &handlers.ComplaintHandler{DB: database, StorageDir: cfg.StorageLocalDir, TurnstileSecret: cfg.TurnstileSecretKey}
	backupHandler := handlers.BackupHandler{DB: database, DatabaseURL: cfg.DatabaseURL, StorageDir: cfg.StorageLocalDir, EncryptionKey: cfg.BackupEncryptionKey, AppVersion: cfg.AppVersion}
	hardiknasHandler := handlers.HardiknasHandler{
		DB:              database,
		StorageLocalDir: cfg.StorageLocalDir,
		Clock:           systemClock,
		FrontendOrigin:  cfg.FrontendOrigin,
	}
	mgmpHandler := handlers.MGMPHandler{DB: database, MasterEncryptionKey: masterKey}
	reportJobService := &services.ReportJobService{
		DB:                    database,
		JWTSecret:             cfg.JWTSecret,
		FrontendOrigin:        cfg.FrontendOrigin,
		ChromeExecutablePath:  cfg.ChromeExecutablePath,
		ReportTokenTTLSeconds: cfg.ReportTokenTTLSeconds,
		CacheTTLSeconds:       cfg.ReportPDFCacheTTL,
		FileTTLHours:          cfg.ReportPDFFileTTLHours,
		ReportDir:             cfg.ReportPDFDir,
	}
	if err := reportJobService.EnsureReportDir(); err != nil {
		logging.Fatalf("server", "report_job_dir", "err=%v", err)
	}
	reportJobService.StartBundleWorker(context.Background())
	reportJobService.StartFileCleanup(context.Background())
	siswaService := services.SiswaService{DB: database}
	siswaService.StartPTOfferReconciliationWorker(context.Background())
	siswaService.StartPTAutoFinalizeWorker(context.Background())
	reportHandler := handlers.ReportHandler{
		DB:                    database,
		JWTSecret:             cfg.JWTSecret,
		FrontendOrigin:        cfg.FrontendOrigin,
		ReportTokenTTLSeconds: cfg.ReportTokenTTLSeconds,
		ChromeExecutablePath:  cfg.ChromeExecutablePath,
		ReportJobs:            reportJobService,
	}

	router.POST("/auth/login", authHandler.Login)
	router.POST("/auth/logout", authHandler.Logout)
	router.GET("/livez", healthHandler.Live)
	router.GET("/readyz", healthHandler.Ready)
	router.GET("/healthz", healthHandler.Check)
	router.GET("/templates/:type", templateHandler.Download)
	router.GET("/api/meta/last-updated", publicHandler.MetaLastUpdated)
	router.GET("/api/public/dashboard", publicHandler.Dashboard)
	router.GET("/api/public/landing-settings", landingSettingsHandler.GetPublic)
	router.GET("/api/public/service-links", serviceLinksHandler.Public)
	router.GET("/api/public/landing-summary", publicHandler.LandingSummary)
	router.GET("/api/public/school-locations", schoolHandler.PublicLocations)
	router.GET("/api/public/prestasi/dashboard", publicHandler.AchievementDashboard)
	router.GET("/api/public/prestasi/summary", publicHandler.AchievementSummary)
	router.GET("/api/public/prestasi/full", publicHandler.AchievementFull)
	router.GET("/api/public/prestasi/analytics", publicHandler.AchievementAnalytics)
	router.GET("/api/public/siswa-perguruan-tinggi/dashboard", publicHandler.AdmissionDashboard)
	router.GET("/api/public/siswa-perguruan-tinggi/top", publicHandler.AdmissionTopUniversities)
	router.GET("/api/public/siswa-perguruan-tinggi/analytics", publicHandler.AdmissionAnalytics)
	router.GET("/api/public/siswa/analytics/final", publicHandler.AdmissionAnalytics)
	router.GET("/api/public/siswa/analytics/summary", publicHandler.AdmissionAnalytics)
	router.GET("/api/public/siswa/outcomes/analytics", publicHandler.SiswaFinalOutcomeAnalytics)
	router.GET("/api/public/siswa/analytics/status", publicHandler.SiswaAnalyticsStatus)
	router.GET("/api/public/siswa/analytics/higher-ed", publicHandler.SiswaAnalyticsHigherEd)
	router.GET("/api/public/siswa/analytics/diterima-perguruan-tinggi", publicHandler.SiswaAnalyticsFinalHigherEd)
	router.GET("/api/public/siswa/analytics/diterima-dunia-kerja", publicHandler.SiswaAnalyticsFinalWork)
	router.GET("/api/public/siswa/analytics/berwirausaha", publicHandler.SiswaAnalyticsFinalEntrepreneur)
	router.GET("/api/public/siswa/analytics/belum-melanjutkan", publicHandler.SiswaAnalyticsFinalUndefined)
	router.GET("/api/public/status-lulusan/analytics-pt", publicHandler.StatusLulusanAnalyticsPT)
	router.GET("/api/public/gaspol/dashboard", gaspolHandler.PublicDashboard)
	router.GET("/api/public/gaspol/leaderboard", gaspolHandler.PublicLeaderboard)
	router.GET("/api/public/gaspol/best-school", gaspolHandler.PublicBestSchool)
	router.POST("/api/public/gaspol/favorite/vote", gaspolHandler.CastFavoriteVote)
	router.GET("/api/public/guide-books/placement/:placement", guideBookHandler.GetPublicActiveByPlacement)
	router.GET("/api/public/guide-books/images/:file", guideBookHandler.ServePublicImage)
	router.GET("/api/public/lomba-hardiknas", hardiknasHandler.PublicList)
	router.POST("/api/public/lomba-hardiknas", hardiknasHandler.PublicCreate)
	router.POST("/api/public/mgmp/registrations", mgmpHandler.PublicRegister)
	router.GET("/api/public/mgmp/registrations/:code", mgmpHandler.PublicRegistrationStatus)
	router.PUT("/api/public/mgmp/registrations/:code/resubmit", mgmpHandler.PublicResubmitRegistration)
	router.GET("/api/public/mgmp/references", mgmpHandler.PublicReferences)
	router.GET("/api/public/mgmp/template.xlsx", mgmpHandler.DownloadMemberTemplate)
	router.GET("/api/public/lomba-hardiknas/summary", hardiknasHandler.PublicSummary)
	router.GET("/api/public/lomba-hardiknas/winners", hardiknasHandler.PublicWinners)
	router.GET("/api/public/lomba-hardiknas/table", hardiknasHandler.PublicListTable)
	router.GET("/api/public/lomba-hardiknas/schools", hardiknasHandler.PublicSuggestSchools)
	router.GET("/api/public/lomba-hardiknas/karya/:id", hardiknasHandler.PublicDetail)
	router.GET("/api/public/lomba-hardiknas/media/:id", hardiknasHandler.PublicMediaOriginal)
	router.GET("/api/public/lomba-hardiknas/media/:id/thumbnail", hardiknasHandler.PublicMediaThumbnail)
	router.GET("/api/public/informasi-pelayanan-cabdin", serviceInfoHandler.PublicList)
	router.GET("/api/public/informasi-pelayanan-cabdin/files/:id", serviceInfoHandler.PublicFile)
	router.GET("/api/public/complaints/categories", complaintHandler.Categories)
	router.GET("/api/public/complaints/schools", complaintHandler.Schools)
	router.GET("/api/public/complaints/popup", complaintHandler.ActivePopup)
	router.GET("/api/public/complaints/popup-image", complaintHandler.PopupImage)
	router.POST("/api/public/complaints", complaintHandler.SubmitPublic)
	router.POST("/api/public/complaints/track", complaintHandler.Track)
	router.POST("/api/public/complaints/clarifications", complaintHandler.SubmitClarification)
	router.GET("/api/public/complaints/attachments/:attachmentId", complaintHandler.PublicAttachment)

	authGroup := router.Group("/")
	authGroup.Use(middleware.AuthMiddleware(cfg.JWTSecret, database))
	authGroup.Use(middleware.ActivityMiddleware(database))
	authGroup.GET("/auth/me", authHandler.Me)
	authGroup.GET("/api/service-links", serviceLinksHandler.Authenticated)
	authGroup.GET("/api/admin/master/service-links", serviceLinksHandler.Admin)
	authGroup.PUT("/api/admin/master/service-links/external/:name", serviceLinksHandler.SaveExternal)
	authGroup.PUT("/api/admin/master/service-links/links", serviceLinksHandler.SaveLinks)
	authGroup.POST("/api/complaints", complaintHandler.SubmitAuthenticated)
	authGroup.GET("/api/complaints/mine", complaintHandler.Mine)
	authGroup.GET("/api/admin/complaints", complaintHandler.AdminList)
	authGroup.GET("/api/admin/complaints/:id", complaintHandler.AdminDetail)
	authGroup.GET("/api/admin/complaints/:id/attachments/:attachmentId", complaintHandler.AdminAttachment)
	authGroup.GET("/api/admin/complaints-assignees", complaintHandler.ComplaintAssignees)
	authGroup.POST("/api/admin/complaints/:id/actions", complaintHandler.ComplaintAction)
	authGroup.POST("/api/admin/complaints/:id/respond", complaintHandler.RespondWithEvidence)
	authGroup.POST("/api/admin/complaints/:id/evidence", complaintHandler.UploadComplaintEvidence)
	authGroup.GET("/api/admin/complaints-popup", complaintHandler.PopupAdminList)
	authGroup.POST("/api/admin/complaints-popup", complaintHandler.PopupAdminUpload)
	authGroup.PATCH("/api/admin/complaints-popup/:id", complaintHandler.PopupAdminActivate)
	authGroup.GET("/api/admin/complaints-popup/:id/image", complaintHandler.PopupAdminImage)
	authGroup.GET("/api/admin/backups", backupHandler.Status)
	authGroup.POST("/api/admin/backups/download", backupHandler.Download)
	authGroup.GET("/api/dashboard", dashboardHandler.Summary)
	authGroup.GET("/api/internal/dashboard", dashboardHandler.Summary)
	authGroup.GET("/api/landing-summary", dashboardHandler.LandingSummary)
	authGroup.GET("/api/sekolah", schoolHandler.List)
	authGroup.GET("/api/schools/:id/location", schoolHandler.GetLocation)
	authGroup.PUT("/api/schools/:id/location", schoolHandler.UpdateLocation)
	authGroup.GET("/api/admin/schools/suggest", adminHandler.SuggestSchools)
	authGroup.GET("/api/admin/sekolah/table", adminHandler.ListMonitoringSchools)
	authGroup.GET("/api/admin/sekolah/table/export.xlsx", adminHandler.ExportMonitoringSchools)
	authGroup.POST("/api/admin/reset-data", adminHandler.ResetSchoolData)
	authGroup.POST("/api/admin/change-password", adminHandler.ChangePassword)
	authGroup.GET("/api/system-time", systemTimeHandler.GetCurrentStatus)
	authGroup.GET("/api/admin/system-time", systemTimeHandler.GetAdminStatus)
	authGroup.POST("/api/admin/system-time/override", systemTimeHandler.SetOverride)
	authGroup.POST("/api/admin/system-time/reset", systemTimeHandler.ResetOverride)
	authGroup.GET("/api/gaspol/access-context", gaspolHandler.AccessContext)
	authGroup.GET("/api/gaspol/competition", gaspolHandler.CompetitionConfig)
	authGroup.PUT("/api/gaspol/admin/competition", gaspolHandler.UpdateCompetitionConfig)
	authGroup.POST("/api/gaspol/admin/stage-assignments", gaspolHandler.AssignCompetitionStage)
	authGroup.POST("/api/gaspol/admin/stage-assignments/bulk", gaspolHandler.BulkAssignCompetitionStage)
	authGroup.POST("/api/gaspol/assessments", gaspolHandler.SaveStageAssessment)
	authGroup.POST("/api/gaspol/stage-assessments/:id/master-return", gaspolHandler.MasterReturnStageAssessment)
	authGroup.POST("/api/gaspol/assessments/:id/review/:level", gaspolHandler.ReviewStageAssessment)
	authGroup.POST("/api/gaspol/admin/favorite/manual-like", gaspolHandler.ManualLikeOverride)
	authGroup.POST("/api/gaspol/admin/favorite/voting/open", gaspolHandler.OpenFavoriteVoting)
	authGroup.POST("/api/gaspol/admin/ba", gaspolHandler.UploadBADocument)
	authGroup.DELETE("/api/gaspol/admin/ba/:id", gaspolHandler.DeleteStagedBA)
	authGroup.GET("/api/gaspol/ba/:id", gaspolHandler.DownloadBADocument)
	authGroup.GET("/api/gaspol/internal/ranking-preview", gaspolHandler.PreviewCompetitionRanking)
	authGroup.GET("/api/gaspol/internal/determinations", gaspolHandler.DeterminationMonitor)
	authGroup.POST("/api/gaspol/admin/final-determination", gaspolHandler.DetermineFinal)
	authGroup.GET("/api/gaspol/internal/ba-zip", gaspolHandler.OfficialDocumentsZIP)
	authGroup.POST("/api/gaspol/admin/result-locks", gaspolHandler.LockCompetitionResult)
	authGroup.POST("/api/gaspol/admin/result-locks/:id/reopen", gaspolHandler.ReopenCompetitionResult)
	authGroup.PUT("/api/gaspol/school/favorite-content", gaspolHandler.SetFavoriteContent)
	authGroup.PUT("/api/gaspol/school/nomination-submission", gaspolHandler.SaveNominationSubmission)
	authGroup.GET("/api/gaspol/school/competition-notices", gaspolHandler.SchoolNotices)
	authGroup.POST("/api/gaspol/school/competition-notices/:id/ack", gaspolHandler.AcknowledgeSchoolNotice)
	authGroup.GET("/api/gaspol/nomination-submissions", gaspolHandler.NominationReport)
	authGroup.GET("/api/gaspol/admin/rubrics", gaspolHandler.Rubrics)
	authGroup.POST("/api/gaspol/admin/rubrics", gaspolHandler.Rubrics)
	authGroup.POST("/api/gaspol/admin/rubrics/:id/activate", gaspolHandler.ActivateRubric)
	authGroup.GET("/api/gaspol/assessment-rubric", gaspolHandler.AssessmentRubric)
	authGroup.GET("/api/gaspol/assessments/:id/details", gaspolHandler.AssessmentDetails)
	authGroup.POST("/api/gaspol/admin/voting", gaspolHandler.ManageVoting)
	authGroup.GET("/api/gaspol/voting", gaspolHandler.VotingView)
	authGroup.GET("/api/gaspol/voting/assistance", gaspolHandler.VotingAssistance)
	authGroup.POST("/api/gaspol/voting/assistance", gaspolHandler.VotingAssistance)
	authGroup.GET("/api/gaspol/phase-overview", gaspolHandler.PhaseOverview)
	authGroup.POST("/api/gaspol/voting/vote", gaspolHandler.AuthenticatedVote)
	authGroup.GET("/api/gaspol/likes", gaspolHandler.Likes)
	authGroup.POST("/api/gaspol/likes", gaspolHandler.Likes)
	authGroup.GET("/api/gaspol/innovation-submission", gaspolHandler.InnovationSubmission)
	authGroup.PUT("/api/gaspol/innovation-submission", gaspolHandler.InnovationSubmission)
	authGroup.POST("/api/gaspol/awards/innovation/propose", gaspolHandler.ProposeInnovationAward)
	authGroup.POST("/api/gaspol/awards/innovation/:id/decision", gaspolHandler.DecideInnovationAward)
	authGroup.POST("/api/gaspol/admin/awards/best/finalize", gaspolHandler.FinalizeBestAward)
	authGroup.POST("/api/gaspol/admin/awards/favorite/finalize", gaspolHandler.FinalizeFavoriteAward)
	authGroup.GET("/api/gaspol/school/dashboard", gaspolHandler.SchoolDashboard)
	authGroup.GET("/api/gaspol/school/submissions", gaspolHandler.SchoolSubmissions)
	authGroup.POST("/api/gaspol/school/submissions", gaspolHandler.UpsertSchoolSubmission)
	authGroup.GET("/api/gaspol/supervisor/dashboard", gaspolHandler.SupervisorDashboard)
	authGroup.GET("/api/gaspol/supervisor/assignments", gaspolHandler.SupervisorAssignments)
	authGroup.POST("/api/gaspol/supervisor/assessments/monthly", gaspolHandler.UpsertSupervisorAssessment)
	authGroup.GET("/api/gaspol/validator/dashboard", gaspolHandler.ValidatorDashboard)
	authGroup.GET("/api/gaspol/validator/ranking", gaspolHandler.ValidatorRanking)
	authGroup.POST("/api/gaspol/validator/assessments/validate", gaspolHandler.ValidateAssessments)
	authGroup.GET("/api/gaspol/jury/dashboard", gaspolHandler.JuryDashboard)
	authGroup.POST("/api/gaspol/jury/assessments/field", gaspolHandler.UpsertFieldAssessment)
	authGroup.POST("/api/gaspol/jury/assessments/presentation", gaspolHandler.UpsertPresentationAssessment)
	authGroup.GET("/api/gaspol/admin/dashboard", gaspolHandler.AdminDashboard)
	authGroup.GET("/api/gaspol/admin/settings/overview", gaspolHandler.AdminSettingsOverview)
	authGroup.PATCH("/api/gaspol/admin/submissions/cancel-submit", gaspolHandler.CancelSubmitSchoolSubmission)
	authGroup.POST("/api/gaspol/admin/year/activate", gaspolHandler.ActivateYear)
	authGroup.GET("/api/gaspol/admin/participants", gaspolHandler.ListParticipants)
	authGroup.PUT("/api/gaspol/admin/participants", gaspolHandler.SaveParticipants)
	authGroup.POST("/api/gaspol/admin/school-users/generate", gaspolHandler.GenerateSchoolUsers)
	authGroup.GET("/api/gaspol/admin/school-users/export.xlsx", gaspolHandler.ExportSchoolUsersExcel)
	authGroup.POST("/api/gaspol/admin/gaspol-users/reset-password", gaspolHandler.ResetGaspolUserPassword)
	authGroup.POST("/api/gaspol/admin/supervisors/roles", gaspolHandler.AssignSupervisors)
	authGroup.PATCH("/api/gaspol/admin/roles/status", gaspolHandler.SetRoleStatus)
	authGroup.POST("/api/gaspol/admin/validators/assign", gaspolHandler.AssignValidator)
	authGroup.POST("/api/gaspol/admin/validators/assign-all", gaspolHandler.AssignAllValidators)
	authGroup.POST("/api/gaspol/admin/assignments/manual", gaspolHandler.ManualAssignment)
	authGroup.POST("/api/gaspol/admin/assignments/random", gaspolHandler.RandomAssignments)
	authGroup.POST("/api/gaspol/admin/assignments/random/preview", gaspolHandler.PreviewRandomAssignments)
	authGroup.POST("/api/gaspol/admin/assignments/random/confirm", gaspolHandler.ConfirmRandomAssignments)
	authGroup.POST("/api/gaspol/admin/jury-account", gaspolHandler.GenerateJuryAccount)
	authGroup.GET("/api/gaspol/admin/monitoring/schools", gaspolHandler.MonitoringSchools)
	authGroup.GET("/api/gaspol/admin/monitoring/supervisors", gaspolHandler.MonitoringSupervisors)
	authGroup.GET("/api/gaspol/admin/export.xlsx", gaspolHandler.ExportExcel)
	authGroup.GET("/api/gaspol/history", gaspolHandler.History)
	authGroup.GET("/api/gaspol/history/export.xlsx", gaspolHandler.ExportHistoryExcel)
	authGroup.GET("/api/gaspol/history/:id", gaspolHandler.HistoryDetail)
	authGroup.GET("/api/mgmp/workspace", mgmpHandler.Workspace)
	authGroup.POST("/api/mgmp/activities", mgmpHandler.SaveActivity)
	authGroup.PUT("/api/mgmp/activities/:id", mgmpHandler.UpdateActivity)
	authGroup.POST("/api/mgmp/members", mgmpHandler.AddMember)
	authGroup.POST("/api/mgmp/members/import", mgmpHandler.ImportMembers)
	authGroup.GET("/api/mgmp/members/imports/:id/result.xlsx", mgmpHandler.DownloadMemberImportResult)
	authGroup.POST("/api/mgmp/notifications/:id/acknowledge", mgmpHandler.AcknowledgeMemberNotification)
	authGroup.PUT("/api/mgmp/no-agenda", mgmpHandler.SetNoAgenda)
	authGroup.GET("/api/mgmp/registrations", mgmpHandler.ListRegistrations)
	authGroup.GET("/api/mgmp/registrations/:id", mgmpHandler.RegistrationDetail)
	authGroup.GET("/api/mgmp/delegates", mgmpHandler.ListDelegates)
	authGroup.PUT("/api/mgmp/registrations/:id/delegate", mgmpHandler.DelegateRegistration)
	authGroup.POST("/api/mgmp/registrations/:id/decision", mgmpHandler.DecideRegistration)
	authGroup.GET("/api/mgmp/monitoring", mgmpHandler.Monitoring)
	authGroup.GET("/api/mgmp/groups/:id", mgmpHandler.GroupDetail)
	authGroup.GET("/api/mgmp/groups/:id/accounts", mgmpHandler.RevealAccounts)
	authGroup.POST("/api/mgmp/groups/:id/accounts/:userId/reset-password", mgmpHandler.ResetAccount)
	authGroup.POST("/api/mgmp/groups/:id/status", mgmpHandler.SetGroupStatus)
	authGroup.POST("/api/mgmp/groups/:id/members/:memberId/status", mgmpHandler.SetMemberStatus)
	authGroup.GET("/api/mgmp/history", mgmpHandler.History)
	authGroup.GET("/api/mgmp/settings", mgmpHandler.Settings)
	authGroup.PUT("/api/mgmp/settings", mgmpHandler.Settings)
	authGroup.GET("/api/lomba-hardiknas", hardiknasHandler.List)
	authGroup.GET("/api/lomba-hardiknas/summary", hardiknasHandler.Summary)
	authGroup.GET("/api/lomba-hardiknas/winners", hardiknasHandler.Winners)
	authGroup.PUT("/api/lomba-hardiknas/winners", hardiknasHandler.SaveWinners)
	authGroup.GET("/api/lomba-hardiknas/table", hardiknasHandler.ListTable)
	authGroup.GET("/api/lomba-hardiknas/schools", hardiknasHandler.SuggestSchools)
	authGroup.GET("/api/lomba-hardiknas/export.xlsx", hardiknasHandler.Export)
	authGroup.POST("/api/lomba-hardiknas", hardiknasHandler.Create)
	authGroup.DELETE("/api/lomba-hardiknas/:id", hardiknasHandler.Delete)
	authGroup.GET("/api/prestasi", achievementHandler.List)
	authGroup.GET("/api/prestasi/list", achievementHandler.ListTable)
	authGroup.GET("/api/prestasi/admin/summary", achievementHandler.AdminSummary)
	authGroup.GET("/api/prestasi/analytics", achievementHandler.Analytics)
	authGroup.POST("/api/prestasi/import/validate", achievementHandler.ValidateImport)
	authGroup.POST("/api/prestasi/import/commit", achievementHandler.CommitImport)
	authGroup.GET("/api/prestasi/export.xlsx", achievementHandler.Export)
	authGroup.PUT("/api/prestasi/:id", achievementHandler.Update)
	authGroup.GET("/api/siswa-perguruan-tinggi", admissionHandler.List)
	authGroup.GET("/api/siswa-perguruan-tinggi/list", admissionHandler.ListTable)
	authGroup.GET("/api/siswa-perguruan-tinggi/export.xlsx", admissionHandler.Export)
	authGroup.PUT("/api/siswa-perguruan-tinggi/:id", admissionHandler.Update)
	authGroup.GET("/api/universities/suggest", admissionHandler.SuggestUniversities)
	authGroup.GET("/api/siswa-perguruan-tinggi/summary", admissionHandler.Summary)
	authGroup.GET("/api/siswa-perguruan-tinggi/pt-summary", admissionHandler.SummaryByUniversity)
	authGroup.GET("/api/siswa-pt/analytics", admissionHandler.Analytics)
	authGroup.GET("/api/siswa/analytics/final", admissionHandler.AnalyticsFinal)
	authGroup.POST("/api/siswa/cohorts/import", siswaHandler.ImportCohorts)
	authGroup.GET("/api/siswa/cohorts/list", siswaHandler.ListCohorts)
	authGroup.GET("/api/siswa/year-summaries", siswaHandler.ListStudentYearSummaries)
	authGroup.PUT("/api/siswa/year-summaries", siswaHandler.UpsertStudentYearSummary)
	authGroup.GET("/api/siswa/final-requests/access", siswaHandler.StudentFinalAccess)
	authGroup.GET("/api/siswa/final-requests", siswaHandler.ListStudentFinalRequests)
	authGroup.POST("/api/siswa/final-requests", siswaHandler.SubmitStudentFinalRequest)
	authGroup.POST("/api/siswa/final-requests/:id/process", siswaHandler.ProcessStudentFinalRequest)
	authGroup.GET("/api/siswa/final-requests/delegates", siswaHandler.ListStudentFinalDelegates)
	authGroup.PUT("/api/siswa/final-requests/delegates", siswaHandler.SetStudentFinalDelegates)
	authGroup.POST("/api/siswa/applications/import", siswaHandler.ImportApplications)
	authGroup.GET("/api/siswa/applications/list", siswaHandler.ListApplications)
	authGroup.GET("/api/siswa/applications/pipeline", siswaHandler.ListApplicationsPipeline)
	authGroup.GET("/api/siswa/applications/detail", siswaHandler.GetApplicationsDetail)
	authGroup.PUT("/api/siswa/applications", siswaHandler.UpdateApplications)
	authGroup.GET("/api/siswa/applications/template.xlsx", siswaHandler.ExportApplicationsTemplate)
	authGroup.GET("/api/siswa/pt-choice-rules", siswaHandler.GetPTChoiceRules)
	authGroup.GET("/api/referensi/kampus", siswaHandler.SearchRefKampus)
	authGroup.GET("/api/siswa/acceptance-offers/template.xlsx", siswaHandler.ExportAcceptanceOffersTemplate)
	authGroup.GET("/api/siswa/acceptance-offers/analytics", siswaHandler.AcceptanceOfferAnalytics)
	authGroup.GET("/api/siswa/acceptance-offers/school-summary", siswaHandler.AcceptanceOfferSchoolSummary)
	authGroup.GET("/api/siswa/acceptance-offers/school-summary.xlsx", siswaHandler.ExportAcceptanceOfferSchoolSummary)
	authGroup.GET("/api/siswa/acceptance-offers/accepted.xlsx", siswaHandler.ExportAcceptedAcceptanceOffers)
	authGroup.GET("/api/siswa/acceptance-offers/supervisor-comparison", siswaHandler.SupervisorPTComparison)
	authGroup.PUT("/api/siswa/acceptance-offers/supervisor-snapshot", siswaHandler.UpsertSupervisorPTSnapshot)
	authGroup.POST("/api/siswa/acceptance-offers/import", siswaHandler.ImportAcceptanceOffers)
	authGroup.POST("/api/siswa/acceptance-offers/admin/import", siswaHandler.ImportAdminPTOffers)
	authGroup.GET("/api/siswa/acceptance-offers/admin/template.xlsx", siswaHandler.ExportAdminPTOffersTemplate)
	authGroup.POST("/api/siswa/acceptance-offers/admin", siswaHandler.CreateAdminPTOffer)
	authGroup.GET("/api/siswa/acceptance-offers/admin", siswaHandler.ListAdminPTOffers)
	authGroup.PUT("/api/siswa/acceptance-offers/admin/:id", siswaHandler.UpdateAdminPTOffer)
	authGroup.GET("/api/siswa/acceptance-offers/official-counts", siswaHandler.ListPTOfficialCoverage)
	authGroup.PUT("/api/siswa/acceptance-offers/official-counts", siswaHandler.UpsertPTOfficialCount)
	authGroup.GET("/api/siswa/eligible-snbp", siswaHandler.EligibleSNBP)
	authGroup.PUT("/api/siswa/eligible-snbp", siswaHandler.UpsertEligibleSNBP)
	authGroup.GET("/api/siswa/pt-registration-counts", siswaHandler.ListPTRegistrationCounts)
	authGroup.PUT("/api/siswa/pt-registration-counts", siswaHandler.UpsertPTRegistrationCounts)
	authGroup.POST("/api/siswa/ukk/import", siswaHandler.ImportUKK)
	authGroup.GET("/api/siswa/ukk/list", siswaHandler.ListUKK)
	authGroup.PUT("/api/siswa/ukk", siswaHandler.UpdateUKK)
	authGroup.GET("/api/siswa/wirausaha/list", siswaHandler.ListWirausahaPlans)
	authGroup.GET("/api/siswa/wirausaha/detail", siswaHandler.GetWirausahaPlans)
	authGroup.PUT("/api/siswa/wirausaha", siswaHandler.UpdateWirausahaPlans)
	authGroup.POST("/api/siswa/wirausaha/import", siswaHandler.ImportWirausahaPlans)
	authGroup.GET("/api/siswa/wirausaha/template.xlsx", siswaHandler.ExportWirausahaTemplate)
	authGroup.GET("/api/siswa/wirausaha-plan-counts", siswaHandler.ListEntrepreneurPlanCounts)
	authGroup.PUT("/api/siswa/wirausaha-plan-counts", siswaHandler.UpsertEntrepreneurPlanCount)
	authGroup.GET("/api/siswa/plans/list", siswaHandler.ListPlans)
	authGroup.GET("/api/siswa/plans/export.xlsx", siswaHandler.ExportPlans)
	authGroup.GET("/api/siswa/plans/:id", siswaHandler.GetPlans)
	authGroup.PUT("/api/siswa/plans/:id", siswaHandler.UpdatePlans)
	authGroup.POST("/api/siswa/plans/import", siswaHandler.ImportPlans)
	authGroup.GET("/api/siswa/outcomes/list", siswaHandler.ListOutcomes)
	authGroup.GET("/api/siswa/outcomes/years", siswaHandler.ListOutcomeYears)
	authGroup.GET("/api/siswa/outcomes/analytics", siswaHandler.OutcomesAnalytics)
	authGroup.GET("/api/siswa/outcomes/dashboard", siswaHandler.OutcomesDashboard)
	authGroup.GET("/api/siswa/status-lulusan/distribution", siswaHandler.StatusDistribution)
	authGroup.GET("/api/siswa/outcomes/export.xlsx", siswaHandler.ExportOutcomes)
	authGroup.GET("/api/siswa/pt/ranking", siswaHandler.PTRanking)
	authGroup.GET("/api/siswa/status-lulusan/analytics-pt", siswaHandler.StatusLulusanAnalyticsPT)
	authGroup.GET("/api/reports/akreditasi.pdf", reportHandler.DownloadAkreditasiPDF)
	authGroup.GET("/api/reports/prestasi.pdf", reportHandler.DownloadPrestasiPDF)
	authGroup.GET("/api/reports/siswa.pdf", reportHandler.DownloadSiswaPDF)
	authGroup.GET("/api/reports/pt-ranking.pdf", reportHandler.DownloadPTRankingPDF)
	authGroup.GET("/api/reports/bundle.pdf", reportHandler.DownloadBundlePDF)
	authGroup.POST("/api/reports/bundle/jobs", reportHandler.CreateBundleJob)
	authGroup.GET("/api/reports/jobs/:id", reportHandler.GetReportJobStatus)
	authGroup.GET("/api/reports/jobs/:id/download", reportHandler.DownloadReportJob)
	authGroup.GET("/api/siswa/outcomes/import-history", siswaHandler.OutcomesImportHistory)
	authGroup.GET("/api/siswa/outcomes/:id", siswaHandler.GetOutcome)
	authGroup.PUT("/api/siswa/outcomes/:id", siswaHandler.UpdateOutcome)
	authGroup.POST("/api/siswa/outcomes/import", siswaHandler.ImportOutcomes)
	authGroup.POST("/api/siswa/status-lulusan/import", siswaHandler.ImportStatusLulusan)
	authGroup.GET("/api/siswa/outcomes/template.xlsx", siswaHandler.ExportOutcomeTemplate)
	authGroup.GET("/api/siswa/tutup-buku", siswaHandler.GetGraduationYearLock)
	authGroup.POST("/api/siswa/tutup-buku", siswaHandler.CloseGraduationYear)
	authGroup.GET("/api/siswa/validation/pending", siswaHandler.ValidationPending)
	authGroup.GET("/api/siswa/validation/summary", siswaHandler.ValidationSummary)
	authGroup.POST("/api/siswa/validation/finalize-bulk", siswaHandler.ValidationFinalizeBulk)
	authGroup.POST("/api/siswa/finalize", siswaHandler.Finalize)
	authGroup.POST("/api/siswa/finalize/cancel", siswaHandler.CancelFinal)
	authGroup.POST("/api/siswa/acceptance-offers/cancel", siswaHandler.CancelAcceptanceOffers)
	authGroup.POST("/api/siswa/applications/cancel", siswaHandler.CancelApplications)
	authGroup.POST("/api/siswa/applications/cancel-path", siswaHandler.CancelApplicationsPath)
	authGroup.POST("/api/siswa/wirausaha/cancel", siswaHandler.CancelWirausaha)
	authGroup.GET("/api/siswa/analytics/higher-ed", siswaHandler.AnalyticsHigherEd)
	authGroup.POST("/api/siswa/higher-ed/final/import", siswaHandler.ImportHistoricalFinal)
	authGroup.POST("/api/siswa/final/work/import", siswaHandler.ImportLegacyWork)
	authGroup.POST("/api/siswa/final/entrepreneur/import", siswaHandler.ImportLegacyEntrepreneur)
	authGroup.POST("/api/siswa/final/undefined/import", siswaHandler.ImportLegacyUndefined)
	authGroup.GET("/api/siswa/import-reports/:id/rejected.xlsx", siswaHandler.DownloadRejectedImportReport)
	authGroup.GET("/api/uploads", uploadHandler.List)
	authGroup.POST("/api/uploads", uploadHandler.Create)
	authGroup.GET("/api/uploads/:id/rejected.xlsx", uploadHandler.DownloadRejectedReport)
	authGroup.GET("/api/histori", uploadHandler.ListHistory)
	authGroup.DELETE("/api/histori/:id", uploadHandler.DeleteHistory)
	authGroup.GET("/api/user/settings", schoolHandler.Settings)
	authGroup.PUT("/api/user/settings", schoolHandler.UpdateSettings)
	authGroup.GET("/user/me/schools", schoolHandler.MeSchools)
	authGroup.PATCH("/api/user/me/password", schoolHandler.ChangeMyPassword)
	authGroup.GET("/api/user/me/school-monitoring", schoolHandler.MeSchoolMonitoring)
	authGroup.GET("/api/user/me/ops-monitoring-schools", schoolHandler.MeOpsMonitoringSchools)
	authGroup.GET("/api/user/me/ops-monitoring-schools/export.xlsx", schoolHandler.MeOpsMonitoringSchoolsExportXLSX)
	authGroup.GET("/user/settings", schoolHandler.Settings)
	authGroup.PUT("/user/settings", schoolHandler.UpdateSettings)
	authGroup.GET("/schools", schoolHandler.List)
	authGroup.PUT("/schools/:id", schoolHandler.Update)
	authGroup.GET("/achievements", achievementHandler.List)
	authGroup.GET("/admissions", admissionHandler.List)
	authGroup.GET("/admissions/summary", admissionHandler.Summary)
	authGroup.GET("/admissions/pt-summary", admissionHandler.SummaryByUniversity)
	authGroup.GET("/uploads", uploadHandler.List)
	authGroup.POST("/uploads", uploadHandler.Create)
	authGroup.POST("/admin/users", adminHandler.CreateUser)
	authGroup.GET("/admin/school-admins", adminHandler.ListSchoolAdmins)
	authGroup.PUT("/admin/school-admins/:id", adminHandler.UpdateSchoolAdmin)
	authGroup.PUT("/admin/school-admins/:id/password", adminHandler.UpdateSchoolAdminPassword)
	authGroup.POST("/admin/school-admins/:id/reveal-password", adminHandler.RevealSchoolAdminPassword)
	authGroup.POST("/admin/school-admins/export", adminHandler.ExportSchoolAdminCredentials)
	authGroup.GET("/admin/schools", adminHandler.ListSchools)
	authGroup.PUT("/admin/schools/:id/supervisor", adminHandler.UpdateSchoolSupervisor)
	authGroup.GET("/admin/pengawas", adminHandler.ListPengawas)
	authGroup.POST("/admin/pengawas", adminHandler.CreatePengawas)
	authGroup.PUT("/admin/pengawas/:id", adminHandler.UpdatePengawas)
	authGroup.POST("/admin/pengawas/:id/reset-password", adminHandler.ResetPengawasPassword)
	authGroup.PUT("/admin/pengawas/:id/schools", adminHandler.UpdatePengawasSchools)
	authGroup.POST("/admin/school-admins", adminHandler.CreateSchoolAdmin)
	authGroup.POST("/admin/school-admins/bulk-generate", adminHandler.GenerateSchoolAdmins)
	authGroup.GET("/api/admin/master/cabdin-admins", adminHandler.ListCabdinAdmins)
	authGroup.POST("/api/admin/master/cabdin-admins", adminHandler.CreateCabdinAdmin)
	authGroup.PATCH("/api/admin/master/cabdin-admins/:id", adminHandler.UpdateCabdinAdminLevels)
	authGroup.PATCH("/api/admin/master/cabdin-admins/:id/password", adminHandler.ResetCabdinAdminPassword)
	authGroup.GET("/api/admin/master/ref-kampus", adminHandler.ListRefKampus)
	authGroup.POST("/api/admin/master/ref-kampus", adminHandler.CreateRefKampus)
	authGroup.PATCH("/api/admin/master/ref-kampus/:id", adminHandler.UpdateRefKampus)
	authGroup.GET("/api/admin/master/pt-choice-rules", adminHandler.ListPTChoiceRules)
	authGroup.POST("/api/admin/master/pt-choice-rules", adminHandler.CreatePTChoiceRule)
	authGroup.PATCH("/api/admin/master/pt-choice-rules/:id", adminHandler.UpdatePTChoiceRule)
	authGroup.POST("/api/admin/master/school-locations/import/preview", schoolHandler.PreviewLocationImport)
	authGroup.POST("/api/admin/master/school-locations/import/apply", schoolHandler.ApplyLocationImport)
	authGroup.GET("/api/admin/master/landing-settings", landingSettingsHandler.GetAdmin)
	authGroup.PUT("/api/admin/master/landing-settings", landingSettingsHandler.UpdateAdmin)
	authGroup.GET("/api/admin/storage/diagnostics/lomba-hardiknas", hardiknasHandler.StorageDiagnostics)
	authGroup.GET("/api/admin/master/legacy-import/settings", legacyImportHandler.GetMasterSettings)
	authGroup.PUT("/api/admin/master/legacy-import/settings", legacyImportHandler.UpdateMasterSettings)
	authGroup.PATCH("/api/admin/master/legacy-import/settings", legacyImportHandler.UpdateMasterSettings)
	authGroup.PUT("/api/admin/master/legacy-import/levels", legacyImportHandler.UpdateMasterLevels)
	authGroup.PATCH("/api/admin/master/legacy-import/levels", legacyImportHandler.UpdateMasterLevels)
	authGroup.GET("/api/admin/cabdin/legacy-import/levels", legacyImportHandler.GetCabdinLevels)
	authGroup.PUT("/api/admin/cabdin/legacy-import/levels", legacyImportHandler.UpdateCabdinLevels)
	authGroup.PATCH("/api/admin/cabdin/legacy-import/levels", legacyImportHandler.UpdateCabdinLevels)
	authGroup.GET("/api/admin/legacy-migrations/pt/status", legacyMigrationHandler.LegacyPTStatus)
	authGroup.GET("/api/admin/legacy-migrations/pt/report.xlsx", legacyMigrationHandler.LegacyPTReport)
	authGroup.GET("/api/admin/legacy-migrations/pt-outcome-offers/audit", legacyMigrationHandler.PTOutcomeOfferAudit)
	authGroup.POST("/api/admin/legacy-migrations/pt-outcome-offers/backfill", legacyMigrationHandler.PTOutcomeOfferBackfill)
	authGroup.GET("/api/legacy-import/status", legacyImportHandler.GetStatus)
	authGroup.GET("/api/legacy-import/templates/:type", legacyImportHandler.DownloadTemplate)
	authGroup.GET("/api/templates/ukk", siswaHandler.DownloadUKKTemplate)
	authGroup.GET("/api/guide-books", guideBookHandler.List)
	authGroup.GET("/api/guide-books/placement/:placement", guideBookHandler.GetActiveByPlacement)
	authGroup.GET("/api/guide-books/images/:file", guideBookHandler.ServeImage)
	authGroup.GET("/api/admin/guide-books", guideBookHandler.ListAdmin)
	authGroup.POST("/api/admin/guide-books", guideBookHandler.CreateAdmin)
	authGroup.PUT("/api/admin/guide-books/:id", guideBookHandler.UpdateAdmin)
	authGroup.DELETE("/api/admin/guide-books/:id", guideBookHandler.DeleteAdmin)
	authGroup.POST("/api/admin/guide-books/images", guideBookHandler.UploadImageAdmin)
	authGroup.GET("/api/admin/informasi-pelayanan-cabdin", serviceInfoHandler.AdminList)
	authGroup.POST("/api/admin/informasi-pelayanan-cabdin", serviceInfoHandler.Upsert)
	authGroup.PUT("/api/admin/informasi-pelayanan-cabdin", serviceInfoHandler.Upsert)
	authGroup.GET("/api/admin/informasi-pelayanan-cabdin/delegates", serviceInfoHandler.ListDelegates)
	authGroup.PUT("/api/admin/informasi-pelayanan-cabdin/delegates", serviceInfoHandler.SetDelegates)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logging.Infof("server", "listen", "addr=:%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logging.Fatalf("server", "listen", "err=%v", err)
	}
}

func shouldAutoMigrate() bool {
	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	isProd := strings.EqualFold(env, "production")
	isZeabur := false
	for _, key := range []string{"ZEABUR", "ZEABUR_ENV", "ZEABUR_PROJECT_ID", "ZEABUR_SERVICE_ID"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			isZeabur = true
			break
		}
	}
	if !isProd && !isZeabur {
		return false
	}

	autoMigrate := strings.TrimSpace(os.Getenv("AUTO_MIGRATE"))
	if autoMigrate == "" {
		return true
	}
	enabled, err := strconv.ParseBool(autoMigrate)
	if err != nil {
		return true
	}
	return enabled
}

func runMigrations(databaseURL string) error {
	migrationsDir := resolveMigrationsDir()
	log.Printf("[migrate] running migrations dir=%s", migrationsDir)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		return err
	}
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		return err
	}
	log.Printf("[migrate] migrations complete")
	return nil
}

func resolveMigrationsDir() string {
	if custom := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")); custom != "" {
		return custom
	}
	candidates := []string{"db/migrations", "migrations", "backend/migrations"}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return "db/migrations"
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			requestHeaders := c.GetHeader("Access-Control-Request-Headers")
			if requestHeaders == "" {
				requestHeaders = "Authorization, Content-Type, Accept, Origin, X-Requested-With"
			}
			c.Writer.Header().Set("Access-Control-Allow-Headers", requestHeaders)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
			c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
