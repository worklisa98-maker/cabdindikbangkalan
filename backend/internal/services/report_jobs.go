package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
	"cabdin-bangkalan-backend/internal/utils"
)

const (
	ReportJobTypeBundle = "bundle"

	ReportJobStatusQueued  = "queued"
	ReportJobStatusRunning = "running"
	ReportJobStatusDone    = "done"
	ReportJobStatusFailed  = "failed"
)

const (
	defaultReportPDFCacheTTL = time.Hour
	defaultReportPDFFileTTL  = 48 * time.Hour
	defaultReportPDFDir      = "./uploads/reports"
)

type ReportJob struct {
	ID                string          `json:"id"`
	JobType           string          `json:"job_type"`
	CacheKey          string          `json:"cache_key"`
	RequestedByUserID int64           `json:"requested_by_user_id"`
	RequestedByRole   string          `json:"requested_by_role"`
	RequestedSchoolID *int64          `json:"requested_school_id,omitempty"`
	Params            json.RawMessage `json:"params"`
	Status            string          `json:"status"`
	Progress          int             `json:"progress"`
	ResultPath        *string         `json:"result_path,omitempty"`
	ErrorMessage      *string         `json:"error_message,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type ReportJobService struct {
	DB                    *db.DB
	JWTSecret             string
	FrontendOrigin        string
	ChromeExecutablePath  string
	ReportTokenTTLSeconds int
	CacheTTLSeconds       int
	FileTTLHours          int
	ReportDir             string
}

func (s ReportJobService) EnsureReportDir() error {
	return os.MkdirAll(s.reportDir(), 0o755)
}

func (s ReportJobService) CreateOrReuseBundleJob(
	ctx context.Context,
	user auth.User,
	params map[string]string,
	forceRegenerate bool,
) (ReportJob, error) {
	if s.DB == nil || s.DB.Pool == nil {
		return ReportJob{}, errors.New("database is not configured")
	}
	paramsJSON, _, err := normalizeBundleParams(params)
	if err != nil {
		return ReportJob{}, err
	}

	requestedSchoolID := resolveRequestedSchoolID(user)
	cacheKey := buildBundleCacheKey(
		user.Role,
		requestedSchoolID,
		paramsJSON,
	)

	tx, err := s.DB.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ReportJob{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(user.ID)); err != nil {
		return ReportJob{}, err
	}

	cacheJob, hasCacheJob, err := getJobByCacheKeyTx(ctx, tx, cacheKey, true)
	if err != nil {
		return ReportJob{}, err
	}
	if hasCacheJob && !forceRegenerate && isFreshDoneJob(cacheJob, s.cacheTTL()) {
		if err := tx.Commit(ctx); err != nil {
			return ReportJob{}, err
		}
		committed = true
		return cacheJob, nil
	}

	activeUserJob, hasActiveUserJob, err := getActiveUserJobTx(ctx, tx, int64(user.ID), ReportJobTypeBundle)
	if err != nil {
		return ReportJob{}, err
	}
	if hasActiveUserJob {
		if err := tx.Commit(ctx); err != nil {
			return ReportJob{}, err
		}
		committed = true
		return activeUserJob, nil
	}

	if hasCacheJob && isActiveJobStatus(cacheJob.Status) {
		if err := tx.Commit(ctx); err != nil {
			return ReportJob{}, err
		}
		committed = true
		return cacheJob, nil
	}

	if hasCacheJob {
		if _, err := tx.Exec(ctx, `
			UPDATE report_jobs
			SET
				requested_by_user_id = $2,
				requested_by_role = $3,
				requested_school_id = $4,
				params = $5,
				status = $6,
				progress = 0,
				result_path = NULL,
				error_message = NULL,
				updated_at = NOW()
			WHERE id = $1
		`, cacheJob.ID, int64(user.ID), strings.TrimSpace(user.Role), requestedSchoolID, paramsJSON, ReportJobStatusQueued); err != nil {
			return ReportJob{}, err
		}

		updatedJob, found, err := getJobByIDTx(ctx, tx, cacheJob.ID, true)
		if err != nil {
			return ReportJob{}, err
		}
		if !found {
			return ReportJob{}, pgx.ErrNoRows
		}
		if err := tx.Commit(ctx); err != nil {
			return ReportJob{}, err
		}
		committed = true
		return updatedJob, nil
	}

	jobID, err := generateUUIDv4()
	if err != nil {
		return ReportJob{}, err
	}

	var createdJob ReportJob
	err = scanReportJob(
		tx.QueryRow(ctx, `
			INSERT INTO report_jobs (
				id,
				job_type,
				cache_key,
				requested_by_user_id,
				requested_by_role,
				requested_school_id,
				params,
				status,
				progress
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0)
			RETURNING
				id,
				job_type,
				cache_key,
				requested_by_user_id,
				requested_by_role,
				requested_school_id,
				params,
				status,
				progress,
				result_path,
				error_message,
				created_at,
				updated_at
		`, jobID, ReportJobTypeBundle, cacheKey, int64(user.ID), strings.TrimSpace(user.Role), requestedSchoolID, paramsJSON, ReportJobStatusQueued),
		&createdJob,
	)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			existingJob, getErr := s.GetJobByCacheKey(ctx, cacheKey)
			if getErr == nil {
				return existingJob, nil
			}
		}
		return ReportJob{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ReportJob{}, err
	}
	committed = true

	return createdJob, nil
}

func (s ReportJobService) GetJobByID(ctx context.Context, id string) (ReportJob, error) {
	if s.DB == nil || s.DB.Pool == nil {
		return ReportJob{}, errors.New("database is not configured")
	}
	job, found, err := getJobByID(ctx, s.DB.Pool, strings.TrimSpace(id))
	if err != nil {
		return ReportJob{}, err
	}
	if !found {
		return ReportJob{}, pgx.ErrNoRows
	}
	return job, nil
}

func (s ReportJobService) GetJobByCacheKey(ctx context.Context, cacheKey string) (ReportJob, error) {
	if s.DB == nil || s.DB.Pool == nil {
		return ReportJob{}, errors.New("database is not configured")
	}
	job, found, err := getJobByCacheKey(ctx, s.DB.Pool, cacheKey)
	if err != nil {
		return ReportJob{}, err
	}
	if !found {
		return ReportJob{}, pgx.ErrNoRows
	}
	return job, nil
}

func (s ReportJobService) CanUserAccessJob(user auth.User, job ReportJob) bool {
	if auth.IsAdmin(user) {
		return true
	}
	if int64(user.ID) == job.RequestedByUserID {
		return true
	}
	if auth.IsPengawas(user) && job.RequestedSchoolID != nil {
		targetSchoolID := int(*job.RequestedSchoolID)
		for _, schoolID := range user.SupervisedSchoolIDs {
			if schoolID == targetSchoolID {
				return true
			}
		}
	}
	return false
}

func (s ReportJobService) ResolveReportFilePath(resultPath string) (string, error) {
	trimmed := strings.TrimSpace(resultPath)
	if trimmed == "" {
		return "", errors.New("result path is empty")
	}
	cleanPath := filepath.Clean(trimmed)
	if cleanPath == "." || strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return "", errors.New("invalid report path")
	}
	if filepath.Base(cleanPath) != cleanPath {
		return "", errors.New("invalid report path")
	}

	reportDirAbs, err := filepath.Abs(s.reportDir())
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(reportDirAbs, cleanPath)
	relativeCheck, err := filepath.Rel(reportDirAbs, fullPath)
	if err != nil || strings.HasPrefix(relativeCheck, "..") {
		return "", errors.New("invalid report path")
	}
	return fullPath, nil
}

func (s ReportJobService) StartBundleWorker(ctx context.Context) {
	if s.DB == nil || s.DB.Pool == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			if err := s.processNextBundleJob(ctx); err != nil {
				logging.Errorf("report_jobs", "worker", "err=%v", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s ReportJobService) StartFileCleanup(ctx context.Context) {
	if s.DB == nil || s.DB.Pool == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()

		for {
			if err := s.cleanupExpiredFiles(); err != nil {
				logging.Errorf("report_jobs", "cleanup", "err=%v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s ReportJobService) processNextBundleJob(ctx context.Context) error {
	job, found, err := s.claimNextQueuedBundleJob(ctx)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	if err := s.updateJobProgress(ctx, job.ID, 15); err != nil {
		logging.Warnf("report_jobs", "progress", "job_id=%s err=%v", job.ID, err)
	}

	pdfData, err := s.renderBundleJob(ctx, job)
	if err != nil {
		_ = s.markJobFailed(ctx, job.ID, 15, err.Error())
		return nil
	}

	filename := fmt.Sprintf("%s.pdf", job.ID)
	filePath, err := s.ResolveReportFilePath(filename)
	if err != nil {
		_ = s.markJobFailed(ctx, job.ID, 85, err.Error())
		return nil
	}
	if writeErr := os.WriteFile(filePath, pdfData, 0o644); writeErr != nil {
		_ = s.markJobFailed(ctx, job.ID, 85, writeErr.Error())
		return nil
	}

	if err := s.markJobDone(ctx, job.ID, filename); err != nil {
		return err
	}
	return nil
}

func (s ReportJobService) renderBundleJob(ctx context.Context, job ReportJob) ([]byte, error) {
	if err := s.EnsureReportDir(); err != nil {
		return nil, err
	}

	userSnapshot, err := s.loadRequesterSnapshot(ctx, job.RequestedByUserID)
	if err != nil {
		return nil, fmt.Errorf("load requester snapshot: %w", err)
	}

	var params map[string]string
	if err := json.Unmarshal(job.Params, &params); err != nil {
		return nil, fmt.Errorf("decode job params: %w", err)
	}

	reportPath := strings.TrimSpace(params["report_path"])
	if reportPath == "" {
		reportPath = "/reports/bundle"
	}
	query := url.Values{}
	for key, value := range params {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" || trimmedKey == "report_path" {
			continue
		}
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		query.Set(trimmedKey, trimmedValue)
	}
	if query.Get("export") == "" {
		query.Set("export", "1")
	}
	if encoded := query.Encode(); encoded != "" {
		reportPath += "?" + encoded
	}

	tokenTTL := time.Duration(s.ReportTokenTTLSeconds) * time.Second
	reportToken, err := utils.GenerateTokenWithTTL(
		s.JWTSecret,
		userSnapshot.ID,
		userSnapshot.Role,
		userSnapshot.SchoolID,
		userSnapshot.NamaInstansi,
		userSnapshot.TokenVersion,
		tokenTTL,
	)
	if err != nil {
		return nil, fmt.Errorf("generate report token: %w", err)
	}

	wib, _ := time.LoadLocation("Asia/Jakarta")
	footer := fmt.Sprintf("SIPRES Cabdin Bangkalan | Generated at %s", time.Now().In(wib).Format("02 Jan 2006 15:04 WIB"))

	renderer := PDFReportRenderer{
		FrontendOrigin:       s.FrontendOrigin,
		ChromeExecutablePath: s.ChromeExecutablePath,
	}
	return renderer.RenderScreenshotPDF(ctx, PDFReportInput{
		Path:        reportPath,
		Token:       reportToken,
		FooterText:  footer,
		PaperWidth:  11.69,
		PaperHeight: 8.27,
	})
}

func (s ReportJobService) cleanupExpiredFiles() error {
	if err := s.EnsureReportDir(); err != nil {
		return err
	}

	dirEntries, err := os.ReadDir(s.reportDir())
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-s.fileTTL())
	removed := 0
	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" || strings.ToLower(filepath.Ext(name)) != ".pdf" {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		targetPath, pathErr := s.ResolveReportFilePath(name)
		if pathErr != nil {
			continue
		}
		if removeErr := os.Remove(targetPath); removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
			removed++
		}
	}

	if removed > 0 {
		logging.Infof("report_jobs", "cleanup", "removed=%d", removed)
	}
	return nil
}

func (s ReportJobService) claimNextQueuedBundleJob(ctx context.Context) (ReportJob, bool, error) {
	tx, err := s.DB.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ReportJob{}, false, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	var job ReportJob
	err = scanReportJob(
		tx.QueryRow(ctx, `
			SELECT
				id,
				job_type,
				cache_key,
				requested_by_user_id,
				requested_by_role,
				requested_school_id,
				params,
				status,
				progress,
				result_path,
				error_message,
				created_at,
				updated_at
			FROM report_jobs
			WHERE status = $1
			  AND job_type = $2
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		`, ReportJobStatusQueued, ReportJobTypeBundle),
		&job,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return ReportJob{}, false, commitErr
			}
			committed = true
			return ReportJob{}, false, nil
		}
		return ReportJob{}, false, err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE report_jobs
		SET
			status = $2,
			progress = 5,
			error_message = NULL,
			updated_at = NOW()
		WHERE id = $1
	`, job.ID, ReportJobStatusRunning); err != nil {
		return ReportJob{}, false, err
	}

	job.Status = ReportJobStatusRunning
	job.Progress = 5
	job.ErrorMessage = nil
	job.UpdatedAt = time.Now()

	if err := tx.Commit(ctx); err != nil {
		return ReportJob{}, false, err
	}
	committed = true
	return job, true, nil
}

