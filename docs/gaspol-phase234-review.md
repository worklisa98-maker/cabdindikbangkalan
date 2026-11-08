# GASPOL Bang — laporan implementasi terintegrasi Phase 2–4

Tanggal: 9 Oktober 2026, Asia/Jakarta. Sumber: branch aktif dan master prompt terlampir. Perubahan Phase 1 yang sudah ada di working tree dipertahankan. Tidak dilakukan commit, push, merge, deploy, reset database, maupun perubahan database aplikasi.

## 1. Ringkasan dan audit awal

Implementasi menggunakan kembali assignment tahap, bulk assignment, penilaian tahap, validasi Koor/Cabdin, cohort/lock resmi, tabel sesi/suara/konten/snapshot like, audit, autentikasi aplikasi, dan mesin perhitungan existing. Phase 1 tetap menangani laporan Nominasi/Banding, status sekolah, acknowledgement, serta penetapan Top 10/Bottom 5/hasil banding. Rincian pekerjaan Phase 1 ada di `docs/gaspol-phase1-review.md`.

Phase 2 menambahkan versi rubrik, pengikatan versi untuk penilaian sekolah/tahap/tahun, form indikator Nominasi dan Inovasi, serta pengajuan inovasi Top 5. Phase 3 menambahkan administrasi voting terautentikasi, pemilih/kehadiran, QR dan kode pendek, bantuan operator dengan persetujuan, serta verifikasi like. Phase 4 mengintegrasikan detail penilaian, status validasi, audit, preview Ter-Favorit, dan gate penetapan.

Tabel baru hanya untuk kemampuan yang sebelumnya belum tersedia: versi/pengikatan rubrik, pengajuan inovasi, kandidat/roster/operator/persetujuan voting, dan delegasi verifikator like. Tidak dibuat sistem assignment atau penilaian paralel. Endpoint shared-token lama dinonaktifkan agar tidak menjadi jalan pintas autentikasi; client lama harus menggunakan alur baru.

## 2. Daftar file

Path di bawah relatif terhadap root repository. File Phase 1 yang sudah ada sebelum pekerjaan ini tidak diklaim sebagai tambahan Phase 2–4.

| File ditambahkan | Fungsi |
|---|---|
| `backend/migrations/068_add_gaspol_rubric_versions.sql` | Versi rubrik, binding, pengajuan inovasi. |
| `backend/migrations/069_add_gaspol_voter_role.sql` | Enum role akun khusus. |
| `backend/migrations/070_add_gaspol_voting_administration.sql` | Managed session, kandidat, roster, delegasi like, review snapshot. |
| `backend/migrations/071_add_gaspol_voting_assistance.sql` | Operator dan persetujuan pemilih. |
| `backend/internal/services/gaspol_rubrics.go` | Validasi, perhitungan, versi draft/aktif dan binding. |
| `backend/internal/services/gaspol_innovation.go` | Pengajuan inovasi dan penilaian rubrik. |
| `backend/internal/services/gaspol_voting_admin.go` | Administrasi akun/sesi, ballot, monitoring dan preview. |
| `backend/internal/services/gaspol_voting_assistance.go` | Delegasi operator dan konsumsi persetujuan sekali. |
| `backend/internal/services/gaspol_like_review.go` | Delegasi, pengajuan, pengembalian, verifikasi like. |
| `backend/internal/services/gaspol_phase_overview.go` | Detail indikator, status penilaian dan audit Master. |
| `backend/internal/handlers/gaspol_rubrics.go` | Handler rubrik, inovasi, detail penilaian. |
| `backend/internal/handlers/gaspol_voting_admin.go` | Handler voting, bantuan, like, overview. |
| `backend/internal/services/gaspol_phase234_test.go` | Uji skor dan integrasi PostgreSQL termasuk request paralel. |
| `backend/internal/handlers/gaspol_phase234_test.go` | Kontrak penolakan request tanpa identitas. |
| `backend/internal/middleware/gaspol_voter_test.go` | Batas permission akun pemilih. |
| `backend/cmd/server/gaspol_migrations_test.go` | Seluruh migration 001–071 pada database pengujian baru. |
| `components/gaspol/GaspolRubrics.tsx` | Editor rubrik, penilaian indikator, form inovasi, detail reviewer. |
| `components/gaspol/GaspolVoting.tsx` | Manajemen sesi, pemilih, QR, konfirmasi, preview/penetapan. |
| `components/gaspol/GaspolSelectors.tsx` | Pilihan searchable sekolah/akun memakai settings overview existing. |
| `components/gaspol/GaspolPhaseTools.tsx` | Tab internal rubrik/voting/like/detail audit dan petugas delegasi. |
| `app/gaspol/voting/page.tsx` | Halaman tujuan QR dan login resmi dengan target tetap tersimpan. |
| `tests/gaspol-phase234-browser.mjs` | Uji browser Edge dengan simulasi API eksplisit. |
| `docs/qa/gaspol-phase234-vote-mobile.png` | Bukti tampilan konfirmasi mobile. |
| `docs/qa/gaspol-phase234-qr-mobile.png` | Bukti panel QR mobile Master. |
| `docs/gaspol-phase234-review.md` | Laporan dan satu checklist akhir ini. |

