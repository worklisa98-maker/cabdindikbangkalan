package config

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv                    string
	DatabaseURL               string
	JWTSecret                 string
	StorageProvider           string
	StorageLocalDir           string
	Port                      string
	MasterEncryptionKey       string
	InitialPasswordKey        string
	FrontendOrigin            string
	ReportTokenTTLSeconds     int
	ChromeExecutablePath      string
	ReportPDFCacheTTL         int
	ReportPDFFileTTLHours     int
	ReportPDFDir              string
	DBPoolMaxConns            int32
	DBPoolMinConns            int32
	DBPoolMaxConnLifetime     time.Duration
	DBPoolMaxConnIdleTime     time.Duration
	DBPoolHealthCheck         time.Duration
	SystemTimeOverrideEnabled bool
	BackupEncryptionKey       string
	TurnstileSecretKey        string
	AppVersion                string
}

var loadEnvOnce sync.Once

func Load() Config {
	loadEnvOnce.Do(func() {
		_ = godotenv.Load(".env", "../.env")
	})

	cfg := Config{
		AppEnv:               os.Getenv("APP_ENV"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		StorageProvider:      os.Getenv("STORAGE_PROVIDER"),
		StorageLocalDir:      os.Getenv("STORAGE_LOCAL_DIR"),
		Port:                 os.Getenv("PORT"),
		MasterEncryptionKey:  os.Getenv("MASTER_ENCRYPTION_KEY"),
		InitialPasswordKey:   os.Getenv("INITIAL_PASSWORD_ENCRYPTION_KEY"),
		FrontendOrigin:       os.Getenv("FRONTEND_ORIGIN"),
		ChromeExecutablePath: os.Getenv("CHROME_EXECUTABLE_PATH"),
		ReportPDFDir:         os.Getenv("REPORT_PDF_DIR"),
		DBPoolMaxConns:       int32(parseEnvInt("DB_POOL_MAX_CONNS", 40)),
		DBPoolMinConns:       int32(parseEnvInt("DB_POOL_MIN_CONNS", 5)),
		DBPoolMaxConnLifetime: parseEnvDuration(
			"DB_POOL_MAX_CONN_LIFETIME",
			30*time.Minute,
		),
		DBPoolMaxConnIdleTime: parseEnvDuration(
			"DB_POOL_MAX_CONN_IDLE_TIME",
			10*time.Minute,
		),
		DBPoolHealthCheck: parseEnvDuration(
			"DB_POOL_HEALTH_CHECK_PERIOD",
			time.Minute,
		),
		SystemTimeOverrideEnabled: parseEnvBool("SYSTEM_TIME_OVERRIDE_ENABLED", false),
		BackupEncryptionKey:       os.Getenv("BACKUP_ENCRYPTION_KEY"),
		TurnstileSecretKey:        os.Getenv("TURNSTILE_SECRET_KEY"),
		AppVersion:                os.Getenv("APP_VERSION"),
	}
	if cfg.FrontendOrigin == "" {
		cfg.FrontendOrigin = os.Getenv("FRONTEND_BASE_URL")
	}
	if ttlRaw := os.Getenv("REPORT_TOKEN_TTL_SECONDS"); ttlRaw != "" {
		if parsed, err := strconv.Atoi(ttlRaw); err == nil && parsed > 0 {
			cfg.ReportTokenTTLSeconds = parsed
		}
	}
	if ttlRaw := os.Getenv("REPORT_PDF_CACHE_TTL_SECONDS"); ttlRaw != "" {
		if parsed, err := strconv.Atoi(ttlRaw); err == nil && parsed > 0 {
			cfg.ReportPDFCacheTTL = parsed
		}
	}
	if ttlRaw := os.Getenv("REPORT_PDF_FILE_TTL_HOURS"); ttlRaw != "" {
		if parsed, err := strconv.Atoi(ttlRaw); err == nil && parsed > 0 {
			cfg.ReportPDFFileTTLHours = parsed
		}
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "dev-secret"
	}
	if cfg.StorageProvider == "" {
		cfg.StorageProvider = "local"
	}
	if cfg.StorageLocalDir == "" {
		cfg.StorageLocalDir = "./uploads"
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.FrontendOrigin == "" {
		cfg.FrontendOrigin = "http://localhost:3000"
	}
	if cfg.ReportTokenTTLSeconds <= 0 {
		cfg.ReportTokenTTLSeconds = 60
	}
	if cfg.ReportPDFCacheTTL <= 0 {
		cfg.ReportPDFCacheTTL = 3600
	}
	if cfg.ReportPDFFileTTLHours <= 0 {
		cfg.ReportPDFFileTTLHours = 48
	}
	if cfg.ReportPDFDir == "" {
		cfg.ReportPDFDir = "./uploads/reports"
	}

	return cfg
}

func parseEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseEnvBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
