# Pengaduan dan backup SIPRES

## Konfigurasi production

- Mount persistent volume pada nilai `STORAGE_LOCAL_DIR` (disarankan `/data/uploads`). Modul pengaduan menambah subfolder `pengaduan-kritik-saran`; tidak ada file lama yang dipindah atau dimigrasikan.
- Isi `TURNSTILE_SECRET_KEY` hanya di backend dan `NEXT_PUBLIC_TURNSTILE_SITE_KEY` di frontend untuk melindungi submit publik.
- Isi `BACKUP_ENCRYPTION_KEY` dengan key acak 32 byte dalam base64 atau hex. Simpan salinan key di password manager/offline, terpisah dari server dan backup. Kehilangan key membuat backup mustahil dipulihkan.
- Isi `APP_VERSION` dengan commit/tag deployment bila tersedia.
- Runtime backend wajib menyediakan `pg_dump` dan `pg_restore`. Dockerfile backend telah menambahkan PostgreSQL client.

## Membuat backup

Login sebagai `ADMIN_MASTER`, buka **Backup & Pemulihan Data**, lalu pilih **Buat & Unduh Backup**. Server membuat PostgreSQL custom dump, mengarsipkan storage beserta manifest/checksum, mengenkripsinya secara chunked dengan AES-256-GCM, dan mengirim file `.sipres-backup`.

Riwayat baru ditandai sukses setelah respons selesai dikirim. Server tidak dapat membuktikan browser benar-benar menyimpan file tersebut. Tidak ada secret environment, key, cache, log, atau backup sementara yang dimasukkan. Temporary directory selalu dijadwalkan untuk dibersihkan.

## Memeriksa dan memulihkan

Jalankan hanya pada server baru atau mode maintenance:

```sh
export BACKUP_ENCRYPTION_KEY='...'
export DATABASE_URL='postgresql://...'
export STORAGE_LOCAL_DIR='/data/uploads'
go run ./cmd/restore-backup --dry-run /path/SIPRES-BACKUP....sipres-backup
go run ./cmd/restore-backup /path/SIPRES-BACKUP....sipres-backup
```

Dry-run memverifikasi format, autentikasi enkripsi, dan keamanan seluruh path arsip. Restore menolak storage yang tidak kosong; `--force` hanya boleh dipakai setelah target dan backup diverifikasi. Setelah restore, jalankan migrasi Goose aplikasi dan health check. Jangan menjalankan restore terhadap database development/production yang berisi data tanpa salinan dan lingkungan uji terisolasi.

## Catatan keamanan

PIN pelacakan hanya disimpan sebagai bcrypt hash. Lampiran diperiksa ukuran, ekstensi, MIME/signature, diberi nama acak, disimpan mode privat, dan tidak memiliki URL publik langsung. Catatan internal tidak disertakan dalam respons tracking. RBAC admin diterapkan pada handler backend.
