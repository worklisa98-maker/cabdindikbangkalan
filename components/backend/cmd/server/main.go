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
	"cabdin-bangkalan-backend/internal/storage"
	"cabdin-bangkalan-backend/internal/utils"
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
	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logging.Fatalf("server", "db_connect", "err=%v", err)
	}
	defer database.Close()
	db.BootstrapAdminMaster(ctx, database)

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
	templateHandler := handlers.TemplateHandler{}
	publicHandler := handlers.PublicHandler{DB: database}
	dashboardHandler := handlers.DashboardHandler{DB: database}
	healthHandler := handlers.HealthHandler{DB: database}
	legacyImportHandler := handlers.LegacyImportHandler{DB: database}
	legacyMigrationHandler := handlers.LegacyMigrationHandler{DB: database}

	router.POST("/auth/login", authHandler.Login)
	router.POST("/auth/logout", authHandler.Logout)
	router.GET("/healthz", healthHandler.Check)
	router.GET("/templates/:type", templateHandler.Download)
	router.GET("/api/meta/last-updated", publicHandler.MetaLastUpdated)
	router.GET("/api/public/dashboard", publicHandler.Dashboard)
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

	authGroup := router.Group("/")
	authGroup.Use(middleware.AuthMiddleware(cfg.JWTSecret, database))
	authGroup.Use(middleware.ActivityMiddleware(database))
	authGroup.GET("/auth/me", authHandler.Me)
	authGroup.GET("/api/dashboard", dashboardHandler.Summary)
	authGroup.GET("/api/internal/dashboard", dashboardHandler.Summary)
	authGroup.GET("/api/sekolah", schoolHandler.List)
	authGroup.GET("/api/admin/schools/suggest", adminHandler.SuggestSchools)
	authGroup.GET("/api/admin/sekolah/table", adminHandler.ListMonitoringSchools)
	authGroup.GET("/api/admin/sekolah/table/export.xlsx", adminHandler.ExportMonitoringSchools)
	authGroup.POST("/api/admin/reset-data", adminHandler.ResetSchoolData)
	authGroup.POST("/api/admin/change-password", adminHandler.ChangePassword)
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
	authGroup.POST("/api/siswa/applications/import", siswaHandler.ImportApplications)
	authGroup.GET("/api/siswa/applications/list", siswaHandler.ListApplications)
	authGroup.GET("/api/siswa/applications/pipeline", siswaHandler.ListApplicationsPipeline)
	authGroup.GET("/api/siswa/applications/detail", siswaHandler.GetApplicationsDetail)
	authGroup.PUT("/api/siswa/applications", siswaHandler.UpdateApplications)
	authGroup.GET("/api/siswa/applications/template.xlsx", siswaHandler.ExportApplicationsTemplate)
	authGroup.POST("/api/siswa/acceptance-offers/import", siswaHandler.ImportAcceptanceOffers)
	authGroup.POST("/api/siswa/ukk/import", siswaHandler.ImportUKK)
	authGroup.GET("/api/siswa/ukk/list", siswaHandler.ListUKK)
	authGroup.PUT("/api/siswa/ukk", siswaHandler.UpdateUKK)
	authGroup.GET("/api/siswa/wirausaha/list", siswaHandler.ListWirausahaPlans)
	authGroup.GET("/api/siswa/wirausaha/detail", siswaHandler.GetWirausahaPlans)
	authGroup.PUT("/api/siswa/wirausaha", siswaHandler.UpdateWirausahaPlans)
	authGroup.POST("/api/siswa/wirausaha/import", siswaHandler.ImportWirausahaPlans)
	authGroup.GET("/api/siswa/wirausaha/template.xlsx", siswaHandler.ExportWirausahaTemplate)
	authGroup.GET("/api/siswa/plans/list", siswaHandler.ListPlans)
	authGroup.GET("/api/siswa/plans/export.xlsx", siswaHandler.ExportPlans)
	authGroup.GET("/api/siswa/plans/:id", siswaHandler.GetPlans)
	authGroup.PUT("/api/siswa/plans/:id", siswaHandler.UpdatePlans)
	authGroup.POST("/api/siswa/plans/import", siswaHandler.ImportPlans)
	authGroup.GET("/api/siswa/outcomes/list", siswaHandler.ListOutcomes)
	authGroup.GET("/api/siswa/outcomes/analytics", siswaHandler.OutcomesAnalytics)
	authGroup.GET("/api/siswa/outcomes/dashboard", siswaHandler.OutcomesDashboard)
	authGroup.GET("/api/siswa/outcomes/import-history", siswaHandler.OutcomesImportHistory)
	authGroup.GET("/api/siswa/outcomes/:id", siswaHandler.GetOutcome)
	authGroup.PUT("/api/siswa/outcomes/:id", siswaHandler.UpdateOutcome)
	authGroup.POST("/api/siswa/outcomes/import", siswaHandler.ImportOutcomes)
	authGroup.GET("/api/siswa/outcomes/template.xlsx", siswaHandler.ExportOutcomeTemplate)
	authGroup.GET("/api/siswa/tutup-buku", siswaHandler.GetGraduationYearLock)
	authGroup.POST("/api/siswa/tutup-buku", siswaHandler.CloseGraduationYear)
	authGroup.GET("/api/siswa/validation/pending", siswaHandler.ValidationPending)
	authGroup.GET("/api/siswa/validation/summary", siswaHandler.ValidationSummary)
	authGroup.POST("/api/siswa/validation/finalize-bulk", siswaHandler.ValidationFinalizeBulk)
	authGroup.POST("/api/siswa/finalize", siswaHandler.Finalize)
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
	authGroup.POST("/admin/school-admins", adminHandler.CreateSchoolAdmin)
	authGroup.POST("/admin/school-admins/bulk-generate", adminHandler.GenerateSchoolAdmins)
	authGroup.GET("/api/admin/master/cabdin-admins", adminHandler.ListCabdinAdmins)
	authGroup.POST("/api/admin/master/cabdin-admins", adminHandler.CreateCabdinAdmin)
	authGroup.PATCH("/api/admin/master/cabdin-admins/:id", adminHandler.UpdateCabdinAdminLevels)
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
	authGroup.GET("/api/legacy-import/status", legacyImportHandler.GetStatus)
	authGroup.GET("/api/legacy-import/templates/:type", legacyImportHandler.DownloadTemplate)
	authGroup.GET("/api/templates/ukk", siswaHandler.DownloadUKKTemplate)

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