| File diubah oleh Phase 2–4 | Fungsi perubahan |
|---|---|
| `backend/cmd/server/main.go` | Registrasi API dan pemeriksaan schema sampai 071. |
| `backend/internal/handlers/gaspol.go` | Error baru dan penolakan endpoint vote publik lama. |
| `backend/internal/middleware/auth.go` | Allowlist GASPOL_VOTER dengan scope terbatas. |
| `backend/internal/services/gaspol_competition.go` | Routing Nominasi/Inovasi ke rubrik, review transaksional, kunci konfigurasi voting, tutup bypass like manual lama. |
| `backend/internal/services/gaspol_favorite.go` | Scope sekolah Top 5, penolakan perubahan link terkunci, pensiun vote shared-token. |
| `backend/internal/services/gaspol_awards.go` | Gate kelengkapan inovasi dan penetapan favorit transaksional. |
| `backend/internal/services/gaspol_schema.go` | Pemeriksaan schema baru pada startup. |
| `components/gaspol/GaspolDashboard.tsx` | Integrasi panel sesuai role/assignment; laporan dan form nilai terpisah. |
| `components/guide-books/GuideBooksPageContent.tsx` | Label role pemilih dalam pemetaan role lengkap. |
| `lib/auth/types.ts`, `lib/auth/role-scope.ts` | Tipe/normalisasi GASPOL_VOTER. |
| `lib/auth/gaspol-school-access.ts`, `lib/navigation/sidebar-menu.ts` | Akses halaman voting dan menu GASPOL terbatas. |
| `package.json`, `package-lock.json` | Library pembuat QR lokal `qrcode` dan tipe. |

Artefak Phase 1 yang dipertahankan: migration 067; service/handler/test `gaspol_school_phase`; `GaspolSchoolPhase.tsx`; `lib/gaspol-stage-navigation.ts`; `tests/gaspol-phase1.test.mjs`; laporan/screenshot Phase 1; perubahan sebelumnya dalam dashboard, competition, lock, schema, routes dan error mapping.

## 3. Migrasi

Jalankan mekanisme Goose existing sampai 071 sebelum backend baru menerima trafik. Migration lama tidak diedit. 067 adalah prasyarat Phase 1; 068–071 milik pekerjaan ini. 069 sengaja terpisah agar enum selesai di-commit sebelum digunakan. Migration yang menyimpan ballot/versi/histori bersifat forward-only; rollback aplikasi memerlukan rencana kompatibilitas, bukan menghapus tabel baru.

Uji fresh database berhasil menjalankan 001–071, menjalankan Up lagi tanpa pekerjaan tambahan, memeriksa schema dan role. Ini membuktikan kompatibilitas rangkaian migration pada PostgreSQL 15 kosong; bukan bukti migrasi terhadap salinan data Staging/Production. Uji salinan Staging masih BLOCKED karena akses/data belum tersedia.

## 4. Endpoint

Semua path di bawah memakai prefix `/api`; seluruh endpoint baru memerlukan autentikasi.

