package ptchoices

import (
	"fmt"
	"strings"
)

const SYSTEM_MAX_PT_CHOICES = 4

var JalurOrder = []string{"SNBP", "SNBT", "MANDIRI", "KEDINASAN", "LAINNYA"}

func NormalizePTJalur(raw string) (string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(raw))
	if upper == "" {
		return "", false
	}

	switch upper {
	case "SNBP":
		return "SNBP", true
	case "SNBT":
		return "SNBT", true
	case "MANDIRI", "SNBM", "SNBM/MANDIRI", "SNBM (MANDIRI)", "MANDIRI PTN":
		return "MANDIRI", true
	case "KEDINASAN", "SEKOLAH KEDINASAN", "KEDINASAN SEKOLAH":
		return "KEDINASAN", true
	case "LAINNYA", "LAINNYA/OTHER", "LAIN":
		return "LAINNYA", true
	}

	switch {
	case strings.Contains(upper, "SNBP"):
		return "SNBP", true
	case strings.Contains(upper, "SNBT"):
		return "SNBT", true
	case strings.Contains(upper, "MANDIRI"), strings.Contains(upper, "SNBM"):
		return "MANDIRI", true
	case strings.Contains(upper, "KEDINAS"):
		return "KEDINASAN", true
	case strings.Contains(upper, "LAIN"):
		return "LAINNYA", true
	default:
		return "", false
	}
}

func ClampMaxChoices(value int) int {
	if value < 1 || value > SYSTEM_MAX_PT_CHOICES {
		return SYSTEM_MAX_PT_CHOICES
	}
	return value
}

func IsValidChoiceSlot(slot int) bool {
	return slot >= 1 && slot <= SYSTEM_MAX_PT_CHOICES
}

func ChoiceSlots() []int {
	result := make([]int, 0, SYSTEM_MAX_PT_CHOICES)
	for slot := 1; slot <= SYSTEM_MAX_PT_CHOICES; slot++ {
		result = append(result, slot)
	}
	return result
}

func DisallowedChoiceSlots(limit int) []int {
	clamped := ClampMaxChoices(limit)
	if clamped >= SYSTEM_MAX_PT_CHOICES {
		return []int{}
	}

	result := make([]int, 0, SYSTEM_MAX_PT_CHOICES-clamped)
	for slot := clamped + 1; slot <= SYSTEM_MAX_PT_CHOICES; slot++ {
		result = append(result, slot)
	}
	return result
}

func FormatChoiceSlots(slots []int) string {
	filtered := make([]int, 0, len(slots))
	for _, slot := range slots {
		if !IsValidChoiceSlot(slot) {
			continue
		}
		filtered = append(filtered, slot)
	}
	if len(filtered) == 0 {
		return ""
	}
	if len(filtered) == 1 {
		return fmt.Sprintf("Pilihan ke-%d", filtered[0])
	}

	parts := make([]string, 0, len(filtered))
	for _, slot := range filtered {
		parts = append(parts, fmt.Sprintf("ke-%d", slot))
	}
	return "Pilihan " + strings.Join(parts[:len(parts)-1], ", ") + " dan " + parts[len(parts)-1]
}
