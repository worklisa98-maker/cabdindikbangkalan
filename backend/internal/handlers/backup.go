package handlers

import (
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"github.com/gin-gonic/gin"
)

var backupMagic = []byte("SIPRESBK1")

const (
	backupErrPgDumpMissing      = "pg_dump tidak ditemukan di server"
	backupErrClientIncompatible = "Versi PostgreSQL client tidak kompatibel"
	backupErrDatabaseConnection = "Koneksi database gagal"
	backupErrDumpProcess        = "Proses dump database gagal"
)

var databaseURLCredentialsPattern = regexp.MustCompile(`(?i)(postgres(?:ql)?://[^:\s/@]+:)[^@\s]+(@)`)
var postgresVersionPattern = regexp.MustCompile(`\d+(?:\.\d+)+`)

type BackupHandler struct {
	DB                                                 *db.DB
	DatabaseURL, StorageDir, EncryptionKey, AppVersion string
}

func sanitizeBackupDiagnostic(detail string, secrets ...string) string {
	safe := strings.TrimSpace(detail)
	safe = strings.NewReplacer("\r\n", `\n`, "\n", `\n`, "\r", `\r`).Replace(safe)
	safe = databaseURLCredentialsPattern.ReplaceAllString(safe, `${1}[REDACTED]${2}`)
	for _, secret := range secrets {
		if secret = strings.TrimSpace(secret); secret != "" {
			safe = strings.ReplaceAll(safe, secret, "[REDACTED]")
		}
	}
	for _, env := range os.Environ() {
		name, value, ok := strings.Cut(env, "=")
		upper := strings.ToUpper(name)
		if !ok || strings.TrimSpace(value) == "" ||
			(!strings.Contains(upper, "PASSWORD") &&
				!strings.Contains(upper, "TOKEN") &&
				!strings.Contains(upper, "SECRET") &&
				!strings.Contains(upper, "DATABASE_URL") &&
				!strings.Contains(upper, "ENCRYPTION_KEY")) {
			continue
		}
		safe = strings.ReplaceAll(safe, value, "[REDACTED]")
	}
	if safe == "" {
		return "(tidak ada detail)"
	}
	return safe
}

func classifyPgDumpFailure(detail string) string {
	lower := strings.ToLower(detail)
	switch {
	case strings.Contains(lower, "server version mismatch"),
		strings.Contains(lower, "aborting because of server version"),
		strings.Contains(lower, "unsupported version"):
		return backupErrClientIncompatible
	case strings.Contains(lower, "connection to server"),
		strings.Contains(lower, "could not connect"),
		strings.Contains(lower, "password authentication failed"),
		strings.Contains(lower, "no password supplied"),
		strings.Contains(lower, "could not translate host name"),
		strings.Contains(lower, "database ") && strings.Contains(lower, " does not exist"):
		return backupErrDatabaseConnection
	default:
		return backupErrDumpProcess
	}
}

func postgresVersionNumber(output string) (string, error) {
	version := postgresVersionPattern.FindString(output)
	if version == "" {
		return "", fmt.Errorf("nomor versi PostgreSQL tidak ditemukan")
	}
	return version, nil
}

func decodeBackupKey(raw string) ([]byte, error) {
	if b, e := base64.StdEncoding.DecodeString(raw); e == nil && len(b) == 32 {
		return b, nil
	}
	if b, e := hex.DecodeString(raw); e == nil && len(b) == 32 {
		return b, nil
	}
	return nil, errors.New("BACKUP_ENCRYPTION_KEY harus base64/hex 32 byte")
}
func requireMaster(c *gin.Context) (auth.User, bool) {
	u, e := auth.GetCurrentUser(c)
	if e != nil || u.Role != "ADMIN_MASTER" {
		c.JSON(403, gin.H{"error": "Khusus ADMIN_MASTER"})
		return u, false
	}
	return u, true
}

