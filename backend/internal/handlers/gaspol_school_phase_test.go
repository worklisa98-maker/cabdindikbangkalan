package handlers

import (
	"bytes"
	"cabdin-bangkalan-backend/internal/auth"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGaspolPhaseEndpointsRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := GaspolHandler{}
	for _, tc := range []struct {
		method, path string
		handler      gin.HandlerFunc
	}{
		{"GET", "/notices", h.SchoolNotices}, {"GET", "/report", h.NominationReport},
		{"POST", "/ack/:id", h.AcknowledgeSchoolNotice}, {"PUT", "/submission", h.SaveNominationSubmission},
	} {
		t.Run(tc.path, func(t *testing.T) {
			router := gin.New()
			router.Handle(tc.method, tc.path, tc.handler)
			path := tc.path
			if path == "/ack/:id" {
				path = "/ack/1"
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(tc.method, path, nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGaspolAcknowledgementRejectsMalformedEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ path, body string }{
		{"/ack/invalid", `{"year":2026,"lockId":1}`},
		{"/ack/0", `{"year":2026,"lockId":1}`},
		{"/ack/1", `{"year":2026,"lockId":0}`},
		{"/ack/1", `invalid-json`},
	} {
		router := gin.New()
		router.Use(func(c *gin.Context) { auth.SetCurrentUser(c, auth.User{ID: 1}) })
		router.POST("/ack/:id", GaspolHandler{}.AcknowledgeSchoolNotice)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", tc.path, bytes.NewBufferString(tc.body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", tc.path, w.Code, w.Body.String())
		}
	}
}