| Method/path | Hak dan perilaku |
|---|---|
| GET/POST `/gaspol/admin/rubrics` | Master membaca/membuat versi draft. |
| POST `/gaspol/admin/rubrics/:id/activate` | Master aktivasi eksplisit. |
| GET `/gaspol/assessment-rubric` | Penilai dengan assignment tepat; pin versi sekolah/tahap/tahun. |
| GET `/gaspol/assessments/:id/details` | Scope Master/sekolah sendiri/penilai/validator yang sah. |
| GET/PUT `/gaspol/innovation-submission` | Baca scoped; tulis sekolah sendiri Top 5 resmi. |
| POST `/gaspol/admin/voting` | Master: CREATE, ACCOUNTS, VOTERS, OPEN, CLOSE. |
| GET `/gaspol/voting` | Master, roster sesi, sekolah kandidat sendiri; hasil kandidat hanya Master sesudah ditutup. |
| POST `/gaspol/voting/vote` | Ballot dari user ID autentikasi, bukan username body. |
| GET/POST `/gaspol/voting/assistance` | ASSIGN oleh Master; REQUEST oleh pemilih; EXECUTE oleh operator aktif. |
| GET/POST `/gaspol/likes` | Scope verifikator; ASSIGN/VERIFY/RETURN khusus Master. |
| GET `/gaspol/phase-overview` | Petugas mendapat tugasnya; Master mendapat penilaian dan 200 audit terbaru. |

Endpoint existing penilaian/validasi/penetapan tetap digunakan. Input Nominasi/Inovasi kini memakai `rubricId`, map `rubric` nilai indikator, dan `indicatorNotes`; nilai `score` dari client tidak menjadi sumber skor. Banding tetap nilai langsung dan catatan.

Endpoint existing `/gaspol/admin/awards/favorite/finalize` kini mensyaratkan managed session CLOSED, cut-off tercapai, lima kandidat resmi dan sumber like sah. `/gaspol/awards/innovation/propose` mensyaratkan seluruh assignment aktif Top 5 FINAL; approval Kasi SMK existing tetap berlaku. Endpoint lama pembukaan shared-token, vote publik, dan override like langsung mengembalikan penolakan eksplisit, bukan mencatat data diam-diam.

## 5. Role dan permission

- Master mengelola rubrik, akun, sesi, roster, kehadiran, delegasi dan verifikasi like.
- `GASPOL_VOTER` adalah role akun khusus dengan allowlist autentikasi, password pribadi, halaman/API voting dan layanan pendukung. Tidak memperoleh akses Admin Cabdin atau modul lain.
- Akun pengawas existing mempertahankan role/kewenangannya; hak voting merupakan baris roster per sesi/tahun. Pencarian username existing case-insensitive; tidak membuat akun baru untuk username yang sudah ada.
- Akun baru menggunakan password acak, bcrypt di database; password awal hanya dikembalikan saat pembuatan kepada Master. Audit tidak memuat password.
- Sekolah hanya menulis pengajuan sendiri dan membaca QR sekolahnya. Penilai harus mempunyai assignment tepat kategori/sekolah/tahun. Presentasi tidak memberi assignment Inovasi; Nominasi tidak memberi assignment Banding.
- Petugas like hanya sekolah delegasinya. Operator hanya menjalankan persetujuan pemilih yang telah login, hadir, aktif, dan memilih kandidat tertentu.

## 6. Rubrik dinamis dan versioning

Template sementara tersedia di editor Master: Nominasi 20/30/30/20, Inovasi 25/25/30/20, berlabel sementara menunggu persetujuan Koordinator. Template tidak otomatis disimpan/diaktifkan. Master dapat mengubah indikator, urutan, bobot dan panduan sebelum menyimpan draft baru.

Aktivasi mewajibkan total tepat 100. Nilai indikator harus lengkap, finite, 0–100, tanpa key asing. Rumus `jumlah(nilai × bobot / 100)` dihitung backend. Setiap penyimpanan menghasilkan versi draft baru; aktivasi mengarsipkan versi aktif sebelumnya. Binding pertama saat penilai membuka rubrik berlaku per tahun/tahap/sekolah, sehingga semua penilai sekolah itu memakai versi sama. Versi baru tidak mengganti binding yang sudah ada. Penilaian menyimpan snapshot lengkap versi, indikator, bobot, nilai dan catatan.

