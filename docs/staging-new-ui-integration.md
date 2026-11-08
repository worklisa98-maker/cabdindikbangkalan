# Audit integrasi New UI ke Staging — 6 Oktober 2026

Staging awal: `39dfc46`. Sumber UI: `a02d6ba`. Merge-base: `59633bf41785e111576529c03a11baa7b9e93c1e`. Merge yang sudah dimulai diteruskan; tidak ada commit, push, reset history, atau penghapusan branch.

## Temuan penting

- Commit Staging 05a035d sudah berisi conflict marker dan model GASPOL lama yang tidak kompatibel. Mengambil ours secara utuh tetap gagal kompilasi. Empat file konflik direkonstruksi dari sisi Staging sebelum cherry-pick rusak tersebut, lalu fix finalitas diterapkan ke model berbobot terbaru.
- Branch sumber hanya menambahkan komponen portal; wiring homepage, CSS, settings, backend routes, dan deklarasi dependency Leaflet tidak ikut tersimpan. Sambungan ini dilengkapi mengikuti komponen dan dokumentasi desain yang tersedia.
- Perbandingan langsung ujung branch menunjukkan ratusan file Staging yang lebih baru. File tersebut tidak diganti massal dengan versi branch lama. Audit kontribusi branch menggunakan diff tiga titik di bawah.
- Dua foto data pengaduan dikeluarkan dari index merge; file lokal tetap disimpan dan diabaikan oleh aturan data/uploads.

## Klasifikasi kontribusi branch sumber

| File | Kategori |
| --- | --- |
| app/api/public/school-locations/route.ts | API |
| app/api/schools/[id]/location/route.ts | API |
| app/assets/Bangkalan Education Portal UI Collage.png | Asset / seed |
| app/assets/Bangkalan Education Portal UI Showcase.png | Asset / seed |
| backend/assets/data-sekolah-map-bangkalan.xlsx | Asset / seed |
| backend/cmd/import-school-locations/main.go | Seed lokasi |
| backend/internal/handlers/gaspol.go | GASPOL |
| backend/internal/handlers/gaspol_test.go | GASPOL |
| backend/internal/handlers/school_locations.go | API |
| backend/internal/handlers/school_locations_test.go | API |
| backend/internal/services/gaspol.go | GASPOL |
| backend/internal/services/gaspol_models.go | GASPOL |
| backend/internal/services/gaspol_monthly.go | GASPOL |
| backend/internal/services/gaspol_monthly_repository.go | GASPOL |
| backend/internal/services/gaspol_monthly_test.go | GASPOL |
| backend/internal/services/gaspol_repository.go | GASPOL |
| backend/internal/services/school_location_seed.go | Seed lokasi |
| backend/internal/services/school_location_seed_test.go | Seed lokasi |
| backend/migrations/066_add_school_locations.sql | Migration |
| components/gaspol/GaspolDashboard.tsx | Campuran UI + GASPOL |
| components/portal/education-map.tsx | New UI / pendukung |
| components/portal/footer.tsx | New UI / pendukung |
| components/portal/map-boundary.tsx | New UI / pendukung |
| components/portal/module-navigation.tsx | New UI / pendukung |
| components/portal/primitives.tsx | New UI / pendukung |
| components/portal/public-navbar.tsx | New UI / pendukung |
| components/portal/school-location-settings.tsx | New UI / pendukung |
| components/portal/school-map-canvas.tsx | New UI / pendukung |
| components/portal/section-navigation.tsx | New UI / pendukung |
| data/uploads/pengaduan-kritik-saran/attachments/PGD-2026-DX2WL3T/EBF785G8TFWP2ECUS9YR.jpg | Data pengaduan (dikecualikan) |
| data/uploads/pengaduan-kritik-saran/follow-up/PGD-2026-DX2WL3T/WWLPMVYM4HMWYPHSQJF4.jpeg | Data pengaduan (dikecualikan) |
| docs/gaspol-monthly-finality.md | GASPOL |
| docs/portal-e2e-validation.md | Dokumentasi |
| docs/portal-redesign.md | Dokumentasi |
| docs/portal-visual-refinement.md | Dokumentasi |
| lib/school-location-context.ts | New UI / pendukung |
| lib/school-map.ts | New UI / pendukung |
| public/images/suramadu-hero.webp | Asset / seed |
| scripts/test-portal-location.mjs | New UI / pendukung |
| scripts/test-portal-navigation.cjs | New UI / pendukung |

