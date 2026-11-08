# Cabdin Bangkalan Dashboard

Repository ini berisi:

- Frontend Next.js di root (`app/`, `components/`, dll)
- Backend Go Gin di `backend/`

Frontend hanya merender UI dan mengambil data dari backend via Next.js `/api` proxy. Base URL backend diset di `BACKEND_URL`. Tidak ada pembacaan CSV di runtime.

## Struktur

- `/app` + `/components`: Frontend Next.js (App Router)
- `/backend`: Go Gin API + migrasi
- `/docker-compose.yml`: Postgres lokal
- `/.env.example`: contoh env frontend + backend

## Local Setup

1. Copy env:

```bash
cp .env.example .env
```

2. Jalankan Postgres:

```bash
docker compose up -d
```

3. Backend (Go Gin):

Pastikan `MASTER_ENCRYPTION_KEY` di `.env` sudah diisi (32 bytes, base64 atau hex).

```bash
cd backend
# install dependencies
go mod tidy

# install goose (if not installed)
go install github.com/pressly/goose/v3/cmd/goose@latest

# migrate
GOOSE_DRIVER=postgres GOOSE_DBSTRING="$DATABASE_URL" goose -dir migrations up
# Migrasi 003_cleanup_npsn.sql akan menghapus sekolah tanpa NPSN valid
# dan menambahkan constraint NOT NULL + minimum length.

# run server
GOOSE_DRIVER=postgres GOOSE_DBSTRING="$DATABASE_URL" go run ./cmd/server
```

4. Frontend (Next.js):

```bash
npm install
npm run dev
```

Frontend memanggil API via Next.js `/api` proxy yang meneruskan request ke backend lewat `BACKEND_URL`.

## One Command (Dev All)

Jalankan semua (DB + migrate + backend + frontend):

```bash
npm run dev:all
```

Jika port frontend bentrok, set `FRONTEND_PORT` di `.env` (default 3000).

Windows PowerShell:

```powershell
npm run dev:all
```

Mac/Linux:

```bash
npm run dev:all
```

Stop Docker:

```bash
docker compose down
```

## Akun Default (migrasi)

Akun default dibuat lewat migrasi `backend/migrations/012_seed_default_users.sql`.

- ADMIN:
  - username: `admin-cabdin`
  - password: `123456`
- SCHOOL_ADMIN:
  - username: `admin-SMAN1bangkalan`
  - password: `sman1bangkalan`

## Endpoint Backend

Auth & Dashboard
- `POST /auth/login` -> token + user
- `POST /auth/logout` -> logout success
- `GET /auth/me` -> user
- `GET /api/dashboard` -> ringkasan dashboard

Sekolah & Admin
- `GET /schools` / `GET /api/sekolah` -> list sekolah (ter-scope role)
- `PUT /schools/:id` -> update sekolah
- `GET /admin/schools` -> list sekolah (ADMIN/ADMIN_MASTER)
- `GET /admin/school-admins` -> list admin sekolah + is_online
- `POST /admin/school-admins` -> buat user SCHOOL_ADMIN
- `PUT /admin/school-admins/:id` -> update username sekolah admin
- `PUT /admin/school-admins/:id/password` -> reset password sekolah admin
- `POST /admin/school-admins/bulk-generate` -> generate massal akun sekolah
- `GET /api/admin/master/cabdin-admins` -> list admin cabdin (ADMIN_MASTER)
- `POST /api/admin/master/cabdin-admins` -> create admin cabdin (ADMIN_MASTER)
- `PATCH /api/admin/master/cabdin-admins/:id` -> update allowed_levels admin cabdin (ADMIN_MASTER)

Prestasi
- `GET /achievements` / `GET /api/prestasi` -> list prestasi
- `GET /api/prestasi/list` -> table list
- `GET /api/prestasi/analytics` -> analytics
- `POST /api/prestasi/import/validate`
- `POST /api/prestasi/import/commit`
- `GET /api/prestasi/export.xlsx`
- `PUT /api/prestasi/:id`

Siswa Perguruan Tinggi (legacy)
- `GET /admissions` / `GET /api/siswa-perguruan-tinggi`
- `GET /api/siswa-perguruan-tinggi/list`
- `GET /api/siswa-perguruan-tinggi/summary`
- `GET /api/siswa-perguruan-tinggi/pt-summary`
- `GET /api/siswa-perguruan-tinggi/export.xlsx`
- `PUT /api/siswa-perguruan-tinggi/:id`