func (h BackupHandler) Status(c *gin.Context) {
	if _, ok := requireMaster(c); !ok {
		return
	}
	var at *time.Time
	_ = h.DB.Pool.QueryRow(c, `SELECT MAX(completed_at) FROM backup_history WHERE status='SUCCESS'`).Scan(&at)
	rows, _ := h.DB.Pool.Query(c, `SELECT started_at,completed_at,status,size_bytes,checksum_sha256,safe_error,duration_ms FROM backup_history ORDER BY started_at DESC LIMIT 30`)
	defer rows.Close()
	history := []gin.H{}
	for rows.Next() {
		var started time.Time
		var completed *time.Time
		var status string
		var size, duration *int64
		var sum, msg *string
		_ = rows.Scan(&started, &completed, &status, &size, &sum, &msg, &duration)
		history = append(history, gin.H{"startedAt": started, "completedAt": completed, "status": status, "sizeBytes": size, "checksum": sum, "message": msg, "durationMs": duration})
	}
	due := at == nil || time.Since(*at) >= 14*24*time.Hour
	c.JSON(200, gin.H{"data": gin.H{"lastBackupAt": at, "due": due, "configured": h.EncryptionKey != "", "history": history}})
}

type manifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type backupManifest struct {
	CreatedAt  time.Time      `json:"createdAt"`
	AppVersion string         `json:"appVersion"`
	Migration  int            `json:"migrationVersion"`
	Files      []manifestFile `json:"files"`
}

func addZipFile(zw *zip.Writer, source, name string, manifest *backupManifest) error {
	f, e := os.Open(source)
	if e != nil {
		return e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return e
	}
	w, e := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: st.ModTime()})
	if e != nil {
		return e
	}
	hash := sha256.New()
	if _, e = io.Copy(w, io.TeeReader(f, hash)); e != nil {
		return e
	}
	manifest.Files = append(manifest.Files, manifestFile{name, st.Size(), hex.EncodeToString(hash.Sum(nil))})
	return nil
}
func encryptChunks(src, dst string, key []byte) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		return e
	}
	defer out.Close()
	block, e := aes.NewCipher(key)
	if e != nil {
		return e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return e
	}
	if _, e = out.Write(backupMagic); e != nil {
		return e
	}
	buf := make([]byte, 1<<20)
	counter := uint64(0)
	for {
		n, re := in.Read(buf)
		if n > 0 {
			nonce := make([]byte, gcm.NonceSize())
			if _, e = rand.Read(nonce); e != nil {
				return e
			}
			counter++
			aad := []byte(fmt.Sprintf("%d", counter))
			sealed := gcm.Seal(nil, nonce, buf[:n], aad)
			length := uint32(len(sealed))
			header := []byte{byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)}
			if _, e = out.Write(header); e != nil {
				return e
			}
			if _, e = out.Write(nonce); e != nil {
				return e
			}
			if _, e = out.Write(sealed); e != nil {
				return e
			}
		}
		if re == io.EOF {
			break
		}
		if re != nil {
			return re
		}
	}
	return out.Sync()
}

