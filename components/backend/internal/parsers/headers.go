package parsers

import (
	"bytes"
	"strings"

	"github.com/xuri/excelize/v2"
)

var AchievementTemplateHeaders = []string{
	"ID_Prestasi",
	"Kategori_Prestasi",
	"Tingkat_Lomba",
	"Tahun_Lomba",
	"Nama_Lomba",
	"Peringkat_Prestasi",
	"Mode_Pelaksanaan",
	"Penyelenggara_Lomba",
	"Jenis_Peserta",
	"Jenis_Lomba",
	"Nama_Peserta",
	"Link_Gdrive",
}

var AcceptanceTemplateHeaders = []string{
	"ID",
	"NISN_Siswa",
	"Nama_Siswa",
	"Rombel_Siswa",
	"Tahun_Lulus",
	"Nama_Perguruan_Tinggi",
	"Jurusan_Perguruan_Tinggi",
	"Tahun_Diterima",
	"Jalur_Penerimaan",
}

var SiswaCohortTemplateHeaders = []string{
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Tahun_Lulus",
}

var SiswaApplicationTemplateHeaders = []string{
	"Tahun Lulus",
	"NISN",
	"Nama",
	"Jalur",
	"Pilihan Ke",
	"Kampus",
	"Prodi",
}

var SiswaPlanTemplateHeaders = []string{
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Tahun_Lulus",
	"Plan_Jalur",
	"Pilihan_1_Kampus",
	"Pilihan_1_Prodi",
	"Pilihan_2_Kampus",
	"Pilihan_2_Prodi",
	"Pilihan_3_Kampus",
	"Pilihan_3_Prodi",
}

var SiswaWirausahaTemplateHeaders = []string{
	"Tahun Lulus",
	"NISN",
	"Rencana Ke",
	"Nama Usaha",
	"Kategori Usaha",
	"Status Kesiapan",
	"Rencana Mulai",
	"Modal Awal",
	"Sumber Modal",
	"Lokasi",
	"Platform",
	"Catatan",
}

var SiswaOutcomeTemplateHeaders = []string{
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Tahun_Lulus",
	"Final_Status",
	"Final_Path",
	"PT_Kampus",
	"PT_Prodi",
	"PT_Tahun_Diterima",
	"Kerja_Perusahaan",
	"Kerja_Jabatan",
	"Kerja_Tahun_Mulai",
	"Wirausaha_Bidang",
	"Wirausaha_Nama",
	"Wirausaha_Tahun_Mulai",
}

func ExtractHeaderRow(buffer []byte, preferredSheet string) ([]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(buffer))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	sheet := strings.TrimSpace(preferredSheet)
	if sheet != "" {
		sheetIndex, err := f.GetSheetIndex(sheet)
		if err != nil || sheetIndex == -1 {
			sheet = f.GetSheetName(0)
		}
	} else {
		sheet = f.GetSheetName(0)
	}
	if strings.TrimSpace(sheet) == "" {
		return []string{}, nil
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []string{}, nil
	}

	headers := make([]string, len(rows[0]))
	for i, header := range rows[0] {
		headers[i] = strings.TrimSpace(header)
	}
	return headers, nil
}

func HeadersMatchExact(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i, header := range expected {
		if actual[i] != header {
			return false
		}
	}
	return true
}

func ValidateExactHeaders(buffer []byte, preferredSheet string, expected []string) ([]string, bool, error) {
	actual, err := ExtractHeaderRow(buffer, preferredSheet)
	if err != nil {
		return nil, false, err
	}
	return actual, HeadersMatchExact(actual, expected), nil
}