Pengajuan inovasi: seluruh sembilan kelompok informasi dan bukti tersedia; draft boleh belum lengkap, submit memerlukan kelengkapan dan tautan sah. Submit mengunci perubahan. Tidak ditambahkan kewenangan bebas membuka laporan sekolah yang sudah dikirim. Penilaian inovasi menggunakan rubrik khusus, validasi Koor lalu Cabdin, dan approval penghargaan existing.

## 7. Voting QR dan integritas ballot

Sesi baru mengambil tepat lima Top 5 dari cohort dengan source lock resmi. Status DRAFT → OPEN → CLOSED → ESTABLISHED. Kapasitas awal 33 dapat diubah saat membuat sesi. Master menetapkan roster/kehadiran selama DRAFT. OPEN membekukan denominator dari pemilih hadir dan aktif; kapasitas bukan denominator. Roster/kandidat tidak dapat diubah lewat alur ini setelah OPEN.

QR dibuat lokal di browser, berisi URL origin aplikasi, tahun, ID sekolah dan sesi. Identitas sekolah/NPSN/tahun/kategori/kode pendek tampil bersama QR. QR tidak memuat password/token autentikasi. QR dari sesi berbeda ditolak halaman voting. Pemilih login dengan token resmi, meninjau kandidat dan mengonfirmasi; kode pendek hanya memilih kandidat, tetap perlu login dan roster.

Transaksi vote mengunci sesi, memeriksa status/waktu/roster/kehadiran, memasukkan ballot dan audit atomik. Constraint existing `UNIQUE(session_id,voter_user_id)` menjadi perlindungan terakhir. Uji dua request bersamaan membuktikan tepat satu sukses dan satu audit vote. Audit mencatat partisipasi tanpa pilihan kandidat. Pemilih/sekolah tidak menerima tally kandidat; hasil resmi mengikuti endpoint publik penghargaan existing.

Bantuan operator memerlukan consent REQUEST dari akun pemilih terautentikasi berisi kandidat dan alasan. EXECUTE hanya operator delegasi, memakai pilihan yang sudah disetujui, mengonsumsi consent sekali, tunduk constraint ballot yang sama. Consent, waktu, identitas, alasan dan eksekusi tersimpan; operator tidak diberi form bebas menentukan pilihan orang lain.

## 8. Like dan perhitungan Ter-Favorit

`SkorVote = min(suaraSah / hadirDisahkan × 100, 100)`.
`SkorLike = min(max(like,0) / 800 × 100, 100)`.
`Total = SkorVote × 0,60 + SkorLike × 0,40`, memakai fungsi pembulatan existing.

Sumber like manual memerlukan sekolah delegasi, URL konten, jumlah, URL screenshot, alasan dan waktu observasi yang tidak melewati cut-off bersama maupun waktu sekarang. Simpan draft/kirim menghasilkan revisi snapshot baru. Master dapat mengembalikan dengan alasan; petugas menyalin menjadi revisi koreksi baru. Revisi lama dan audit tetap tersimpan. Hanya VERIFIED menjadi effective count; tidak ditambahkan ke hitungan otomatis. Snapshot otomatis hanya layak final bila berstatus AUTO/FINAL dan memenuhi cut-off. Manual tanpa verifikasi tidak bisa dipakai penetapan.

Preview sesudah CLOSE menampilkan vote, skor vote, skor like, total, status sumber/bukti dan kelayakan. Finalisasi memeriksa ulang semua syarat dalam transaksi, menyimpan metadata semua kandidat, menetapkan penghargaan, mengubah sesi ESTABLISHED dan membekukan konten. Tie-break existing dipertahankan: total, skor internal/rasio hadir, actual likes; jika semuanya seri perlu keputusan manual atas salah satu kandidat seri. UI menyediakan pilihan keputusan seri dan server memvalidasinya.

