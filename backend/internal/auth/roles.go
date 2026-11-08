package auth

import "strings"

const (
	RoleAdminCabdin      = "ADMIN"
	RoleAdminCabdinAlias = "ADMIN_CABDIN"
	RoleSchoolAdmin      = "SCHOOL_ADMIN"
	RoleGaspolSchoolUser = "USER_SEKOLAH_GASPOL"
	RoleMGMPUser         = "USER_MGMP"
	RoleAdminMaster      = "ADMIN_MASTER"
	RolePengawas         = "PENGAWAS"
)

func NormalizeRole(role string) string {
	normalized := strings.ToUpper(strings.TrimSpace(role))
	if normalized == RoleAdminCabdinAlias {
		return RoleAdminCabdin
	}
	return normalized
}

func IsAdmin(user User) bool {
	role := NormalizeRole(user.Role)
	return role == RoleAdminCabdin || role == RoleAdminMaster
}

func IsAdminMaster(user User) bool {
	return NormalizeRole(user.Role) == RoleAdminMaster
}

func IsSchoolAdmin(user User) bool {
	return NormalizeRole(user.Role) == RoleSchoolAdmin
}

func IsGaspolSchoolUser(user User) bool {
	return NormalizeRole(user.Role) == RoleGaspolSchoolUser
}

func IsMGMPUser(user User) bool {
	return NormalizeRole(user.Role) == RoleMGMPUser
}

func IsPengawas(user User) bool {
	return NormalizeRole(user.Role) == RolePengawas
}

func NormalizeAllowedLevels(levels []string) []string {
	if len(levels) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(levels))
	for _, level := range levels {
		value := strings.ToUpper(strings.TrimSpace(level))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}
