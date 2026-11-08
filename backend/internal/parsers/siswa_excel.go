package parsers

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type SiswaCohortRow struct {
	RowNumber  int
	NISN       string
	NamaSiswa  string
	Rombel     string
	TahunLulus string
}

type SiswaApplicationRow struct {
	RowNumber       int
	NISN            string
	NamaSiswa       string
	Rombel          string
	TahunLulus      string
	JalurPT         string
	Pilihan1Kampus  string
	Pilihan1Prodi   string
	Pilihan1Jenjang string
	Pilihan2Kampus  string
	Pilihan2Prodi   string
	Pilihan2Jenjang string
	Pilihan3Kampus  string
	Pilihan3Prodi   string
	Pilihan3Jenjang string
	Pilihan4Kampus  string
	Pilihan4Prodi   string
	Pilihan4Jenjang string
}

type SiswaAcceptanceOfferRow struct {
	RowNumber       int
	NISN            string
	NamaSiswa       string
	Rombel          string
	TahunLulus      string
	JalurPenerimaan string
	PerguruanTinggi string
	Jurusan         string
	TahunDiterima   string
}

type SiswaAcceptanceOfferChoiceRow struct {
	RowNumber        int
	TahunLulus       string
	NISN             string
	NamaSiswa        string
	Rombel           string
	Pilihan1Jalur    string
	Pilihan1NamaPT   string
	Pilihan1Prodi    string
	Pilihan2Jalur    string
	Pilihan2NamaPT   string
	Pilihan2Prodi    string
	Pilihan3Jalur    string
	Pilihan3NamaPT   string
	Pilihan3Prodi    string
	Pilihan4Jalur    string
	Pilihan4NamaPT   string
	Pilihan4Prodi    string
	DiterimaPilihan1 string
	DiterimaPilihan2 string
	DiterimaPilihan3 string
	DiterimaPilihan4 string
}

type SiswaAcceptanceOfferAnnouncementRow struct {
	RowNumber         int
	TahunLulus        string
	GroupID           string
	NISN              string
	NamaSiswa         string
	Sekolah           string
	Jalur             string
	Pilihan1NamaPT    string
	Pilihan1Prodi     string
	Pilihan2NamaPT    string
	Pilihan2Prodi     string
	Pilihan3NamaPT    string
	Pilihan3Prodi     string
	Pilihan4NamaPT    string
	Pilihan4Prodi     string
	HasilPengumuman   string
	Pilihan           string
	TanggalPengumuman string
	Catatan           string
}

type SiswaAcceptanceOfferCompactRow struct {
	RowNumber      int
	NISN           string
	NamaSiswa      string
	Rombel         string
	TahunLulus     string
	NamaPT         string
	JenjangJurusan string
	ProgramJurusan string
	Jalur          string
}

type AdminPTOfferRow struct {
	RowNumber    int
	TahunLulus   string
	Jalur        string
	NamaSiswa    string
	NISN         string
	NamaSekolah  string
	NamaPT       string
	ProgramStudi string
	Status       string
}

type SiswaAcceptanceOfferAnnouncementLegacyRow struct {
	RowNumber         int
	EntryID           string
	NISN              string
	NamaSiswa         string
	Sekolah           string
	Jalur             string
	NamaPT            string
	Prodi             string
	HasilPengumuman   string
	Pilihan           string
	TanggalPengumuman string
	Catatan           string
}

type SiswaUKKRow struct {
	RowNumber       int
	NISN            string
	NamaSiswa       string
	TahunLulus      string
	JenisUKK        string
	SkemaUKK        string
	Hasil           string
	NilaiAkhir      string
	Predikat        string
	NomorSertifikat string
	TanggalUKK      string
	Assessors       []SiswaUKKAssessor
}

type SiswaUKKAssessor struct {
	Nama     string
	Instansi string
}

type SiswaWirausahaRow struct {
	RowNumber      int
	TahunLulus     string
	NISN           string
	RencanaKe      string
	NamaUsaha      string
	KategoriUsaha  string
	StatusKesiapan string
	RencanaMulai   string
	ModalAwal      string
	SumberModal    string
	Lokasi         string
	Platform       string
	Catatan        string
}

type SiswaHistoricalFinalRow struct {
	RowNumber       int
	NISN            string
	NamaSiswa       string
	Rombel          string
	TahunLulus      string
	TahunDiterima   string
	JalurPenerimaan string
	PerguruanTinggi string
	Jurusan         string
}

type SiswaLegacyFinalSimpleRow struct {
	RowNumber     int
	NISN          string
	NamaSiswa     string
	Rombel        string
	TahunLulus    string
	TahunDiterima string
}

type SiswaLegacyWorkRow struct {
	RowNumber             int
	TahunLulus            string
	NISN                  string
	NamaSiswa             string
	Rombel                string
	TahunMasukKerja       string
	NamaPerusahaan        string
	BidangFieldPerusahaan string
	DivisiJabatan         string
}

type SiswaLegacyEntrepreneurRow struct {
	RowNumber     int
	TahunLulus    string
	NISN          string
	NamaSiswa     string
	Rombel        string
	NamaUsaha     string
	KategoriUsaha string
	ProdukJasa    string
	LokasiUsaha   string
	TahunMulai    string
	Keterangan    string
}

type SiswaLegacyUndefinedRow struct {
	RowNumber  int
	TahunLulus string
	NISN       string
	NamaSiswa  string
	Rombel     string
	Keterangan string
}

type SiswaPlanRow struct {
	RowNumber      int
	NISN           string
	NamaSiswa      string
	Rombel         string
	TahunLulus     string
	PlanJalur      string
	Pilihan1Kampus string
	Pilihan1Prodi  string
	Pilihan2Kampus string
	Pilihan2Prodi  string
	Pilihan3Kampus string
	Pilihan3Prodi  string
	Pilihan4Kampus string
	Pilihan4Prodi  string
}

type SiswaOutcomeRow struct {
	RowNumber         int
	NISN              string
	NamaSiswa         string
	Rombel            string
	TahunLulus        string
	StatusFinal       string
	JalurFinal        string
	TanggalFinalisasi string
}

type SiswaOutcomeWorkRow struct {
	RowNumber             int
	TahunLulus            string
	NISN                  string
	NamaSiswa             string
	Rombel                string
	NamaPerusahaan        string
	BidangFieldPerusahaan string
	DivisiJabatan         string
	TahunMulai            string
}

type SiswaOutcomeEntrepreneurRow struct {
	RowNumber           int
	TahunLulus          string
	NISN                string
	NamaSiswa           string
	Rombel              string
	Pilihan1NamaUsaha   string
	Pilihan1Kategori    string
	Pilihan1ProdukJasa  string
	Pilihan1LokasiUsaha string
	Pilihan2NamaUsaha   string
	Pilihan2Kategori    string
	Pilihan2ProdukJasa  string
	Pilihan2LokasiUsaha string
	PilihanFinal        string
	FixPilihan1         string
	FixPilihan2         string
}

type SiswaOutcomeUndefinedRow struct {
	RowNumber  int
	TahunLulus string
	NISN       string
	NamaSiswa  string
	Rombel     string
	Keterangan string
}

