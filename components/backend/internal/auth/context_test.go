package auth

import "testing"

func TestGetSchoolScopeRejectsMismatchedSchool(t *testing.T) {
	requested := 2
	user := User{
		ID:       10,
		Role:     "SCHOOL_ADMIN",
		SchoolID: intPtr(1),
	}

	if _, err := GetSchoolScope(user, &requested); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func intPtr(value int) *int {
	return &value
}
