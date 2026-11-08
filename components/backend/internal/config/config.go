package config

import (
    "os"
    "sync"

    "github.com/joho/godotenv"
)

type Config struct {
    DatabaseURL     string
    JWTSecret       string
    StorageProvider string
    StorageLocalDir string
    Port            string
    MasterEncryptionKey string
    InitialPasswordKey  string
}

var loadEnvOnce sync.Once

func Load() Config {
    loadEnvOnce.Do(func() {
        _ = godotenv.Load(".env", "../.env")
    })

    cfg := Config{
        DatabaseURL:     os.Getenv("DATABASE_URL"),
        JWTSecret:       os.Getenv("JWT_SECRET"),
        StorageProvider: os.Getenv("STORAGE_PROVIDER"),
        StorageLocalDir: os.Getenv("STORAGE_LOCAL_DIR"),
        Port:            os.Getenv("PORT"),
        MasterEncryptionKey: os.Getenv("MASTER_ENCRYPTION_KEY"),
        InitialPasswordKey:  os.Getenv("INITIAL_PASSWORD_ENCRYPTION_KEY"),
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

    return cfg
}
