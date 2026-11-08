package ptchoices

import "testing"

func TestNormalizePTJalur(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		raw   string
		want  string
		valid bool
	}{
		{name: "snbp", raw: "SNBP", want: "SNBP", valid: true},
		{name: "snbt", raw: "snbt", want: "SNBT", valid: true},
		{name: "snbm alias", raw: "SNBM (Mandiri)", want: "MANDIRI", valid: true},
		{name: "kedinasan alias", raw: "Sekolah Kedinasan", want: "KEDINASAN", valid: true},
		{name: "lainnya alias", raw: "jalur lainnya", want: "LAINNYA", valid: true},
		{name: "invalid", raw: "tes", want: "", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizePTJalur(tt.raw)
			if ok != tt.valid {
				t.Fatalf("valid mismatch: got %v want %v", ok, tt.valid)
			}
			if got != tt.want {
				t.Fatalf("jalur mismatch: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestChoiceHelpers(t *testing.T) {
	t.Parallel()

	if got := ClampMaxChoices(99); got != SYSTEM_MAX_PT_CHOICES {
		t.Fatalf("ClampMaxChoices(99) = %d, want %d", got, SYSTEM_MAX_PT_CHOICES)
	}

	disallowed := DisallowedChoiceSlots(2)
	if len(disallowed) != 2 || disallowed[0] != 3 || disallowed[1] != 4 {
		t.Fatalf("DisallowedChoiceSlots(2) = %#v, want [3 4]", disallowed)
	}

	if got := FormatChoiceSlots([]int{3, 4}); got != "Pilihan ke-3 dan ke-4" {
		t.Fatalf("FormatChoiceSlots mismatch: got %q", got)
	}
}
