package handlers

import (
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var complaintWhatsAppPattern = regexp.MustCompile(`^\+?[0-9][0-9 -]{7,19}$`)

type complaintSubmission struct {
	ReportType         string
	Title              string
	CategoryID         int64
	TargetType         string
	SchoolID           int
	OtherTargetName    string
	EventDate          string
	EventLocation      string
	RelatedParties     string
	Details            string
	Impact             string
	ExpectedResolution string
	Reason             string
	ExpectedBenefit    string
	Confidentiality    string
	ReporterName       string
	WhatsApp           string
	Email              string
}

type complaintValidationErrors map[string]string

func readComplaintSubmission(postForm func(string) string, authenticated bool) (complaintSubmission, complaintValidationErrors) {
	input := complaintSubmission{
		ReportType:         strings.ToUpper(strings.TrimSpace(postForm("reportType"))),
		Title:              strings.TrimSpace(postForm("title")),
		TargetType:         strings.ToUpper(strings.TrimSpace(postForm("targetType"))),
		OtherTargetName:    strings.TrimSpace(postForm("otherTargetName")),
		EventDate:          strings.TrimSpace(postForm("eventDate")),
		EventLocation:      strings.TrimSpace(postForm("eventLocation")),
		RelatedParties:     strings.TrimSpace(postForm("relatedParties")),
		Details:            strings.TrimSpace(postForm("details")),
		Impact:             strings.TrimSpace(postForm("impact")),
		ExpectedResolution: strings.TrimSpace(postForm("expectedResolution")),
		Reason:             strings.TrimSpace(postForm("reason")),
		ExpectedBenefit:    strings.TrimSpace(postForm("expectedBenefit")),
		Confidentiality:    strings.ToUpper(strings.TrimSpace(postForm("confidentiality"))),
		ReporterName:       strings.TrimSpace(postForm("reporterName")),
		WhatsApp:           strings.TrimSpace(postForm("whatsapp")),
		Email:              strings.TrimSpace(postForm("email")),
	}
	input.CategoryID, _ = strconv.ParseInt(strings.TrimSpace(postForm("categoryId")), 10, 64)
	input.SchoolID, _ = strconv.Atoi(strings.TrimSpace(postForm("schoolId")))

	errors := complaintValidationErrors{}
	switch input.ReportType {
	case "PENGADUAN", "KRITIK", "SARAN":
	default:
		errors["reportType"] = "Jenis laporan tidak valid."
	}
	if input.Title == "" {
		errors["title"] = "Judul wajib diisi."
	} else if len(input.Title) < 3 || len(input.Title) > 200 {
		errors["title"] = "Judul harus terdiri dari 3 sampai 200 karakter."
	}
	if input.CategoryID <= 0 {
		errors["categoryId"] = "Kategori wajib dipilih."
	}
	switch input.TargetType {
	case "":
		errors["targetType"] = "Unit/layanan tujuan wajib dipilih."
	case "CABDIN", "SIPRES":
		input.SchoolID = 0
		input.OtherTargetName = ""
	case "SCHOOL":
		input.OtherTargetName = ""
		if input.SchoolID <= 0 {
			errors["schoolId"] = "Pilih sekolah dari daftar yang tersedia."
		}
	case "OTHER":
		input.SchoolID = 0
		if input.OtherTargetName == "" {
			errors["otherTargetName"] = "Nama unit/layanan tujuan lainnya wajib diisi."
		} else if len(input.OtherTargetName) > 255 {
			errors["otherTargetName"] = "Nama unit/layanan tujuan lainnya terlalu panjang."
		}
	default:
		errors["targetType"] = "Unit/layanan tujuan tidak valid."
		input.SchoolID = 0
		input.OtherTargetName = ""
	}
	if input.ReportType == "PENGADUAN" {
		if input.EventDate == "" {
			errors["eventDate"] = "Tanggal kejadian wajib diisi."
		} else if _, err := time.Parse("2006-01-02", input.EventDate); err != nil {
			errors["eventDate"] = "Tanggal kejadian tidak valid."
		}
		if input.RelatedParties == "" {
			errors["relatedParties"] = "Pihak terkait wajib diisi."
		}
	} else {
		input.EventDate = ""
		input.EventLocation = ""
		input.RelatedParties = ""
	}
	if len(input.EventLocation) > 255 {
		errors["eventLocation"] = "Lokasi kejadian terlalu panjang."
	}
	if len(input.RelatedParties) > 500 {
		errors["relatedParties"] = "Pihak terkait terlalu panjang."
	}
	if input.Details == "" {
		errors["details"] = "Kronologi/isi utama wajib diisi."
	} else if len(input.Details) < 10 || len(input.Details) > 10000 {
		errors["details"] = "Kronologi/isi utama harus terdiri dari 10 sampai 10.000 karakter."
	}
	if input.ReportType == "SARAN" {
		input.Impact = ""
		input.ExpectedResolution = ""
		if input.Reason == "" {
			errors["reason"] = "Dampak/alasan wajib diisi."
		}
		if input.ExpectedBenefit == "" {
			errors["expectedBenefit"] = "Perbaikan/manfaat yang diharapkan wajib diisi."
		}
	} else {
		input.Reason = ""
		input.ExpectedBenefit = ""
		if input.Impact == "" {
			errors["impact"] = "Dampak/alasan wajib diisi."
		}
		if input.ExpectedResolution == "" {
			errors["expectedResolution"] = "Perbaikan/manfaat yang diharapkan wajib diisi."
		}
	}
	for field, value := range map[string]string{
		"impact": input.Impact, "expectedResolution": input.ExpectedResolution,
		"reason": input.Reason, "expectedBenefit": input.ExpectedBenefit,
	} {
		if len(value) > 5000 {
			errors[field] = "Isian terlalu panjang (maksimal 5.000 karakter)."
		}
	}
	switch input.Confidentiality {
	case "":
		errors["confidentiality"] = "Tingkat kerahasiaan wajib dipilih."
	case "OPEN", "CONFIDENTIAL":
		if !authenticated && input.ReporterName == "" {
			errors["reporterName"] = "Nama pelapor wajib diisi."
		}
	case "ANONYMOUS":
		if authenticated {
			errors["confidentiality"] = "Pengguna login tidak dapat memilih anonim."
		}
		input.ReporterName = ""
		input.WhatsApp = ""
		input.Email = ""
	default:
		errors["confidentiality"] = "Tingkat kerahasiaan tidak valid."
	}
	if len(input.ReporterName) > 200 {
		errors["reporterName"] = "Nama pelapor terlalu panjang."
	}
	if input.WhatsApp != "" && !complaintWhatsAppPattern.MatchString(input.WhatsApp) {
		errors["whatsapp"] = "Format nomor WhatsApp tidak valid."
	}
	if input.Email != "" {
		address, err := mail.ParseAddress(input.Email)
		if err != nil || address.Address != input.Email {
			errors["email"] = "Format email tidak valid."
		}
	}
	return input, errors
}

func firstComplaintValidationError(errors complaintValidationErrors) string {
	order := []string{
		"reportType", "title", "categoryId", "targetType", "schoolId", "otherTargetName",
		"eventDate", "eventLocation", "relatedParties", "details", "impact", "reason",
		"expectedResolution", "expectedBenefit", "confidentiality", "reporterName",
		"whatsapp", "email", "attachments", "truthConfirmed",
	}
	for _, field := range order {
		if message := errors[field]; message != "" {
			return message
		}
	}
	return "Data laporan tidak valid."
}

func validateComplaintAttachmentCount(count int, errors complaintValidationErrors) {
	if count == 0 {
		errors["attachments"] = "Minimal satu lampiran/bukti wajib diunggah."
	} else if count > 3 {
		errors["attachments"] = "Maksimal 3 lampiran diperbolehkan."
	}
}
