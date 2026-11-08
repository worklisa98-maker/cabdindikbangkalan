# GASPOL Phase 1 — laporan review

Tanggal: 9 Oktober 2026 (Asia/Jakarta). Branch: `codex/gaspol-phase-1`, dibuat dari branch aktif `codex/portal-phase-4-6` dengan working tree awal bersih. Perubahan belum di-commit, push, merge, atau deploy.

## Audit awal

- Sudah tersedia: `SaveNominationSubmission`, endpoint PUT sekolah, tabel nomination submissions dengan unique key tahun/sekolah/jenis, cohort peringkat 11–21, penetapan Top 10/Bottom 5/hasil banding, assignments per tahap, antrean Koor/Cabdin, history/audit, dan rumus skor utama serta Zona Tergagal.
- Belum tersedia: form nominasi sekolah, API baca laporan untuk penilai, status eksplisit enam sekolah yang lolos seleksi kandidat, acknowledgement persisten, pop-up resmi, serta pilihan detail satu sekolah pada assignment tahap.
- Rawan regresi: endpoint penyimpanan awal hanya memeriksa SchoolID dan cohort; laporan LOCKED dapat memberi respons sukses tanpa perubahan; hasil terdahulu dapat menimpa pesan tahap aktual; form juri lama menduplikasi assignment tahap; pembacaan kandidat melalui pool setelah transaksi dimulai berpotensi tertahan pada pool kecil.
- File yang diaudit: dashboard, handlers, competition, models, repository, awards, history, scoring, lock, schema, dan migration 039/062/065/066. Implementasi menggunakan kode aktif. Rumus skor, voting, finalisasi bulanan, dan mekanisme validator existing tidak diganti.
- ZIP `website(20261009-013153).zip` tidak ditemukan dalam workspace, folder induk, pencarian di `D:\CABDIN BANGKALAN`, atau lampiran Codex. `website.zip` tersedia di folder induk tetapi tidak dianggap identik tanpa bukti. Perbandingan dengan ZIP yang diminta belum dilakukan.

## Perubahan dan file

| File | Alasan |
|---|---|
| `components/gaspol/GaspolDashboard.tsx` | Integrasi panel sekolah; tab sesuai assignment; pencarian/filter status; satu detail sekolah; hapus duplikasi form juri; laporan/bukti dalam review Koor/Cabdin; menolak nilai kosong sebelum dikirim. |
| `components/gaspol/GaspolSchoolPhase.tsx` | Form draft/kirim, ringkasan, baca laporan penilai, banner, histori pemberitahuan, dialog aksesibel dan acknowledgement saat dialog pertama ditampilkan. |
| `lib/gaspol-stage-navigation.ts` | Urutan tab dan filter berdasarkan pasangan stage/assignment role, nama/NPSN, dan status. |
| `backend/internal/services/gaspol_competition.go` | Role dan peserta aktif, tahun eksplisit, gate hasil resmi, validasi tautan, upsert existing, error LOCKED, audit atomik, notice legacy mengikuti hasil resmi terbaru. |
| `backend/internal/services/gaspol_school_phase.go` | API layanan baca laporan dengan scope, status resmi, gate edit, dan acknowledgement sekolah/tahun/cohort/revisi lock. |
| `backend/internal/services/gaspol_lock.go` | Menetapkan PASSED_NOMINATION; mengunci laporan setelah tahap ditetapkan; serialisasi save/lock; menolak lock resmi duplikat; menolak reopen upstream selama hasil turunannya masih dikunci; memakai kandidat yang telah dimuat sebelum transaksi. |
| `backend/internal/services/gaspol_schema.go` | Pemeriksaan startup skema 067 agar tabel acknowledgement yang belum tersedia tidak tersembunyi sebagai error dashboard. |
| `backend/internal/handlers/gaspol_school_phase.go` | Handler GET notice/report dan POST acknowledgement dengan autentikasi dan validasi payload. |
| `backend/internal/handlers/gaspol.go` | Mapping error hasil duplikat/dependensi reopen. |
| `backend/cmd/server/main.go` | Registrasi tiga endpoint tambahan dan pemeriksaan skema 067. |
| `backend/migrations/067_add_gaspol_school_notices.sql` | Constraint PASSED_NOMINATION, backfill enam sekolah berdasarkan hasil resmi, tabel acknowledgement; forward-only. |
| `backend/internal/services/gaspol_school_phase_test.go` | Regresi kontrak submission, 21 sekolah, migration/backfill, izin, persistensi dua akun/perangkat, penetapan Bottom 5 dan final langsung, serta reopen. |
| `backend/internal/handlers/gaspol_school_phase_test.go` | Endpoint tanpa autentikasi dan payload event malformed ditolak. |
| `tests/gaspol-phase1.test.mjs` | Regresi tab/assignment role, multi-sekolah, pencarian nama/NPSN, dan status. |
| `docs/qa/gaspol-phase1-mobile.png` | Bukti visual dialog sukses pada viewport 390 × 844 px, menggunakan API simulasi. |
| `docs/gaspol-phase1-review.md` | Laporan ini. |