var siswaCohortColumnMap = ColumnMap{
	"nisn":        {"nisn", "nisn_siswa"},
	"nama_siswa":  {"nama_siswa", "nama siswa", "nama"},
	"rombel":      {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus": {"tahun_lulus", "tahun lulus"},
}

var siswaApplicationColumnMap = ColumnMap{
	"nisn":              {"nisn", "nisn_siswa"},
	"nama_siswa":        {"nama_siswa", "nama siswa", "nama"},
	"rombel":            {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus":       {"tahun_lulus", "tahun lulus"},
	"jalur_pt":          {"jalur_pt", "jalur pt", "jalur_penerimaan", "jalur penerimaan", "jalur"},
	"pilihan_1_kampus":  {"pilihan_1_kampus", "pilihan 1 kampus", "pilihan_1_pt", "pilihan 1 pt"},
	"pilihan_1_prodi":   {"pilihan_1_prodi", "pilihan 1 prodi", "pilihan_1_jurusan", "pilihan 1 jurusan"},
	"pilihan_1_jenjang": {"pilihan_1_jenjang", "pilihan 1 jenjang"},
	"pilihan_2_kampus":  {"pilihan_2_kampus", "pilihan 2 kampus", "pilihan_2_pt", "pilihan 2 pt"},
	"pilihan_2_prodi":   {"pilihan_2_prodi", "pilihan 2 prodi", "pilihan_2_jurusan", "pilihan 2 jurusan"},
	"pilihan_2_jenjang": {"pilihan_2_jenjang", "pilihan 2 jenjang"},
	"pilihan_3_kampus":  {"pilihan_3_kampus", "pilihan 3 kampus", "pilihan_3_pt", "pilihan 3 pt"},
	"pilihan_3_prodi":   {"pilihan_3_prodi", "pilihan 3 prodi", "pilihan_3_jurusan", "pilihan 3 jurusan"},
	"pilihan_3_jenjang": {"pilihan_3_jenjang", "pilihan 3 jenjang"},
	"pilihan_4_kampus":  {"pilihan_4_kampus", "pilihan 4 kampus", "pilihan_4_pt", "pilihan 4 pt"},
	"pilihan_4_prodi":   {"pilihan_4_prodi", "pilihan 4 prodi", "pilihan_4_jurusan", "pilihan 4 jurusan"},
	"pilihan_4_jenjang": {"pilihan_4_jenjang", "pilihan 4 jenjang"},
}

var siswaWirausahaColumnMap = ColumnMap{
	"tahun_lulus":     {"tahun_lulus", "tahun lulus"},
	"nisn":            {"nisn", "nisn_siswa"},
	"rencana_ke":      {"rencana_ke", "rencana ke", "rencana_ke (1/2)"},
	"nama_usaha":      {"nama_usaha", "nama usaha"},
	"kategori_usaha":  {"kategori_usaha", "kategori usaha"},
	"status_kesiapan": {"status_kesiapan", "status kesiapan"},
	"rencana_mulai":   {"rencana_mulai", "rencana mulai"},
	"modal_awal":      {"modal_awal", "modal awal"},
	"sumber_modal":    {"sumber_modal", "sumber modal"},
	"lokasi":          {"lokasi"},
	"platform":        {"platform"},
	"catatan":         {"catatan"},
}

var siswaAcceptanceOfferColumnMap = ColumnMap{
	"tahun_lulus":        {"tahun_lulus", "tahun lulus"},
	"group_id":           {"group_id", "group id"},
	"nisn":               {"nisn", "nisn_siswa"},
	"nama_siswa":         {"nama_siswa", "nama siswa", "nama"},
	"sekolah":            {"sekolah", "nama_sekolah"},
	"jalur":              {"jalur", "jalur_penerimaan", "jalur penerimaan"},
	"pilihan_1_nama_pt":  {"pilihan_1_nama_pt", "pilihan 1 nama pt"},
	"pilihan_1_prodi":    {"pilihan_1_prodi", "pilihan 1 prodi", "pilihan_1_jurusan/program", "pilihan 1 jurusan/program"},
	"pilihan_2_nama_pt":  {"pilihan_2_nama_pt", "pilihan 2 nama pt"},
	"pilihan_2_prodi":    {"pilihan_2_prodi", "pilihan 2 prodi", "pilihan_2_jurusan/program", "pilihan 2 jurusan/program"},
	"pilihan_3_nama_pt":  {"pilihan_3_nama_pt", "pilihan 3 nama pt"},
	"pilihan_3_prodi":    {"pilihan_3_prodi", "pilihan 3 prodi", "pilihan_3_jurusan/program", "pilihan 3 jurusan/program"},
	"pilihan_4_nama_pt":  {"pilihan_4_nama_pt", "pilihan 4 nama pt"},
	"pilihan_4_prodi":    {"pilihan_4_prodi", "pilihan 4 prodi", "pilihan_4_jurusan/program", "pilihan 4 jurusan/program"},
	"hasil_pengumuman":   {"hasil_pengumuman", "hasil pengumuman", "hasil"},
	"pilihan":            {"pilihan", "pilihan_ke", "pilihan diterima"},
	"tanggal_pengumuman": {"tanggal_pengumuman", "tanggal pengumuman"},
	"catatan":            {"catatan", "keterangan"},
}

var siswaAcceptanceOfferCompactColumnMap = ColumnMap{
	"nisn":            {"nisn", "nisn_siswa"},
	"nama_siswa":      {"nama_siswa", "nama siswa", "nama"},
	"rombel":          {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus":     {"tahun_lulus", "tahun lulus"},
	"nama_pt":         {"nama_pt", "nama pt", "nama_perguruan_tinggi", "perguruan_tinggi"},
	"jenjang_jurusan": {"jenjang_jurusan", "jenjang jurusan", "jenjang"},
	"program_jurusan": {"program/jurusan", "program_jurusan", "program jurusan", "jurusan", "prodi"},
	"jalur":           {"jalur", "jalur_penerimaan", "jalur penerimaan"},
}

var adminPTOfferColumnMap = ColumnMap{
	"tahun_lulus":   {"tahun_lulus", "tahun lulus"},
	"jalur":         {"jalur", "jalur_penerimaan", "jalur penerimaan"},
	"nama_siswa":    {"nama_siswa", "nama siswa"},
	"nisn":          {"nisn", "nisn_siswa"},
	"nama_sekolah":  {"nama_sekolah", "nama sekolah", "sekolah"},
	"nama_pt":       {"nama_pt", "nama pt", "nama_perguruan_tinggi"},
	"program_studi": {"program_studi", "program studi", "prodi", "jurusan"},
	"status":        {"status_pengumuman", "status pengumuman", "hasil_pengumuman", "hasil pengumuman", "status"},
}

var siswaAcceptanceOfferLegacyColumnMap = ColumnMap{
	"entry_id":           {"entry_id", "application_id", "app_id", "row_id", "reference_id"},
	"nisn":               {"nisn", "nisn_siswa"},
	"nama_siswa":         {"nama_siswa", "nama siswa", "nama"},
	"sekolah":            {"sekolah", "nama_sekolah"},
	"jalur":              {"jalur", "jalur_penerimaan", "jalur penerimaan"},
	"nama_pt":            {"nama_pt", "pt", "nama_perguruan_tinggi", "perguruan_tinggi", "nama perguruan tinggi"},
	"prodi":              {"prodi", "jurusan", "jurusan_perguruan_tinggi", "program_studi"},
	"hasil_pengumuman":   {"hasil_pengumuman", "hasil pengumuman", "hasil"},
	"pilihan":            {"pilihan", "pilihan_ke", "pilihan diterima"},
	"tanggal_pengumuman": {"tanggal_pengumuman", "tanggal pengumuman"},
	"catatan":            {"catatan", "keterangan"},
}

var siswaAcceptanceOfferChoiceColumnMap = ColumnMap{
	"tahun_lulus":        {"tahun_lulus", "tahun lulus"},
	"nisn":               {"nisn", "nisn_siswa"},
	"nama_siswa":         {"nama_siswa", "nama siswa", "nama"},
	"rombel":             {"rombel", "rombel_siswa", "kelas"},
	"pilihan_1_jalur":    {"pilihan_1_jalur", "pilihan 1 jalur"},
	"pilihan_1_nama_pt":  {"pilihan_1_nama_pt", "pilihan 1 nama pt", "pilihan_1_nama_perguruan_tinggi"},
	"pilihan_1_prodi":    {"pilihan_1_prodi", "pilihan 1 prodi"},
	"pilihan_2_jalur":    {"pilihan_2_jalur", "pilihan 2 jalur"},
	"pilihan_2_nama_pt":  {"pilihan_2_nama_pt", "pilihan 2 nama pt", "pilihan_2_nama_perguruan_tinggi"},
	"pilihan_2_prodi":    {"pilihan_2_prodi", "pilihan 2 prodi"},
	"pilihan_3_jalur":    {"pilihan_3_jalur", "pilihan 3 jalur"},
	"pilihan_3_nama_pt":  {"pilihan_3_nama_pt", "pilihan 3 nama pt", "pilihan_3_nama_perguruan_tinggi"},
	"pilihan_3_prodi":    {"pilihan_3_prodi", "pilihan 3 prodi"},
	"pilihan_4_jalur":    {"pilihan_4_jalur", "pilihan 4 jalur"},
	"pilihan_4_nama_pt":  {"pilihan_4_nama_pt", "pilihan 4 nama pt", "pilihan_4_nama_perguruan_tinggi"},
	"pilihan_4_prodi":    {"pilihan_4_prodi", "pilihan 4 prodi"},
	"diterima_pilihan_1": {"diterima_pilihan_1", "diterima pilihan 1"},
	"diterima_pilihan_2": {"diterima_pilihan_2", "diterima pilihan 2"},
	"diterima_pilihan_3": {"diterima_pilihan_3", "diterima pilihan 3"},
	"diterima_pilihan_4": {"diterima_pilihan_4", "diterima pilihan 4"},
}

var siswaUKKColumnMap = ColumnMap{
	"nisn":             {"nisn", "nisn_siswa"},
	"nama_siswa":       {"nama_siswa", "nama siswa", "nama"},
	"tahun_lulus":      {"tahun_lulus", "tahun lulus"},
	"jenis_ukk":        {"jenis_ukk", "jenis ukk"},
	"skema_ukk":        {"skema_ukk", "skema ukk"},
	"hasil":            {"hasil", "hasil_ukk", "nilai_akhir_ukk", "predikat_ukk"},
	"nilai_akhir":      {"nilai_akhir", "nilai akhir", "nilai akhir (ns)", "nilai akhir ukk", "nilai_akhir_ukk", "ns"},
	"predikat":         {"predikat", "predikat_ukk", "predikat ukk"},
	"nomor_sertifikat": {"nomor_sertifikat", "nomor sertifikat", "no sertifikat", "no_sertifikat", "nomor sertifikat ukk"},
	"tanggal_ukk":      {"tanggal_ukk", "tanggal ukk", "tanggal"},
	"assessor":         {"assessor", "asesor", "nama_assessor", "nama asesor"},
}

var siswaHistoricalFinalColumnMap = ColumnMap{
	"nisn":        {"nisn", "nisn_siswa"},
	"nama_siswa":  {"nama_siswa", "nama siswa", "nama"},
	"rombel":      {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus": {"tahun_lulus", "tahun lulus"},
	"tahun_diterima": {
		"tahun_diterima",
		"tahun diterima",
	},
	"jalur":   {"jalur", "jalur_penerimaan", "jalur penerimaan"},
	"pt":      {"pt", "nama_perguruan_tinggi", "perguruan_tinggi", "nama perguruan tinggi"},
	"jurusan": {"jurusan", "jurusan_perguruan_tinggi", "program_studi", "prodi"},
}

var siswaLegacySimpleFinalColumnMap = ColumnMap{
	"nisn":           {"nisn", "nisn_siswa"},
	"nama_siswa":     {"nama_siswa", "nama siswa", "nama"},
	"rombel":         {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus":    {"tahun_lulus", "tahun lulus"},
	"tahun_diterima": {"tahun_diterima", "tahun diterima"},
}

var siswaLegacyWorkColumnMap = ColumnMap{
	"tahun_lulus":             {"tahun_lulus", "tahun lulus"},
	"nisn":                    {"nisn", "nisn_siswa"},
	"nama_siswa":              {"nama_siswa", "nama siswa", "nama"},
	"rombel":                  {"rombel", "rombel_siswa", "kelas"},
	"tahun_masuk_kerja":       {"tahun_masuk_kerja", "tahun masuk kerja", "tahun_mulai", "tahun mulai"},
	"nama_perusahaan":         {"nama_perusahaan", "nama perusahaan", "perusahaan"},
	"bidang_field_perusahaan": {"bidang_field_perusahaan", "bidang field perusahaan", "bidang_perusahaan", "bidang perusahaan"},
	"divisi_jabatan":          {"divisi_jabatan", "divisi jabatan", "jabatan", "divisi"},
}

var siswaLegacyEntrepreneurColumnMap = ColumnMap{
	"tahun_lulus":    {"tahun_lulus", "tahun lulus"},
	"nisn":           {"nisn", "nisn_siswa"},
	"nama_siswa":     {"nama_siswa", "nama siswa", "nama"},
	"rombel":         {"rombel", "rombel_siswa", "kelas"},
	"nama_usaha":     {"nama_usaha", "nama usaha"},
	"kategori_usaha": {"kategori_usaha", "kategori usaha", "bidang_usaha", "bidang usaha"},
	"produk_jasa":    {"produk_jasa", "produk jasa"},
	"lokasi_usaha":   {"lokasi_usaha", "lokasi usaha", "lokasi"},
	"tahun_mulai":    {"tahun_mulai", "tahun mulai", "tahun_masuk"},
	"keterangan":     {"keterangan", "catatan"},
}

var siswaLegacyUndefinedColumnMap = ColumnMap{
	"tahun_lulus": {"tahun_lulus", "tahun lulus"},
	"nisn":        {"nisn", "nisn_siswa"},
	"nama_siswa":  {"nama_siswa", "nama siswa", "nama"},
	"rombel":      {"rombel", "rombel_siswa", "kelas"},
	"keterangan":  {"keterangan", "catatan"},
}

var siswaPlanColumnMap = ColumnMap{
	"nisn":             {"nisn", "nisn_siswa"},
	"nama_siswa":       {"nama_siswa", "nama siswa", "nama"},
	"rombel":           {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus":      {"tahun_lulus", "tahun lulus"},
	"plan_jalur":       {"plan_jalur", "rencana_jalur", "jalur_rencana"},
	"pilihan_1_kampus": {"pilihan_1_kampus", "pilihan 1 kampus", "pilihan_1_pt", "pilihan 1 pt"},
	"pilihan_1_prodi":  {"pilihan_1_prodi", "pilihan 1 prodi", "pilihan_1_jurusan", "pilihan 1 jurusan"},
	"pilihan_2_kampus": {"pilihan_2_kampus", "pilihan 2 kampus", "pilihan_2_pt", "pilihan 2 pt"},
	"pilihan_2_prodi":  {"pilihan_2_prodi", "pilihan 2 prodi", "pilihan_2_jurusan", "pilihan 2 jurusan"},
	"pilihan_3_kampus": {"pilihan_3_kampus", "pilihan 3 kampus", "pilihan_3_pt", "pilihan 3 pt"},
	"pilihan_3_prodi":  {"pilihan_3_prodi", "pilihan 3 prodi", "pilihan_3_jurusan", "pilihan 3 jurusan"},
	"pilihan_4_kampus": {"pilihan_4_kampus", "pilihan 4 kampus", "pilihan_4_pt", "pilihan 4 pt"},
	"pilihan_4_prodi":  {"pilihan_4_prodi", "pilihan 4 prodi", "pilihan_4_jurusan", "pilihan 4 jurusan"},
}

var siswaOutcomeColumnMap = ColumnMap{
	"nisn":               {"nisn", "nisn_siswa"},
	"nama_siswa":         {"nama_siswa", "nama siswa", "nama"},
	"rombel":             {"rombel", "rombel_siswa", "kelas"},
	"tahun_lulus":        {"tahun_lulus", "tahun lulus"},
	"status_final":       {"status_final", "final_status", "status akhir", "final"},
	"jalur_final":        {"jalur_final", "final_path", "jalur akhir", "jalur"},
	"tanggal_finalisasi": {"tanggal_finalisasi", "tanggal finalisasi", "finalized_at", "tanggal"},
}

var siswaOutcomeWorkColumnMap = ColumnMap{
	"tahun_lulus":             {"tahun_lulus", "tahun lulus"},
	"nisn":                    {"nisn", "nisn_siswa"},
	"nama_siswa":              {"nama_siswa", "nama siswa", "nama"},
	"rombel":                  {"rombel", "rombel_siswa", "kelas"},
	"nama_perusahaan":         {"nama_perusahaan", "nama perusahaan", "perusahaan"},
	"bidang_field_perusahaan": {"bidang_field_perusahaan", "bidang field perusahaan", "bidang_perusahaan", "bidang perusahaan"},
	"divisi_jabatan":          {"divisi_jabatan", "divisi jabatan", "jabatan", "divisi"},
	"tahun_mulai":             {"tahun_mulai", "tahun mulai", "tahun_masuk_kerja", "tahun masuk kerja"},
}

var siswaOutcomeEntrepreneurColumnMap = ColumnMap{
	"tahun_lulus":              {"tahun_lulus", "tahun lulus"},
	"nisn":                     {"nisn", "nisn_siswa"},
	"nama_siswa":               {"nama_siswa", "nama siswa", "nama"},
	"rombel":                   {"rombel", "rombel_siswa", "kelas"},
	"pilihan_1_nama_usaha":     {"pilihan_1_nama_usaha", "pilihan 1 nama usaha"},
	"pilihan_1_kategori_usaha": {"pilihan_1_kategori_usaha", "pilihan 1 kategori usaha"},
	"pilihan_1_produk_jasa":    {"pilihan_1_produk_jasa", "pilihan 1 produk jasa"},
	"pilihan_1_lokasi_usaha":   {"pilihan_1_lokasi_usaha", "pilihan 1 lokasi usaha"},
	"pilihan_2_nama_usaha":     {"pilihan_2_nama_usaha", "pilihan 2 nama usaha"},
	"pilihan_2_kategori_usaha": {"pilihan_2_kategori_usaha", "pilihan 2 kategori usaha"},
	"pilihan_2_produk_jasa":    {"pilihan_2_produk_jasa", "pilihan 2 produk jasa"},
	"pilihan_2_lokasi_usaha":   {"pilihan_2_lokasi_usaha", "pilihan 2 lokasi usaha"},
	"pilihan_final":            {"pilihan_final", "pilihan final"},
	"fix_pilihan_1":            {"fix_pilihan_1", "fix pilihan 1"},
	"fix_pilihan_2":            {"fix_pilihan_2", "fix pilihan 2"},
}

var siswaOutcomeUndefinedColumnMap = ColumnMap{
	"tahun_lulus": {"tahun_lulus", "tahun lulus"},
	"nisn":        {"nisn", "nisn_siswa"},
	"nama_siswa":  {"nama_siswa", "nama siswa", "nama"},
	"rombel":      {"rombel", "rombel_siswa", "kelas"},
	"keterangan":  {"keterangan", "catatan"},
}

var siswaCohortRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus"}
var siswaApplicationRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus", "jalur_pt", "pilihan_1_kampus", "pilihan_1_prodi"}
var siswaAcceptanceOfferRequired = []string{"tahun_lulus", "nisn", "nama_siswa", "sekolah", "jalur", "pilihan_1_nama_pt", "pilihan_1_prodi", "pilihan_2_nama_pt", "pilihan_2_prodi", "pilihan_3_nama_pt", "pilihan_3_prodi", "pilihan_4_nama_pt", "pilihan_4_prodi", "hasil_pengumuman", "pilihan"}
var siswaAcceptanceOfferCompactRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus", "nama_pt", "jenjang_jurusan", "program_jurusan", "jalur"}
var siswaAcceptanceOfferLegacyRequired = []string{"entry_id", "nisn", "nama_siswa", "sekolah", "jalur", "nama_pt", "prodi", "hasil_pengumuman", "pilihan", "tanggal_pengumuman", "catatan"}
var siswaAcceptanceOfferChoiceRequired = []string{"tahun_lulus", "nisn", "nama_siswa", "rombel", "diterima_pilihan_1", "diterima_pilihan_2", "diterima_pilihan_3", "diterima_pilihan_4"}
var siswaUKKRequired = []string{"nisn", "nama_siswa", "tahun_lulus"}
var siswaHistoricalFinalRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus", "tahun_diterima", "pt", "jurusan"}
var siswaLegacySimpleFinalRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus", "tahun_diterima"}
var siswaLegacyWorkRequired = []string{"tahun_lulus", "nisn", "nama_siswa", "rombel", "tahun_masuk_kerja", "nama_perusahaan", "bidang_field_perusahaan"}
var siswaLegacyEntrepreneurRequired = []string{"tahun_lulus", "nisn", "nama_siswa", "rombel", "nama_usaha", "kategori_usaha", "tahun_mulai"}
var siswaLegacyUndefinedRequired = []string{"tahun_lulus", "nisn", "nama_siswa", "rombel"}
var siswaPlanRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus"}
var siswaOutcomeRequired = []string{"nisn", "nama_siswa", "rombel", "tahun_lulus", "status_final"}
var siswaOutcomeWorkRequired = []string{"tahun_lulus", "nisn", "nama_siswa", "rombel", "nama_perusahaan", "bidang_field_perusahaan"}
var siswaOutcomeEntrepreneurRequired = []string{"tahun_lulus", "nisn", "nama_siswa", "rombel", "pilihan_final"}
var siswaOutcomeEntrepreneurCheckboxLegacyRequired = []string{"tahun_lulus", "nisn", "nama_siswa", "rombel", "fix_pilihan_1", "fix_pilihan_2"}
var siswaOutcomeUndefinedRequired = []string{"tahun_lulus", "nisn", "nama_siswa", "rombel"}
var siswaWirausahaRequired = []string{
	"tahun_lulus",
	"nisn",
	"rencana_ke",
	"nama_usaha",
	"kategori_usaha",
	"status_kesiapan",
	"rencana_mulai",
	"modal_awal",
	"sumber_modal",
}

func ParseSiswaCohorts(buffer []byte) (ParseResult[SiswaCohortRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaCohortRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaCohortRow]{MissingColumns: siswaCohortRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaCohortColumnMap)
	missing := missingColumns(headerMap, siswaCohortRequired)

	parsed := make([]SiswaCohortRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaCohortRow{
			RowNumber:  idx + 2,
			NISN:       getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:  getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:     getCellByKey(row, headerMap, "rombel"),
			TahunLulus: getCellByKey(row, headerMap, "tahun_lulus"),
		})
	}

	return ParseResult[SiswaCohortRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaApplications(buffer []byte) (ParseResult[SiswaApplicationRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaApplicationRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaApplicationRow]{MissingColumns: siswaApplicationRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaApplicationColumnMap)
	missing := missingColumns(headerMap, siswaApplicationRequired)

	parsed := make([]SiswaApplicationRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaApplicationRow{
			RowNumber:       idx + 2,
			NISN:            getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:       getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:          getCellByKey(row, headerMap, "rombel"),
			TahunLulus:      getCellByKey(row, headerMap, "tahun_lulus"),
			JalurPT:         getCellByKey(row, headerMap, "jalur_pt"),
			Pilihan1Kampus:  getCellByKey(row, headerMap, "pilihan_1_kampus"),
			Pilihan1Prodi:   getCellByKey(row, headerMap, "pilihan_1_prodi"),
			Pilihan1Jenjang: getCellByKey(row, headerMap, "pilihan_1_jenjang"),
			Pilihan2Kampus:  getCellByKey(row, headerMap, "pilihan_2_kampus"),
			Pilihan2Prodi:   getCellByKey(row, headerMap, "pilihan_2_prodi"),
			Pilihan2Jenjang: getCellByKey(row, headerMap, "pilihan_2_jenjang"),
			Pilihan3Kampus:  getCellByKey(row, headerMap, "pilihan_3_kampus"),
			Pilihan3Prodi:   getCellByKey(row, headerMap, "pilihan_3_prodi"),
			Pilihan3Jenjang: getCellByKey(row, headerMap, "pilihan_3_jenjang"),
			Pilihan4Kampus:  getCellByKey(row, headerMap, "pilihan_4_kampus"),
			Pilihan4Prodi:   getCellByKey(row, headerMap, "pilihan_4_prodi"),
			Pilihan4Jenjang: getCellByKey(row, headerMap, "pilihan_4_jenjang"),
		})
	}

	return ParseResult[SiswaApplicationRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaWirausaha(buffer []byte) (ParseResult[SiswaWirausahaRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaWirausahaRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaWirausahaRow]{MissingColumns: siswaWirausahaRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaWirausahaColumnMap)
	missing := missingColumns(headerMap, siswaWirausahaRequired)

	parsed := make([]SiswaWirausahaRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaWirausahaRow{
			RowNumber:      idx + 2,
			TahunLulus:     getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:           getCellByKey(row, headerMap, "nisn"),
			RencanaKe:      getCellByKey(row, headerMap, "rencana_ke"),
			NamaUsaha:      getCellByKey(row, headerMap, "nama_usaha"),
			KategoriUsaha:  getCellByKey(row, headerMap, "kategori_usaha"),
			StatusKesiapan: getCellByKey(row, headerMap, "status_kesiapan"),
			RencanaMulai:   getCellByKey(row, headerMap, "rencana_mulai"),
			ModalAwal:      getCellByKey(row, headerMap, "modal_awal"),
			SumberModal:    getCellByKey(row, headerMap, "sumber_modal"),
			Lokasi:         getCellByKey(row, headerMap, "lokasi"),
			Platform:       getCellByKey(row, headerMap, "platform"),
			Catatan:        getCellByKey(row, headerMap, "catatan"),
		})
	}

	return ParseResult[SiswaWirausahaRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaAcceptanceOffers(buffer []byte) (ParseResult[SiswaAcceptanceOfferAnnouncementRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaAcceptanceOfferAnnouncementRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaAcceptanceOfferAnnouncementRow]{MissingColumns: siswaAcceptanceOfferRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaAcceptanceOfferColumnMap)
	missing := missingColumns(headerMap, siswaAcceptanceOfferRequired)

	parsed := make([]SiswaAcceptanceOfferAnnouncementRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaAcceptanceOfferAnnouncementRow{
			RowNumber:         idx + 2,
			TahunLulus:        getCellByKey(row, headerMap, "tahun_lulus"),
			GroupID:           getCellByKey(row, headerMap, "group_id"),
			NISN:              getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:         getCellByKey(row, headerMap, "nama_siswa"),
			Sekolah:           getCellByKey(row, headerMap, "sekolah"),
			Jalur:             getCellByKey(row, headerMap, "jalur"),
			Pilihan1NamaPT:    getCellByKey(row, headerMap, "pilihan_1_nama_pt"),
			Pilihan1Prodi:     getCellByKey(row, headerMap, "pilihan_1_prodi"),
			Pilihan2NamaPT:    getCellByKey(row, headerMap, "pilihan_2_nama_pt"),
			Pilihan2Prodi:     getCellByKey(row, headerMap, "pilihan_2_prodi"),
			Pilihan3NamaPT:    getCellByKey(row, headerMap, "pilihan_3_nama_pt"),
			Pilihan3Prodi:     getCellByKey(row, headerMap, "pilihan_3_prodi"),
			Pilihan4NamaPT:    getCellByKey(row, headerMap, "pilihan_4_nama_pt"),
			Pilihan4Prodi:     getCellByKey(row, headerMap, "pilihan_4_prodi"),
			HasilPengumuman:   getCellByKey(row, headerMap, "hasil_pengumuman"),
			Pilihan:           getCellByKey(row, headerMap, "pilihan"),
			TanggalPengumuman: getCellByKey(row, headerMap, "tanggal_pengumuman"),
			Catatan:           getCellByKey(row, headerMap, "catatan"),
		})
	}

	return ParseResult[SiswaAcceptanceOfferAnnouncementRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseAdminPTOffers(buffer []byte) (ParseResult[AdminPTOfferRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[AdminPTOfferRow]{}, err
	}
	required := []string{"tahun_lulus", "jalur", "nama_siswa", "nisn", "nama_sekolah", "nama_pt", "program_studi", "status"}
	if len(headers) == 0 {
		return ParseResult[AdminPTOfferRow]{MissingColumns: required}, nil
	}
	headerMap := buildHeaderMap(headers, adminPTOfferColumnMap)
	missing := missingColumns(headerMap, required)
	parsed := make([]AdminPTOfferRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, AdminPTOfferRow{
			RowNumber:    idx + 2,
			TahunLulus:   getCellByKey(row, headerMap, "tahun_lulus"),
			Jalur:        getCellByKey(row, headerMap, "jalur"),
			NamaSiswa:    getCellByKey(row, headerMap, "nama_siswa"),
			NISN:         getCellByKey(row, headerMap, "nisn"),
			NamaSekolah:  getCellByKey(row, headerMap, "nama_sekolah"),
			NamaPT:       getCellByKey(row, headerMap, "nama_pt"),
			ProgramStudi: getCellByKey(row, headerMap, "program_studi"),
			Status:       getCellByKey(row, headerMap, "status"),
		})
	}
	return ParseResult[AdminPTOfferRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaAcceptanceOfferCompact(buffer []byte) (ParseResult[SiswaAcceptanceOfferCompactRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaAcceptanceOfferCompactRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaAcceptanceOfferCompactRow]{MissingColumns: siswaAcceptanceOfferCompactRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaAcceptanceOfferCompactColumnMap)
	missing := missingColumns(headerMap, siswaAcceptanceOfferCompactRequired)

	parsed := make([]SiswaAcceptanceOfferCompactRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaAcceptanceOfferCompactRow{
			RowNumber:      idx + 2,
			NISN:           getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:      getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:         getCellByKey(row, headerMap, "rombel"),
			TahunLulus:     getCellByKey(row, headerMap, "tahun_lulus"),
			NamaPT:         getCellByKey(row, headerMap, "nama_pt"),
			JenjangJurusan: getCellByKey(row, headerMap, "jenjang_jurusan"),
			ProgramJurusan: getCellByKey(row, headerMap, "program_jurusan"),
			Jalur:          getCellByKey(row, headerMap, "jalur"),
		})
	}

	return ParseResult[SiswaAcceptanceOfferCompactRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaAcceptanceOfferVerticalLegacy(buffer []byte) (ParseResult[SiswaAcceptanceOfferAnnouncementLegacyRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaAcceptanceOfferAnnouncementLegacyRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaAcceptanceOfferAnnouncementLegacyRow]{MissingColumns: siswaAcceptanceOfferLegacyRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaAcceptanceOfferLegacyColumnMap)
	missing := missingColumns(headerMap, siswaAcceptanceOfferLegacyRequired)

	parsed := make([]SiswaAcceptanceOfferAnnouncementLegacyRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaAcceptanceOfferAnnouncementLegacyRow{
			RowNumber:         idx + 2,
			EntryID:           getCellByKey(row, headerMap, "entry_id"),
			NISN:              getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:         getCellByKey(row, headerMap, "nama_siswa"),
			Sekolah:           getCellByKey(row, headerMap, "sekolah"),
			Jalur:             getCellByKey(row, headerMap, "jalur"),
			NamaPT:            getCellByKey(row, headerMap, "nama_pt"),
			Prodi:             getCellByKey(row, headerMap, "prodi"),
			HasilPengumuman:   getCellByKey(row, headerMap, "hasil_pengumuman"),
			Pilihan:           getCellByKey(row, headerMap, "pilihan"),
			TanggalPengumuman: getCellByKey(row, headerMap, "tanggal_pengumuman"),
			Catatan:           getCellByKey(row, headerMap, "catatan"),
		})
	}

	return ParseResult[SiswaAcceptanceOfferAnnouncementLegacyRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaAcceptanceOfferChoices(buffer []byte) (ParseResult[SiswaAcceptanceOfferChoiceRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaAcceptanceOfferChoiceRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaAcceptanceOfferChoiceRow]{MissingColumns: siswaAcceptanceOfferChoiceRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaAcceptanceOfferChoiceColumnMap)
	missing := missingColumns(headerMap, siswaAcceptanceOfferChoiceRequired)

	parsed := make([]SiswaAcceptanceOfferChoiceRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaAcceptanceOfferChoiceRow{
			RowNumber:        idx + 2,
			TahunLulus:       getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:             getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:        getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:           getCellByKey(row, headerMap, "rombel"),
			Pilihan1Jalur:    getCellByKey(row, headerMap, "pilihan_1_jalur"),
			Pilihan1NamaPT:   getCellByKey(row, headerMap, "pilihan_1_nama_pt"),
			Pilihan1Prodi:    getCellByKey(row, headerMap, "pilihan_1_prodi"),
			Pilihan2Jalur:    getCellByKey(row, headerMap, "pilihan_2_jalur"),
			Pilihan2NamaPT:   getCellByKey(row, headerMap, "pilihan_2_nama_pt"),
			Pilihan2Prodi:    getCellByKey(row, headerMap, "pilihan_2_prodi"),
			Pilihan3Jalur:    getCellByKey(row, headerMap, "pilihan_3_jalur"),
			Pilihan3NamaPT:   getCellByKey(row, headerMap, "pilihan_3_nama_pt"),
			Pilihan3Prodi:    getCellByKey(row, headerMap, "pilihan_3_prodi"),
			Pilihan4Jalur:    getCellByKey(row, headerMap, "pilihan_4_jalur"),
			Pilihan4NamaPT:   getCellByKey(row, headerMap, "pilihan_4_nama_pt"),
			Pilihan4Prodi:    getCellByKey(row, headerMap, "pilihan_4_prodi"),
			DiterimaPilihan1: getCellByKey(row, headerMap, "diterima_pilihan_1"),
			DiterimaPilihan2: getCellByKey(row, headerMap, "diterima_pilihan_2"),
			DiterimaPilihan3: getCellByKey(row, headerMap, "diterima_pilihan_3"),
			DiterimaPilihan4: getCellByKey(row, headerMap, "diterima_pilihan_4"),
		})
	}

	return ParseResult[SiswaAcceptanceOfferChoiceRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaUKK(buffer []byte) (ParseResult[SiswaUKKRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaUKKRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaUKKRow]{MissingColumns: siswaUKKRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaUKKColumnMap)
	missing := missingColumns(headerMap, siswaUKKRequired)

	type assessorSlot struct {
		NameIdx     int
		InstansiIdx int
	}
	assessorSlots := map[int]*assessorSlot{}
	parseAssessorHeader := func(normalized string) (int, string, bool) {
		if !(strings.HasPrefix(normalized, "assessor") || strings.HasPrefix(normalized, "asesor")) {
			return 0, "", false
		}
		slot := 1
		fields := strings.Fields(normalized)
		for _, field := range fields {
			if parsed, err := strconv.Atoi(field); err == nil {
				slot = parsed
				break
			}
		}
		field := "nama"
		if strings.Contains(normalized, "instansi") {
			field = "instansi"
		}
		return slot, field, true
	}
	for idx, header := range headers {
		normalized := normalizeHeader(header)
		if normalized == "assessor" || normalized == "asesor" {
			continue
		}
		slot, field, ok := parseAssessorHeader(normalized)
		if !ok {
			continue
		}
		entry := assessorSlots[slot]
		if entry == nil {
			entry = &assessorSlot{NameIdx: -1, InstansiIdx: -1}
			assessorSlots[slot] = entry
		}
		if field == "instansi" {
			entry.InstansiIdx = idx
		} else {
			entry.NameIdx = idx
		}
	}

	parsed := make([]SiswaUKKRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}

		assessorSet := map[string]struct{}{}
		assessors := make([]SiswaUKKAssessor, 0)
		appendAssessor := func(name, instansi string) {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				return
			}
			trimmedInstansi := strings.TrimSpace(instansi)
			key := strings.ToUpper(trimmed + "|" + trimmedInstansi)
			if _, ok := assessorSet[key]; ok {
				return
			}
			assessorSet[key] = struct{}{}
			assessors = append(assessors, SiswaUKKAssessor{
				Nama:     trimmed,
				Instansi: trimmedInstansi,
			})
		}
		appendAssessorTokens := func(value, instansi string) {
			for _, token := range strings.FieldsFunc(value, func(r rune) bool {
				return r == ',' || r == ';'
			}) {
				appendAssessor(token, instansi)
			}
		}

		appendAssessorTokens(getCellByKey(row, headerMap, "assessor"), "")
		for _, slot := range assessorSlots {
			if slot == nil {
				continue
			}
			name := ""
			if slot.NameIdx >= 0 {
				name = getCell(row, slot.NameIdx)
			}
			instansi := ""
			if slot.InstansiIdx >= 0 {
				instansi = getCell(row, slot.InstansiIdx)
			}
			if strings.TrimSpace(name) == "" {
				continue
			}
			appendAssessorTokens(name, instansi)
		}

		parsed = append(parsed, SiswaUKKRow{
			RowNumber:       idx + 2,
			NISN:            getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:       getCellByKey(row, headerMap, "nama_siswa"),
			TahunLulus:      getCellByKey(row, headerMap, "tahun_lulus"),
			JenisUKK:        getCellByKey(row, headerMap, "jenis_ukk"),
			SkemaUKK:        getCellByKey(row, headerMap, "skema_ukk"),
			Hasil:           getCellByKey(row, headerMap, "hasil"),
			NilaiAkhir:      getCellByKey(row, headerMap, "nilai_akhir"),
			Predikat:        getCellByKey(row, headerMap, "predikat"),
			NomorSertifikat: getCellByKey(row, headerMap, "nomor_sertifikat"),
			TanggalUKK:      getCellByKey(row, headerMap, "tanggal_ukk"),
			Assessors:       assessors,
		})
	}

	return ParseResult[SiswaUKKRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaHistoricalFinals(buffer []byte) (ParseResult[SiswaHistoricalFinalRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaHistoricalFinalRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaHistoricalFinalRow]{MissingColumns: siswaHistoricalFinalRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaHistoricalFinalColumnMap)
	missing := missingColumns(headerMap, siswaHistoricalFinalRequired)

	parsed := make([]SiswaHistoricalFinalRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaHistoricalFinalRow{
			RowNumber:       idx + 2,
			NISN:            getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:       getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:          getCellByKey(row, headerMap, "rombel"),
			TahunLulus:      getCellByKey(row, headerMap, "tahun_lulus"),
			TahunDiterima:   getCellByKey(row, headerMap, "tahun_diterima"),
			JalurPenerimaan: getCellByKey(row, headerMap, "jalur"),
			PerguruanTinggi: getCellByKey(row, headerMap, "pt"),
			Jurusan:         getCellByKey(row, headerMap, "jurusan"),
		})
	}

	return ParseResult[SiswaHistoricalFinalRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaLegacyFinalSimple(buffer []byte) (ParseResult[SiswaLegacyFinalSimpleRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaLegacyFinalSimpleRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaLegacyFinalSimpleRow]{MissingColumns: siswaLegacySimpleFinalRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaLegacySimpleFinalColumnMap)
	missing := missingColumns(headerMap, siswaLegacySimpleFinalRequired)

	parsed := make([]SiswaLegacyFinalSimpleRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaLegacyFinalSimpleRow{
			RowNumber:     idx + 2,
			NISN:          getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:     getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:        getCellByKey(row, headerMap, "rombel"),
			TahunLulus:    getCellByKey(row, headerMap, "tahun_lulus"),
			TahunDiterima: getCellByKey(row, headerMap, "tahun_diterima"),
		})
	}

	return ParseResult[SiswaLegacyFinalSimpleRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaLegacyWork(buffer []byte) (ParseResult[SiswaLegacyWorkRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaLegacyWorkRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaLegacyWorkRow]{MissingColumns: siswaLegacyWorkRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaLegacyWorkColumnMap)
	missing := missingColumns(headerMap, siswaLegacyWorkRequired)

	parsed := make([]SiswaLegacyWorkRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaLegacyWorkRow{
			RowNumber:             idx + 2,
			TahunLulus:            getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:                  getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:             getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:                getCellByKey(row, headerMap, "rombel"),
			TahunMasukKerja:       getCellByKey(row, headerMap, "tahun_masuk_kerja"),
			NamaPerusahaan:        getCellByKey(row, headerMap, "nama_perusahaan"),
			BidangFieldPerusahaan: getCellByKey(row, headerMap, "bidang_field_perusahaan"),
			DivisiJabatan:         getCellByKey(row, headerMap, "divisi_jabatan"),
		})
	}

	return ParseResult[SiswaLegacyWorkRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaLegacyEntrepreneur(buffer []byte) (ParseResult[SiswaLegacyEntrepreneurRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaLegacyEntrepreneurRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaLegacyEntrepreneurRow]{MissingColumns: siswaLegacyEntrepreneurRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaLegacyEntrepreneurColumnMap)
	missing := missingColumns(headerMap, siswaLegacyEntrepreneurRequired)

	parsed := make([]SiswaLegacyEntrepreneurRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaLegacyEntrepreneurRow{
			RowNumber:     idx + 2,
			TahunLulus:    getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:          getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:     getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:        getCellByKey(row, headerMap, "rombel"),
			NamaUsaha:     getCellByKey(row, headerMap, "nama_usaha"),
			KategoriUsaha: getCellByKey(row, headerMap, "kategori_usaha"),
			ProdukJasa:    getCellByKey(row, headerMap, "produk_jasa"),
			LokasiUsaha:   getCellByKey(row, headerMap, "lokasi_usaha"),
			TahunMulai:    getCellByKey(row, headerMap, "tahun_mulai"),
			Keterangan:    getCellByKey(row, headerMap, "keterangan"),
		})
	}

	return ParseResult[SiswaLegacyEntrepreneurRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaLegacyUndefined(buffer []byte) (ParseResult[SiswaLegacyUndefinedRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaLegacyUndefinedRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaLegacyUndefinedRow]{MissingColumns: siswaLegacyUndefinedRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaLegacyUndefinedColumnMap)
	missing := missingColumns(headerMap, siswaLegacyUndefinedRequired)

	parsed := make([]SiswaLegacyUndefinedRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaLegacyUndefinedRow{
			RowNumber:  idx + 2,
			TahunLulus: getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:       getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:  getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:     getCellByKey(row, headerMap, "rombel"),
			Keterangan: getCellByKey(row, headerMap, "keterangan"),
		})
	}

	return ParseResult[SiswaLegacyUndefinedRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaPlans(buffer []byte) (ParseResult[SiswaPlanRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaPlanRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaPlanRow]{MissingColumns: siswaPlanRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaPlanColumnMap)
	missing := missingColumns(headerMap, siswaPlanRequired)

	parsed := make([]SiswaPlanRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaPlanRow{
			RowNumber:      idx + 2,
			NISN:           getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:      getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:         getCellByKey(row, headerMap, "rombel"),
			TahunLulus:     getCellByKey(row, headerMap, "tahun_lulus"),
			PlanJalur:      getCellByKey(row, headerMap, "plan_jalur"),
			Pilihan1Kampus: getCellByKey(row, headerMap, "pilihan_1_kampus"),
			Pilihan1Prodi:  getCellByKey(row, headerMap, "pilihan_1_prodi"),
			Pilihan2Kampus: getCellByKey(row, headerMap, "pilihan_2_kampus"),
			Pilihan2Prodi:  getCellByKey(row, headerMap, "pilihan_2_prodi"),
			Pilihan3Kampus: getCellByKey(row, headerMap, "pilihan_3_kampus"),
			Pilihan3Prodi:  getCellByKey(row, headerMap, "pilihan_3_prodi"),
			Pilihan4Kampus: getCellByKey(row, headerMap, "pilihan_4_kampus"),
			Pilihan4Prodi:  getCellByKey(row, headerMap, "pilihan_4_prodi"),
		})
	}

	return ParseResult[SiswaPlanRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaOutcomes(buffer []byte) (ParseResult[SiswaOutcomeRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaOutcomeRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaOutcomeRow]{MissingColumns: siswaOutcomeRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaOutcomeColumnMap)
	missing := missingColumns(headerMap, siswaOutcomeRequired)

	parsed := make([]SiswaOutcomeRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaOutcomeRow{
			RowNumber:         idx + 2,
			NISN:              getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:         getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:            getCellByKey(row, headerMap, "rombel"),
			TahunLulus:        getCellByKey(row, headerMap, "tahun_lulus"),
			StatusFinal:       getCellByKey(row, headerMap, "status_final"),
			JalurFinal:        getCellByKey(row, headerMap, "jalur_final"),
			TanggalFinalisasi: getCellByKey(row, headerMap, "tanggal_finalisasi"),
		})
	}

	return ParseResult[SiswaOutcomeRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaOutcomeWork(buffer []byte) (ParseResult[SiswaOutcomeWorkRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaOutcomeWorkRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaOutcomeWorkRow]{MissingColumns: siswaOutcomeWorkRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaOutcomeWorkColumnMap)
	missing := missingColumns(headerMap, siswaOutcomeWorkRequired)

	parsed := make([]SiswaOutcomeWorkRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaOutcomeWorkRow{
			RowNumber:             idx + 2,
			TahunLulus:            getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:                  getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:             getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:                getCellByKey(row, headerMap, "rombel"),
			NamaPerusahaan:        getCellByKey(row, headerMap, "nama_perusahaan"),
			BidangFieldPerusahaan: getCellByKey(row, headerMap, "bidang_field_perusahaan"),
			DivisiJabatan:         getCellByKey(row, headerMap, "divisi_jabatan"),
			TahunMulai:            getCellByKey(row, headerMap, "tahun_mulai"),
		})
	}

	return ParseResult[SiswaOutcomeWorkRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaOutcomeEntrepreneur(buffer []byte) (ParseResult[SiswaOutcomeEntrepreneurRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaOutcomeEntrepreneurRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaOutcomeEntrepreneurRow]{MissingColumns: siswaOutcomeEntrepreneurRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaOutcomeEntrepreneurColumnMap)
	missing := missingColumns(headerMap, siswaOutcomeEntrepreneurRequired)

	parsed := make([]SiswaOutcomeEntrepreneurRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaOutcomeEntrepreneurRow{
			RowNumber:           idx + 2,
			TahunLulus:          getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:                getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:           getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:              getCellByKey(row, headerMap, "rombel"),
			Pilihan1NamaUsaha:   getCellByKey(row, headerMap, "pilihan_1_nama_usaha"),
			Pilihan1Kategori:    getCellByKey(row, headerMap, "pilihan_1_kategori_usaha"),
			Pilihan1ProdukJasa:  getCellByKey(row, headerMap, "pilihan_1_produk_jasa"),
			Pilihan1LokasiUsaha: getCellByKey(row, headerMap, "pilihan_1_lokasi_usaha"),
			Pilihan2NamaUsaha:   getCellByKey(row, headerMap, "pilihan_2_nama_usaha"),
			Pilihan2Kategori:    getCellByKey(row, headerMap, "pilihan_2_kategori_usaha"),
			Pilihan2ProdukJasa:  getCellByKey(row, headerMap, "pilihan_2_produk_jasa"),
			Pilihan2LokasiUsaha: getCellByKey(row, headerMap, "pilihan_2_lokasi_usaha"),
			PilihanFinal:        getCellByKey(row, headerMap, "pilihan_final"),
			FixPilihan1:         getCellByKey(row, headerMap, "fix_pilihan_1"),
			FixPilihan2:         getCellByKey(row, headerMap, "fix_pilihan_2"),
		})
	}

	return ParseResult[SiswaOutcomeEntrepreneurRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaOutcomeEntrepreneurCheckboxLegacy(buffer []byte) (ParseResult[SiswaOutcomeEntrepreneurRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaOutcomeEntrepreneurRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaOutcomeEntrepreneurRow]{MissingColumns: siswaOutcomeEntrepreneurCheckboxLegacyRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaOutcomeEntrepreneurColumnMap)
	missing := missingColumns(headerMap, siswaOutcomeEntrepreneurCheckboxLegacyRequired)

	parsed := make([]SiswaOutcomeEntrepreneurRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaOutcomeEntrepreneurRow{
			RowNumber:           idx + 2,
			TahunLulus:          getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:                getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:           getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:              getCellByKey(row, headerMap, "rombel"),
			Pilihan1NamaUsaha:   getCellByKey(row, headerMap, "pilihan_1_nama_usaha"),
			Pilihan1Kategori:    getCellByKey(row, headerMap, "pilihan_1_kategori_usaha"),
			Pilihan1ProdukJasa:  getCellByKey(row, headerMap, "pilihan_1_produk_jasa"),
			Pilihan1LokasiUsaha: getCellByKey(row, headerMap, "pilihan_1_lokasi_usaha"),
			Pilihan2NamaUsaha:   getCellByKey(row, headerMap, "pilihan_2_nama_usaha"),
			Pilihan2Kategori:    getCellByKey(row, headerMap, "pilihan_2_kategori_usaha"),
			Pilihan2ProdukJasa:  getCellByKey(row, headerMap, "pilihan_2_produk_jasa"),
			Pilihan2LokasiUsaha: getCellByKey(row, headerMap, "pilihan_2_lokasi_usaha"),
			FixPilihan1:         getCellByKey(row, headerMap, "fix_pilihan_1"),
			FixPilihan2:         getCellByKey(row, headerMap, "fix_pilihan_2"),
		})
	}

	return ParseResult[SiswaOutcomeEntrepreneurRow]{Rows: parsed, MissingColumns: missing}, nil
}

func ParseSiswaOutcomeUndefined(buffer []byte) (ParseResult[SiswaOutcomeUndefinedRow], error) {
	headers, rows, err := readFirstSheetRows(buffer)
	if err != nil {
		return ParseResult[SiswaOutcomeUndefinedRow]{}, err
	}
	if len(headers) == 0 {
		return ParseResult[SiswaOutcomeUndefinedRow]{MissingColumns: siswaOutcomeUndefinedRequired}, nil
	}

	headerMap := buildHeaderMap(headers, siswaOutcomeUndefinedColumnMap)
	missing := missingColumns(headerMap, siswaOutcomeUndefinedRequired)

	parsed := make([]SiswaOutcomeUndefinedRow, 0, len(rows))
	for idx, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		parsed = append(parsed, SiswaOutcomeUndefinedRow{
			RowNumber:  idx + 2,
			TahunLulus: getCellByKey(row, headerMap, "tahun_lulus"),
			NISN:       getCellByKey(row, headerMap, "nisn"),
			NamaSiswa:  getCellByKey(row, headerMap, "nama_siswa"),
			Rombel:     getCellByKey(row, headerMap, "rombel"),
			Keterangan: getCellByKey(row, headerMap, "keterangan"),
		})
	}

	return ParseResult[SiswaOutcomeUndefinedRow]{Rows: parsed, MissingColumns: missing}, nil
}

func readFirstSheetRows(buffer []byte) ([]string, [][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(buffer))
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	sheet := f.GetSheetName(0)
	if strings.TrimSpace(sheet) == "" {
		return nil, nil, nil
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, nil
	}
	return rows[0], rows[1:], nil
}
