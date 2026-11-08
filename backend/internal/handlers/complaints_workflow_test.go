package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func TestComplaintActionPolicy(t *testing.T) {
	tests := []struct {
		name, role, status, action string
		assigned, want             bool
	}{
		{"master sees and self assigns new", "ADMIN_MASTER", "RECEIVED", "VERIFY_SELF", false, true},
		{"master assigns cabdin with workflow action", "ADMIN_MASTER", "RECEIVED", "ASSIGN", false, true},
		{"cabdin cannot assign", "ADMIN_CABDIN", "RECEIVED", "ASSIGN", true, false},
		{"cabdin cannot process someone else report", "ADMIN_CABDIN", "FOLLOW_UP", "PROGRESS", false, false},
		{"assigned cabdin adds progress", "ADMIN_CABDIN", "FOLLOW_UP", "PROGRESS", true, true},
		{"response precedes completion", "ADMIN_CABDIN", "FOLLOW_UP", "COMPLETE", true, false},
		{"responded report can complete", "ADMIN_CABDIN", "RESPONDED", "COMPLETE", true, true},
		{"cabdin cannot reopen", "ADMIN_CABDIN", "COMPLETED", "REOPEN", true, false},
		{"master can reopen rejected", "ADMIN_MASTER", "REJECTED", "REOPEN", false, true},
		{"only submitted clarification can be accepted", "ADMIN_MASTER", "NEEDS_INFO", "ACCEPT_CLARIFICATION", true, false},
		{"submitted clarification accepted", "ADMIN_CABDIN", "CLARIFICATION_SUBMITTED", "ACCEPT_CLARIFICATION", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := complaintActionAllowed(tt.role, tt.status, tt.action, tt.assigned); got != tt.want {
				t.Fatalf("complaintActionAllowed()=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestComplaintPINVerification(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("A7BC29"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if !verifyComplaintPIN(string(hash), "A7BC29") {
		t.Fatal("correct PIN must verify")
	}
	if verifyComplaintPIN(string(hash), "WRONG1") {
		t.Fatal("wrong PIN must not verify")
	}
}

func TestRespondWithEvidenceRejectsInvalidMultipartBeforeDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0, 'I', 'H', 'D', 'R'}
	tests := []struct {
		name, response, filename string
		data                     []byte
	}{
		{name: "respond without file", response: "Tanggapan resmi"},
		{name: "respond with pdf only", response: "Tanggapan resmi", filename: "bukti.pdf", data: []byte("%PDF-1.4\n1 0 obj\n")},
		{name: "respond without response", filename: "bukti.png", data: png},
		{name: "respond with oversized file", response: "Tanggapan resmi", filename: "besar.png", data: make([]byte, complaintMaxFile+1)},
		{name: "respond with fake extension", response: "Tanggapan resmi", filename: "palsu.jpg", data: []byte("plain text pretending to be an image")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			if tt.response != "" {
				_ = writer.WriteField("response", tt.response)
			}
			if tt.filename != "" {
				part, err := writer.CreateFormFile("attachments", tt.filename)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = part.Write(tt.data); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			ginContext, _ := gin.CreateTestContext(recorder)
			ginContext.Request = httptest.NewRequest(http.MethodPost, "/api/admin/complaints/PGD-TEST/respond", &body)
			ginContext.Request.Header.Set("Content-Type", writer.FormDataContentType())
			ginContext.Params = gin.Params{{Key: "id", Value: "PGD-TEST"}}
			auth.SetCurrentUser(ginContext, auth.User{ID: 101, Role: auth.RoleAdminCabdin})

			ComplaintHandler{}.RespondWithEvidence(ginContext)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestClarificationExpiryReasonIsProfessional(t *testing.T) {
	if strings.Contains(strings.ToLower(clarificationExpiredReason), "hoaks") {
		t.Fatal("expiry reason must never label the report as hoaks")
	}
	if !strings.Contains(clarificationExpiredReason, "batas waktu") {
		t.Fatalf("unexpected expiry reason: %q", clarificationExpiredReason)
	}
}

func TestComplaintAssigneeQueriesMatchUsersSchemaAndEnum(t *testing.T) {
	listQuery := strings.ToUpper(complaintAssigneesQuery)
	validationQuery := strings.ToUpper(complaintAssigneeExistsQuery)

	for name, query := range map[string]string{
		"list":       listQuery,
		"validation": validationQuery,
	} {
		t.Run(name, func(t *testing.T) {
			if strings.Contains(query, "IS_ACTIVE") {
				t.Fatal("query must not reference users.is_active")
			}
			if !strings.Contains(query, "ROLE::TEXT") {
				t.Fatal("role enum must be cast to text before alias-compatible comparison")
			}
			if !strings.Contains(query, "'ADMIN'") {
				t.Fatal("query must accept the database Admin Cabdin role ADMIN")
			}
			if strings.Contains(query, "ROLE IN") {
				t.Fatal("query must not compare role_type directly to string literals")
			}
			for _, forbiddenRole := range []string{"ADMIN_MASTER", "SCHOOL_ADMIN", "PENGAWAS", "USER_SEKOLAH_GASPOL"} {
				if strings.Contains(query, "'"+forbiddenRole+"'") {
					t.Fatalf("query must not include non-Cabdin role %s", forbiddenRole)
				}
			}
		})
	}

	if !strings.Contains(listQuery, "COALESCE(NULLIF(TRIM(NAMA_INSTANSI), ''), USERNAME)") {
		t.Fatal("list query must use institution name with username fallback")
	}
}

func TestComplaintAssigneesRejectsNonMasterRolesBeforeDatabaseAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{
		auth.RoleAdminCabdin,
		auth.RoleAdminCabdinAlias,
		auth.RoleSchoolAdmin,
		auth.RolePengawas,
		auth.RoleGaspolSchoolUser,
	} {
		t.Run(role, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/admin/complaints-assignees", nil)
			auth.SetCurrentUser(context, auth.User{ID: 99, Role: role})

			ComplaintHandler{}.ComplaintAssignees(context)

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status=%d, want %d", recorder.Code, http.StatusForbidden)
			}
			if strings.Contains(strings.ToLower(recorder.Body.String()), "sql") {
				t.Fatalf("response leaked database detail: %s", recorder.Body.String())
			}
		})
	}
}

func TestComplaintWorkflowQueriesAgainstPostgresRoleEnum(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("COMPLAINTS_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set COMPLAINTS_TEST_DATABASE_URL to run the PostgreSQL handler integration test")
	}

	ctx := context.Background()
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	config.MaxConns = 1
	config.MinConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, `
		CREATE TEMP TABLE users (
			id INTEGER PRIMARY KEY,
			username TEXT NOT NULL,
			role role_type NOT NULL,
			nama_instansi TEXT
		);
		CREATE TEMP TABLE complaints (
			id BIGINT PRIMARY KEY,
			report_id TEXT UNIQUE NOT NULL,
			status TEXT NOT NULL,
			assigned_to INTEGER,
			official_response TEXT,
			closed_reason TEXT,
			clarification_deadline TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		);
		CREATE TEMP TABLE complaint_timeline (
			complaint_id BIGINT,
			actor_user_id INTEGER,
			actor_role TEXT,
			action TEXT,
			old_status TEXT,
			new_status TEXT,
			message TEXT,
			is_internal BOOLEAN
		);
		CREATE TEMP TABLE complaint_audit_log (
			complaint_id BIGINT,
			actor_user_id INTEGER,
			actor_role TEXT,
			action TEXT,
			old_value JSONB,
			new_value JSONB,
			note TEXT,
			is_public BOOLEAN
		);
		INSERT INTO users (id, username, role, nama_instansi) VALUES
			(101, 'cabdin_a', 'ADMIN', 'Admin Cabdin A'),
			(102, 'master', 'ADMIN_MASTER', 'Admin Master'),
			(103, 'school', 'SCHOOL_ADMIN', 'Sekolah A'),
			(104, 'pengawas', 'PENGAWAS', 'Pengawas A'),
			(105, 'gaspol', 'USER_SEKOLAH_GASPOL', 'Sekolah Gaspol'),
			(106, 'cabdin_b', 'ADMIN', '');
		INSERT INTO complaints (id, report_id, status, updated_at) VALUES
			(201, 'PGD-TEST-ASSIGN', 'RECEIVED', NOW()),
			(202, 'PGD-TEST-INVALID-MASTER', 'RECEIVED', NOW()),
			(203, 'PGD-TEST-INVALID-ROLE', 'RECEIVED', NOW()),
			(204, 'PGD-TEST-NOT-FOUND', 'RECEIVED', NOW());
	`)
	if err != nil {
		t.Fatalf("prepare temporary complaint schema: %v", err)
	}

	handler := ComplaintHandler{DB: &db.DB{Pool: pool}}
	t.Run("master lists only database ADMIN accounts", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(recorder)
		ginContext.Request = httptest.NewRequest(http.MethodGet, "/api/admin/complaints-assignees", nil)
		auth.SetCurrentUser(ginContext, auth.User{ID: 102, Role: auth.RoleAdminMaster})

		handler.ComplaintAssignees(ginContext)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var response struct {
			Data []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(response.Data) != 2 {
			t.Fatalf("assignees=%+v, want exactly two ADMIN accounts", response.Data)
		}
		got := map[int]string{}
		for _, item := range response.Data {
			got[item.ID] = item.Name
		}
		if got[101] != "Admin Cabdin A" || got[106] != "cabdin_b" {
			t.Fatalf("unexpected Admin Cabdin display names: %+v", got)
		}
		for _, forbiddenID := range []int{102, 103, 104, 105} {
			if _, exists := got[forbiddenID]; exists {
				t.Fatalf("non-Cabdin user %d appeared in response", forbiddenID)
			}
		}
	})

	runAction := func(reportID, action string, assignedTo int) *httptest.ResponseRecorder {
		t.Helper()
		body := fmt.Sprintf(`{"action":%q,"message":"Catatan disposisi","assignedTo":%d}`, action, assignedTo)
		recorder := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(recorder)
		ginContext.Request = httptest.NewRequest(http.MethodPost, "/api/admin/complaints/"+reportID+"/actions", strings.NewReader(body))
		ginContext.Request.Header.Set("Content-Type", "application/json")
		ginContext.Params = gin.Params{{Key: "id", Value: reportID}}
		auth.SetCurrentUser(ginContext, auth.User{ID: 102, Role: auth.RoleAdminMaster})
		handler.ComplaintAction(ginContext)
		return recorder
	}

	t.Run("assign and reassign accept database ADMIN accounts", func(t *testing.T) {
		if recorder := runAction("PGD-TEST-ASSIGN", "ASSIGN", 101); recorder.Code != http.StatusOK {
			t.Fatalf("ASSIGN status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		if recorder := runAction("PGD-TEST-ASSIGN", "REASSIGN", 106); recorder.Code != http.StatusOK {
			t.Fatalf("REASSIGN status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var assignedTo int
		if err := pool.QueryRow(ctx, `SELECT assigned_to FROM complaints WHERE report_id='PGD-TEST-ASSIGN'`).Scan(&assignedTo); err != nil {
			t.Fatalf("read assigned_to: %v", err)
		}
		if assignedTo != 106 {
			t.Fatalf("assigned_to=%d, want 106 after REASSIGN", assignedTo)
		}
		if recorder := runAction("PGD-TEST-ASSIGN", "RESPOND", 0); recorder.Code != http.StatusConflict {
			t.Fatalf("legacy RESPOND status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM complaints WHERE report_id='PGD-TEST-ASSIGN'`).Scan(&status); err != nil {
			t.Fatalf("read status after rejected legacy RESPOND: %v", err)
		}
		if status != "FOLLOW_UP" {
			t.Fatalf("legacy RESPOND changed status to %s", status)
		}
	})

	for _, test := range []struct {
		name, reportID string
		target         int
	}{
		{"rejects Admin Master", "PGD-TEST-INVALID-MASTER", 102},
		{"rejects non-Cabdin role", "PGD-TEST-INVALID-ROLE", 103},
		{"rejects missing user", "PGD-TEST-NOT-FOUND", 999999},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := runAction(test.reportID, "ASSIGN", test.target)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if strings.Contains(strings.ToLower(recorder.Body.String()), "sql") ||
				strings.Contains(strings.ToLower(recorder.Body.String()), "role_type") {
				t.Fatalf("response leaked database detail: %s", recorder.Body.String())
			}
		})
	}

	t.Run("query failure is logged but response remains safe", func(t *testing.T) {
		var logs bytes.Buffer
		originalWriter := log.Writer()
		log.SetOutput(&logs)
		t.Cleanup(func() { log.SetOutput(originalWriter) })

		pool.Close()
		recorder := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(recorder)
		ginContext.Request = httptest.NewRequest(http.MethodGet, "/api/admin/complaints-assignees", nil)
		auth.SetCurrentUser(ginContext, auth.User{ID: 102, Role: auth.RoleAdminMaster})
		handler.ComplaintAssignees(ginContext)

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(logs.String(), "action=assignees_query") ||
			!strings.Contains(logs.String(), "user_id=102") {
			t.Fatalf("database failure log lacks operation or actor context: %s", logs.String())
		}
		if strings.Contains(strings.ToLower(recorder.Body.String()), "closed pool") {
			t.Fatalf("response leaked database error: %s", recorder.Body.String())
		}
	})
}
