package handlers

import (
	"bytes"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestComplaintIDsAndPINAreRandomAndHashable(t *testing.T) {
	a, _ := randomToken(7)
	b, _ := randomToken(7)
	if a == b || len(a) != 7 || len(b) != 7 {
		t.Fatal("random token tidak unik/valid")
	}
	pin, _ := randomToken(6)
	hash, _ := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.MinCost)
	if string(hash) == pin || bcrypt.CompareHashAndPassword(hash, []byte(pin)) != nil {
		t.Fatal("PIN tidak di-hash dengan benar")
	}
}

func TestComplaintAttachmentValidation(t *testing.T) {
	png := append([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 32)...)
	if _, ok := validComplaintFile(&multipart.FileHeader{Filename: "bukti.png"}, png); !ok {
		t.Fatal("PNG valid ditolak")
	}
	if _, ok := validComplaintFile(&multipart.FileHeader{Filename: "malware.exe"}, []byte("MZ executable")); ok {
		t.Fatal("executable diterima")
	}
	if _, ok := validComplaintFile(&multipart.FileHeader{Filename: "besar.pdf"}, make([]byte, complaintMaxFile+1)); ok {
		t.Fatal("file >1MB diterima")
	}
}

func TestBackupEncryptionAuthenticated(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "plain")
	dst := filepath.Join(dir, "backup")
	if err := os.WriteFile(src, bytes.Repeat([]byte("sipres"), 300000), 0600); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{7}, 32)
	if err := encryptChunks(src, dst, key); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("sipres")) {
		t.Fatal("plaintext bocor pada backup")
	}
}

func TestClassifyPgDumpFailure(t *testing.T) {
	tests := []struct {
		name, detail, want string
	}{
		{"version mismatch", "aborting because of server version mismatch", backupErrClientIncompatible},
		{"connection refused", `pg_dump: error: connection to server at "db" failed: Connection refused`, backupErrDatabaseConnection},
		{"authentication", `pg_dump: error: password authentication failed for user "app"`, backupErrDatabaseConnection},
		{"generic dump error", "pg_dump: error: query failed: permission denied", backupErrDumpProcess},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyPgDumpFailure(tt.detail); got != tt.want {
				t.Fatalf("classifyPgDumpFailure() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPostgresVersionNumber(t *testing.T) {
	tests := []struct {
		name, output, want string
	}{
		{"pg_dump", "pg_dump (PostgreSQL) 17.5", "17.5"},
		{"packaged client", "pg_dump (PostgreSQL) 16.4 (Ubuntu 16.4-1.pgdg22.04+1)", "16.4"},
		{"server", "15.13 (Debian 15.13-1.pgdg120+1)", "15.13"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := postgresVersionNumber(tt.output)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("postgresVersionNumber() = %q, want %q", got, tt.want)
			}
		})
	}

	if _, err := postgresVersionNumber("unknown"); err == nil {
		t.Fatal("output tanpa nomor versi seharusnya ditolak")
	}
}

func TestSanitizeBackupDiagnosticRedactsSecrets(t *testing.T) {
	databaseURL := "postgresql://backup_user:very-secret-password@db.internal:5432/app"
	encryptionKey := "backup-encryption-key-secret"
	token := "runtime-token-secret"
	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("BACKUP_ENCRYPTION_KEY", encryptionKey)
	t.Setenv("SERVICE_TOKEN", token)

	detail := "failed url=" + databaseURL + " key=" + encryptionKey + " token=" + token
	got := sanitizeBackupDiagnostic(detail, databaseURL, encryptionKey)
	for _, secret := range []string{databaseURL, "very-secret-password", encryptionKey, token} {
		if strings.Contains(got, secret) {
			t.Fatalf("diagnostic membocorkan rahasia %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("diagnostic tidak menandai hasil redaksi: %s", got)
	}
}