## New UI yang terintegrasi

- Seluruh sembilan komponen portal, dua library lokasi, dua script pemeriksaan, hero Suramadu, dan dua gambar referensi desain diambil. Komponen navigasi tambahan tersedia sebagai fondasi; tidak mengganti workflow modul GASPOL.
- Homepage memakai navbar berkelompok, hero, pencarian sekolah/NPSN/layanan, kartu statistik dari API, education map, kartu layanan dengan filter role yang ada, informasi Cabdin dan footer.
- AppShell menyediakan layout homepage penuh, sambil mempertahankan guard GASPOL/MGMP, auth server snapshot, backup reminder, dan system-time indicator.
- Settings lokasi terhubung ke halaman settings yang sudah ada tanpa menghapus pengaturan lain.
- API baru: GET /api/public/school-locations; GET dan PUT /api/schools/:id/location. Proxy Next.js dan handler Go terhubung. Endpoint privat memakai middleware autentikasi dan scope sekolah yang sudah ada.
- Leaflet 1.9.4 dan @types/leaflet 1.9.22 dicatat pada manifest/lockfile. Tidak ada downgrade dependency existing.

## GASPOL yang dipertahankan dan diperbaiki

- Model komponen numerik dengan label, bobot, effectiveScore, status dan finalitas dipertahankan. Tidak memakai tipe lama dengan score pointer atau MonthlyScores.
- Bobot April–September: 20/15/20/15/15/15 persen. Bobot tahap utama: bulanan 20%, lapangan 1 40%, lapangan 2 20%, presentasi 20%. Top 5 tetap Top 5, bukan Top 6 lama.
- April–Juni tanpa laporan sesudah deadline menjadi ZERO_FINAL. Nilai tervalidasi 0 tetap final. Laporan yang pernah dikirim tidak dianggap tidak pernah mengirim hanya karena status berubah. Juli–September tidak otomatis menjadi nol final.
- Finalitas enam bulan dipakai oleh ranking, dashboard sekolah, preview master, kandidat kompetisi, dan ekspor. Clock efektif aplikasi/Jakarta dipertahankan.
- Workflow stage saat ini mengalahkan assessment legacy; nilai legacy tervalidasi tidak mengalahkan RETURNED_BY_MASTER/KOOR/CABDIN pada stage saat ini.
- File workflow, histori, resubmit, assignment, awards, schema master-return dan migration 065 tetap utuh dari Staging. Query rekap lapangan mempertahankan kelengkapan Koor/Cabdin, returned states, dan aturan penetapan minimal 10 peserta. Tidak ada synthetic assessment atau perubahan histori saat membaca finalitas.
- Empat conflict awal: gaspol.go, gaspol_models.go, gaspol_repository.go, GaspolDashboard.tsx selesai. Duplikasi tipe/helper lama disatukan; ekspor tetap membedakan nilai nol final dan nilai belum final.

## Migration

066_add_school_locations.sql tidak bentrok dan berada setelah 065_add_gaspol_master_return.sql. Semua migration sebelumnya dipertahankan. Migration bersifat aditif pada data_sekolah, memvalidasi pasangan/rentang koordinat dan verifikasi lokasi, serta memakai FK editor. lock_timeout 5 detik membatasi tunggu lock tabel. Downgrade schema ditolak secara eksplisit agar Goose tidak menandai kolom yang masih ada sebagai belum diaplikasikan; rollback aplikasi lama tetap dapat memakai schema aditif ini.

Migration belum diaplikasikan ke database pada sesi integrasi ini. Jalankan lewat prosedur Goose deployment yang sudah ada sebelum endpoint lokasi dipakai. Importer default dry-run; --apply tetap tindakan terpisah.

## Validasi

