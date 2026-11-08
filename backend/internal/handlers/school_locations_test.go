package handlers

import (
	"cabdin-bangkalan-backend/internal/auth"
	"github.com/gin-gonic/gin"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidLocation(t *testing.T) {
	ptr := func(v float64) *float64 { return &v }
	cases := []struct {
		name  string
		p     locationInput
		valid bool
	}{
		{"empty", locationInput{}, true},
		{"verified empty", locationInput{Verified: true}, false},
		{"partial", locationInput{Latitude: ptr(-7)}, false},
		{"valid", locationInput{Latitude: ptr(-7.1), Longitude: ptr(112.7), Verified: true}, true},
		{"invalid latitude", locationInput{Latitude: ptr(-91), Longitude: ptr(112)}, false},
		{"invalid longitude", locationInput{Latitude: ptr(-7), Longitude: ptr(181)}, false},
		{"nan", locationInput{Latitude: ptr(math.NaN()), Longitude: ptr(112)}, false},
		{"infinity", locationInput{Latitude: ptr(-7), Longitude: ptr(math.Inf(1))}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validLocation(c.p); got != c.valid {
				t.Fatalf("got %v want %v", got, c.valid)
			}
		})
	}
}

func TestLocationAccessRejectsOutOfScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	schoolID := 1
	for _, role := range []string{"SCHOOL_ADMIN", "USER_SEKOLAH_GASPOL", "USER_MGMP", "VALIDATOR", "PENGAWAS"} {
		t.Run(role, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPut, "/api/schools/2/location", nil)
			c.Params = gin.Params{{Key: "id", Value: "2"}}
			auth.SetCurrentUser(c, auth.User{ID: 1, Role: role, SchoolID: &schoolID, SupervisedSchoolIDs: []int{1}})
			(SchoolHandler{}).UpdateLocation(c)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status=%d", recorder.Code)
			}
		})
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	(SchoolHandler{}).GetLocation(c)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", recorder.Code)
	}
}