## API dan kontrak

Endpoint existing tetap digunakan:

`PUT /api/gaspol/school/nomination-submission`

Payload: `year`, `submissionType` (`NOT_FAILED_NOMINATION` atau `BOTTOM5_APPEAL`), `improvementDetails`, `improvementResult`, `responseText`, `driveUrl`, `evidenceUrls`, `status` (`DRAFT`/`SUBMITTED`). Kontrak existing mewajibkan tautan laporan/Drive ketika SUBMITTED; narasi tidak dibuat wajib baru. Semua tautan nonkosong wajib HTTP(S) valid. Draft boleh belum berisi tautan.

Tambahan:

- `GET /api/gaspol/school/competition-notices?year=2026`: hanya sekolah dengan role GASPOL tahun tersebut dan peserta aktif. Mengembalikan status, pesan/tone, waktu resmi, cohort ID, source lock ID, dan penanda baca.
- `POST /api/gaspol/school/competition-notices/:id/ack`: payload `{ "year": 2026, "lockId": 123 }`. Server memverifikasi event milik sekolah, tahun, lock resmi yang masih aktif, dan status cohort; idempotent. Penanda baca berlaku lintas akun/perangkat sekolah yang sama.
- `GET /api/gaspol/nomination-submissions?year=2026&schoolId=11&stage=NOT_FAILED_NOMINATION`: sekolah sendiri, penilai dengan assignment tepat sekolah/tahap/role, Koor aktif, Validator aktif yang ditugaskan ke sekolah, atau Master. Mengembalikan laporan dan `canEdit`; penilai hanya membaca melalui endpoint ini.

Nomination hanya dapat ditulis dari NOT_FAILED yang bersumber pada lock resmi aktif sampai Bottom 5 resmi dikunci. Banding mengikuti BOTTOM_5 hingga FINAL resmi. Status LOCKED selalu menolak perubahan. Submission disimpan lewat unique key existing, bukan tabel/endpoint duplikat.

Status baru `PASSED_NOMINATION` berarti enam sekolah tidak masuk kandidat. `ESCAPED_BOTTOM_ZONE` tetap khusus dua sekolah yang lolos banding. Dialog menampilkan perubahan resmi paling mutakhir yang belum dibaca, dan menyimpan acknowledgement saat ditampilkan. Banner dan histori tetap tersedia. Preview/config state tanpa lock tidak menghasilkan event.

## Migrasi

Jalankan 067 melalui mekanisme Goose existing sebelum memakai backend baru. Migrasi lama tidak diedit. Backfill hanya memakai Bottom 5 LOCKED terbaru per tahun dan NOT_FAILED aktif, serta mengecualikan kandidat BOTTOM_5 aktif. Data submission, assignment, penilaian, snapshot, dan hasil historis tidak dihapus. Down migration sengaja menolak downgrade untuk melindungi status dan acknowledgement.

Reopen upstream sekarang memerlukan reopen hasil turunannya terlebih dahulu. Contoh: FINAL dibuka sebelum BOTTOM_5; TOP_10 tidak dapat dibuka selama TOP_5/BOTTOM_5/FINAL masih dikunci. Laporan yang sudah LOCKED tetap read-only, sesuai batasan status existing; tidak ada pembukaan edit otomatis.

## Testing

Database pengujian: container PostgreSQL 15 terpisah `gaspol-phase1-test`, port loopback 55432; fixture tabel session-local. Database aplikasi di port 5432 tidak dimigrasi atau diubah. Container pengujian dihentikan setelah tes. Untuk menjalankan kembali tes integrasi:

```powershell
docker start gaspol-phase1-test
$env:GASPOL_TEST_DATABASE_URL = 'postgres://postgres:gaspol_test@127.0.0.1:55432/gaspol_test?sslmode=disable'
cd backend
go test ./...
```

Credential tersebut hanya untuk container pengujian terisolasi yang dibuat pada pekerjaan ini.

