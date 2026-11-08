package services

import (
	"context"
	"errors"
	"fmt"
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

func splitTableName(table string) (string, string, error) {
	name := strings.TrimSpace(table)
	if name == "" {
		return "", "", errors.New("table name is empty")
	}
	parts := strings.SplitN(name, ".", 2)
	if len(parts) == 1 {
		return "public", parts[0], nil
	}
	schema := strings.TrimSpace(parts[0])
	tableName := strings.TrimSpace(parts[1])
	if schema == "" || tableName == "" {
		return "", "", fmt.Errorf("invalid table name %q", table)
	}
	return schema, tableName, nil
}

func tableColumnsExist(ctx context.Context, database *db.DB, table string, columns ...string) (map[string]bool, error) {
	if database == nil || database.Pool == nil {
		return nil, errors.New("database not initialized")
	}
	schema, tableName, err := splitTableName(table)
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(columns))
	normalizedColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		name := strings.TrimSpace(column)
		if name == "" {
			return nil, errors.New("column name is empty")
		}
		if _, exists := result[name]; exists {
			continue
		}
		result[name] = false
		normalizedColumns = append(normalizedColumns, name)
	}
	if len(normalizedColumns) == 0 {
		return result, nil
	}

	rows, err := database.Pool.Query(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = $2
		  AND column_name = ANY($3)
	`, schema, tableName, normalizedColumns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var columnName string
		if scanErr := rows.Scan(&columnName); scanErr != nil {
			return nil, scanErr
		}
		result[columnName] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
