package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"cabdin-bangkalan-backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func TestServiceLinksAdminRejectsNonMaster(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, role := range []string{"SCHOOL_ADMIN", "USER_SEKOLAH_GASPOL", "ADMIN"} {
		t.Run(role, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/admin/master/service-links", nil)
			auth.SetCurrentUser(context, auth.User{ID: 1, Role: role})
			(ServiceLinksHandler{}).Admin(context)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status=%d", recorder.Code)
			}
		})
	}
}
