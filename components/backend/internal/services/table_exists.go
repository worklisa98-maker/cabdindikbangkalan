package services

import (
	"context"
	"errors"
	"strings"

	"cabdin-bangkalan-backend/internal/db"
)

func tableExists(ctx context.Context, database *db.DB, table string) (bool, error) {
	if database == nil || database.Pool == nil {
		return false, errors.New("database not initialized")
	}
	name := strings.TrimSpace(table)
	if name == "" {
		return false, errors.New("table name is empty")
	}
	if !strings.Contains(name, ".") {
		name = "public." + name
	}

	var exists bool
	if err := database.Pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func tablesExist(ctx context.Context, database *db.DB, tables ...string) (bool, []string, error) {
	if len(tables) == 0 {
		return true, nil, nil
	}
	missing := make([]string, 0)
	for _, table := range tables {
		exists, err := tableExists(ctx, database, table)
		if err != nil {
			return false, nil, err
		}
		if !exists {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		return false, missing, nil
	}
	return true, nil, nil
}
