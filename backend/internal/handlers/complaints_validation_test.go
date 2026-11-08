package handlers

import "testing"

func complaintPost(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func validComplaintValues() map[string]string {
	return map[string]string{
		"reportType":         "PENGADUAN",
		"title":              "Laporan pelayanan",
		"categoryId":         "1",
		"targetType":         "CABDIN",
		"eventDate":          "2026-07-29",
		"relatedParties":     "Petugas pelayanan",
		"details":            "Kronologi laporan yang lengkap.",
		"impact":             "Pelayanan menjadi terhambat.",
		"expectedResolution": "Pelayanan dapat diperbaiki.",
		"confidentiality":    "OPEN",
		"reporterName":       "Pelapor",
	}
}

func TestComplaintValidationRequiresAllCoreFields(t *testing.T) {
	_, errors := readComplaintSubmission(complaintPost(map[string]string{}), false)
	for _, field := range []string{
		"reportType", "title", "categoryId", "targetType", "details",
		"confidentiality",
	} {
		if errors[field] == "" {
			t.Errorf("field %s seharusnya wajib", field)
		}
	}
}

func TestComplaintConditionalTargetNormalization(t *testing.T) {
	values := validComplaintValues()
	values["targetType"] = "SCHOOL"
	values["schoolId"] = ""
	_, errors := readComplaintSubmission(complaintPost(values), false)
	if errors["schoolId"] == "" {
		t.Fatal("sekolah wajib untuk tujuan Satuan Pendidikan")
	}

	values["schoolId"] = "42"
	values["otherTargetName"] = "nilai lama"
	input, errors := readComplaintSubmission(complaintPost(values), false)
	if errors["schoolId"] != "" || input.SchoolID != 42 || input.OtherTargetName != "" {
		t.Fatalf("normalisasi sekolah tidak sesuai: input=%+v errors=%v", input, errors)
	}

	values["targetType"] = "OTHER"
	values["otherTargetName"] = ""
	input, errors = readComplaintSubmission(complaintPost(values), false)
	if errors["otherTargetName"] == "" || input.SchoolID != 0 {
		t.Fatalf("tujuan lainnya harus mewajibkan nama dan menghapus sekolah: input=%+v errors=%v", input, errors)
	}

	values["targetType"] = "SIPRES"
	values["schoolId"] = "42"
	values["otherTargetName"] = "nilai lama"
	input, _ = readComplaintSubmission(complaintPost(values), false)
	if input.SchoolID != 0 || input.OtherTargetName != "" {
		t.Fatalf("nilai kondisional lama tidak dibersihkan: %+v", input)
	}
}

func TestComplaintTypeSpecificFields(t *testing.T) {
	values := validComplaintValues()
	values["eventDate"] = ""
	values["relatedParties"] = ""
	_, errors := readComplaintSubmission(complaintPost(values), false)
	if errors["eventDate"] == "" || errors["relatedParties"] == "" {
		t.Fatal("Pengaduan harus mewajibkan tanggal dan pihak terkait")
	}

	values["reportType"] = "KRITIK"
	values["impact"] = "Dampak kritik."
	values["expectedResolution"] = "Perbaikan kritik."
	input, errors := readComplaintSubmission(complaintPost(values), false)
	if errors["eventDate"] != "" || errors["relatedParties"] != "" {
		t.Fatalf("Kritik tidak boleh mewajibkan field Pengaduan: %v", errors)
	}
	if input.EventDate != "" || input.RelatedParties != "" {
		t.Fatalf("field Pengaduan lama harus dibersihkan: %+v", input)
	}
}

func TestComplaintConfidentialityRules(t *testing.T) {
	values := validComplaintValues()
	values["confidentiality"] = "ANONYMOUS"
	values["reporterName"] = "Harus dibuang"
	values["whatsapp"] = "081234567890"
	values["email"] = "pelapor@example.com"
	input, errors := readComplaintSubmission(complaintPost(values), false)
	if len(errors) != 0 {
		t.Fatalf("laporan anonim publik valid ditolak: %v", errors)
	}
	if input.ReporterName != "" || input.WhatsApp != "" || input.Email != "" {
		t.Fatalf("identitas anonim tidak dibersihkan: %+v", input)
	}

	_, errors = readComplaintSubmission(complaintPost(values), true)
	if errors["confidentiality"] == "" {
		t.Fatal("pengguna login tidak boleh mengirim sebagai anonim")
	}

	values["confidentiality"] = "OPEN"
	values["reporterName"] = ""
	_, errors = readComplaintSubmission(complaintPost(values), false)
	if errors["reporterName"] == "" {
		t.Fatal("nama pelapor publik non-anonim wajib")
	}
}

func TestComplaintOptionalContactFormat(t *testing.T) {
	values := validComplaintValues()
	values["whatsapp"] = ""
	values["email"] = ""
	_, errors := readComplaintSubmission(complaintPost(values), false)
	if errors["whatsapp"] != "" || errors["email"] != "" {
		t.Fatalf("kontak kosong seharusnya valid: %v", errors)
	}
	values["whatsapp"] = "nomor-tidak-valid"
	values["email"] = "bukan-email"
	_, errors = readComplaintSubmission(complaintPost(values), false)
	if errors["whatsapp"] == "" || errors["email"] == "" {
		t.Fatalf("format kontak tidak valid harus ditolak: %v", errors)
	}
}

func TestComplaintAttachmentCountRequired(t *testing.T) {
	errors := complaintValidationErrors{}
	validateComplaintAttachmentCount(0, errors)
	if errors["attachments"] == "" {
		t.Fatal("laporan tanpa lampiran harus ditolak")
	}
	errors = complaintValidationErrors{}
	validateComplaintAttachmentCount(1, errors)
	if errors["attachments"] != "" {
		t.Fatal("satu lampiran harus diterima")
	}
}

func TestValidComplaintPassesValidation(t *testing.T) {
	input, errors := readComplaintSubmission(complaintPost(validComplaintValues()), false)
	validateComplaintAttachmentCount(1, errors)
	if len(errors) != 0 {
		t.Fatalf("laporan valid ditolak: input=%+v errors=%v", input, errors)
	}
}
