# Pelaksanaan portal Phase 4–6

Tanggal: 6–7 Oktober 2026 (Asia/Jakarta).
Branch: `codex/portal-phase-4-6`, dibuat dari `origin/Staging` setelah fetch; Staging lokal dan remote identik (0 ahead/0 behind). Working tree awal bersih. Tidak menggunakan atau merge branch lama. Tidak push ke Staging/Production.

## Audit awal

- Foundation Phase 1–3 sudah tersedia: portal primitives, navbar publik, landing, Leaflet map, form lokasi, API lokasi, migration 066, serta importer Excel. Komponen tersebut digunakan kembali.
- `components/app-shell.tsx` masih mengimpor dan merender AppSidebar untuk hampir semua halaman selain homepage/report/export. Navbar publik hanya dipasang pada homepage.
- AppSidebar memuat bukan hanya navigasi: logout, pending-validasi, unduh PDF dan regenerate. Fitur ini perlu dipindahkan, bukan dihilangkan.
- Permission frontend bersumber dari `lib/navigation/sidebar-menu.ts`, role-scope, dan gaspol-school-access. Scope lokasi backend memakai `GetMultiSchoolScope`/`EnsureSchoolInScope`. Semua tetap digunakan.
- Homepage memiliki placeholder Peserta Didik. `currentYearGuruTendik` menghitung prestasi berjenis GURU/TENDIK, bukan pegawai. Jumlah pegawai sebenarnya tersedia pada empat kolom jumlah guru/tendik di master `data_sekolah`.
- `prestasiTotal` sebelumnya seluruh tahun. Analytics penerimaan PT juga tidak tepat untuk headline berdasarkan tahun lulusan. Agregat publik status lulusan final sudah tersedia.
- GASPOL menggunakan view berdasar role dan section/card dalam satu halaman; MGMP menggunakan workspace/monitoring, route histori, pendaftaran dan settings. Form dan workflow tidak perlu dipindah atau ditulis ulang.
- Settings Lokasi sudah memakai `resolveLocationSchool`; SCHOOL_ADMIN tidak memiliki dropdown sekolah dan harus memiliki schoolId yang cocok dengan daftar scope API. Perilaku ini dipertahankan dan dites.
- Lokasi menggunakan enam kolom di `data_sekolah`; tidak ada kebutuhan tabel lokasi kedua. Frontend map membaca API, bukan Excel. Filter jenjang/status tersedia; kecamatan belum tersedia pada proyeksi lokasi/master yang digunakan.

## Phase 4: navigasi dan layout

- AppShell normal memakai navbar atas bersama, tanpa AppSidebar. File lama tetap disimpan tetapi tidak diimpor AppShell. Report/export tetap mengikuti layout khusus existing.
- Navbar: menu kelompok role-aware, Pusat Tautan, pencarian, pending-validasi, menu akun, Settings/Panduan/Backup sesuai akses, logout, unduh/regenerate PDF.
- Logika job laporan disalin dari sidebar ke `report-download.tsx` tanpa mengubah endpoint, filter, polling, atau kewenangan.
- USER_MGMP hanya mendapat kelompok MGMP sesuai redirect workspace existing. Akun GASPOL khusus tetap memakai allowlist existing; Beranda menuju workspace masing-masing.
- Child menu publik MGMP yang sebelumnya hilang karena filter root diterapkan secara rekursif kini tampil sesuai allowedRoles.
- Drawer modal native mempunyai tombol tutup dan Escape. Dropdown native mendukung keyboard/Escape. Navbar sticky; navigasi horizontal penuh mulai viewport 1440px, ukuran di bawahnya memakai drawer.
- ModuleNavigation menampilkan child route yang diizinkan. PageSectionNavigation membuat tautan horizontal ke heading/card yang benar-benar dirender GASPOL, MGMP dan Settings. Form tetap mounted dan workflow tidak diubah; bukan membuat route/tab tahapan yang tidak ada.
- Indikator waktu dipindah dari overlay kanan atas ke alur layout agar tidak menutupi menu akun. Logika waktu tidak berubah.

## Phase 5: integrasi halaman

