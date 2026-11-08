export const achievementTemplateHeaders = [
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
];

export const acceptanceTemplateHeaders = [
  "ID",
  "NISN_Siswa",
  "Nama_Siswa",
  "Rombel_Siswa",
  "Tahun_Lulus",
  "Nama_Perguruan_Tinggi",
  "Jurusan_Perguruan_Tinggi",
  "Tahun_Diterima",
  "Jalur_Penerimaan",
];

export const expectedHeadersByUploadType = {
  ACHIEVEMENT: achievementTemplateHeaders,
  ACCEPTANCE: acceptanceTemplateHeaders,
} as const;
