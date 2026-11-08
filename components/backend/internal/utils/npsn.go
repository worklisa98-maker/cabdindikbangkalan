package utils

import "strings"

func NormalizeNPSN(value string) string {
	return strings.TrimSpace(value)
}

func IsValidNPSN(value string) bool {
	trimmed := NormalizeNPSN(value)
	return trimmed != "" && len(trimmed) >= 5
}