Bobot jalur utama tetap 20/40/20/20; Zona Tergagal tetap 20/40/30/10. Mesin scoring existing tidak diubah. Inovasi tidak membaca skor Presentasi.

## 9. Pengujian teknis

Database pengujian PostgreSQL 15 terisolasi: container `gaspol-phase1-test`, loopback port 55432, database `gaspol_test`. Test integrasi menggunakan schema/fixture sendiri; test migration membuat database sementara khusus dan menghapus database buatannya sendiri. Database aplikasi/Production tidak digunakan.

| Pemeriksaan | Hasil dan bukti |
|---|---|
| `go test ./... -count=1` dengan `GASPOL_TEST_DATABASE_URL` | PASS seluruh package, termasuk integration Phase 1, migration, handler, middleware dan services. |
| Ulang uji services/handlers/middleware setelah integrasi preview | PASS. |
| Uji terarah Phase234 setelah assertion preview/audit terakhir | PASS; skor dinamis dan PostgreSQL terintegrasi. |
| `npx tsc --noEmit` | PASS; build juga menjalankan TypeScript. |
| `npm run lint` | PASS. |
| `npm run build` | PASS; Next.js 16.1.1, 158 halaman, route `/gaspol/voting` tercakup. |
| `node --test tests/gaspol-phase1.test.mjs` | PASS 3 tes assignment/tab/filter. Warning module-type existing tidak menghentikan tes. |
| `node tests/gaspol-phase234-browser.mjs` | PASS Edge headless, viewport 390×844, login/target/confirmation/receipt/fallback, QR lima sekolah, unduh PNG, kontrol print, tanpa overflow/pageerror pada skenario ini. API disimulasikan; bukan E2E staging. |
| Migration compatibility | PASS fresh DB 001–071 dan Up ulang/schema/enum; salinan Staging BLOCKED. |
| Authorization/concurrent vote | PASS melalui handler, middleware dan integrasi PostgreSQL: assignment salah, sekolah lain, nonroster, bypass Koor, consent reuse, shared-token dan vote ganda ditolak. |
| `go vet ./internal/services ./internal/middleware` | PASS. |
| `go vet ./...` (diagnostik tambahan) | FAIL baseline di modul pengaduan (`ComplaintHandler` receiver menyalin `sync.Map`) dan unreachable code `siswa.go:758`, `uploads.go:510`. File tersebut tidak diubah pekerjaan ini; bukan klaim bahwa seluruh repository lolos vet. |
| `git diff --check` | PASS. |

Uji integrasi baru juga membuktikan: submit inovasi tidak lengkap ditolak; submit terkunci; histori draft/submit ada; versi lama tetap terikat setelah aktivasi baru; nilai posted langsung diabaikan; kedua rubrik divalidasi dua pihak; akun baru bcrypt dan reuse existing; password tidak ada di audit; denominator dua hadir dari kapasitas33; vote paralel satu sukses; operator sesuai consent; bukti like belum terverifikasi menggagalkan finalisasi; pengembalian/koreksi mempertahankan enam snapshot untuk lima sekolah; total favorit100 dalam fixture; audit validasi tepat empat event.

## 10. Satu checklist final Phase 1–4

PASS berarti bukti lokal yang disebutkan pada kolom bukti. BLOCKED berarti pengujian menyeluruh memerlukan data/lingkungan/perangkat eksternal; tidak disamakan dengan PASS karena inspeksi kode saja.

