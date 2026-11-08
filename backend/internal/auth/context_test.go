package auth

import "testing"

func TestGetSchoolScopeRejectsMismatchedSchool(t *testing.T) {
	requested := 2
	user := User{
		ID:       10,
		Role:     RoleSchoolAdmin,
		SchoolID: intPtr(1),
	}

	if _, err := GetSchoolScope(user, &requested); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestGetSchoolScopeV2PengawasScopedSchool(t *testing.T) {
	requested := 11
	user := User{
		ID:                  12,
		Role:                RolePengawas,
		SupervisedSchoolIDs: []int{11, 22},
	}

	scope, err := GetSchoolScopeV2(user, &requested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scope.All {
		t.Fatalf("expected scoped ids, got all=true")
	}
	if len(scope.IDs) != 1 || scope.IDs[0] != 11 {
		t.Fatalf("unexpected ids: %#v", scope.IDs)
	}
}

func TestEnsureSchoolInScope(t *testing.T) {
	scope := MultiSchoolScope{
		Role:      RolePengawas,
		SchoolIDs: []int{11, 22},
	}

	if err := EnsureSchoolInScope(scope, 22); err != nil {
		t.Fatalf("expected allowed school, got error: %v", err)
	}
	if err := EnsureSchoolInScope(scope, 99); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for out-of-scope school, got: %v", err)
	}
}

func intPtr(value int) *int {
	return &value
}
