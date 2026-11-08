package handlers

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestValidateMGMPRegistration(t *testing.T) {
	valid := mgmpRegistrationInput{
		Name:    "MGMP Bahasa Inggris",
		Subject: "Bahasa Inggris",
		Region:  "Kabupaten Bangkalan",
		Members: []mgmpMemberInput{
			{FullName: "Ketua", SchoolName: "SMKN 1", Position: "Ketua", Email: "ketua@example.id", WhatsApp: "081234567890", Status: "Aktif"},
			{FullName: "Sekretaris", SchoolName: "SMKN 2", Position: "Sekretaris/Wakil", Email: "sekretaris@example.id", WhatsApp: "+6281234567891", Status: "Aktif"},
		},
	}
	if problems := validateMGMPRegistration(&valid); len(problems) != 0 {
		t.Fatalf("valid registration rejected: %v", problems)
	}
	if valid.Members[1].Position != "SEKRETARIS_WAKIL" {
		t.Fatalf("secretary position was not normalized: %q", valid.Members[1].Position)
	}

	invalid := valid
	invalid.Members = append([]mgmpMemberInput(nil), valid.Members...)
	invalid.Members[1].Email = invalid.Members[0].Email
	if problems := validateMGMPRegistration(&invalid); len(problems) == 0 {
		t.Fatal("duplicate leader email should be rejected")
	}
}

func TestValidateJarkomRegistrationAndOrganizationType(t *testing.T) {
	valid := mgmpRegistrationInput{
		OrganizationType: "jarkom",
		Name:             "JARKOM Teknik Jaringan",
		Subject:          "Teknik Jaringan Komputer",
		Region:           "Kabupaten Bangkalan",
		Members: []mgmpMemberInput{
			{FullName: "Ketua", SchoolName: "SMKN 1", Position: "KETUA", Email: "ketua.jarkom@example.id", WhatsApp: "081234567890", Status: "AKTIF"},
			{FullName: "Wakil", SchoolName: "SMKN 2", Position: "SEKRETARIS_WAKIL", Email: "wakil.jarkom@example.id", WhatsApp: "081234567891", Status: "AKTIF"},
		},
	}
	if problems := validateMGMPRegistration(&valid); len(problems) != 0 {
		t.Fatalf("valid JARKOM registration rejected: %v", problems)
	}
	if valid.OrganizationType != "JARKOM" {
		t.Fatalf("organization type not normalized: %q", valid.OrganizationType)
	}
	valid.OrganizationType = "KOMUNITAS"
	if problems := validateMGMPRegistration(&valid); len(problems) == 0 {
		t.Fatal("arbitrary organization type should be rejected")
	}
}

func TestValidateMGMPActivityURLs(t *testing.T) {
	valid := mgmpActivityInput{Title: "Workshop", ActivityDate: "2026-09-15", Venue: "SMKN 1", Description: "Kegiatan", ParticipantCount: 20, AttendanceURL: "https://drive.google.com/file/d/1", DocumentationURL: "https://drive.google.com/drive/folders/1"}
	if err := validateActivity(valid); err != nil {
		t.Fatalf("valid activity rejected: %v", err)
	}
	valid.DocumentationURL = "javascript:alert(1)"
	if err := validateActivity(valid); err == nil {
		t.Fatal("non-http documentation URL should be rejected")
	}
}

func TestValidateAttendanceRulesAndAutomaticCounts(t *testing.T) {
	req := mgmpActivityInput{Title: "Workshop", ActivityDate: "2026-09-15", Venue: "SMKN 1", Description: "Kegiatan", AttendanceURL: "https://drive.google.com/file/d/1", DocumentationURL: "https://drive.google.com/drive/folders/1"}
	req.Attendance = append(req.Attendance,
		struct {
			MemberID    int64  `json:"memberId"`
			Status      string `json:"status"`
			AbsenceType string `json:"absenceType"`
			AbsenceNote string `json:"absenceNote"`
		}{MemberID: 1, Status: "PRESENT"},
		struct {
			MemberID    int64  `json:"memberId"`
			Status      string `json:"status"`
			AbsenceType string `json:"absenceType"`
			AbsenceNote string `json:"absenceNote"`
		}{MemberID: 2, Status: "ABSENT", AbsenceType: "EXCUSED", AbsenceNote: "Sakit"},
		struct {
			MemberID    int64  `json:"memberId"`
			Status      string `json:"status"`
			AbsenceType string `json:"absenceType"`
			AbsenceNote string `json:"absenceNote"`
		}{MemberID: 3, Status: "ABSENT", AbsenceType: "UNEXCUSED"},
	)
	if err := validateActivity(req); err != nil {
		t.Fatalf("valid attendance rejected: %v", err)
	}
	present, excused, unexcused := attendanceCounts(req)
	if present != 1 || excused != 1 || unexcused != 1 {
		t.Fatalf("unexpected counts present=%d excused=%d unexcused=%d", present, excused, unexcused)
	}
	req.Attendance[1].AbsenceNote = ""
	if err := validateActivity(req); err == nil || !strings.Contains(err.Error(), "Keterangan") {
		t.Fatalf("missing excused note should be rejected, got %v", err)
	}
}

