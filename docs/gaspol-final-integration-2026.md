# GASPOL Bang 2026 — integrasi akhir dan penguatan workflow

Tanggal verifikasi lokal: 13 Oktober 2026. Implementasi menggunakan kode workspace aktual. Tidak dilakukan commit, push, deployment, migrasi database staging/production, atau perubahan nilai produksi.

## Perubahan per fase

| Fase                      | Hasil implementasi                                                                                                                                                                                                                                                                                                                            |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 — Audit                 | Alur yang sudah ada dipakai kembali; pusat penetapan ditambahkan di dashboard, tanpa sidebar/halaman publik baru. Matriks operasional tersedia di bawah.                                                                                                                                                                                      |
| 2 — Nilai dan kelengkapan | Kewajiban dihitung per pasangan sekolah–penilai aktif; semua penilai wajib final. Nilai final nol sah; draft, return, dan sekolah tanpa penugasan tidak lengkap. Penugasan batal tidak dihitung. Bobot utama 20/40/20/20 dan jalur nominasi 20/40/30/10 dipertahankan. Perhitungan bulanan enam bulan tetap menggunakan jalur yang sudah ada. |
| 3 — Top 10/Top 5          | Dashboard memakai snapshot resmi. Top 10 tidak dibuka ulang lewat alur biasa. Top 5 mengambil sepuluh peserta resmi, memeriksa seluruh T2, dan wajib BA PDF bernomor/bertanggal melalui wizard tiga langkah.                                                                                                                                  |
| 4 — Presentasi/inovasi    | Peserta berasal dari lima finalis resmi. Inovasi memakai penilaian khusus final langsung dari juri; semua lima sekolah harus selesai, lalu persetujuan Kasi terhadap versi nilai terkini. Persetujuan tidak membuat award. Master dapat menunjuk Kasi dari UI.                                                                                |
| 5 — Favorit               | Memakai lima finalis, voting internal 60% dan like 40%, batas like 800, verifikasi Master, sesi voting ditutup dan cutoff terpenuhi. Seri menggunakan rasio internal, kemudian keputusan musyawarah.                                                                                                                                          |
| 6 — Penetapan bersama     | Preview, ekspor JSON/CSV, versi data, BA gabungan, dan satu transaksi BEST/INNOVATIVE/FAVORITE. Satu sekolah boleh menang beberapa kategori. Perubahan input relevan membatalkan ekspor; endpoint finalisasi terpisah menolak dan mengarahkan ke alur bersama.                                                                                |
| 7 — Nominasi              | Sebelas sekolah di luar Top 10 boleh dinilai tanpa laporan. Nol memerlukan alasan; validasi Koor dan Cabdin tetap wajib. Lima kandidat dipilih manual dari preview dan dikunci tanpa BA.                                                                                                                                                      |
| 8 — Banding/Ter Gagal     | Lima kandidat resmi dinilai ulang dengan banding 10%. Tiga penerima ditetapkan dengan BA wajib; kartu penghargaan tidak menampilkan nilai/peringkat kategori.                                                                                                                                                                                 |
| 9 — Klasemen              | Snapshot 21 sekolah unik dalam zona tetap 5/5/6/2/3, memakai nilai masing-masing jalur. Seri posisi memerlukan urutan dan alasan musyawarah. Preview memiliki versi yang harus cocok saat lock. Tidak memakai global sort nilai lintas zona.                                                                                                  |
| 10 — Return/revisi        | Master dapat mengembalikan T2, presentasi, inovasi, nominasi, dan banding sebelum lock terkait. Nilai lama masuk histori; validasi/persetujuan lama dibatalkan. Submit final tidak dapat ditimpa langsung. Reopen resmi terbatas dengan alasan dan pemeriksaan ketergantungan.                                                                |
| 11 — URL                  | HTTP/HTTPS dan domain tanpa skema dinormalisasi; Google Drive/Docs diterima. Bukti kosong opsional disaring. Field error mempertahankan isian. Protokol berbahaya, kredensial URL, dan spasi ditolak.                                                                                                                                         |
| 12 — Dashboard            | Master mendapat tab Penetapan & BA; Cabdin monitoring; juri inovasi pengajuan; Kasi persetujuan/return. Form sekolah, validasi, voting, rubrik, penugasan dan histori memakai UI yang sudah ada. Publik memakai snapshot terkunci setelah kontrol publikasi aktif.                                                                            |
| 13 — Dokumen              | Daftar lock menampilkan jenis, tahun konteks, revisi, nomor/tanggal BA, pengunggah/waktu, serta preview/unduh. ZIP mengambil BA yang terikat lock resmi aktif pada tahun yang dipilih.                                                                                                                                                        |
| 14 — Histori/migrasi      | Histori penetapan berada dalam transaksi. Persetujuan Kasi tidak lagi membuat histori lock ganda. Migrasi baru bersifat aditif dan tidak membuat hasil resmi dari nilai lama.                                                                                                                                                                 |
| 15 — Pengujian            | Unit, integrasi PostgreSQL dengan migrasi asli, regresi frontend, typecheck, lint terarah, dan build lulus. Browser/manual per role belum dijalankan; batasannya dijelaskan di bawah.                                                                                                                                                         |

