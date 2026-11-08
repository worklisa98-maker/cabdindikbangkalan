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

var SiswaLegacyHigherEdTemplateHeaders = []string{
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Tahun_Lulus",
	"Tahun_Diterima",
	"Jalur_Penerimaan",
	"Nama_Perguruan_Tinggi",
	"Jurusan",
}

var SiswaLegacySimpleTemplateHeaders = []string{
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Tahun_Lulus",
	"Tahun_Diterima",
}

var SiswaLegacyWorkTemplateHeaders = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Tahun_Masuk_Kerja",
	"Nama_Perusahaan",
	"Bidang_Field_Perusahaan",
	"Divisi_Jabatan",
}

var SiswaLegacyEntrepreneurTemplateHeaders = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Nama_Usaha",
	"Kategori_Usaha",
	"Produk_Jasa",
	"Lokasi_Usaha",
	"Tahun_Mulai",
	"Keterangan",
}

var SiswaLegacyUndefinedTemplateHeaders = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Keterangan",
}

var SiswaApplicationTemplateHeaders = []string{
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Tahun_Lulus",
	"Jalur_PT",
	"Pilihan_1_Kampus",
	"Pilihan_1_Prodi",
	"Pilihan_1_Jenjang",
	"Pilihan_2_Kampus",
	"Pilihan_2_Prodi",
	"Pilihan_2_Jenjang",
	"Pilihan_3_Kampus",
	"Pilihan_3_Prodi",
	"Pilihan_3_Jenjang",
	"Pilihan_4_Kampus",
	"Pilihan_4_Prodi",
	"Pilihan_4_Jenjang",
}

var SiswaPTOfferTemplateHeaders = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Sekolah",
	"Jalur",
	"Pilihan_1_Nama_PT",
	"Pilihan_1_Jurusan/Program",
	"Pilihan_2_Nama_PT",
	"Pilihan_2_Jurusan/Program",
	"Pilihan_3_Nama_PT",
	"Pilihan_3_Jurusan/Program",
	"Pilihan_4_Nama_PT",
	"Pilihan_4_Jurusan/Program",
	"Hasil_Pengumuman",
	"Pilihan",
}

var SiswaPTOfferCompactTemplateHeaders = []string{
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Tahun_Lulus",
	"Nama_PT",
	"Jenjang_Jurusan",
	"Program/Jurusan",
	"Jalur",
}

var AdminPTOfferTemplateHeaders = []string{
	"Tahun_Lulus",
	"Jalur",
	"Nama_Siswa",
	"NISN",
	"Nama_Sekolah",
	"Nama_PT",
	"Program_Studi",
	"Status_Pengumuman",
}

var SiswaPTOfferTemplateHeadersLegacyProdi = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Sekolah",
	"Jalur",
	"Pilihan_1_Nama_PT",
	"Pilihan_1_Prodi",
	"Pilihan_2_Nama_PT",
	"Pilihan_2_Prodi",
	"Pilihan_3_Nama_PT",
	"Pilihan_3_Prodi",
	"Pilihan_4_Nama_PT",
	"Pilihan_4_Prodi",
	"Hasil_Pengumuman",
	"Pilihan",
}

var SiswaPTOfferVerticalLegacyTemplateHeaders = []string{
	"Entry_ID",
	"NISN",
	"Nama_Siswa",
	"Sekolah",
	"Jalur",
	"Nama_PT",
	"Prodi",
	"Hasil_Pengumuman",
	"Pilihan",
	"Tanggal_Pengumuman",
	"Catatan",
}

var SiswaPTOfferChoiceLegacyTemplateHeaders = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Pilihan_1_Jalur",
	"Pilihan_1_Nama_PT",
	"Pilihan_1_Prodi",
	"Pilihan_2_Jalur",
	"Pilihan_2_Nama_PT",
	"Pilihan_2_Prodi",
	"Pilihan_3_Jalur",
	"Pilihan_3_Nama_PT",
	"Pilihan_3_Prodi",
	"Pilihan_4_Jalur",
	"Pilihan_4_Nama_PT",
	"Pilihan_4_Prodi",
	"Diterima_Pilihan_1",
	"Diterima_Pilihan_2",
	"Diterima_Pilihan_3",
	"Diterima_Pilihan_4",
}

var SiswaPTOfferLegacyTemplateHeaders = []string{
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Tahun_Lulus",
	"Jalur_Penerimaan",
	"Nama_Perguruan_Tinggi",
	"Jurusan",
	"Tahun_Diterima",
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
	"Pilihan_4_Kampus",
	"Pilihan_4_Prodi",
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
	"Status_Final",
	"Jalur_Final",
	"Tanggal_Finalisasi",
}

var SiswaOutcomeWorkTemplateHeaders = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Nama_Perusahaan",
	"Bidang_Field_Perusahaan",
	"Divisi_Jabatan",
	"Tahun_Mulai",
}

var SiswaOutcomeEntrepreneurTemplateHeaders = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Pilihan_1_Nama_Usaha",
	"Pilihan_1_Kategori_Usaha",
	"Pilihan_1_Produk_Jasa",
	"Pilihan_1_Lokasi_Usaha",
	"Pilihan_2_Nama_Usaha",
	"Pilihan_2_Kategori_Usaha",
	"Pilihan_2_Produk_Jasa",
	"Pilihan_2_Lokasi_Usaha",
	"Pilihan_Final",
}

var SiswaOutcomeEntrepreneurCheckboxLegacyTemplateHeaders = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Pilihan_1_Nama_Usaha",
	"Pilihan_1_Kategori_Usaha",
	"Pilihan_1_Produk_Jasa",
	"Pilihan_1_Lokasi_Usaha",
	"Pilihan_2_Nama_Usaha",
	"Pilihan_2_Kategori_Usaha",
	"Pilihan_2_Produk_Jasa",
	"Pilihan_2_Lokasi_Usaha",
	"Fix_Pilihan_1",
	"Fix_Pilihan_2",
}

var SiswaOutcomeUndefinedTemplateHeaders = []string{
	"Tahun_Lulus",
	"NISN",
	"Nama_Siswa",
	"Rombel",
	"Keterangan",
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
