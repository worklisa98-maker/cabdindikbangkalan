package middleware

import (
	"net/http"
	"testing"
)

func TestIsAllowedGaspolSchoolUserPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		path    string
		allowed bool
	}{
		{path: "/auth/me", allowed: true},
		{path: "/auth/me/", allowed: true},
		{path: "/auth/logout", allowed: true},
		{path: "/api/user/me/password", allowed: true},
		{path: "/api/gaspol", allowed: true},
		{path: "/api/gaspol/school/dashboard", allowed: true},
		{path: "/api/gaspol/school/dashboard/", allowed: true},
		{path: "/api/public/gaspol", allowed: true},
		{path: "/api/public/gaspol/dashboard", allowed: true},
		{path: "/api/dashboard", allowed: false},
		{path: "/api/admin/school-admins", allowed: false},
		{path: "/user/settings", allowed: false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			if got := isAllowedGaspolSchoolUserPath(tc.path); got != tc.allowed {
				t.Fatalf("isAllowedGaspolSchoolUserPath(%q) = %v, want %v", tc.path, got, tc.allowed)
			}
		})
	}
}

func TestIsAllowedGaspolSchoolUserRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		method  string
		path    string
		allowed bool
	}{
		{name: "allow system time get", method: "GET", path: "/api/system-time", allowed: true},
		{name: "allow service links get", method: "GET", path: "/api/service-links", allowed: true},
		{name: "deny service links admin", method: "GET", path: "/api/admin/master/service-links", allowed: false},
		{name: "allow system time status get", method: "GET", path: "/api/system-time/status", allowed: true},
		{name: "deny system time post", method: "POST", path: "/api/system-time", allowed: false},
		{name: "deny system time patch", method: "PATCH", path: "/api/system-time/status", allowed: false},
		{name: "allow gaspol post", method: "POST", path: "/api/gaspol/school/submissions", allowed: true},
		{name: "deny non gaspol get", method: "GET", path: "/api/prestasi/list", allowed: false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isAllowedGaspolSchoolUserRequest(tc.method, tc.path); got != tc.allowed {
				t.Fatalf("isAllowedGaspolSchoolUserRequest(%q, %q) = %v, want %v", tc.method, tc.path, got, tc.allowed)
			}
		})
	}
}

func TestIsGaspolMutationPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		path      string
		isAllowed bool
	}{
		{path: "/api/gaspol/school/submissions", isAllowed: true},
		{path: "/api/gaspol/school/submissions/", isAllowed: true},
		{path: "/api/gaspol", isAllowed: true},
		{path: "/api/siswa/finalize", isAllowed: false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			if got := isGaspolMutationPath(tc.path); got != tc.isAllowed {
				t.Fatalf("isGaspolMutationPath(%q) = %v, want %v", tc.path, got, tc.isAllowed)
			}
		})
	}
}

func TestIsAllowedPengawasMutationRequest(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{
			name:   "allows supervisor snapshot upsert",
			method: http.MethodPut,
			path:   "/api/siswa/acceptance-offers/supervisor-snapshot",
			want:   true,
		},
		{
			name:   "allows supervisor snapshot upsert with trailing slash",
			method: http.MethodPut,
			path:   "/api/siswa/acceptance-offers/supervisor-snapshot/",
			want:   true,
		},
		{
			name:   "keeps gaspol mutations allowed",
			method: http.MethodPost,
			path:   "/api/gaspol/supervisor/assessments",
			want:   true,
		},
		{
			name:   "blocks other siswa mutations",
			method: http.MethodPost,
			path:   "/api/siswa/acceptance-offers/import",
			want:   false,
		},
		{
			name:   "blocks other methods on snapshot path",
			method: http.MethodPost,
			path:   "/api/siswa/acceptance-offers/supervisor-snapshot",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAllowedPengawasMutationRequest(tt.method, tt.path)
			if got != tt.want {
				t.Fatalf("isAllowedPengawasMutationRequest(%q, %q) = %t, want %t", tt.method, tt.path, got, tt.want)
			}
		})
	}
}

func TestIsAllowedMGMPUserRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{method: http.MethodGet, path: "/auth/me", want: true},
		{method: http.MethodPost, path: "/auth/logout", want: true},
		{method: http.MethodPatch, path: "/api/user/me/password", want: true},
		{method: http.MethodGet, path: "/api/mgmp/workspace", want: true},
		{method: http.MethodPost, path: "/api/mgmp/activities", want: true},
		{method: http.MethodGet, path: "/api/system-time", want: true},
		{method: http.MethodPost, path: "/api/system-time", want: false},
		{method: http.MethodGet, path: "/api/prestasi/list", want: false},
		{method: http.MethodGet, path: "/api/admin/school-admins", want: false},
	}
	for _, test := range tests {
		if got := isAllowedMGMPUserRequest(test.method, test.path); got != test.want {
			t.Fatalf("isAllowedMGMPUserRequest(%q, %q) = %v, want %v", test.method, test.path, got, test.want)
		}
	}
}