## File penting

- `backend/migrations/072_harden_gaspol_determination.sql`: metadata BA, sumber snapshot, ekspor berversi, constraint lock/posisi, proteksi perubahan tahap terkunci.
- `backend/internal/services/gaspol_determination.go`: kewajiban, versi, monitor, penetapan atomik, klasemen lima zona, publikasi snapshot, ZIP dan cleanup BA staging.
- `backend/internal/services/gaspol_lock.go`: preview/lock tahap, sumber resmi, BA wajib, pemeriksaan ulang dalam transaksi dan revisi beralasan.
- `backend/internal/services/gaspol_competition.go`: return, laporan/URL, BA, publikasi, penugasan Kasi, larangan menimpa submit.
- `backend/internal/services/gaspol_awards.go`: persetujuan inovasi terpisah, penghargaan publik, penonaktifan finalisasi parsial.
- `backend/internal/services/gaspol_innovation.go`: nominasi tanpa laporan, alasan nilai nol, inovasi final langsung.
- `backend/internal/services/gaspol.go`, `gaspol_ranking_v2.go`, `gaspol_school_phase.go`: konsistensi snapshot dan laporan.
- `backend/internal/handlers/gaspol.go`, `backend/cmd/server/main.go`: integrasi API dan pemetaan error.
- `components/gaspol/GaspolDeterminations.tsx`: pusat monitoring, wizard BA, ekspor, keputusan seri, persetujuan Kasi, return, revisi, klasemen dan publikasi.
- `components/gaspol/GaspolDashboard.tsx`, `GaspolPhaseTools.tsx`, `GaspolVoting.tsx`, `GaspolRubrics.tsx`, `GaspolSchoolPhase.tsx`: integrasi dashboard dan form yang sudah ada.
- `lib/gaspol-report-url.ts`: normalisasi URL frontend.
- `backend/internal/services/gaspol_determination_test.go`, `gaspol_competition_test.go`, `gaspol_phase234_test.go`, `gaspol_school_phase_test.go`, `tests/gaspol-hardening.test.mjs`: pengujian baru/regresi.

## Matriks backend → API → UI → role → status → test

Semua path di bawah berawalan `/api`. “Lulus” merujuk pengujian kode/service; klik UI aktual tetap memerlukan checklist manual.