func (h BackupHandler) Download(c *gin.Context) {
	u, ok := requireMaster(c)
	if !ok {
		return
	}
	key, err := decodeBackupKey(h.EncryptionKey)
	if err != nil {
		c.JSON(503, gin.H{"error": "Backup dinonaktifkan: BACKUP_ENCRYPTION_KEY belum valid"})
		return
	}
	start := time.Now()
	var historyID int64
	if err = h.DB.Pool.QueryRow(c, `INSERT INTO backup_history(requested_by,status) VALUES($1,'RUNNING') RETURNING id`, u.ID).Scan(&historyID); err != nil {
		c.JSON(409, gin.H{"error": "Proses backup lain sedang berjalan"})
		return
	}
	fail := func(stage, safeMessage, detail string) {
		log.Printf("[backup] stage=%s failed: %s", stage, sanitizeBackupDiagnostic(detail, h.DatabaseURL, h.EncryptionKey))
		_, _ = h.DB.Pool.Exec(c, `UPDATE backup_history SET status='FAILED',completed_at=NOW(),safe_error=$1,duration_ms=$2 WHERE id=$3`, safeMessage, time.Since(start).Milliseconds(), historyID)
		c.JSON(500, gin.H{"error": safeMessage})
	}
	pgDumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		fail("pg_dump_lookup", backupErrPgDumpMissing, err.Error())
		return
	}
	versionOutput, err := exec.CommandContext(c, pgDumpPath, "--version").CombinedOutput()
	if err != nil {
		fail("pg_dump_version", backupErrClientIncompatible, fmt.Sprintf("%v: %s", err, versionOutput))
		return
	}
	clientVersion, err := postgresVersionNumber(string(versionOutput))
	if err != nil {
		fail("pg_dump_version", backupErrClientIncompatible, err.Error())
		return
	}
	log.Printf("[backup] pg_dump client version: %s", clientVersion)

	var serverVersionOutput string
	if err = h.DB.Pool.QueryRow(c, `SHOW server_version`).Scan(&serverVersionOutput); err != nil {
		fail("postgres_server_version", backupErrDatabaseConnection, err.Error())
		return
	}
	serverVersion, err := postgresVersionNumber(serverVersionOutput)
	if err != nil {
		fail("postgres_server_version", backupErrDatabaseConnection, err.Error())
		return
	}
	log.Printf("[backup] PostgreSQL server version: %s", serverVersion)

	tmp, err := os.MkdirTemp("", "sipres-backup-*")
	if err != nil {
		fail("temporary_storage", "Temporary storage tidak tersedia", err.Error())
		return
	}
	defer os.RemoveAll(tmp)
	dump := filepath.Join(tmp, "database.dump")
	cmd := exec.CommandContext(c, pgDumpPath, "--format=custom", "--no-owner", "--file", dump, h.DatabaseURL)
	if out, e := cmd.CombinedOutput(); e != nil {
		detail := fmt.Sprintf("%v: %s", e, out)
		fail("pg_dump", classifyPgDumpFailure(detail), detail)
		return
	}
	archive := filepath.Join(tmp, "payload.zip")
	zf, e := os.OpenFile(archive, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		fail("archive_create", "Gagal membuat arsip", e.Error())
		return
	}
	zw := zip.NewWriter(zf)
	manifest := backupManifest{CreatedAt: start, AppVersion: h.AppVersion}
	_ = h.DB.Pool.QueryRow(c, `SELECT COALESCE(MAX(version_id),0) FROM goose_db_version WHERE is_applied`).Scan(&manifest.Migration)
	if e = addZipFile(zw, dump, "database/database.dump", &manifest); e == nil {
		_ = filepath.Walk(h.StorageDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(h.StorageDir, path)
			if strings.Contains(filepath.ToSlash(rel), "/backups/") {
				return nil
			}
			return addZipFile(zw, path, filepath.ToSlash(filepath.Join("storage", rel)), &manifest)
		})
	}
	mw, _ := zw.Create("manifest.json")
	_ = json.NewEncoder(mw).Encode(manifest)
	if e = zw.Close(); e == nil {
		e = zf.Close()
	}
	if e != nil {
		fail("archive_finalize", "Gagal menyelesaikan arsip", e.Error())
		return
	}
	encrypted := filepath.Join(tmp, "backup.sipres-backup")
	if e = encryptChunks(archive, encrypted, key); e != nil {
		fail("encrypt", "Enkripsi backup gagal", e.Error())
		return
	}
	f, e := os.Open(encrypted)
	if e != nil {
		fail("encrypted_backup_open", "Backup tidak dapat dibaca", e.Error())
		return
	}
	defer f.Close()
	hash := sha256.New()
	st, _ := f.Stat()
	name := fmt.Sprintf("SIPRES-BACKUP-%s.sipres-backup", time.Now().Format("2006-01-02-1504"))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprint(st.Size()))
	if _, e = io.Copy(c.Writer, io.TeeReader(f, hash)); e != nil {
		log.Printf("[backup] stage=download failed: %s", sanitizeBackupDiagnostic(e.Error(), h.DatabaseURL, h.EncryptionKey))
		_, _ = h.DB.Pool.Exec(c, `UPDATE backup_history SET status='FAILED',completed_at=NOW(),safe_error='Pengiriman download terputus',duration_ms=$1 WHERE id=$2`, time.Since(start).Milliseconds(), historyID)
		return
	}
	_, _ = h.DB.Pool.Exec(c, `UPDATE backup_history SET status='SUCCESS',completed_at=NOW(),size_bytes=$1,checksum_sha256=$2,duration_ms=$3 WHERE id=$4`, st.Size(), hex.EncodeToString(hash.Sum(nil)), time.Since(start).Milliseconds(), historyID)
}
