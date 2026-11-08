package db

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/utils"
)

const defaultAdminMasterInstansi = "Cabdin Master"

func BootstrapAdminMaster(ctx context.Context, database *DB) {
	if database == nil || database.Pool == nil {
		return
	}

	username := strings.TrimSpace(os.Getenv("INIT_ADMIN_MASTER_USERNAME"))
	password := strings.TrimSpace(os.Getenv("INIT_ADMIN_MASTER_PASSWORD"))
	if username == "" || password == "" {
		return
	}
	namaInstansi := strings.TrimSpace(os.Getenv("INIT_ADMIN_MASTER_NAMA_INSTANSI"))
	if namaInstansi == "" {
		namaInstansi = defaultAdminMasterInstansi
	}

	var count int
	if err := database.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'ADMIN_MASTER'`).Scan(&count); err != nil {
		logging.Errorf("bootstrap", "admin-master", "err=%v", err)
		return
	}
	if count > 0 {
		logging.Infof("bootstrap", "admin-master", "ensured")
		return
	}

	var existingRole string
	err := database.Pool.QueryRow(ctx, `SELECT role FROM users WHERE username = $1`, username).Scan(&existingRole)
	if err == nil {
		if existingRole == "ADMIN_MASTER" {
			logging.Infof("bootstrap", "admin-master", "ensured")
			return
		}
		logging.Warnf("bootstrap", "admin-master", "username_taken role=%s", existingRole)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		logging.Errorf("bootstrap", "admin-master", "err=%v", err)
		return
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		logging.Errorf("bootstrap", "admin-master", "err=%v", err)
		return
	}

	if _, err := database.Pool.Exec(ctx, `
        INSERT INTO users (username, password_hash, role, nama_instansi)
        VALUES ($1, $2, 'ADMIN_MASTER', $3)
        ON CONFLICT (username) DO NOTHING
    `, username, hash, namaInstansi); err != nil {
		logging.Errorf("bootstrap", "admin-master", "err=%v", err)
		return
	}

	logging.Infof("bootstrap", "admin-master", "ensured")
}