| Backend/fungsi                             | API                                                                                              | UI                                          | Role                                                             | Status/alur                                                         | Verifikasi                                        |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------ | ------------------------------------------- | ---------------------------------------------------------------- | ------------------------------------------------------------------- | ------------------------------------------------- |
| AdminDashboard / snapshot resmi            | GET `/gaspol/admin/dashboard`                                                                    | Dashboard Top 10/Top 5                      | Master, Cabdin                                                   | LOCKED menjadi sumber resmi                                         | Integrasi snapshot/migrasi lulus                  |
| Penugasan tahap dan Kasi                   | POST `/gaspol/admin/stage-assignments`, `/bulk`                                                  | Penugasan; selector Kasi di Penetapan & BA  | Master                                                           | ACTIVE/CANCELLED sesuai jalur penugasan                             | Integrasi penunjukan Kasi lulus                   |
| SaveStageAssessment / rubrik               | POST `/gaspol/assessments`, GET `/gaspol/assessment-rubric`                                      | Dashboard tugas dan form rubrik             | Penilai aktif                                                    | Submit; validasi tahap atau FINAL langsung                          | Regresi fase 2–4 lulus                            |
| Penilaian bulanan/T1/presentasi lama       | POST `/gaspol/supervisor/assessments/monthly`, `/gaspol/jury/assessments/field`, `/presentation` | Form dashboard yang sudah ada               | Pengawas/juri ditugaskan                                         | Menjaga workflow dan proteksi lock                                  | Suite Go lulus                                    |
| ReviewStageAssessment                      | POST `/gaspol/assessments/:id/review/:level`                                                     | Form validasi yang sudah ada                | Koor/Cabdin sesuai penugasan                                     | Nominasi dua pihak → FINAL                                          | Nol final + validasi dua pihak lulus              |
| DeterminationMonitor                       | GET `/gaspol/internal/determinations`                                                            | Penetapan & BA/monitoring sesuai role       | Master, Cabdin; inovasi terbatas untuk juri/Kasi                 | Progres, blokir, nilai, approval, dokumen                           | Integrasi layanan lulus; browser pending          |
| LockCompetitionResult                      | POST `/gaspol/admin/result-locks`                                                                | Wizard Top 5/kandidat/Ter Gagal             | Master                                                           | TOP_5 + BA; BOTTOM_5 tanpa BA; FINAL + BA                           | Kelengkapan, nol, BA, banding lulus               |
| MasterReturnStageAssessment                | POST `/gaspol/stage-assessments/:id/master-return`                                               | Kembalikan pada daftar kewajiban            | Master                                                           | RETURNED_BY_MASTER + alasan                                         | Approval batal, kelengkapan turun, resubmit lulus |
| ProposeInnovationAward                     | POST `/gaspol/awards/innovation/propose`                                                         | Ajukan seluruh nilai inovasi kepada Kasi    | INNOVATION_ASSESSOR aktif                                        | SUBMITTED untuk versi seluruh nilai                                 | Integrasi lulus                                   |
| DecideInnovationAward                      | POST `/gaspol/awards/innovation/:id/decision`                                                    | Setujui/Kembalikan usulan                   | KASI_SMK_APPROVER aktif                                          | APPROVED/RETURNED, belum award                                      | Tidak ada award sebelum final lulus               |
| Voting/likes                               | GET/POST `/gaspol/voting`, `/gaspol/voting/vote`, `/gaspol/likes`; POST `/gaspol/admin/voting`   | Voting, pengelolaan sesi, verifikasi like   | Master, GASPOL_VOTER, sekolah sesuai kewenangan                  | CLOSED + like terverifikasi                                         | Favorit 60:40, nol, seri lulus                    |
| DetermineFinal preview/export/lock         | POST `/gaspol/admin/final-determination`                                                         | Preview nilai, JSON/CSV, wizard BA gabungan | Master                                                           | Preview → ekspor versi → BA → AWARDING                              | Kedaluwarsa, multi-award, rollback parsial lulus  |
| Finalisasi parsial lama                    | POST `/gaspol/admin/awards/best/finalize`, `/favorite/finalize`                                  | Diganti penetapan bersama                   | Master                                                           | JOINT_FINALIZATION_REQUIRED                                         | Regresi lulus; sengaja bukan aksi terpisah        |
| DetermineFinal standings-preview/standings | POST `/gaspol/admin/final-determination`                                                         | Preview 21 sekolah + konfirmasi lock        | Master                                                           | STANDINGS terkunci, versi cocok                                     | Zona, unik, tanpa perpindahan lintas zona lulus   |
| Upload/Download/DeleteStagedBA             | POST `/gaspol/admin/ba`, GET `/gaspol/ba/:id`, DELETE `/gaspol/admin/ba/:id`                     | Wizard PDF dan preview/unduh BA             | Master upload/cleanup milik sendiri; akses baca sesuai otorisasi | STAGED → terikat lock; cleanup otomatis saat lock gagal             | BA wajib/signature + ZIP lulus                    |
| OfficialDocumentsZIP                       | GET `/gaspol/internal/ba-zip`                                                                    | Unduh ZIP BA resmi per tahun                | Master, Cabdin                                                   | Hanya dokumen lock aktif                                            | Isi/tahun/draft tersaring lulus                   |
| ReopenCompetitionResult                    | POST `/gaspol/admin/result-locks/:id/reopen`                                                     | Revisi penetapan + alasan                   | Master                                                           | Top 5/kandidat/FINAL sebelum dependensi; Top 10/AWARDING dilindungi | Suite Go lulus                                    |
| SaveNominationSubmission                   | PUT `/gaspol/school/nomination-submission`                                                       | Laporan perbaikan sekolah, error per field  | Sekolah terkait                                                  | Laporan opsional; URL dinormalisasi                                 | Drive/Docs/HTTPS/domain/kosong/berbahaya lulus    |
| UpdateCompetitionConfig                    | PUT `/gaspol/admin/competition`                                                                  | Checkbox publikasi setelah 21 snapshot      | Master                                                           | Lock tidak otomatis publikasi                                       | Sebelum klasemen ditolak, sesudahnya lulus        |
| PublicDashboard / PublicAwards             | GET `/public/gaspol/dashboard`, `/public/gaspol/best-school`                                     | Interface publik yang sudah ada             | Publik                                                           | Snapshot resmi; kartu Ter Gagal setara tanpa nilai                  | Gate publikasi dan snapshot lulus                 |
| History                                    | GET `/gaspol/history`, `/:id`, `/export.xlsx`                                                    | Histori yang sudah ada                      | Sesuai hak akses histori                                         | Submit, return, validasi, approval, BA, lock, revisi, publikasi     | Histori transaksi + suite regresi lulus           |

