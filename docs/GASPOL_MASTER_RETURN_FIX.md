# GASPOL Master Return: perbaikan dan verifikasi

## Root cause dan perubahan

`ListStageTasksByAssessor` membaca `sa.master_return_notes`, `sa.koor_notes`,
dan `sa.cabdin_notes`, tetapi subquery LATERAL `sa` hanya memproyeksikan empat
kolom. PostgreSQL gagal saat merencanakan query, bahkan bila tidak ada assessment
berstatus returned. Ini bug projection SQL; keberadaan migration 065 saja tidak
memperbaikinya.

Subquery kini memproyeksikan ketiga kolom catatan. SELECT luar tetap 11 field
dengan 11 tujuan Scan dalam urutan yang sama. ID assessment dan raw score memakai
pointer karena LEFT JOIN dapat kosong; status, notes, dan alasan pengembalian
memakai COALESCE. Nilai lama tidak diubah.

File implementasi:

- `backend/internal/services/gaspol_repository.go`: projection dan konteks error query.
- `backend/internal/services/gaspol_schema.go`: pemeriksaan kolom dan kedua CHECK
  workflow sebelum server menerima request, dengan timeout 10 detik.
- `backend/cmd/server/main.go`: panggil schema check setelah migrasi/koneksi,
  termasuk saat AUTO_MIGRATE dinonaktifkan. Schema yang belum sesuai menghentikan startup.
- `backend/internal/handlers/gaspol.go`: pertahankan action `supervisor_dashboard`,
  tambah assessor_user_id/year/month numerik. Respons HTTP 500 tetap generik.
- `components/gaspol/GaspolDashboard.tsx`: label Koor, Cabdin, dan Master sesuai
  permintaan. Alasan tetap berasal dari returnNotes yang dipilih berdasarkan status.
- `backend/internal/services/gaspol_master_return_integration_test.go`: regresi SQL,
  lifecycle, dan schema check menggunakan PostgreSQL.

Migration `065_add_gaspol_master_return.sql` sudah lengkap: ketiga kolom baru
tersedia dan CHECK pada tabel stage maupun legacy menerima RETURNED_BY_MASTER.
Tidak ada migration baru atau perubahan migration lama. Tidak menjalankan Down,
reset schema, restore database, atau perubahan data produksi.

## Audit query terkait

Pencarian seluruh file terlacak mencakup RETURNED_BY_MASTER, master_return_notes,
returned_by_master_user_id, master_returned_at, dan MASTER_RETURN.

| Area | Hasil pemeriksaan source |
| --- | --- |
| Dashboard pengawas | Projection LATERAL diperbaiki; 11 SELECT/Scan; NULL ditangani. |
| Preview ranking Master | CTE field memproyeksikan semua field luar; 12 SELECT/Scan. Agregat nullable memakai pointer. Returned Master dihitung unfinished. |
| Endpoint Master Return | SELECT awal 5 field/Scan; legacy ID nullable memakai pointer. Update stage, legacy, dan audit berada dalam transaksi. Raw score/rubric dipertahankan; final_score/locked_at dikosongkan. |
| Histori dan detail event | Join langsung stage assessment; tidak ada projection LATERAL yang kehilangan field baru. MASTER_RETURN tersimpan pada payload audit dengan before/after/notes. Timeline membaca payload. Catatan ringkasan event memakai notes assessment lebih dahulu, sedangkan alasan Master tetap tersedia pada payload timeline. |
| Detail/antrean assessment | SELECT/Scan 13 field. final_score memakai pointer; notes Koor/Cabdin pada migration 062 adalah NOT NULL. Master Return hanya diizinkan untuk FIELD_STAGE_1, sehingga reader bulanan tidak diperluas. |
| Resubmit | RETURNED_BY_MASTER diterima pada pengecekan deadline dan upsert; RETURNING/Scan 10 field. Kembali WAITING_KOOR, kemudian validasi Koor/Cabdin. |
| Top 10 readiness/lock | Returned Master bukan final; preview memblokir kesiapan, locking menolak stage assessment non-FINAL. Advisory lock tahun dipakai bersama Master Return. |
| Ranking aggregation | Agregasi stage menggunakan workflow_status FINAL; skor yang dikembalikan tidak masuk hasil final. |
| Export | Export histori memakai query event yang sama; export monitoring/ranking tidak membaca tiga kolom baru melalui alias subquery. |

Tidak ditemukan projection/alias/Scan mismatch tambahan pada jalur yang diaudit.
Ini pemeriksaan source, bukan bukti eksekusi seluruh endpoint produksi. Logika
bobot bulanan, Juli/Agustus tanpa laporan, September, tahap lapangan/presentasi,
Ter Gagal, Ter Inovatif, validasi, serta delegasi tidak diubah.

## Pengujian

Test PostgreSQL memanggil repository/service produksi, bukan salinan query.
Fixture memakai tabel TEMP, satu koneksi, dan search_path pg_temp; data existing
tidak disentuh. Tetap gunakan database khusus test.

- Sembilan kasus query: normal, returned Koor, returned Cabdin, returned Master,
  semua notes NULL, assessment tidak ada, legacy bersama stage, WAITING_CABDIN,
  dan FINAL dengan catatan lama. Memeriksa nilai, catatan, ID, serta isolasi assessor.
