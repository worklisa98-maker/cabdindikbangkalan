package services

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
)

func TestHardiknasCutoffBoundary(t *testing.T) {
	jakarta := JakartaLocation()

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{
			name: "before_cutoff_is_open",
			now:  time.Date(2026, 4, 24, 11, 59, 59, 0, jakarta),
			want: false,
		},
		{
			name: "exact_cutoff_is_closed",
			now:  time.Date(2026, 4, 24, 12, 0, 0, 0, jakarta),
			want: true,
		},
		{
			name: "after_cutoff_is_closed",
			now:  time.Date(2026, 4, 24, 12, 0, 1, 0, jakarta),
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isHardiknasCutoffPassedAt(tc.now); got != tc.want {
				t.Fatalf("expected cutoff passed=%v, got %v", tc.want, got)
			}
		})
	}
}

func TestNormalizeHardiknasTimestamp(t *testing.T) {
	got := normalizeHardiknasTimestamp(time.Date(2026, 4, 22, 7, 59, 0, 0, time.UTC))

	if got.Location().String() != JakartaLocation().String() {
		t.Fatalf("expected Asia/Jakarta location, got %s", got.Location())
	}
	if got.Format(time.RFC3339) != "2026-04-22T14:59:00+07:00" {
		t.Fatalf("unexpected normalized timestamp: %s", got.Format(time.RFC3339))
	}
}

func TestHardiknasCanCreate(t *testing.T) {
	service := HardiknasService{}

	cases := []struct {
		name   string
		scope  hardiknasScope
		passed bool
		want   bool
	}{
		{
			name:  "public_before_cutoff_can_create",
			scope: hardiknasScope{All: true, Role: "PUBLIC"},
			want:  true,
		},
		{
			name:  "school_admin_before_cutoff_can_create",
			scope: hardiknasScope{SchoolIDs: []int{10}, Role: auth.RoleSchoolAdmin},
			want:  true,
		},
		{
			name:  "admin_before_cutoff_cannot_create",
			scope: hardiknasScope{All: true, Role: auth.RoleAdminMaster},
			want:  false,
		},
		{
			name:   "public_after_cutoff_cannot_create",
			scope:  hardiknasScope{All: true, Role: "PUBLIC"},
			passed: true,
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := service.canCreate(tc.scope, tc.passed)
			if got != tc.want {
				t.Fatalf("expected canCreate=%v, got %v", tc.want, got)
			}
		})
	}
}

func TestHardiknasCanDeleteEntry(t *testing.T) {
	service := HardiknasService{}

	schoolAdminEntry := hardiknasEntryRecord{
		ID:            1,
		SchoolID:      10,
		SubmitterType: hardiknasSubmitterTypeSchoolAdmin,
		CreatedBy:     sql.NullInt64{Int64: 99, Valid: true},
	}
	publicEntry := hardiknasEntryRecord{
		ID:            2,
		SchoolID:      10,
		SubmitterType: hardiknasSubmitterTypePublic,
	}

	cases := []struct {
		name   string
		scope  hardiknasScope
		record hardiknasEntryRecord
		passed bool
		want   bool
	}{
		{
			name:   "public_never_can_delete",
			scope:  hardiknasScope{All: true, Role: "PUBLIC"},
			record: publicEntry,
			want:   false,
		},
		{
			name:   "school_admin_can_delete_own_logged_in_entry",
			scope:  hardiknasScope{SchoolIDs: []int{10}, Role: auth.RoleSchoolAdmin},
			record: schoolAdminEntry,
			want:   true,
		},
		{
			name:   "school_admin_cannot_delete_public_entry_same_school",
			scope:  hardiknasScope{SchoolIDs: []int{10}, Role: auth.RoleSchoolAdmin},
			record: publicEntry,
			want:   false,
		},
		{
			name:  "school_admin_cannot_delete_entry_without_creator",
			scope: hardiknasScope{SchoolIDs: []int{10}, Role: auth.RoleSchoolAdmin},
			record: hardiknasEntryRecord{
				ID:            3,
				SchoolID:      10,
				SubmitterType: hardiknasSubmitterTypeSchoolAdmin,
			},
			want: false,
		},
		{
			name:   "admin_cabdin_can_delete_public_entry_in_scope",
			scope:  hardiknasScope{SchoolIDs: []int{10, 20}, Role: auth.RoleAdminCabdin},
			record: publicEntry,
			want:   true,
		},
		{
			name:   "admin_cabdin_cannot_delete_school_admin_entry",
			scope:  hardiknasScope{SchoolIDs: []int{10, 20}, Role: auth.RoleAdminCabdin},
			record: schoolAdminEntry,
			want:   false,
		},
		{
			name:   "admin_master_can_delete_public_entry_globally",
			scope:  hardiknasScope{All: true, Role: auth.RoleAdminMaster},
			record: publicEntry,
			want:   true,
		},
		{
			name:   "any_role_after_cutoff_cannot_delete",
			scope:  hardiknasScope{All: true, Role: auth.RoleAdminMaster},
			record: publicEntry,
			passed: true,
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := service.canDeleteEntry(tc.scope, tc.record, tc.passed)
			if got != tc.want {
				t.Fatalf("expected canDelete=%v, got %v", tc.want, got)
			}
		})
	}
}

