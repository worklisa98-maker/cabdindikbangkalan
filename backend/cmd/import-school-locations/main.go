// Explicit initial seed only. Default validates the workbook without connecting
// to a database. --apply updates existing NPSNs, never creating school records.
package main

import (
	"cabdin-bangkalan-backend/internal/config"
	"cabdin-bangkalan-backend/internal/services"
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"log"
	"time"
)

func main() {
	path := flag.String("file", services.LocationSeedPath(), "Seed workbook")
	checkDB := flag.Bool("check-db", false, "Read-only matching and migration audit against configured database")
	apply := flag.Bool("apply", false, "Apply initial locations to configured database")
	flag.Parse()
	_, report, err := services.ReadLocationSeed(*path)
	if err != nil {
		log.Fatal(err)
	}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(encoded))
	if !*apply && !*checkDB {
		fmt.Println("DRY RUN: database unchanged. Use --apply after migration 066.")
		return
	}
	cfg := config.Load()
	// Running from backend/ must also work when only the project-root .env exists.
	// godotenv.Load stops at the first missing file; preserve explicit environment values.
	if cfg.DatabaseURL == "" {
		_ = godotenv.Load("../.env")
		cfg = config.Load()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("database configuration failed")
	}
	defer pool.Close()
	result, err := services.RunLocationImport(ctx, pool, *path, *apply, nil)
	if err != nil {
		log.Fatal(err)
	}
	encoded, _ = json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
}
