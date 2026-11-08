package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"cabdin-bangkalan-backend/internal/auth"
)

func TestListRefKampus_NonAdminMasterForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := AdminHandler{}
	router := gin.New()
	router.GET("/api/admin/master/ref-kampus", func(c *gin.Context) {
		auth.SetCurrentUser(c, auth.User{ID: 1, Role: auth.RoleAdminCabdin})
		h.ListRefKampus(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/master/ref-kampus", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "FORBIDDEN") {
		t.Fatalf("expected FORBIDDEN body, got %s", rec.Body.String())
	}
}

func TestCreateRefKampus_NonAdminMasterForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := AdminHandler{}
	router := gin.New()
	router.POST("/api/admin/master/ref-kampus", func(c *gin.Context) {
		auth.SetCurrentUser(c, auth.User{ID: 1, Role: auth.RoleSchoolAdmin})
		h.CreateRefKampus(c)
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/master/ref-kampus",
		strings.NewReader(`{"nama":"Universitas Baru"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "FORBIDDEN") {
		t.Fatalf("expected FORBIDDEN body, got %s", rec.Body.String())
	}
}

func TestUpdateRefKampus_NonAdminMasterForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := AdminHandler{}
	router := gin.New()
	router.PATCH("/api/admin/master/ref-kampus/:id", func(c *gin.Context) {
		auth.SetCurrentUser(c, auth.User{ID: 1, Role: auth.RolePengawas})
		h.UpdateRefKampus(c)
	})

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/admin/master/ref-kampus/10",
		strings.NewReader(`{"isActive":false}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "FORBIDDEN") {
		t.Fatalf("expected FORBIDDEN body, got %s", rec.Body.String())
	}
}