- go test ./... -count=1 dari backend: LULUS dengan GASPOL_TEST_DATABASE_URL diarahkan ke PostgreSQL lokal. Seluruh tes integrasi GASPOL aktif; tes integrasi PT yang memakai PT_TEST_DATABASE_URL tetap di luar pengujian ini.
- npm run build: LULUS, termasuk kompilasi TypeScript dan generasi 157 halaman.
- npx tsc --noEmit: LULUS.
- ESLint seluruh file frontend yang diubah/ditambahkan: LULUS.
- npm run lint seluruh repository: LULUS, 0 error dan 0 warning setelah tindak lanjut perbaikan lint. Aturan lint tidak dinonaktifkan.
- Script navigasi: LULUS untuk delapan role dan publik. Script pemetaan lokasi: LULUS 13 assertion.
- Importer dry-run: LULUS, 144 baris, 30 koordinat valid, 114 dilewati; tidak menulis database.
- PostgreSQL regression test: LULUS setelah database lokal dihidupkan. Mencakup finalitas ZERO_FINAL, bobot bulanan, preview master/kompetisi, prioritas ketiga status returned terhadap legacy, daftar tugas, lifecycle master return → resubmit → approval KOOR → approval CABDIN, audit events, serta validasi schema. Fixture memakai tabel sementara dengan search_path pg_temp; tidak mengubah data aplikasi.
- Browser desktop: homepage, hero, kartu layanan, dropdown Program & Layanan dan Leaflet tampil. API backend tidak tersedia sehingga peta/statistik menampilkan fallback. Save lokasi terautentikasi, data marker aktual, dan alur GASPOL end-to-end belum diverifikasi pada sesi ini. Laporan portal lama yang ikut branch adalah arsip, bukan bukti pengujian saat ini.

## Menyelesaikan merge

Seluruh file integrasi disiapkan di index. Review lalu commit merge dan push:

```powershell
git diff --cached --stat
git diff --cached --check
git commit -m "Merge New UI into Staging while preserving current GASPOL workflows"
git push origin Staging
```

Tidak perlu mengulang git merge. Untuk klaim siap production, selesaikan pengujian database di atas dan jalankan migration 066 melalui deployment.

## Daftar lengkap file hasil integrasi (49)

- `app/api/public/school-locations/route.ts`
- `app/api/schools/[id]/location/route.ts`
- `app/assets/Bangkalan Education Portal UI Collage.png`
- `app/assets/Bangkalan Education Portal UI Showcase.png`
- `app/globals.css`
- `app/page.tsx`
- `app/user/settings/page.tsx`
- `backend/assets/data-sekolah-map-bangkalan.xlsx`
- `backend/cmd/import-school-locations/main.go`
- `backend/cmd/server/main.go`
- `backend/internal/handlers/gaspol.go`
- `backend/internal/handlers/gaspol_test.go`
- `backend/internal/handlers/school_locations.go`
- `backend/internal/handlers/school_locations_test.go`
- `backend/internal/services/gaspol.go`
- `backend/internal/services/gaspol_lock.go`
- `backend/internal/services/gaspol_models.go`
- `backend/internal/services/gaspol_monthly.go`
- `backend/internal/services/gaspol_monthly_integration_test.go`
- `backend/internal/services/gaspol_monthly_repository.go`
- `backend/internal/services/gaspol_monthly_test.go`
- `backend/internal/services/gaspol_repository.go`
- `backend/internal/services/school_location_seed.go`
- `backend/internal/services/school_location_seed_test.go`
- `backend/migrations/066_add_school_locations.sql`
- `components/app-shell.tsx`
- `components/gaspol/GaspolDashboard.tsx`
- `components/portal/education-map.tsx`
- `components/portal/footer.tsx`
- `components/portal/map-boundary.tsx`
- `components/portal/module-navigation.tsx`
- `components/portal/primitives.tsx`
- `components/portal/public-navbar.tsx`
- `components/portal/school-location-settings.tsx`
- `components/portal/school-map-canvas.tsx`
- `components/portal/section-navigation.tsx`
- `components/ui/status-badge.tsx`
- `docs/gaspol-monthly-finality.md`
- `docs/portal-e2e-validation.md`
- `docs/portal-redesign.md`
- `docs/portal-visual-refinement.md`
- `docs/staging-new-ui-integration.md`
- `lib/school-location-context.ts`
- `lib/school-map.ts`
- `package-lock.json`
- `package.json`
- `public/images/suramadu-hero.webp`
- `scripts/test-portal-location.mjs`
- `scripts/test-portal-navigation.cjs`