Siswa Pipeline (Kelas Akhir)
- `POST /api/siswa/cohorts/import`
- `GET /api/siswa/cohorts/list`
- `POST /api/siswa/applications/import`
- `GET /api/siswa/applications/list`
- `POST /api/siswa/acceptance-offers/import`
- `POST /api/siswa/ukk/import`
- `POST /api/siswa/higher-ed/final/import`
- `POST /api/siswa/finalize`
- `GET /api/siswa/validation/pending`
- `GET /api/siswa/validation/summary`
- `GET /api/siswa/analytics/higher-ed`
- `GET /api/siswa/plans/list`
- `GET /api/siswa/plans/:id`
- `PUT /api/siswa/plans/:id`
- `GET /api/siswa/plans/export.xlsx`
- `GET /api/siswa/import-reports/:id/rejected.xlsx`

Uploads & Histori
- `POST /uploads` / `POST /api/uploads` -> import prestasi / siswa PT (SCHOOL_ADMIN saja)
- `GET /uploads` / `GET /api/uploads` -> list uploads
- `GET /api/uploads/:id/rejected.xlsx` -> rejected report
- `GET /api/histori` -> histori import (filter module/date)
- `DELETE /api/histori/:id` -> hapus batch import (ADMIN_MASTER/SCHOOL_ADMIN)

## Role & Data Isolation

- `SCHOOL_ADMIN` hanya bisa melihat data untuk `school_id` miliknya, baik di UI maupun API.
- Semua endpoint data (`/schools`, `/achievements`, `/admissions`, `/uploads`, `/api/siswa/*`) otomatis di-scope oleh `school_id` dari token.
- Jika `SCHOOL_ADMIN` mengirim `school_id` berbeda (query/body), API akan mengembalikan `403 { "error": "FORBIDDEN" }`.
- `ADMIN` melihat data sesuai `allowed_levels` (SMA/SMK/SLB) dan bisa filter via query `school_id`, `jenjang`, atau `tahun`.
- `ADMIN_MASTER` akses penuh dan dapat mengelola akun `ADMIN` + `allowed_levels`.
- Error format konsisten: `401 { "error": "UNAUTHORIZED" }`, `403 { "error": "FORBIDDEN" }`, `400 { "error": "BAD_REQUEST", "details": ... }`, `500 { "error": "INTERNAL_SERVER_ERROR" }`.

### Cara test manual (anti data leak)

1. Login sebagai `SCHOOL_ADMIN`.
2. Coba request data sekolah lain, contoh:
   - `GET /achievements?school_id=2`
   - `GET /uploads?school_id=2`
3. Pastikan API mengembalikan `403 { "error": "FORBIDDEN" }` dan UI tidak menampilkan dropdown sekolah.

## Auto Logout 10 Menit (Idle)

- Aktivitas yang dihitung: `mousemove`, `mousedown`, `click`, `scroll`, `keydown`, `touchstart`.
- Jika idle > 10 menit -> token dihapus dan user diarahkan ke `/login`.
- Logout sinkron antar tab via localStorage broadcast key.
- JWT backend expired setelah 10 menit (exp di token).

### Cara test manual (idle/logout)

1. Login, diam > 10 menit -> otomatis logout dan redirect `/login`.
2. Login, lakukan aktivitas (scroll/klik) -> tidak logout.
3. Login di 2 tab -> logout di tab A -> tab B ikut logout.
4. Paksa token expired (tunggu 10 menit) -> request API -> 401 -> redirect `/login`.

## User Settings & Admin Sekolah

- `SCHOOL_ADMIN` melihat data sekolah dalam mode read-only, klik Edit untuk ubah lalu Simpan/Batal.
- Endpoint sekolah admin:
  - `GET /user/settings` -> data sekolah + user login (tanpa password hash)
  - `PUT /user/settings` -> update data sekolah milik user
- Perubahan akun (username/password) wajib menyertakan `current_password` yang benar.
- Rate limit percobaan password lama: max 5 gagal dalam 10 menit per user.
- Error akun yang mungkin muncul:
  - `PASSWORD_LAMA_REQUIRED`
  - `PASSWORD_LAMA_SALAH`
  - `USERNAME_TAK_TERSEDIA`
  - `PASSWORD_MIN_8`
- `ADMIN` melihat tabel akun admin sekolah + status online/offline.
- Status online dihitung dari `last_seen_at` (aktif jika <= 10 menit).
- Endpoint admin:
  - `GET /admin/school-admins` -> list admin sekolah + is_online
  - `PUT /admin/school-admins/:id` -> update username/password
  - `GET /admin/schools` -> list sekolah (id, npsn, nama)
  - `POST /admin/school-admins` -> buat user SCHOOL_ADMIN dari sekolah
- `ADMIN_MASTER` dapat mengelola akun `ADMIN` cabdin dan `allowed_levels` via `/api/admin/master/cabdin-admins`.

## Import Data Sekolah (Excel)