- Beranda, dashboard internal, Prestasi, Status Lulusan, Pengaduan, GASPOL, MGMP dan Settings memakai container portal, latar terang dan navbar bersama melalui AppShell.
- Shared card/header/filter/table existing dipertahankan. Tidak ada rewrite scoring/ranking/approval/attendance/import-export.
- Peserta Didik dihapus dari headline tanpa placeholder pengganti.
- Guru & Tendik memakai penjumlahan master sekolah, tanpa label tahun buatan.
- Prestasi memakai field API tambahan `portalPrestasiYear` dan `portalPrestasiTotal`, difilter tahun berjalan WIB. Field lama tetap utuh untuk konsumen existing.
- Lulusan PT memakai `/public/siswa/analytics/diterima-perguruan-tinggi`, memilih agregat tahun lulus terbaru dan menampilkan `Tahun Lulusan XXXX`. Tidak lagi memakai total penerimaan lintas tahun.
- Definisi existing Sekolah Aktif tetap dipertahankan: sekolah yang pernah memperbarui data prestasi/PT. Keterangan ini ditampilkan; angka ini bukan jumlah seluruh 144 master sekolah.
- Respons API bertanda fallback tidak ditampilkan sebagai nol faktual pada homepage.
- Cek Pengaduan mendapat tautan `?tab=LACAK` menuju tab pelacakan existing. Submit, bukti, PIN, status, kerahasiaan dan tindak lanjut tidak diubah.

## Phase 6: map, lokasi dan seed

- API publik terverifikasi mengembalikan 144 sekolah, 30 di antaranya berkoordinat. Marker dan popup tetap memakai ID, NPSN, jenjang, status, akreditasi dan koordinat database.
- Form lokasi menampilkan nama dan NPSN dari detail API, field koordinat, sumber, verified, timestamp/editor, preview dan simpan. Field lat/lon dapat diedit; tidak menambahkan drag/reset destructive.
- Migration 066 diverifikasi lewat tes baca-saja database lokal: tercatat applied, enam kolom ada, empat constraint koordinat ada. Tidak menjalankan migration ulang dan tidak membuat migration/tabel duplikat.
- Importer tetap default validasi workbook; `--apply` diperlukan untuk update. Matching NPSN hanya UPDATE sekolah existing, tidak INSERT. Guard koordinat/audit melindungi titik terisi, hasil import terdahulu, maupun titik yang sengaja dikosongkan admin.
- Logging dipisah: invalid coordinate, invalid NPSN, duplicate NPSN, matched, imported, not found, existing/manual.
- Opsi baru `--check-db` melakukan matching dengan transaksi READ ONLY, tanpa update.

### Hasil Excel dan matching lokal

| Kategori | Jumlah |
|---|---:|
| Total baris | 144 |
| Koordinat valid dan NPSN valid/unik | 30 |
| Koordinat kosong/tidak valid | 114 |
| NPSN invalid | 0 |
| NPSN duplikat valid | 0 |
| Match sekolah existing (baris valid) | 30 |
| Tidak match (baris valid) | 0 |
| Sudah berkoordinat/ada riwayat perubahan | 30 |
| Eligible untuk initial seed | 0 |
| Diimpor dalam sesi ini | 0 |

Hasil di atas berasal dari validasi dan `--check-db`. `--apply` tidak dijalankan karena seluruh 30 baris valid sudah terlindungi guard existing/manual. Tidak mengklaim import/migration Staging atau Production.

## File utama

- `components/app-shell.tsx`
- `components/portal/public-navbar.tsx`
- `components/portal/module-navigation.tsx`
- `components/portal/page-section-navigation.tsx` (baru)
- `components/portal/report-download.tsx` (baru)
- `components/system-time/SystemTimeIndicator.tsx`
- `lib/navigation/sidebar-menu.ts`
- `app/page.tsx`
- `app/pengaduan-kritik-saran/page.tsx`
- `components/portal/school-location-settings.tsx`
- `backend/internal/services/landing_summary.go`
- `backend/internal/services/school_location_seed.go`
- `backend/cmd/import-school-locations/main.go`
- `backend/internal/services/school_location_seed_test.go`
- `backend/internal/services/portal_readonly_integration_test.go` (baru)
- `backend/internal/handlers/portal_location_readonly_integration_test.go` (baru)
- `scripts/test-portal-navigation.cjs`

## Validasi yang benar-benar dilakukan

- `npm run lint`: PASS, tanpa error/warning ESLint.
- `npx tsc --noEmit`: PASS.
- `npm run build`: PASS, 157 halaman diproses.
- `go build ./...`: PASS, termasuk server/importer.
- `go test ./...`: PASS. Tes integrasi lain yang membutuhkan konfigurasi terpisah tetap mengikuti skip existing.
- `node scripts/test-portal-navigation.cjs`: PASS untuk publik dan delapan role, termasuk batasan MGMP/GASPOL dan child menu publik.
- `node scripts/test-portal-location.mjs`: PASS, 13 assertion own mapping, mapping hilang, scope dan pemilihan admin. Node memberi warning MODULE_TYPELESS_PACKAGE_JSON existing; tidak mengubah package type global hanya untuk menghilangkannya.
- Tes opt-in dengan `PORTAL_TEST_DATABASE_URL`: PASS. READ ONLY dipaksakan di koneksi. Memeriksa registry migration, kolom/constraint, statistik master/tahun dan delapan kasus akses lokasi (own school, other school forbidden, master, admin allowed/forbidden level, cabdin, MGMP/GASPOL forbidden).
- Agregat lulusan final publik dites melalui handler dan database lokal: PASS, mengembalikan tiga tahun lulusan, tanpa fallback.
- Tes existing handler lokasi menolak anonymous dan PUT lintas sekolah/role; PASS.
- `git diff --check`: PASS.

