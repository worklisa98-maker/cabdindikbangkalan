package middleware

import "testing"

func TestGaspolVoterPermissionBoundary(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		allowed      bool
	}{
		{"GET", "/api/gaspol/voting", true}, {"POST", "/api/gaspol/voting/vote", true}, {"POST", "/api/gaspol/voting/assistance", true},
		{"GET", "/api/gaspol/admin/dashboard", false}, {"POST", "/api/gaspol/admin/voting", false}, {"GET", "/api/data-sekolah", false}, {"GET", "/api/gaspol/history", false}, {"POST", "/api/gaspol/assessments", false}, {"PUT", "/api/user/me/password", true},
	} {
		if got := isAllowedGaspolVoterRequest(tc.method, tc.path); got != tc.allowed {
			t.Fatalf("%s %s got %v", tc.method, tc.path, got)
		}
	}
}