| Perintah / pemeriksaan | Hasil |
|---|---|
| `go test ./...` dari `backend`, dengan `GASPOL_TEST_DATABASE_URL` menuju database terisolasi | Lulus, termasuk tes integrasi existing dan regresi Phase 1. |
| `go test ./internal/services -run 'TestGaspolSchoolPhasePostgres\|TestNominationSubmissionValidation' -v` | Lulus. |
| `node --test tests/gaspol-phase1.test.mjs` | 3 tes lulus. Node 24 memberi warning module type untuk impor TS; tes tetap lulus. |
| `npx tsc --noEmit` | Lulus. Referensi route tes sementara pada tipe dev Next sempat menyebabkan error; file tipe dev yang stale dibersihkan, lalu pemeriksaan diulang. |
| `npm run lint` | Lulus. |
| `npm run build` | Lulus: kompilasi produksi, TypeScript, dan generasi halaman. |
| `go vet ./internal/services` | Lulus. |
| `go vet ./...` | Belum bersih: copylock pada ComplaintHandler dan unreachable code di siswa.go/uploads.go yang tidak diubah. Tidak ada perubahan modul tersebut dalam Phase 1. |
| `git diff --check` | Lulus. |
| Browser 390 × 844, real React panel dengan API simulasi | Draft/kirim/baca ulang lulus; dialog tidak berulang setelah refresh; acknowledgement otomatis sebelum dismiss lulus; laporan LOCKED tanpa tombol edit; tidak ada horizontal overflow. |

Browser bawaan gagal terhubung karena native bridge tidak tersedia; pemeriksaan dilanjutkan dengan browser Playwright terpisah. Uji tunggu response sempat timeout karena response acknowledgement sudah lebih dulu selesai; reload berikutnya membuktikan popup tidak muncul lagi. Error jaringan pada service-links global terjadi karena backend aplikasi tidak dijalankan dalam preview terisolasi; endpoint GASPOL yang diuji disimulasikan. Route preview sementara dihapus dan tidak menjadi bagian produk.

## Acceptance checklist

- [x] Top 10 tidak dapat menulis nominasi; hanya cohort 11–21 resmi dapat menulis.
- [x] Draft, SUBMITTED, baca ulang, validasi, LOCKED, unique submission, dan data existing/backfill diuji di PostgreSQL.
- [x] Status enam lolos nominasi, lima kandidat, dua lolos banding, tiga hasil akhir berasal dari hasil resmi.
- [x] Preview tidak membuat notice; acknowledgement persisten dan diverifikasi per sekolah/tahun/event/revisi.
- [x] Manipulasi sekolah/event/lock, role tanpa akses, tahun salah, peserta nonaktif, dan assignment sekolah lain ditolak dalam tes.
- [x] Tab penilai hanya mengikuti assignment role/tahap; multi-sekolah dapat dicari dan difilter; satu detail aktif.
- [x] Nominasi menyediakan laporan/bukti tanpa form skor generik pengganti rubrik Phase 2.
- [x] Presentasi tetap terpisah; T2/Presentasi tetap memakai workflow direct final existing; Banding tetap input 0–100/catatan dengan assignment APPEAL_PANEL.
- [x] Banner, histori, dialog keyboard/native focus, dan warna sukses/perhatian/kandidat tersedia; panel sekolah diuji mobile.
- [x] Rumus 20/40/20/20 dan 20/40/30/10 serta finalisasi bulanan tidak diedit.
- [ ] Pengujian menyeluruh pada staging dengan login nyata untuk sekolah/pengawas/Koor/Cabdin/juri/Master, semua tab dan multi-perangkat fisik.
- [ ] Validasi migrasi terhadap salinan data produksi lengkap dan perbandingan dengan ZIP bernama persis yang diminta.
- [ ] `go vet ./...` seluruh repository bersih; temuan modul lain masih ada.

## Risiko, asumsi, dan pekerjaan lanjutan

- Tahun API baru harus eksplisit dan >= 2024; hak GASPOL dan partisipasi harus aktif pada tahun tersebut. Tampilan dashboard existing menggunakan tahun aktif yang sudah dipakai repository.
- Keharusan field mengikuti kontrak backend existing: hanya tautan wajib ketika kirim. Uraian, hasil, dan tanggapan tetap dapat dilengkapi tanpa menambah aturan wajib baru.
- Assignment lama dan skor nominasi existing dipertahankan. Input penilaian rubrik nominasi sengaja tidak dibuat pada Phase 1; nilai existing dapat dibaca.
- Pengujian mobile mencakup panel sekolah/dialog/form, bukan seluruh kombinasi dashboard role nyata. Checklist staging masih harus dijalankan sebelum deployment.
- Pop-up menyajikan status resmi terkini; event tahap sebelumnya tetap dapat dibaca dari histori pemberitahuan. Hasil yang dibuka kembali tidak diperlakukan sebagai hasil resmi aktif.
- Voting QR/Ter-Favorit, engine rubrik dinamis, serta perluasan Phase 2–4 tidak dikerjakan. Section Presentasi menjelaskan jalur award sesuai kewenangan; Ter-Inovatif hanya muncul pada assignment yang sesuai.
- Tidak ada data historis yang dibersihkan, sidebar baru, perubahan navigasi global, push, merge, deploy, atau penghapusan branch.

![Dialog resmi pada layar mobile](qa/gaspol-phase1-mobile.png)
