-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS guide_books (
  id BIGSERIAL PRIMARY KEY,
  role TEXT NOT NULL CHECK (role IN ('ALL', 'SCHOOL_ADMIN', 'ADMIN', 'ADMIN_MASTER')),
  title TEXT NOT NULL,
  content_html TEXT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS guide_books_role_is_active_idx
  ON guide_books (role, is_active);

CREATE INDEX IF NOT EXISTS guide_books_sort_order_idx
  ON guide_books (sort_order);

CREATE TABLE IF NOT EXISTS guide_book_media (
  id BIGSERIAL PRIMARY KEY,
  file_name TEXT NOT NULL,
  mime TEXT NOT NULL,
  size INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO guide_books (role, title, content_html, sort_order, is_active)
VALUES
  (
    'ALL',
    'Aturan Sesi, Import, dan Status Data',
    $$<h3>Aturan Sesi</h3><ul><li>Sistem melakukan idle logout setelah 10 menit tidak ada aktivitas.</li><li>Status login sinkron antar-tab browser, jadi logout di satu tab akan ikut menutup sesi di tab lain.</li></ul><h3>Import Excel</h3><ul><li>Header file harus sama persis dengan template resmi.</li><li>Jika ada mismatch header, import akan ditolak agar struktur data tetap konsisten.</li></ul><h3>Pending vs Final</h3><ul><li><strong>Pending</strong>: data masih perlu verifikasi / perbaikan.</li><li><strong>Final</strong>: data sudah diverifikasi dan siap dipakai untuk rekap/audit.</li></ul><h3>Empty State</h3><p>Jika data kosong, halaman akan menampilkan state kosong (bukan error 500). Cek filter, tahun, dan scope sekolah sebelum melakukan import ulang.</p>$$,
    10,
    TRUE
  ),
  (
    'SCHOOL_ADMIN',
    'Panduan Sekolah: Roster Kelas XII, Rencana Lulusan, dan QC',
    $$<h3>Roster Kelas XII (Step-by-Step)</h3><ol><li>Unduh template roster kelas akhir.</li><li>Isi data sesuai format dan header template.</li><li>Import file melalui menu terkait.</li><li>Baca file rejection untuk baris yang ditolak, perbaiki, lalu import ulang.</li></ol><h3>Rencana Lulusan</h3><p>Isi data per tab jalur (PT, kerja, wirausaha, belum melanjutkan, dan jalur lain yang tersedia) sesuai kondisi siswa terbaru.</p><h3>Checklist QC Sekolah</h3><ul><li>NISN unik dan valid.</li><li>Tahun lulus benar.</li><li>Jalur rencana/pengumuman konsisten.</li><li>Lakukan finalisasi setelah verifikasi internal selesai.</li></ul><h3>Catatan UKK (SMK)</h3><p>Khusus SMK, pastikan data UKK dan keterkaitan status lulusan sudah sinkron sebelum finalisasi.</p>$$,
    20,
    TRUE
  ),
  (
    'ADMIN',
    'Panduan Cabdin: Monitoring, Audit, dan Troubleshooting',
    $$<h3>Monitoring &amp; Audit Cabdin</h3><ul><li>Gunakan dashboard dan histori import untuk memantau progres per sekolah.</li><li>Fokuskan review pada sekolah dengan rejection tinggi atau data pending yang tidak turun.</li></ul><h3>Interpretasi Pending</h3><p>Status pending menunjukkan data belum final/masih perlu tindakan. Audit berdasarkan jenis penolakan dan konsistensi antar modul.</p><h3>Troubleshooting Umum</h3><ul><li>Cek template/header saat import gagal.</li><li>Cek filter tahun/scope saat data terlihat kosong.</li><li>Cek status finalisasi bila data tidak bisa diubah.</li></ul>$$,
    30,
    TRUE
  ),
  (
    'ADMIN_MASTER',
    'Panduan Admin Master: Role, Operasional, dan Release QC',
    $$<h3>Manajemen Role &amp; Scope</h3><ul><li>Atur role user sesuai kewenangan (SCHOOL_ADMIN, ADMIN, ADMIN_MASTER).</li><li>Pastikan scope sekolah/level sesuai kebutuhan operasional.</li></ul><h3>Operasional Teknis</h3><ul><li>Monitor health aplikasi, status migrasi, dan storage path persistent.</li><li>Verifikasi konfigurasi environment sebelum deploy.</li></ul><h3>Checklist Release / QC</h3><ul><li>Uji login + RBAC endpoint utama.</li><li>Uji import template dan rejection report.</li><li>Uji gambar/panduan protected endpoint dengan session aktif.</li><li>Review empty state agar tidak menampilkan 500.</li></ul>$$,
    40,
    TRUE
  );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS guide_book_media;
DROP TABLE IF EXISTS guide_books;
-- +goose StatementEnd
