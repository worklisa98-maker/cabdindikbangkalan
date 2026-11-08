package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
)

// TestSaveActivityPostgresEndToEnd exercises the same handler and JSON payload
// used by the frontend. It is opt-in because it creates and removes isolated
// fixture rows in a PostgreSQL database migrated through version 63.
func TestSaveActivityPostgresEndToEnd(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("MGMP_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set MGMP_TEST_DATABASE_URL to run the PostgreSQL MGMP activity integration test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	var schemaVersion int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(version_id),0) FROM goose_db_version WHERE is_applied=TRUE`).Scan(&schemaVersion); err != nil || schemaVersion < 63 {
		t.Fatalf("database schema must be migrated through version 63: version=%d err=%v", schemaVersion, err)
	}
	var untypedHistoryJSON []byte
	untypedErr := pool.QueryRow(ctx, `SELECT jsonb_build_object('title',$1)`, "activity").Scan(&untypedHistoryJSON)
	var pgErr *pgconn.PgError
	if !errors.As(untypedErr, &pgErr) || pgErr.Code != "42P18" {
		t.Fatalf("expected the former untyped history parameter to fail with 42P18, got %v", untypedErr)
	}

	marker := fmt.Sprintf("codex-mgmp-activity-%d", time.Now().UnixNano())
	var userID int
	if err := pool.QueryRow(ctx, `INSERT INTO users(username,password_hash,role,nama_instansi) VALUES($1,'integration-test','USER_MGMP',$2) RETURNING id`, marker, marker).Scan(&userID); err != nil {
		t.Fatalf("insert fixture user: %v", err)
	}
	var groupID int64
	if err := pool.QueryRow(ctx, `INSERT INTO mgmp_groups(organization_type,name,subject,region,status) VALUES('MGMP',$1,$2,$3,'ACTIVE') RETURNING id`, marker, marker, marker).Scan(&groupID); err != nil {
		t.Fatalf("insert fixture group: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM mgmp_history WHERE group_id=$1`, groupID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM mgmp_activity_attendance WHERE activity_id IN (SELECT id FROM mgmp_activities WHERE group_id=$1)`, groupID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM mgmp_activities WHERE group_id=$1`, groupID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM mgmp_monthly_statuses WHERE group_id=$1`, groupID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM mgmp_user_links WHERE group_id=$1`, groupID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM mgmp_members WHERE group_id=$1`, groupID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM mgmp_groups WHERE id=$1`, groupID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})

	if _, err := pool.Exec(ctx, `INSERT INTO mgmp_user_links(group_id,user_id,position) VALUES($1,$2,'KETUA')`, groupID, userID); err != nil {
		t.Fatalf("insert fixture link: %v", err)
	}
	memberIDs := make([]int64, 5)
	for i := range memberIDs {
		if err := pool.QueryRow(ctx, `INSERT INTO mgmp_members(group_id,full_name,school_name,position,email,whatsapp,member_status) VALUES($1,$2,$3,'ANGGOTA',$4,$5,'AKTIF') RETURNING id`, groupID, fmt.Sprintf("Member %d", i+1), fmt.Sprintf("School %d", i+1), fmt.Sprintf("%s-%d@example.test", marker, i), fmt.Sprintf("0812345678%02d", i)).Scan(&memberIDs[i]); err != nil {
			t.Fatalf("insert fixture member %d: %v", i+1, err)
		}
	}

	handler := MGMPHandler{DB: &db.DB{Pool: pool}}
	activityDate := "2099-11-15"
	if _, err := pool.Exec(ctx, `INSERT INTO mgmp_monthly_statuses(group_id,year,month,no_agenda_reason,created_by_user_id) VALUES($1,2099,11,'fixture',$2)`, groupID, userID); err != nil {
		t.Fatalf("insert fixture monthly status: %v", err)
	}

	allPresent := make([]map[string]any, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		allPresent = append(allPresent, map[string]any{"memberId": memberID, "status": "PRESENT", "absenceType": "", "absenceNote": ""})
	}
	firstID := postTestActivity(t, handler, userID, map[string]any{
		"title": "Semua Hadir", "activityDate": activityDate, "venue": "Ruang Rapat", "description": "Integration test",
		"attendanceUrl": "https://example.test/attendance/all", "documentationUrl": "https://example.test/documentation/all", "notes": "", "attendance": allPresent,
	})
	assertStoredActivity(t, pool, firstID, 5, 5, 5, "https://example.test/attendance/all", "https://example.test/documentation/all")

	mixed := []map[string]any{
		{"memberId": memberIDs[0], "status": "PRESENT", "absenceType": "", "absenceNote": ""},
		{"memberId": memberIDs[1], "status": "PRESENT", "absenceType": "", "absenceNote": ""},
		{"memberId": memberIDs[2], "status": "PRESENT", "absenceType": "", "absenceNote": ""},
		{"memberId": memberIDs[3], "status": "ABSENT", "absenceType": "EXCUSED", "absenceNote": "Sakit"},
		{"memberId": memberIDs[4], "status": "ABSENT", "absenceType": "UNEXCUSED", "absenceNote": ""},
	}
	secondID := postTestActivity(t, handler, userID, map[string]any{
		"title": "Kehadiran Campuran", "activityDate": activityDate, "venue": "Ruang Rapat", "description": "Integration test",
		"attendanceUrl": "https://example.test/attendance/mixed", "documentationUrl": "https://example.test/documentation/mixed", "notes": "", "attendance": mixed,
	})
	assertStoredActivity(t, pool, secondID, 3, 5, 5, "https://example.test/attendance/mixed", "https://example.test/documentation/mixed")
	assertActivityHistoryVisible(t, handler, userID)

	var monthlyStatuses int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM mgmp_monthly_statuses WHERE group_id=$1 AND year=2099 AND month=11`, groupID).Scan(&monthlyStatuses); err != nil || monthlyStatuses != 0 {
		t.Fatalf("monthly status was not cleared: count=%d err=%v", monthlyStatuses, err)
	}
}

func assertActivityHistoryVisible(t *testing.T, handler MGMPHandler, userID int) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/mgmp/history", nil)
	auth.SetCurrentUser(c, auth.User{ID: userID, Role: auth.RoleMGMPUser})
	handler.History(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET activity history returned %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []struct {
			EventType string `json:"eventType"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode history response: %v", err)
	}
	activityCreated, attendanceCreated := 0, 0
	for _, item := range response.Data {
		switch item.EventType {
		case "ACTIVITY_CREATED":
			activityCreated++
		case "ATTENDANCE_CREATED":
			attendanceCreated++
		}
	}
	if activityCreated != 2 || attendanceCreated != 2 {
		t.Fatalf("activity history is incomplete: ACTIVITY_CREATED=%d ATTENDANCE_CREATED=%d", activityCreated, attendanceCreated)
	}
}

func postTestActivity(t *testing.T, handler MGMPHandler, userID int, payload map[string]any) int64 {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/mgmp/activities", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	auth.SetCurrentUser(c, auth.User{ID: userID, Role: auth.RoleMGMPUser})
	handler.SaveActivity(c)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST activity returned %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Data.ID == 0 {
		t.Fatalf("decode activity response: id=%d err=%v body=%s", response.Data.ID, err, recorder.Body.String())
	}
	return response.Data.ID
}

func assertStoredActivity(t *testing.T, pool *pgxpool.Pool, activityID int64, participantCount, activeMemberCount, attendanceRows int, attendanceURL, documentationURL string) {
	t.Helper()
	var gotParticipants, gotActiveMembers, gotAttendanceRows, gotHistoryRows int
	var gotAttendanceURL, gotDocumentationURL string
	err := pool.QueryRow(context.Background(), `SELECT a.participant_count,a.active_member_count,a.attendance_url,a.documentation_url,(SELECT COUNT(*) FROM mgmp_activity_attendance att WHERE att.activity_id=a.id),(SELECT COUNT(*) FROM mgmp_history h WHERE h.activity_id=a.id AND h.event_type IN ('ACTIVITY_CREATED','ATTENDANCE_CREATED')) FROM mgmp_activities a WHERE a.id=$1`, activityID).Scan(&gotParticipants, &gotActiveMembers, &gotAttendanceURL, &gotDocumentationURL, &gotAttendanceRows, &gotHistoryRows)
	if err != nil {
		t.Fatalf("query stored activity %d: %v", activityID, err)
	}
	if gotParticipants != participantCount || gotActiveMembers != activeMemberCount || gotAttendanceRows != attendanceRows || gotHistoryRows != 2 || gotAttendanceURL != attendanceURL || gotDocumentationURL != documentationURL {
		t.Fatalf("stored activity mismatch: participants=%d active=%d attendance=%d history=%d attendanceURL=%q documentationURL=%q", gotParticipants, gotActiveMembers, gotAttendanceRows, gotHistoryRows, gotAttendanceURL, gotDocumentationURL)
	}
}
