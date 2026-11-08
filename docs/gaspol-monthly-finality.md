# Finalitas nilai bulanan GASPOL pada Staging

Dokumen ini menggantikan catatan branch lama yang memakai formula 30/40/30 dan belum mengenal workflow stage. Audit integrasi lengkap tersedia pada [laporan integrasi](staging-new-ui-integration.md).

## Aturan

- Enam komponen April–September memakai bobot 20/15/20/15/15/15 persen. Tahap utama memakai bulanan 20%, lapangan 1 40%, lapangan 2 20%, presentasi 20%.
- April–Juni tanpa laporan setelah deadline menghasilkan ZERO_FINAL. Nilai tervalidasi 0 tetap VALIDATED dan final.
- Laporan terkirim/yang pernah terkirim tetap menunggu validasi. Juli–September tanpa penilaian final tidak otomatis ZERO_FINAL.
- Clock efektif aplikasi dengan zona Asia/Jakarta tetap menjadi acuan.
- Komponen memakai kontrak numerik Staging: score 0 pada komponen belum final harus dibaca bersama status/isFinal. UI dan ekspor menampilkan nilai belum final sebagai kosong/dash. Label, weight, effectiveScore dan detail status dipertahankan.

## Sumber dan readiness

ListMonthlyCompetitionComponents menyatukan stage workflow dan fallback legacy. Jika stage saat ini ada, hanya assessment dengan aktivitas workflow terbaru per sekolah/tahun/stage/bulan yang menentukan nilai dan finalitas: `ORDER BY updated_at DESC NULLS LAST, created_at DESC NULLS LAST, id DESC LIMIT 1`. Current harus FINAL dengan final_score tersedia; status returned/pending tidak ditutupi nilai tervalidasi legacy. Helper yang sama dipakai ranking/dashboard, preview master dan kandidat kompetisi. Nol final dihitung tanpa insert assessment palsu. Assessment belum final April–Juni tanpa laporan tetap WAITING, bukan ZERO_FINAL.

Finalitas memerlukan enam bulan unik berstatus VALIDATED atau ZERO_FINAL. Preview Top 10 tetap tersedia sebelum final, sementara eligibility tahap berikutnya memerlukan finalitas. Penetapan Top 10 master tetap mempertahankan minimal 10 peserta, kelengkapan lapangan, persetujuan Koor/Cabdin, returned-state blocking dan tie handling dari Staging.

## Validasi

Unit regression test mempertahankan bobot terbaru, finalitas April–Juni, penilaian langsung Juli–September, nilai tervalidasi 0, duplicate/missing month, dan konsistensi ranking/eligibility. Test PostgreSQL opsional memakai tabel TEMP dan GASPOL_TEST_DATABASE_URL untuk memeriksa query dan preview aktual. Hasil serta keterbatasan lingkungan dicatat pada laporan integrasi.

## Audit bugfix 7 Oktober 2026

Root cause pada source: query lama menggunakan `BOOL_AND(workflow_status='FINAL' AND final_score IS NOT NULL)` dan `AVG(final_score)` atas seluruh row sekolah/tahun/bulan. Baris returned historis dapat memblokir current FINAL; dua nilai FINAL juga dirata-ratakan secara keliru. Reproduksi PostgreSQL lokal membuktikan kondisi tersebut dapat direpresentasikan oleh schema. Ini bukan konfirmasi bahwa row yang sama ada di Staging.

Schema migration 062 memiliki UNIQUE(year,stage,month,school_id,assessor_user_id), tanpa current flag, revision, atau superseded relation. Resubmit oleh assessor sama memperbarui row di tempat dan mencatat audit log; assessor berbeda dapat memiliki row dalam bulan yang sama. Karena submit/validasi/return memperbarui updated_at, timestamp tersebut menjadi urutan aktivitas current, bukan MAX(id) atau memilih FINAL saja. Aturan satu current ini khusus agregasi MONTHLY; agregasi panel tahap lain tidak diubah.