- Sumber data: `backend/assets/data-sekolah.xlsx`.
- Data sekolah dimuat lewat migrasi `backend/migrations/011_seed_data_sekolah.sql`.
- Kolom yang dipakai: `NPSN`, `Nama Sekolah`/`Nama_Satuan_Pendidikan`, `Akreditasi`, `Guru`, `Tendik`.
- Field lain (bentuk_pendidikan/status_sekolah/tmt_akreditasi) akan kosong jika tidak ada di Excel.

## Data Source

- Semua halaman mengambil data dari database via API backend.
- Folder `/mocks` tidak dipakai untuk runtime.

## Upload Excel

Endpoint: `POST /uploads` (multipart/form-data, hanya `SCHOOL_ADMIN`)

- `file` (xlsx)
- `type` = `ACHIEVEMENT` | `ACCEPTANCE`
- `year` (int)
- `password` (password akun yang login)

File disimpan di storage adapter:

- Local: `./uploads/<school_id>/...`
- R2/Supabase: stub via env (belum diimplement)

## Storage Upload Lomba Hardiknas

- Upload poster Lomba Hardiknas disimpan relatif ke `STORAGE_LOCAL_DIR`.
- Data baru di DB hanya menyimpan relative path seperti:
  - `lomba-hardiknas/original/<filename>`
  - `lomba-hardiknas/thumbnail/<filename>`
- Saat file dibaca, backend me-resolve path relatif tersebut ke `STORAGE_LOCAL_DIR`.
- Jika `STORAGE_LOCAL_DIR` kosong, backend fallback ke `./uploads` untuk local development dan menulis warning bahwa konfigurasi itu tidak aman untuk staging/production.

### Deploy staging/production

- Gunakan env:
  - `STORAGE_LOCAL_DIR=/data/uploads`
- Path `/data/uploads` harus di-mount sebagai persistent volume di platform deploy, termasuk Zeabur.
- Jangan gunakan `/app/uploads` atau `./uploads` untuk upload production/staging karena isi container bisa hilang saat redeploy/restart.

### Checklist verifikasi deploy

1. Upload 1 poster Lomba Hardiknas.
2. Pastikan preview poster bisa dibuka.
3. Redeploy service backend.
4. Pastikan preview poster masih bisa dibuka.
5. Jika gagal setelah redeploy, persistent volume belum terpasang atau `STORAGE_LOCAL_DIR` belum diarahkan ke volume tersebut.

## Template Excel

Sheet pertama akan dibaca (case-insensitive).

### Generator template input data

Template input data (multi-sheet) dibuat via generator:

```bash
go run ./backend/cmd/gen-template/main.go
```

Generator membaca daftar kampus dari `backend/assets/kampus_indonesia.csv` (kolom `nama_kampus`) untuk dropdown.
Ganti file CSV tersebut dengan daftar kampus yang valid jika diperlukan.
Nilai dropdown mengikuti ENUM backend dan harus uppercase.
Contoh input valid: `DARING`, `PERSEORANGAN`, `NEGERI`, `SNBP`, `LKSN`.

### Download & Import (SCHOOL_ADMIN)

- Template Prestasi: `GET /templates/achievement` (sheet: `Achievement` saja)
- Template Siswa PT: `GET /templates/acceptance` (sheet: `Siswa_Perguruan_Tinggi` + `_ref` hidden)
- Template Siswa Kelas Akhir: `GET /templates/siswa-cohorts` (sheet: `Siswa_Kelas_Akhir`)
- Template Siswa Pendaftaran: `GET /templates/siswa-applications` (sheet: `Pendaftaran_Jalur`)
- Import prestasi: `POST /uploads` (multipart/form-data: `file`, `type=ACHIEVEMENT`, `year`, `password`)
- Import siswa PT: `POST /uploads` (multipart/form-data: `file`, `type=ACCEPTANCE`, `year`, `password`)
- Import siswa kelas akhir: `POST /api/siswa/cohorts/import` (multipart/form-data: `file`)
- Import pendaftaran: `POST /api/siswa/applications/import` (multipart/form-data: `file`)
- Import diterima PT: `POST /api/siswa/acceptance-offers/import` (multipart/form-data: `file`)
- Import UKK: `POST /api/siswa/ukk/import` (multipart/form-data: `file`)
- Import final historis: `POST /api/siswa/higher-ed/final/import` (multipart/form-data: `file`)

Catatan: `POST /api/siswa/cohorts/import` dan `POST /api/siswa/applications/import` hanya untuk `SCHOOL_ADMIN`. Untuk `ADMIN`/`ADMIN_MASTER`, endpoint import siswa lainnya mewajibkan `school_id` di form-data.

## Siswa Pipeline (Kelas Akhir)