Kesenjangan UI yang diintegrasikan: wizard BA/lock tahap, penetapan tiga penghargaan bersama, ekspor berversi, persetujuan Kasi dan penugasannya, pengajuan juri inovasi, return lintas tahap, revisi beralasan, preview/lock klasemen 21, ZIP BA resmi, serta gate publikasi. Tidak dibuat implementasi penilaian/validasi/voting tandingan.

## Bug yang diperbaiki

1. Sekolah dianggap selesai walau ada penilai aktif lain belum submit atau tidak mempunyai penugasan.
2. Nilai final nol tercampur dengan data kosong; nominasi bergantung pada laporan sekolah.
3. Keanggotaan resmi dashboard mengikuti ranking live dan nilai dasar tahap sebelumnya dihitung ulang.
4. Persetujuan Kasi langsung membuat award dan tidak terikat versi nilai seluruh finalis.
5. Finalisasi kategori berjalan terpisah sehingga berisiko menghasilkan lock parsial.
6. BA final/BA Ter Gagal tidak wajib, metadata belum lengkap, dan ekspor lama dapat dipakai setelah perubahan nilai.
7. Return tidak menjangkau tahap lanjutan dan tidak membatalkan approval inovasi lama; submit dapat ditimpa langsung.
8. Ranking lengkap menggunakan data live/urutan lintas zona yang dapat mengubah posisi resmi.
9. Finalisasi Favorit terpisah dari BA bersama; like/voting belum final berpotensi ikut ditetapkan.
10. URL tanpa skema ditolak tanpa petunjuk field; bukti kosong opsional ikut menggagalkan form.
11. Dokumen publik tidak cukup terkait status lock/publikasi; ZIP resmi lintas tahap belum terintegrasi.

## Migrasi dan staging

Migrasi baru: `072_harden_gaspol_determination.sql`. Jangan mengubah migrasi historis. Goose tetap memakai mekanisme startup proyek. Periksa konfigurasi aktivasi migrasi yang digunakan deployment sebelum rilis.

- Kolom baru: nomor/tanggal BA, `source_payload` lock, `source_version` approval inovasi.
- Tabel baru `gaspol_result_exports` menyimpan bahan BA berversi beserta pelaku/waktu.
- Snapshot v2 menerima STANDINGS; indeks menjamin satu lock aktif per jenis/tahun dan satu posisi per revisi klasemen.
- Trigger melindungi penilaian/penugasan tahap yang sudah terkunci. Migrasi tidak melakukan update skor, mengubah snapshot Top 10, atau membuat award resmi.
- Goose melacak migrasi; pengulangan deployment diuji. Down sengaja ditolak karena hasil resmi harus dipulihkan melalui prosedur backup yang ditinjau.

Urutan staging:

