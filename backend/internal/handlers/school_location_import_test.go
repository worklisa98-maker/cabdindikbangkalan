package handlers

import (
	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/db"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocationImportMissingAsset(t *testing.T) {
	t.Setenv("SCHOOL_LOCATION_SEED_PATH", filepath.Join(t.TempDir(), "missing.xlsx"))
	for _, handler := range []gin.HandlerFunc{(SchoolHandler{DB: &db.DB{}}).PreviewLocationImport, (SchoolHandler{DB: &db.DB{}}).ApplyLocationImport} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("POST", "/", nil)
		auth.SetCurrentUser(c, auth.User{ID: 1, Role: "ADMIN_MASTER"})
		handler(c)
		if recorder.Code != 500 || !strings.Contains(recorder.Body.String(), "file referensi") {
			t.Fatalf("missing asset response=%s", recorder.Body.String())
		}
	}
}

func TestLocationImportAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{"", "ADMIN", "ADMIN_CABDIN", "SCHOOL_ADMIN", "PENGAWAS", "VALIDATOR", "USER_MGMP", "USER_SEKOLAH_GASPOL"} {
		for _, handler := range []gin.HandlerFunc{(SchoolHandler{}).PreviewLocationImport, (SchoolHandler{}).ApplyLocationImport, (LandingSettingsHandler{}).GetAdmin, (LandingSettingsHandler{}).UpdateAdmin} {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/", nil)
			if role != "" {
				auth.SetCurrentUser(c, auth.User{ID: 1, Role: role})
			}
			handler(c)
			want := 403
			if role == "" {
				want = 401
			}
			if recorder.Code != want {
				t.Fatalf("role=%s status=%d want=%d", role, recorder.Code, want)
			}
		}
	}
}