| Phase | Checklist | Status | Bukti/batas |
|---|---|---|---|
| 1 | Form laporan Nominasi berjalan | PASS | Test kontrak dan PostgreSQL school phase. |
| 1 | Draft dan submit berjalan | PASS | Test lifecycle school phase. |
| 1 | Status sekolah sesuai penetapan resmi | PASS | Fixture cohort/lock school phase. |
| 1 | Pop-out tidak berulang setelah dibaca | PASS | Test acknowledgement persisten per sekolah/lock. |
| 1 | Perubahan Top10, Bottom5, hasil Banding benar | PASS | Test ranking/cohort/lock Phase1 dan competition. |
| 1 | Tab penilaian mengikuti assignment | PASS | 3 tes frontend dan backend scope. |
| 1 | Histori dan penilaian lama tetap benar | BLOCKED | Tes legacy/history lokal lulus; rekonsiliasi data Staging sebelum/sesudah migration belum tersedia. |
| 2 | Master dapat membuat rubrik | PASS | SaveRubricVersion integration. |
| 2 | Bobot total100 | PASS | 99% gagal aktivasi; skor unit. |
| 2 | Aktivasi eksplisit | PASS | Draft tidak dipakai sebelum Activate. |
| 2 | Versi lama tidak berubah | PASS | Binding tetap versi pertama setelah aktivasi versi baru. |
| 2 | Perubahan menghasilkan versi baru | PASS | ID versi draft/aktif baru pada integration. |
| 2 | Penilai hanya assignment sah | PASS | Akses sekolah/tahap lain ditolak. |
| 2 | Form Nominasi menggunakan rubrik | PASS | Integrasi skor indikator80 mengabaikan score99; typecheck/build UI. |
| 2 | Form pengajuan Inovasi | PASS | Draft, kelengkapan, submit, read, histori dan lock integration. |
| 2 | Form Inovasi menggunakan rubrik | PASS | Rubrik terpisah, skor75 mengabaikan score100. |
| 2 | Validasi dua pihak | PASS | CABDIN sebelum KOOR ditolak; kedua tahap menyelesaikan kedua rubrik. |
| 2 | Nilai dihitung benar | PASS | Unit weighted59, integrasi80/75. |
| 2 | Multi-select sekolah | PASS | Tes bulk assignment/eligibility dan frontend stage filter; kontrol searchable memakai overview existing. |
| 3 | GASPOL_VOTER dapat dibuat | PASS | Integration membuat akun, cek role/bcrypt/no audit password. |
| 3 | Pengawas existing dipakai ulang | PASS | Username existing tidak menghasilkan credential/akun baru; voter3 role pengawas berhasil vote. |
| 3 | Hak akses tidak bocor | PASS | Allowlist middleware, handler tanpa auth, nonroster dan assignment salah ditolak. |
| 3 | QR sekolah valid | PASS | QR generator lima kandidat/current-session dan login target browser. |
| 3 | QR diunduh dan dicetak | BLOCKED | Download PNG nyata dan kontrol print lulus; dialog/hasil printer fisik belum diuji. |
| 3 | Scan QR menuju sekolah benar | BLOCKED | Navigasi URL target browser lulus; scan kamera perangkat fisik belum tersedia. |
| 3 | Login dan konfirmasi | PASS | Browser memakai form login, bearer token, kandidat benar dan receipt. |
| 3 | Satu akun satu suara | PASS | Constraint + repeated vote ditolak. |
| 3 | Request paralel tidak menggandakan suara | PASS | Dua koneksi/request satu sukses, satu audit. |
| 3 | QR fallback | PASS | Kode pendek lowercase di browser tetap kandidat benar; backend tetap auth/roster. |
| 3 | Partisipasi akurat | PASS | Integration Master present2/voted1; browser rendering statistik. |
| 3 | Daftar hadir/denominator | PASS | Kapasitas33, dua hadir, roster terkunci OPEN. |
| 3 | Hasil tidak bocor sebelum rilis | PASS | VotingView pemilih tidak memuat results; UI tidak mendapat tally. |
| 3 | Rumus vote60% | PASS | Test favorite normalization dan final fixture. |
| 3 | Batas800 like | PASS | Test normalization cap/threshold existing. |
| 3 | Input like manual | PASS | Lima submit snapshot berisi bukti/waktu/cut-off. |
| 3 | Verifikasi Master wajib | PASS | Finalisasi tanpa/unverified likes ditolak. |
| 3 | Koreksi like tercatat | PASS | RETURN lalu submit revisi; enam snapshot tetap tersedia. |
| 3 | Gabungan60%+40% | PASS | Final fixture total100 dan unit normalization. |
| 4 | Preview sesuai | PASS | Integration lima preview, readyToEstablish; regression preview readiness existing. |
| 4 | Penetapan resmi mengikuti workflow | PASS | OPEN/missing/unverified dicegah, CLOSED+valid sukses; policy two-level/direct-final existing lulus. Approval penghargaan lengkap tetap perlu UAT staging. |
| 4 | Skor antarjalur terpisah | PASS | Test main/not-failed dan Inovasi rubric stage terpisah. |
| 4 | Semua bobot tetap | PASS | Test main20/40/20/20, failed20/40/30/10, favorite60/40. |
| 4 | Histori lengkap | BLOCKED | Event baru dan Phase1 diuji lokal; audit seluruh perjalanan data riil semua role masih perlu UAT staging. |
| 4 | Tidak ada histori duplikat | PASS | Empat review audit untuk dua assessment, satu audit vote paralel; regression history fallback existing. |
| 4 | Tidak ada lintas-role tanpa izin | PASS | Kasus negatif middleware/handler/service; tidak diklaim sebagai pentest seluruh aplikasi. |
| 4 | Mobile responsive | PASS | Vote/QR390×844, screenshot dan scrollWidth check. Form seluruh role perlu UAT tambahan. |
| 4 | Tidak ada runtime error baru | PASS | Tidak ada pageerror pada browser skenario vote/QR; build dan backend lulus. |
| 4 | Tidak ada regresi GASPOL existing | BLOCKED | Seluruh automated tests lulus; UAT data Staging dan produksi belum dilakukan. |