Current final_score NULL tidak mengambil nilai legacy. Workflow FINAL merupakan hasil transisi validasi Cabdin pada service existing; WAITING_CABDIN dan seluruh returned state tetap belum final. Bobot dan rule waktu tidak diubah.

### Hasil regression PostgreSQL lokal

| Kasus | Hasil |
| --- | --- |
| 1: Apr–Jun ZERO_FINAL, Jul 80, Agu 90, Sep 100 FINAL | Final; bulanan 40.50; /20 = 8.10 |
| 2: historical returned, latest final 90 | Juli 90; final |
| 3: historical final, latest returned 95 | Juli belum final; nilai historis tidak dipakai |
| 4: Agustus WAITING_CABDIN | Bulanan belum final |
| 5: current FINAL bernilai 0 | Juli 0 valid; final |
| 6: returned/resubmitted/returned/final | Hanya current final 90 dipakai |
| 7: sekolah sama, tahun berbeda | Tahun lain tidak memengaruhi rekap 2026 |
| 8: sekolah berbeda, bulan sama | Sekolah lain tidak memengaruhi sekolah 1 |

Semua lulus, termasuk tambahan updated_at mengalahkan ID lebih besar, timestamp seri dipecahkan ID, FINAL dengan skor NULL, dan pending April tanpa laporan. Tes menjalankan query lingkup satu sekolah dan seluruh peserta, helper ranking, kontribusi /20, preview tahap 1 dan kandidat kompetisi. Contoh lanjutan lapangan 1=80, lapangan 2=90, presentasi=100 menghasilkan 8.10+32+18+20=78.10 (subtotal /60=40.10). Banding mengikuti formula existing tanpa perubahan.

Dashboard sekolah dan leaderboard menggunakan repository monthly yang sama; React membaca monthlyFinal dari backend. Preview memiliki agregasi SQL bulanan lama yang hasilnya sudah selalu diganti oleh komponen repository sebelum dikembalikan. Tidak diperlukan perubahan frontend atau migration.

### Preservasi histori dan batas verifikasi

Tes membandingkan snapshot JSON seluruh field seluruh row assessment sebelum/sesudah pembacaan, preview, dan perhitungan; hasil identik, termasuk baris returned lama. Tidak ada DELETE/UPDATE data aplikasi dalam fix. Query Histori GASPOL dan audit log tidak diubah atau difilter dengan pemilihan current. Tab assessment histori existing berbasis legacy gaspol_assessments, sedangkan timeline event memakai audit log dan relasi stage; keterlihatan stage row yang tidak memiliki legacy/audit relation mengikuti perilaku existing. Tampilan histori melalui browser belum diverifikasi.

Database yang diuji adalah PostgreSQL Docker lokal, menggunakan tabel TEMP session-local. Staging belum diinspeksi dan belum dideploy, sehingga ID/status row penyebab aktual serta hilangnya pesan pada UI Staging belum dapat dikonfirmasi.

### File dan validasi

- gaspol_monthly_repository.go: pemilihan current dan pencegahan fallback skor NULL.
- gaspol.go: assessment pending tidak menjadi ZERO_FINAL.
- gaspol_monthly_integration_test.go: delapan kasus wajib dan empat kasus tambahan beserta downstream checks.
- gaspol_master_return_integration_test.go: melengkapi created_at pada schema fixture.
- docs/gaspol-monthly-finality.md: laporan ini.

Dengan GASPOL_TEST_DATABASE_URL menuju PostgreSQL lokal:

- `go test ./internal/services ./internal/handlers -run Gaspol -count=1`: lulus.
- `go test ./internal/services ./internal/handlers -count=1`: lulus, termasuk integration tests (tidak skip).
- `go build ./...`: lulus.
- `git diff --check`: lulus.
- Frontend tidak disentuh; lint/typecheck/build frontend tidak dijalankan.