1. Backup database dan dokumen. Catat ID, status, payload, revisi, dan BA Top 10 2026 sebelum rilis.
2. Periksa duplikasi lock aktif `(year, lock_type)` sebelum indeks unik dibuat. Bila ditemukan, hentikan rilis dan rekonsiliasi melalui keputusan resmi; jangan hapus otomatis.
3. Periksa hasil resmi lama selain Top 10. Lock Top 5/kandidat/FINAL lama yang belum mempunyai sumber cohort lengkap tidak diberi backfill nilai live. Rekonsiliasi snapshot resmi diperlukan sebelum klasemen 21 dapat dibentuk. Award lama terpisah juga harus ditinjau sebelum penetapan bersama; implementasi tidak menimpa hasil tersebut.
4. Deploy backend dengan migrasi 072 dan frontend sebagai rilis terkoordinasi. Pastikan direktori migrasi disertakan dalam image/artefak.
5. Verifikasi versi Goose 72, kolom baru, dan Top 10 identik dengan catatan sebelum rilis.
6. Jalankan checklist role di bawah menggunakan akun staging yang ditugaskan dan dokumen uji. Pastikan tidak memakai nilai fiktif di production.
7. Biarkan publikasi mati sampai seluruh penetapan dan snapshot 21 selesai. Lock tidak mengaktifkan publikasi otomatis.
8. Setelah staging diterima, gunakan migrasi yang sama di production dengan backup dan pemeriksaan lock yang sama. Tidak ada langkah deployment yang telah dijalankan oleh pekerjaan lokal ini.

## Hasil pengujian aktual

| Pemeriksaan                                                                | Hasil                                                                                                                                                                                                   |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `go test ./... -count=1` dari backend, dengan `GASPOL_TEST_DATABASE_URL`   | Lulus seluruh paket, termasuk integrasi database nyata                                                                                                                                                  |
| Migrasi asli hingga 71, seed Top 10 lama, migrasi 72, lalu Up berulang     | Lulus; payload/status Top 10 tetap; memakai database sementara                                                                                                                                          |
| Workflow PostgreSQL lengkap                                                | Lulus: T2 incomplete/nol, BA Top 5, semua inovasi + Kasi, return/resubmit/reapproval, voting/like, ekspor stale, rollback award gagal, multi-award, nominasi, banding, 21 snapshot, gate publikasi, ZIP |
| `node --test tests/gaspol-phase1.test.mjs tests/gaspol-hardening.test.mjs` | 6 lulus, 0 gagal/skip; warning MODULE_TYPELESS_PACKAGE_JSON tidak menggagalkan                                                                                                                          |
| `npx tsc --noEmit`                                                         | Lulus                                                                                                                                                                                                   |
| ESLint terarah pada komponen GASPOL yang berubah dan normalizer URL        | Lulus                                                                                                                                                                                                   |
| `npm run build`                                                            | Lulus; compile dan 158 halaman selesai                                                                                                                                                                  |
| `git diff --check`                                                         | Lulus                                                                                                                                                                                                   |
| Browser/peran/responsivitas                                                | Belum dijalankan; bukan dianggap lulus                                                                                                                                                                  |

Database integrasi memakai PostgreSQL 15 Docker khusus `127.0.0.1:55432/gaspol_test`; test membuat database sementara sendiri, menjalankan migrasi asli, dan menghapusnya setelah test. Fixture nilai hanya ada di database test. Container test lokal dihentikan/dihapus setelah verifikasi. Database aplikasi dari `.env` tidak digunakan.

Browser bawaan gagal terhubung karena `privileged native pipe bridge is not available; browser-client is not trusted`. Peninjauan otomatis menolak perintah menjalankan server UI lokal (`npm run start ... --port 3012`) dengan alasan yang tersedia hanya `blocked by policy`. Tidak dilakukan workaround atas penolakan. Script browser `tests/gaspol-phase234-browser.mjs` dan klik UI lintas role belum dijalankan.

## Checklist manual berbasis role — masih perlu dijalankan di staging