### Browser sebelum akses diblokir

- Homepage publik dan drawer: diperiksa; tombol buka/tutup bekerja.
- Desktop 1440x900: screenshot homepage diperiksa, tidak overflow (scrollWidth 1425).
- Mobile 390x844: halaman login diperiksa, tidak overflow (scrollWidth 390).
- Login ADMIN_MASTER disediakan pengguna; navbar akun, Settings dan GASPOL berhasil dibuka dengan data lokal.
- Settings: navigasi bagian menuju Lokasi, memilih SMA Negeri 1 Bangkalan, nama/NPSN/lat/lon/sumber/verifikasi/editor terlihat; Leaflet preview termuat. Tidak menyimpan perubahan lokasi sekolah nyata.
- GASPOL ADMIN_MASTER: data peserta/ranking dan kontrol existing termuat; tidak menjalankan penilaian/penetapan/approval.
- Pemeriksaan tersebut menemukan overlay waktu yang menutupi akun; source sudah diperbaiki.

### Batas verifikasi dan tindakan lanjutan

**Definition of Done penuh belum tercapai.** Setelah sesi berlanjut, browser menolak localhost karena saved user permission. Pengguna menyetujui kembali lewat chat, tetapi percobaan ulang masih ditolak oleh izin tersimpan. Tidak memakai jalur lain untuk melewati blokir.

Masih perlu setelah izin blokir localhost dicabut:
1. Periksa build terakhir pada desktop 1440, tablet 768, mobile 390: dropdown, Escape, drawer, active state, overflow, indikator waktu.
2. Uji halaman publik Prestasi, Status Lulusan, Pengaduan/Cek Pengaduan, map marker/filter/popup; sebagian halaman ini belum diperiksa secara visual pada sesi ini.
3. Uji UI login SCHOOL_ADMIN, ADMIN dan USER_MGMP, termasuk default sekolah sendiri. Permission dan resolver sudah dites otomatis; login UI per role belum.
4. Uji round-trip simpan lokasi pada data uji yang disepakati dan regression lengkap alur GASPOL/MGMP/SIPRES/Pengaduan. Jangan menganggap tes baca-saja membuktikan workflow tulis.
5. Validasi ulang navigasi section GASPOL/CardTitle, child dropdown dan perubahan sumber lulusan pada build terakhir; perbaikan ini dibuat setelah pemeriksaan browser awal.

Log backend lokal juga menampilkan kegagalan worker auto-finalize existing pada FK `student_outcomes_finalized_by_fkey`. Tidak mengubah rule/worker SIPRES untuk menyelesaikan task layout ini; perlu diperiksa terpisah sebelum klaim regression seluruh workflow bersih.

## Perintah deploy/seed yang aman

Dari folder `backend`, dengan `DATABASE_URL` target sudah diatur secara aman:

```powershell
goose -dir migrations postgres "$env:DATABASE_URL" status
goose -dir migrations postgres "$env:DATABASE_URL" up
go run ./cmd/import-school-locations --file assets/data-sekolah-map-bangkalan.xlsx
go run ./cmd/import-school-locations --file assets/data-sekolah-map-bangkalan.xlsx --check-db
# Hanya initial seed setelah meninjau report matching:
go run ./cmd/import-school-locations --file assets/data-sekolah-map-bangkalan.xlsx --apply
```

Tidak menggunakan script reset/dump-restore. Migration Down 066 memang menolak downgrade untuk melindungi koordinat; rollback aplikasi dapat dilakukan tanpa menghapus kolom.

Tes integrasi read-only (database yang dipilih secara eksplisit):

```powershell
$env:PORTAL_TEST_DATABASE_URL = $env:DATABASE_URL
go test ./internal/services ./internal/handlers -run 'TestPortal.*ReadOnlyIntegration' -v
```

Setelah deploy, pastikan migration 066 applied, endpoint publik membaca DB, scope GET/PUT lokasi menolak sekolah lain, label tahun sesuai agregat, dan reimport melewati koordinat terisi/manual. Tidak ada deploy/push yang dilakukan dalam sesi ini.