Audit statis: nomor migration unik, route Go eksplisit unik, seluruh komponen portal/library sekolah identik dengan sumber UI, file workflow/histori/assignment/awards/master-return schema tetap identik dengan HEAD Staging.

## Tindak lanjut: 12 error dan 44 warning lint

Atas permintaan pengguna, masalah lint existing juga diperbaiki pada 27 file frontend. Perubahan meliputi tipe respons backup dan pemeriksaan payload import, dependency hook, penghapusan deklarasi yang tidak digunakan, reset modal berdasarkan key, hydration guard, serta penggunaan next/image untuk gambar. Gambar unggahan memakai unoptimized agar tetap dimuat langsung dari URL asli; watermark print dimuat eager.

Pilihan delegasi diambil dari hasil query sampai pengguna mulai mengedit, sehingga refetch tidak menimpa draft. Toast pembaruan dashboard muncul setelah invalidasi/refetch selesai dan tidak dijalankan setelah effect dibersihkan. Reset tab lulusan dijalankan hanya saat status filter berubah.

Validasi ulang: npm run lint bersih (0 error/0 warning), npm run build lulus, dan npx tsc --noEmit lulus. Tidak ada perubahan tambahan pada backend maupun components/gaspol selama tindak lanjut ini. Batasan pengujian database dan alur live di bagian Validasi masih berlaku.

File tambahan yang diperbaiki untuk lint:

- `app/admin/backup/page.tsx`
- `app/admin/settings/landing/page.tsx`
- `app/api/admin/generate-school-admins/route.ts`
- `app/internal/dashboard/page.tsx`
- `app/prestasi/page.tsx`
- `app/siswa/kelas-akhir/page.tsx`
- `app/siswa/pendaftaran/page.tsx`
- `app/siswa/rencana-lulusan/RencanaLulusanClient.tsx`
- `app/user/settings/informasi-pelayanan-cabdin/page.tsx`
- `app/user/settings/siswa-akhir/page.tsx`
- `components/app/api/admin/generate-school-admins/route.ts`
- `components/app/dashboard/page.tsx`
- `components/app/internal/dashboard/page.tsx`
- `components/app/prestasi/page.tsx`
- `components/app/siswa-perguruan-tinggi/page.tsx`
- `components/app/siswa/pendaftaran/page.tsx`
- `components/app/siswa/rencana-lulusan/RencanaLulusanClient.tsx`
- `components/app/status-lulusan/page.tsx`
- `components/app/user/settings/page.tsx`
- `components/guide-books/GuideBookSlideRenderer.tsx`
- `components/guide-books/GuideBookViewerModal.tsx`
- `components/guide-books/GuideBooksPageContent.tsx`
- `components/hardiknas/HardiknasDetailPage.tsx`
- `components/hardiknas/HardiknasPage.tsx`
- `components/reports/report-shell.tsx`
- `components/siswa/AdminRouteGuard.tsx`
- `components/siswa/LegacyImportActions.tsx`

## Tindak lanjut PostgreSQL lokal

Pengujian database menemukan dua masalah existing yang tidak terlihat saat tes integrasi masih skip. Keduanya diperbaiki dan seluruh go test ./... -count=1 lulus dengan GASPOL_TEST_DATABASE_URL aktif:

- `backend/internal/services/gaspol_competition.go`: cast eksplisit parameter user ID ke integer pada CASE sinkronisasi assessment legacy agar approval KOOR/CABDIN tidak gagal SQLSTATE 42804. Transisi workflow terbaru tetap dipertahankan.
- `backend/internal/services/gaspol_schema.go`: error yang diterima melalui rows.Err() kini menyertakan petunjuk migration 065, konsisten dengan error query langsung.

Pengecualian terhadap catatan audit awal bahwa file workflow identik dengan Staging: gaspol_competition.go kini mendapat perbaikan tipe SQL di atas. Tidak ada migration yang diterapkan pada database aplikasi. Pengujian UI terautentikasi dan end-to-end dengan data aplikasi masih belum dilakukan.