- [ ] Master: Top 10 identik dengan snapshot lama; tidak ada tombol reset; Cabdin melihat daftar yang sama.
- [ ] Master: preview 10 peserta T2, penilai ganda/belum ditugaskan/return terlihat; lock terblokir hingga semua final, termasuk nol sah.
- [ ] Master: wizard Top 5 menolak tanpa BA, file non-PDF/lebih 10 MB, nomor/tanggal kosong; preview PDF dan konfirmasi lima finalis bekerja.
- [ ] Juri: semua tugas aktif dapat dibuka; submit final tidak dapat diedit; setelah return nilai dapat diperbaiki dan disubmit ulang.
- [ ] Sekolah nominasi: laporan opsional; Drive/Docs/domain tanpa skema/bukti kosong diterima; error tepat pada field dan isian tetap ada.
- [ ] Pengawas nominasi: tanpa laporan tetap bisa menilai, nol wajib alasan; Koor dan Cabdin memvalidasi hingga FINAL.
- [ ] Juri inovasi: hanya lima finalis resmi; pengajuan menunggu semua penilai selesai.
- [ ] Kasi: persetujuan/return bisa dijalankan; approval belum menampilkan award; setelah return Master wajib approval baru.
- [ ] Master/voter: voting satu akun satu suara, lima kandidat, cutoff dan sesi; like 800 maksimal 100, verifikasi wajib; bobot 60:40.
- [ ] Master: ekspor JSON/CSV lengkap; perubahan nilai/approval/vote/like/keputusan membuat ekspor stale; backend menolak lock versi lama.
- [ ] Master: BA gabungan satu dokumen, tiga award bersama, satu sekolah boleh multi-award; keputusan seri wajib alasan.
- [ ] Master: kandidat lima tanpa BA; banding mengubah penerima; BA Ter Gagal wajib; tiga kartu setara tanpa nilai/peringkat kategori.
- [ ] Master: preview klasemen 21 unik, zona 5/5/6/2/3, seri diurutkan dengan alasan; konfirmasi versi sebelum lock.
- [ ] Cabdin: monitoring/BA/ZIP tersedia; aksi Master tidak tersedia; kewenangan validasi tetap benar.
- [ ] Master: revisi penetapan meminta alasan dan menolak ketika ada ketergantungan resmi; T1/Top 10 tetap terlindungi.
- [ ] Master/Cabdin: ZIP tahun yang dipilih hanya berisi BA resmi aktif; draft dan dokumen versi yang dibuka ulang tidak ikut.
- [ ] Publik: sebelum publikasi tidak melihat hasil internal; setelah aktif melihat 21 snapshot/zona dan penghargaan resmi; nilai live tidak mengubahnya.
- [ ] Semua role: histori pelaku/alasan/versi benar; uji layar ponsel, tabel scroll, modal keyboard, loading/error dan unduh PDF/ZIP.

## Batasan dan keputusan teknis

- Verifikasi visual dan penerimaan operasional per role masih pending karena batasan browser/server lokal di atas. Build/typecheck tidak menggantikan pengujian klik UI.
- Pemeriksaan PDF memastikan ekstensi, ukuran maksimal 10 MB, signature, tahun dan keterkaitan dokumen. Isi/pengesahan tanda tangan BA diperiksa Master; tidak ada pemeriksaan semantik PDF otomatis.
- BA disimpan sebagai bytea STAGED sebelum lock. UI membersihkan unggahan ketika lock gagal; request yang terputus dapat menyisakan baris STAGED, tidak masuk ZIP/publikasi. Endpoint cleanup hanya menerima dokumen staging milik Master yang bersangkutan dan belum direferensikan.
- Penetapan mengunci tabel terkait secara singkat dan memeriksa sumber di dalam transaksi untuk mencegah race. Pada jam ramai operasi ini dapat menunggu transaksi penilaian/voting aktif.
- Top 10 dan AWARDING tidak dibuka ulang melalui endpoint revisi biasa. Koreksi resmi setelah penghargaan bersama/klasemen membutuhkan prosedur revisi terkontrol terpisah; implementasi memilih menolak daripada membatalkan hasil hilir otomatis.
- Tidak dilakukan rekonsiliasi otomatis untuk histori parsial/hasil lama yang tidak mempunyai snapshot sumber lengkap. Kondisi tersebut dilaporkan sebagai blocker, bukan diisi menggunakan nilai live.
- Semua perubahan terbatas pada GASPOL dan laporan ini; tidak mengubah modul Sipres/MGMP.