Tidak ada FAIL fungsional yang tersisa pada skenario penerimaan lokal yang dijalankan. Diagnostik tambahan vet global masih FAIL baseline sebagaimana tabel teknis; tidak disembunyikan sebagai PASS.

## 11. Masalah tersisa dan risiko deployment

1. Belum ada koneksi/data Staging untuk uji migrasi dari salinan historis, approval penghargaan dengan seluruh role, dan pengumuman publik end-to-end. ZIP baseline lama tidak digunakan untuk menimpa branch aktif.
2. Rubrik awal tetap sementara. Koordinator/Master perlu meninjau dan melakukan aktivasi eksplisit. Tanpa rubrik aktif, form penilaian menolak pengiriman.
3. Penilaian legacy tanpa snapshot versi tidak dikonversi otomatis. Nilai final historis dipertahankan; kasus legacy yang dikembalikan untuk penilaian ulang perlu kebijakan migrasi/binding terverifikasi sebelum dapat memakai kontrak rubrik baru.
4. Client voting/override like lama harus diperbarui bersama aplikasi karena bypass shared-token/manual langsung dinonaktifkan. Sesi legacy tidak digabung ke skor managed session baru.
5. Penyampaian password awal dilakukan Master kepada pemilik akun melalui kanal organisasi yang aman; tidak ada pengiriman email/WhatsApp otomatis. Credential hanya tersedia saat pembuatan dan tidak dicatat di audit.
6. Provider API like resmi dan credential eksternal belum diverifikasi. Alur manual terverifikasi dapat dipakai. Waktu screenshot adalah deklarasi petugas yang diverifikasi Master; sistem tidak memverifikasi metadata gambar eksternal secara otomatis.
7. Snapshot draft/koreksi like bersifat append-only; revisi baru tidak menghapus revisi lama. Pengajuan inovasi terkunci setelah submit; tidak ada fitur bebas reopen.
8. Pemilih/sekolah tidak menerima tally detail melalui API voting bahkan sesudah penetapan; pemenang resmi memakai publikasi penghargaan existing. Histori Master menampilkan 200 event terbaru; histori existing tetap tersedia untuk penelusuran lebih lama.
9. Dependensi npm menunjukkan 53 temuan audit keseluruhan repository saat pemasangan QR (1 low,33 moderate,18 high,1 critical). Belum dilakukan remediasi dependensi global di luar scope GASPOL. Vet global juga mempunyai temuan baseline yang dijelaskan di atas.
10. Origin URL QR mengikuti origin aplikasi saat dibuat. Generate ulang di domain Staging/Production yang benar; QR localhost hanya untuk pengujian lokal. Pastikan HTTPS dan kebijakan popup/print browser di perangkat nyata.

## 12. Panduan manual Staging

