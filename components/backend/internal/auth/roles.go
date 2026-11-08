package auth

import "strings"

const (
	RoleAdminCabdin = "ADMIN"
	RoleSchoolAdmin = "SCHOOL_ADMIN"
	RoleAdminMaster = "ADMIN_MASTER"
)

func IsAdmin(user User) bool {
	return user.Role == RoleAdminCabdin || user.Role == RoleAdminMaster
}

func IsAdminMaster(user User) bool {
	return user.Role == RoleAdminMaster
}

func IsSchoolAdmin(user User) bool {
	return user.Role == RoleSchoolAdmin
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