func (s ReportJobService) updateJobProgress(ctx context.Context, jobID string, progress int) error {
	_, err := s.DB.Pool.Exec(ctx, `
		UPDATE report_jobs
		SET
			progress = $2,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(jobID), clampProgress(progress))
	return err
}

func (s ReportJobService) markJobDone(ctx context.Context, jobID string, resultPath string) error {
	_, err := s.DB.Pool.Exec(ctx, `
		UPDATE report_jobs
		SET
			status = $2,
			progress = 100,
			result_path = $3,
			error_message = NULL,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(jobID), ReportJobStatusDone, strings.TrimSpace(resultPath))
	return err
}

func (s ReportJobService) markJobFailed(ctx context.Context, jobID string, progress int, message string) error {
	_, err := s.DB.Pool.Exec(ctx, `
		UPDATE report_jobs
		SET
			status = $2,
			progress = $3,
			error_message = $4,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(jobID), ReportJobStatusFailed, clampProgress(progress), strings.TrimSpace(message))
	return err
}

func (s ReportJobService) loadRequesterSnapshot(ctx context.Context, userID int64) (auth.User, error) {
	var (
		role         string
		namaInstansi string
		tokenVersion int
		schoolIDRaw  sql.NullInt64
	)
	if err := s.DB.Pool.QueryRow(ctx, `
		SELECT role, school_id, COALESCE(nama_instansi, ''), token_version
		FROM users
		WHERE id = $1
	`, userID).Scan(&role, &schoolIDRaw, &namaInstansi, &tokenVersion); err != nil {
		return auth.User{}, err
	}

	var schoolID *int
	if schoolIDRaw.Valid {
		sid := int(schoolIDRaw.Int64)
		schoolID = &sid
	}

	return auth.User{
		ID:           int(userID),
		Role:         strings.TrimSpace(role),
		SchoolID:     schoolID,
		NamaInstansi: strings.TrimSpace(namaInstansi),
		TokenVersion: tokenVersion,
	}, nil
}

func (s ReportJobService) reportDir() string {
	if trimmed := strings.TrimSpace(s.ReportDir); trimmed != "" {
		return trimmed
	}
	return defaultReportPDFDir
}

func (s ReportJobService) cacheTTL() time.Duration {
	if s.CacheTTLSeconds > 0 {
		return time.Duration(s.CacheTTLSeconds) * time.Second
	}
	return defaultReportPDFCacheTTL
}

func (s ReportJobService) fileTTL() time.Duration {
	if s.FileTTLHours > 0 {
		return time.Duration(s.FileTTLHours) * time.Hour
	}
	return defaultReportPDFFileTTL
}

func getJobByID(ctx context.Context, conn queryable, id string) (ReportJob, bool, error) {
	var job ReportJob
	err := scanReportJob(
		conn.QueryRow(ctx, `
			SELECT
				id,
				job_type,
				cache_key,
				requested_by_user_id,
				requested_by_role,
				requested_school_id,
				params,
				status,
				progress,
				result_path,
				error_message,
				created_at,
				updated_at
			FROM report_jobs
			WHERE id = $1
		`, strings.TrimSpace(id)),
		&job,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReportJob{}, false, nil
		}
		return ReportJob{}, false, err
	}
	return job, true, nil
}

func getJobByIDTx(ctx context.Context, tx pgx.Tx, id string, lock bool) (ReportJob, bool, error) {
	query := `
		SELECT
			id,
			job_type,
			cache_key,
			requested_by_user_id,
			requested_by_role,
			requested_school_id,
			params,
			status,
			progress,
			result_path,
			error_message,
			created_at,
			updated_at
		FROM report_jobs
		WHERE id = $1
	`
	if lock {
		query += " FOR UPDATE"
	}
	var job ReportJob
	err := scanReportJob(tx.QueryRow(ctx, query, strings.TrimSpace(id)), &job)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReportJob{}, false, nil
		}
		return ReportJob{}, false, err
	}
	return job, true, nil
}

func getJobByCacheKey(ctx context.Context, conn queryable, cacheKey string) (ReportJob, bool, error) {
	var job ReportJob
	err := scanReportJob(
		conn.QueryRow(ctx, `
			SELECT
				id,
				job_type,
				cache_key,
				requested_by_user_id,
				requested_by_role,
				requested_school_id,
				params,
				status,
				progress,
				result_path,
				error_message,
				created_at,
				updated_at
			FROM report_jobs
			WHERE cache_key = $1
		`, strings.TrimSpace(cacheKey)),
		&job,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReportJob{}, false, nil
		}
		return ReportJob{}, false, err
	}
	return job, true, nil
}

func getJobByCacheKeyTx(ctx context.Context, tx pgx.Tx, cacheKey string, lock bool) (ReportJob, bool, error) {
	query := `
		SELECT
			id,
			job_type,
			cache_key,
			requested_by_user_id,
			requested_by_role,
			requested_school_id,
			params,
			status,
			progress,
			result_path,
			error_message,
			created_at,
			updated_at
		FROM report_jobs
		WHERE cache_key = $1
	`
	if lock {
		query += " FOR UPDATE"
	}
	var job ReportJob
	err := scanReportJob(tx.QueryRow(ctx, query, strings.TrimSpace(cacheKey)), &job)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReportJob{}, false, nil
		}
		return ReportJob{}, false, err
	}
	return job, true, nil
}

func getActiveUserJobTx(ctx context.Context, tx pgx.Tx, userID int64, jobType string) (ReportJob, bool, error) {
	var job ReportJob
	err := scanReportJob(
		tx.QueryRow(ctx, `
			SELECT
				id,
				job_type,
				cache_key,
				requested_by_user_id,
				requested_by_role,
				requested_school_id,
				params,
				status,
				progress,
				result_path,
				error_message,
				created_at,
				updated_at
			FROM report_jobs
			WHERE requested_by_user_id = $1
			  AND job_type = $2
			  AND status IN ($3, $4)
			ORDER BY created_at ASC
			LIMIT 1
			FOR UPDATE
		`, userID, strings.TrimSpace(jobType), ReportJobStatusQueued, ReportJobStatusRunning),
		&job,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReportJob{}, false, nil
		}
		return ReportJob{}, false, err
	}
	return job, true, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

type queryable interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func scanReportJob(scanner rowScanner, target *ReportJob) error {
	var (
		requestedSchoolID sql.NullInt64
		resultPath        sql.NullString
		errorMessage      sql.NullString
		paramsRaw         []byte
		job               ReportJob
	)
	if err := scanner.Scan(
		&job.ID,
		&job.JobType,
		&job.CacheKey,
		&job.RequestedByUserID,
		&job.RequestedByRole,
		&requestedSchoolID,
		&paramsRaw,
		&job.Status,
		&job.Progress,
		&resultPath,
		&errorMessage,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return err
	}
	if requestedSchoolID.Valid {
		value := requestedSchoolID.Int64
		job.RequestedSchoolID = &value
	}
	if resultPath.Valid {
		value := strings.TrimSpace(resultPath.String)
		job.ResultPath = &value
	}
	if errorMessage.Valid {
		value := strings.TrimSpace(errorMessage.String)
		job.ErrorMessage = &value
	}
	job.Params = append([]byte(nil), paramsRaw...)
	*target = job
	return nil
}

func normalizeBundleParams(params map[string]string) ([]byte, map[string]string, error) {
	normalized := map[string]string{
		"report_path": "/reports/bundle",
		"export":      "1",
	}
	for key, value := range params {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" || trimmedKey == "report_path" || trimmedKey == "export" {
			continue
		}
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		normalized[trimmedKey] = trimmedValue
	}
	content, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, err
	}
	return content, normalized, nil
}

func buildBundleCacheKey(role string, requestedSchoolID *int64, paramsJSON []byte) string {
	roleValue := strings.ToUpper(strings.TrimSpace(role))
	if roleValue == "" {
		roleValue = "UNKNOWN"
	}
	schoolValue := "null"
	if requestedSchoolID != nil {
		schoolValue = fmt.Sprintf("%d", *requestedSchoolID)
	}
	sum := sha256.Sum256(paramsJSON)
	return fmt.Sprintf(
		"bundle|role=%s|school_id=%s|params_sha=%x",
		roleValue,
		schoolValue,
		sum[:],
	)
}

func resolveRequestedSchoolID(user auth.User) *int64 {
	if !auth.IsSchoolAdmin(user) || user.SchoolID == nil {
		return nil
	}
	value := int64(*user.SchoolID)
	return &value
}

func isFreshDoneJob(job ReportJob, ttl time.Duration) bool {
	if strings.TrimSpace(job.Status) != ReportJobStatusDone {
		return false
	}
	if job.ResultPath == nil || strings.TrimSpace(*job.ResultPath) == "" {
		return false
	}
	return time.Since(job.UpdatedAt) <= ttl
}

func isActiveJobStatus(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case ReportJobStatusQueued, ReportJobStatusRunning:
		return true
	default:
		return false
	}
}

func isUniqueConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func generateUUIDv4() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	buf := make([]byte, 36)
	hex.Encode(buf[0:8], raw[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], raw[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], raw[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], raw[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], raw[10:16])
	return string(buf), nil
}

func resolveReportUserName(userID int, namaInstansi string) string {
	trimmed := strings.TrimSpace(namaInstansi)
	if trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("User-%d", userID)
}

func clampProgress(progress int) int {
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}