1. Siapkan backup/salinan database Staging, catat jumlah assignment, submission, assessment FINAL, snapshot, awards dan audit per tahun sebelum migration. Jalankan Goose sampai071 di salinan itu; cocokkan jumlah/nilai historis dan schema, kemudian smoke startup backend/frontend versi sama. Jangan reset database.
2. Login Master, buka GASPOL dan tahun uji. Periksa status Phase1, pemberitahuan sekali, Top10/Bottom5/Banding dan historinya. Gunakan akun sekolah berbeda untuk menguji scope.
3. Buat draft rubrik Nominasi/Inovasi. Coba total99, aktivasi harus gagal. Tinjau label sementara/persetujuan Koordinator; aktivasi eksplisit100. Tugaskan pengawas melalui multi-select searchable existing. Pastikan Presentasi tidak memberi akses Inovasi.
4. Sekolah Nominasi draft/submit laporan; Top5 resmi draft/submit inovasi dan buka ulang memastikan lock. Penilai buka laporan/bukti, isi indikator/catatan; coba URL/API sekolah lain. Aktifkan versi baru saat penilaian berlangsung dan pastikan versi lama tetap tampil. Validasi Koor, return/resubmit bila perlu, lalu Cabdin. Banding tetap input langsung.
5. Master buat sesi favorit dengan lima kandidat resmi, kapasitas33 dan cut-off bersama. Buat akun baru/reuse pengawas, tetapkan kehadiran. Periksa password tidak muncul pada audit; sampaikan hanya kepada pemilik. Coba cabut/tambah roster sebelum OPEN; buka lalu pastikan perubahan roster ditolak. Cocokkan denominator dengan hadir aktif.
6. Sekolah lihat QR sendiri; Master lihat kelimanya. Unduh PNG, cetak ke printer/PDF, scan dari dua jenis ponsel. Pastikan nama/NPSN/tahun/kategori dan URL HTTPS tepat, target tetap sesudah login, QR sesi lain ditolak. Coba short code, nonroster, absen, expired session dan login salah.
7. Konfirmasi satu vote; ulang dan kirim dua request bersamaan untuk akun uji lain. Pastikan hanya satu ballot. Cocokkan monitor registered/present/voted/notVoted/percent. Pemilih/sekolah tidak boleh membaca tally kandidat dengan manipulasi API.
8. Delegasikan operator; pemilih login dan memberikan consent kandidat/alasan. Operator hanya mengonsumsi consent itu sekali; operator lain tidak sah. Cocokkan jejak pemilih/operator/waktu/alasan tanpa membuka pilihan pemilih lain.
9. Delegasikan verifikator like multi-sekolah. Ajukan screenshot/cut-off sama, coba waktu sesudah cut-off (ditolak), return dengan alasan, kirim koreksi dan verify Master. Pastikan manual tidak dijumlahkan ke auto; 400like=50,800/1000like=100. Belum verified harus menghalangi finalisasi.
10. CLOSE sesi, bandingkan preview60/40 dengan hitungan manual. Uji kandidat seri penuh dan keputusan tie-break existing. Tetapkan; pastikan ESTABLISHED, data frozen, audit dan penghargaan publik benar. Inovasi harus semua Top5 tervalidasi dan melalui approval Kasi SMK existing sebelum resmi.
11. Periksa semua role pada390px, tablet dan desktop; navigasi/tabel/form/error state, histori tanpa duplikasi, bukti eksternal dan print. Jalankan ulang test teknis serta catat UAT riil untuk mengganti status BLOCKED yang relevan.

Perintah reproduksi lokal: dari `backend`, set `GASPOL_TEST_DATABASE_URL` hanya ke database pengujian terisolasi lalu `go test ./... -count=1`. Dari root: `npm run lint`, `npx tsc --noEmit`, `npm run build`, `node --test tests/gaspol-phase1.test.mjs`. Untuk browser set `GASPOL_PLAYWRIGHT_MODULE` ke module Playwright yang tersedia dan `GASPOL_UI_TEST_URL` ke server lokal, lalu `node tests/gaspol-phase234-browser.mjs`. Test browser menggunakan Edge dan API simulasi; tidak mengirim ballot ke Staging/Production.