func TestNormalizeHardiknasStoredPath(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "relative_path_is_kept",
			input: "lomba-hardiknas/original/file.jpg",
			want:  "lomba-hardiknas/original/file.jpg",
		},
		{
			name:  "uploads_prefix_is_stripped",
			input: "uploads/lomba-hardiknas/thumbnail/file_thumb.jpg",
			want:  "lomba-hardiknas/thumbnail/file_thumb.jpg",
		},
		{
			name:  "legacy_app_uploads_absolute_path_is_normalized",
			input: "/app/uploads/lomba-hardiknas/original/file.jpg",
			want:  "lomba-hardiknas/original/file.jpg",
		},
		{
			name:  "windows_absolute_path_is_normalized",
			input: `C:\data\uploads\lomba-hardiknas\thumbnail\file_thumb.jpg`,
			want:  "lomba-hardiknas/thumbnail/file_thumb.jpg",
		},
		{
			name:    "path_traversal_is_rejected",
			input:   "../../lomba-hardiknas/original/file.jpg",
			wantErr: true,
		},
		{
			name:    "non_hardiknas_path_is_rejected",
			input:   "reports/file.pdf",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeHardiknasStoredPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got path %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected normalized path %q, got %q", tc.want, got)
			}
		})
	}
}

func TestResolveStoragePathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	service := HardiknasService{StorageLocalDir: root}

	if _, err := service.resolveStoragePath("../../outside.txt"); err == nil {
		t.Fatal("expected resolveStoragePath to reject traversal path")
	}
}

func TestResolveStoragePathNormalizesLegacyPathIntoConfiguredRoot(t *testing.T) {
	root := t.TempDir()
	service := HardiknasService{StorageLocalDir: root}

	got, err := service.resolveStoragePath("/app/uploads/lomba-hardiknas/original/file.jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(root, "lomba-hardiknas", "original", "file.jpg")
	if got != want {
		t.Fatalf("expected absolute path %q, got %q", want, got)
	}
}

func TestHardiknasStorageDiagnosticsCountsFiles(t *testing.T) {
	root := t.TempDir()
	originalDir := filepath.Join(root, "lomba-hardiknas", "original")
	thumbnailDir := filepath.Join(root, "lomba-hardiknas", "thumbnail")
	if err := os.MkdirAll(originalDir, 0o755); err != nil {
		t.Fatalf("mkdir original dir: %v", err)
	}
	if err := os.MkdirAll(thumbnailDir, 0o755); err != nil {
		t.Fatalf("mkdir thumbnail dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(originalDir, "one.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write original file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(thumbnailDir, "one_thumb.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write thumbnail file: %v", err)
	}

	diagnostics := HardiknasService{StorageLocalDir: root}.StorageDiagnostics()
	if !diagnostics.RootExists {
		t.Fatal("expected storage root to exist")
	}
	if !diagnostics.RootWritable {
		t.Fatal("expected storage root to be writable")
	}
	if diagnostics.OriginalFileCount != 1 {
		t.Fatalf("expected original file count 1, got %d", diagnostics.OriginalFileCount)
	}
	if diagnostics.ThumbnailFileCount != 1 {
		t.Fatalf("expected thumbnail file count 1, got %d", diagnostics.ThumbnailFileCount)
	}
}