- Lifecycle FINAL → RETURNED_BY_MASTER → dashboard menampilkan alasan/nilai lama
  → resubmit WAITING_KOOR → approve Koor WAITING_CABDIN → approve Cabdin FINAL.
  Memeriksa rubric, final score, sinkronisasi legacy setelah final, dan empat audit event.
- Schema siap diterima; constraint/kolom hilang ditolak.

Hasil lokal pada 2 Oktober 2026:

| Command | Hasil |
| --- | --- |
| `cd backend; go test ./...` | Lulus; integration test yang memerlukan URL database di-skip. |
| `npm run lint` | Gagal: 12 error dan 45 warning di repository, termasuk no-explicit-any, no-unescaped-entities, dan set-state-in-effect. |
| `npx eslint components/gaspol/GaspolDashboard.tsx` | Lulus tanpa output. |
| `npx tsc --noEmit` | Lulus. |
| `npm run build` | Lulus; 156 halaman selesai diproses. |
| `git diff --check` | Lulus. |

Test integrasi baru **SKIP**, bukan berhasil mengeksekusi query: environment belum
menyediakan GASPOL_TEST_DATABASE_URL. `psql`/`postgres` tidak ditemukan pada PATH.
Percobaan `docker info` gagal: `open //./pipe/docker_engine: The system cannot find
the file specified` (daemon tidak aktif); juga ada warning akses docker/config.json.

Untuk menjalankan test integrasi, set GASPOL_TEST_DATABASE_URL ke PostgreSQL khusus
test melalui environment, kemudian dari folder backend:

```powershell
go test ./internal/services -run 'Test(ListStageTasksByAssessorPostgres|MasterReturnLifecyclePostgres|GaspolMasterReturnSchemaPostgres)' -v -count=1
go test ./...
```

## Deploy Zeabur

Deploy dan verifikasi produksi belum dilakukan dalam task lokal ini.

1. Jalankan test PostgreSQL di atas pada database test sebelum rilis.
2. Deploy backend dari folder `backend`/Dockerfile yang memang digunakan service.
   Jika build context root, repository menyediakan `Dockerfile.backend`; jika context
   backend, gunakan `backend/Dockerfile`. Keduanya menyalin migrations ke `/app/db/migrations`.
   Jangan menggunakan salinan lama `components/backend` untuk perbaikan ini.
3. Pastikan backend memakai database target yang benar, `APP_ENV=production`,
   `AUTO_MIGRATE=true`, dan `MIGRATIONS_DIR=/app/db/migrations` sesuai image.
   Startup menjalankan goose Up sebelum schema check. Jangan menjalankan skrip dump/restore/reset.
4. Periksa log `[migrate] migrations complete` dan GASPOL `action=schema_check
   status=ready required_migration=065`. Jika schema_check gagal, benahi migrasi/schema;
   jangan mengaktifkan fallback atau mengembalikan fitur Master Return.
5. Deploy frontend untuk memperbarui label. Periksa backend sehat sebelum membuka
   kembali alur pengawas.

Pemeriksaan read-only di database target melalui konsol SQL:

```sql
SELECT version_id,is_applied,tstamp
FROM goose_db_version WHERE version_id=65 ORDER BY id DESC LIMIT 1;

SELECT attname,format_type(atttypid,atttypmod)
FROM pg_attribute
WHERE attrelid='gaspol_stage_assessments'::regclass AND NOT attisdropped
  AND attname IN ('master_return_notes','returned_by_master_user_id','master_returned_at');

SELECT conrelid::regclass,conname,pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid IN ('gaspol_stage_assessments'::regclass,'gaspol_assessments'::regclass)
  AND conname IN ('gaspol_stage_assessments_workflow_status_check','gaspol_assessments_workflow_status_check');
```

Harus terlihat versi 65 applied, tiga kolom, serta kedua CHECK menerima status Master.
Schema check memverifikasi schema nyata; riwayat goose tetap perlu diperiksa operator.

## Verifikasi setelah deploy

1. Pastikan migration terbaru applied dan schema_check siap.
2. Login akun Pengawas, buka GASPOL; pesan "Gagal memuat dashboard pengawas GASPOL."
   tidak muncul dan request dashboard mengembalikan 200.
3. Periksa RETURNED_BY_MASTER: label "Dikembalikan Master - perlu review", alasan
   Master, nilai lama, dan tombol review/kirim ulang tampil.
4. Periksa RETURNED_BY_KOOR dan RETURNED_BY_CABDIN: label perbaikan dan alasan masing-masing benar.
5. Periksa WAITING_KOOR, WAITING_CABDIN, serta FINAL tetap tampil sesuai alur.
6. Pada data uji yang disepakati, jalankan lifecycle lengkap hingga FINAL kembali.
   Cek histori Master Return/resubmit/approve dan preview/Top 10 selama status belum final.
7. Smoke test bulanan (termasuk Juli/Agustus tanpa laporan dan September), Lapangan
   Tahap 1/2, Presentasi, Ter Gagal, Ter Inovatif, export, dan delegasi beberapa sekolah.
8. Pastikan tidak ada log `column sa.* does not exist` atau Scan error.
   `action=schema_check` gagal menunjukkan masalah schema/koneksi saat startup;
   `action=supervisor_dashboard` dengan konteks ListStageTasksByAssessor setelah
   schema siap mengarahkan investigasi ke query/alias (juga periksa perubahan schema pasca-startup).