- Cohorts: data siswa kelas akhir (`class12_students`).
- Applications: pendaftaran PT (maks 3 pilihan per siswa).
- Acceptance offers: siswa diterima PT.
- UKK: hasil uji kompetensi (jenis `UTAMA` / `TAMBAHAN`).
- Final outcomes: finalisasi pilihan diterima.
- Plans: rencana lulusan (`PT` / `KERJA` / `WIRAUSAHA`).

Validasi kunci:
- Import applications/offers/UKK/final ditolak bila data cohort belum ada.
- NISN wajib numerik; `UTAMA` hanya satu per siswa per tahun lulus; SNBP tidak boleh dicampur SNBT.

## Upload Validation & Duplicate Handling

- Achievement dianggap duplikat jika semua field kunci sama (kategori, jenjang, tingkat, tahun, nama lomba, peringkat, mode, penyelenggara, jenis peserta, nama peserta) dalam sekolah yang sama.
- Acceptance dianggap duplikat jika `NISN_Siswa` sama dalam sekolah yang sama.
- Baris bermasalah tidak diinsert dan masuk daftar rejected.
- Format response upload `/uploads`:
  ```
  {
    "status": "SUCCESS" | "SUCCESS_WITH_REJECTIONS",
    "totalRows": number,
    "inserted": number,
    "rejected": number,
    "rejectedReportUrl": "/api/uploads/:id/rejected.xlsx",
    "rejections": [
      { "rowNumber": number, "reason": "...", "notes": "..." }
    ]
  }
  ```
- Format response import siswa `/api/siswa/*`:
  ```
  {
    "total_rows": number,
    "accepted": number,
    "rejected": number,
    "rejected_excel_url": "/api/siswa/import-reports/:id/rejected.xlsx",
    "rejections": [
      { "row_number": number, "reason": "..." }
    ]
  }
  ```

### Prestasi Siswa (ACHIEVEMENT)

Header harus sesuai template (urutan persis):

- `ID_Prestasi`
- `Kategori_Prestasi`
- `Tingkat_Lomba`
- `Tahun_Lomba`
- `Nama_Lomba`
- `Peringkat_Prestasi`
- `Mode_Pelaksanaan`
- `Penyelenggara_Lomba`
- `Jenis_Peserta`
- `Jenis_Lomba`
- `Nama_Peserta`
- `Link_Gdrive`

### Siswa Perguruan Tinggi (ACCEPTANCE)

Header harus sesuai template (urutan persis):

- `ID`
- `NISN_Siswa`
- `Nama_Siswa`
- `Rombel_Siswa`
- `Tahun_Lulus`
- `Nama_Perguruan_Tinggi`
- `Jurusan_Perguruan_Tinggi`
- `Tahun_Diterima`
- `Jalur_Penerimaan`

NPSN/Nama sekolah tidak perlu di template (diambil dari user login).

## Logout

Klik tombol logout di sidebar. Token akan dihapus dari browser dan user diarahkan ke `/login`.

## Environment Variables

Frontend:

- `BACKEND_URL` (server-only, untuk Next.js `/api` proxy)
- `NEXT_PUBLIC_APP_NAME` (opsional)

Backend:

- `DATABASE_URL`
- `JWT_SECRET`
- `MASTER_ENCRYPTION_KEY` (required, 32 bytes base64/hex)
- `PORT`
- `STORAGE_PROVIDER`
- `STORAGE_LOCAL_DIR` (`./uploads` hanya aman untuk local dev; staging/production harus memakai persistent volume seperti `/data/uploads`)
- `INIT_ADMIN_MASTER_USERNAME`, `INIT_ADMIN_MASTER_PASSWORD`, `INIT_ADMIN_MASTER_NAMA_INSTANSI` (opsional bootstrap ADMIN_MASTER)
- `INITIAL_PASSWORD_ENCRYPTION_KEY` (legacy, dipakai jika `MASTER_ENCRYPTION_KEY` kosong)
- `R2_*` / `SUPABASE_*` placeholder

## Deploy

Backend (Render/Railway/Fly):

- Set env `DATABASE_URL`, `JWT_SECRET`, `MASTER_ENCRYPTION_KEY`, `PORT`, `STORAGE_PROVIDER`, `STORAGE_LOCAL_DIR`
- Untuk staging/production, set `STORAGE_LOCAL_DIR=/data/uploads` dan mount `/data/uploads` sebagai persistent volume
- Opsional: `INIT_ADMIN_MASTER_*` untuk bootstrap akun ADMIN_MASTER pertama
- Jalankan migrasi: `goose -dir migrations up`

Frontend (Vercel):

- Set env `BACKEND_URL`
- Deploy Next.js seperti biasa