func TestBuildMGMPMemberTemplate(t *testing.T) {
	data, err := buildMGMPMemberTemplate([]string{"SMK Negeri 1 Bangkalan", "SMK Negeri 2 Bangkalan"})
	if err != nil {
		t.Fatalf("build template: %v", err)
	}
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer func() { _ = file.Close() }()

	sheets := file.GetSheetList()
	if len(sheets) != 2 || sheets[0] != "Daftar Anggota" || sheets[1] != "Referensi Sekolah SMK" {
		t.Fatalf("unexpected sheets: %v", sheets)
	}
	foundRange := false
	for _, definedName := range file.GetDefinedName() {
		if definedName.Name == "MGMP_SCHOOL_LIST" {
			foundRange = true
			if !strings.Contains(definedName.RefersTo, "'Referensi Sekolah SMK'!$A$1:$A$2") {
				t.Fatalf("unexpected school range: %q", definedName.RefersTo)
			}
		}
	}
	if !foundRange {
		t.Fatal("MGMP_SCHOOL_LIST named range not found")
	}
	validations, err := file.GetDataValidations("Daftar Anggota")
	if err != nil {
		t.Fatalf("read validations: %v", err)
	}
	wantValidations := map[string]string{"C2:C2000": "MGMP_SCHOOL_LIST", "D2:D2000": `"Ketua,Sekretaris/Wakil,Bendahara,Anggota,Lainnya"`, "G2:G2000": `"Aktif,Nonaktif"`}
	for _, validation := range validations {
		if expected, ok := wantValidations[validation.Sqref]; ok && validation.Formula1 == expected {
			if validation.Sqref == "C2:C2000" && validation.AllowBlank {
				t.Fatal("school validation must reject blank/free-text input")
			}
			delete(wantValidations, validation.Sqref)
		}
	}
	if len(wantValidations) != 0 {
		t.Fatalf("missing validations: %v; got %#v", wantValidations, validations)
	}
	for _, cell := range []string{"B2", "F2"} {
		styleID, err := file.GetCellStyle("Daftar Anggota", cell)
		if err != nil {
			t.Fatalf("get style %s: %v", cell, err)
		}
		style, err := file.GetStyle(styleID)
		if err != nil || style.NumFmt != 49 {
			t.Fatalf("%s must use Text format, style=%#v err=%v", cell, style, err)
		}
	}
	if err := file.SetCellStr("Daftar Anggota", "B2", "0012345678901234"); err != nil {
		t.Fatalf("set NIP/NIK: %v", err)
	}
	if err := file.SetCellStr("Daftar Anggota", "F2", "081234567890"); err != nil {
		t.Fatalf("set WhatsApp: %v", err)
	}
	saved, err := file.WriteToBuffer()
	if err != nil {
		t.Fatalf("save populated template: %v", err)
	}
	reopened, err := excelize.OpenReader(bytes.NewReader(saved.Bytes()))
	if err != nil {
		t.Fatalf("reopen populated template: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	for cell, expected := range map[string]string{"B2": "0012345678901234", "F2": "081234567890"} {
		actual, err := reopened.GetCellValue("Daftar Anggota", cell)
		if err != nil || actual != expected {
			t.Fatalf("%s changed after reopen: got %q want %q err=%v", cell, actual, expected, err)
		}
	}
}

func TestPublicMGMPStatusLabels(t *testing.T) {
	want := map[string]string{
		"PENDING":           "Menunggu Verifikasi",
		"REVISION_REQUIRED": "Perlu Perbaikan",
		"APPROVED":          "Disetujui",
		"REJECTED":          "Ditolak",
	}
	for status, label := range want {
		if got := publicMGMPStatusLabel(status); got != label {
			t.Fatalf("status %s: got %q want %q", status, got, label)
		}
	}
}

func TestPublicMGMPHistoryLabels(t *testing.T) {
	want := map[string]string{
		"REGISTRATION_SUBMITTED":          "Pendaftaran dikirim",
		"REGISTRATION_DELEGATED":          "Pendaftaran diteruskan untuk proses verifikasi",
		"REGISTRATION_REVISION_REQUESTED": "Permintaan perbaikan",
		"REGISTRATION_RESUBMITTED":        "Pendaftaran diperbaiki dan dikirim ulang",
		"REGISTRATION_APPROVED":           "Pendaftaran disetujui",
		"REGISTRATION_REJECTED":           "Pendaftaran ditolak",
	}
	for event, label := range want {
		got, ok := publicMGMPHistoryLabel(event)
		if !ok || got != label {
			t.Fatalf("event %s: got %q, %v want %q", event, got, ok, label)
		}
	}
	if _, ok := publicMGMPHistoryLabel("ACCOUNT_PASSWORD_RESET"); ok {
		t.Fatal("internal credential audit event must not be public")
	}
	if got := publicMGMPHistoryEvent("REGISTRATION_REVISION_REQUESTED"); got != "REGISTRATION_REVISION_REQUIRED" {
		t.Fatalf("revision event not normalized: %q", got)
	}
}

func TestMGMPRegistrationDecisionTransitions(t *testing.T) {
	allowed := [][2]string{{"REVISION", "PENDING"}, {"APPROVE", "PENDING"}, {"REJECT", "PENDING"}, {"REJECT", "REVISION_REQUIRED"}}
	for _, transition := range allowed {
		if !canDecideMGMPRegistration(transition[0], transition[1]) {
			t.Fatalf("expected %s from %s to be allowed", transition[0], transition[1])
		}
	}
	denied := [][2]string{{"REVISION", "REVISION_REQUIRED"}, {"APPROVE", "REVISION_REQUIRED"}, {"APPROVE", "APPROVED"}, {"REJECT", "REJECTED"}}
	for _, transition := range denied {
		if canDecideMGMPRegistration(transition[0], transition[1]) {
			t.Fatalf("expected duplicate/invalid %s from %s to be denied", transition[0], transition[1])
		}
	}
}
