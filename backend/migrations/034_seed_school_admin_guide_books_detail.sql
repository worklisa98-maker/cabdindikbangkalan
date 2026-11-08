-- +goose Up
-- +goose StatementBegin
UPDATE guide_books
SET is_active = FALSE,
    updated_at = NOW()
WHERE role = 'SCHOOL_ADMIN'
  AND title = 'Panduan Sekolah: Roster Kelas XII, Rencana Lulusan, dan QC';

CREATE TEMP TABLE tmp_guide_book_seed_school_admin (
  role TEXT NOT NULL,
  title TEXT NOT NULL,
  content_html TEXT NOT NULL,
  sort_order INT NOT NULL,
  is_active BOOLEAN NOT NULL
) ON COMMIT DROP;

INSERT INTO tmp_guide_book_seed_school_admin (role, title, content_html, sort_order, is_active)
VALUES
(
  'SCHOOL_ADMIN',
  'Siswa: Roster Kelas XII (Import, Update, Rejection)',
  $g1$
<h3>Tujuan Modul</h3>
<p><strong>Roster Kelas XII</strong> adalah data dasar siswa untuk pengisian <strong>Rencana Lulusan</strong>, status lulusan, dan rekap. Rapikan roster terlebih dahulu sebelum lanjut ke modul lain.</p>
<h3>Persiapan Sebelum Input</h3>
<ul>
  <li>Login sebagai <strong>admin-sekolah (SCHOOL_ADMIN)</strong> pada sekolah yang benar.</li>
  <li>Tentukan <strong>Tahun Lulus</strong> yang sedang dikerjakan.</li>
  <li>Siapkan data siswa (NISN, nama, rombel, tahun lulus) dan gunakan template resmi dari sistem.</li>
  <li>Gunakan format spreadsheet yang menjaga NISN tetap utuh (hindari hilang leading zero).</li>
</ul>
<h3>Langkah-Langkah (Step-by-Step)</h3>
<ol>
  <li>Buka menu <strong>Siswa &rarr; Roster Kelas XII</strong>.</li>
  <li>Pilih <strong>Tahun Lulus</strong> yang benar.</li>
  <li>Klik <strong>Download Template</strong>.</li>
  <li>Isi file template:
    <ul>
      <li><strong>NISN</strong> sesuai data resmi, tanpa karakter tambahan.</li>
      <li><strong>Nama</strong> siswa dengan ejaan rapi/konsisten.</li>
      <li><strong>Rombel</strong> wajib diisi.</li>
      <li><strong>Tahun Lulus</strong> harus konsisten dengan filter.</li>
    </ul>
  </li>
  <li><strong>Jangan ubah header</strong> (nama kolom/urutan). Header mismatch akan ditolak.</li>
  <li>Import file Excel dari halaman roster.</li>
  <li>Baca hasil import: cek ringkasan <strong>insert</strong>, <strong>update</strong>, dan <strong>duplicate/ditolak</strong>.</li>
  <li>Jika ada <strong>rejection</strong>, unduh file rejection, baca kolom alasan, perbaiki baris terkait, lalu import ulang.</li>
</ol>
<h3>Checklist QC (Audit)</h3>
<ul>
  <li><strong>NISN unik per tahun lulus</strong>.</li>
  <li><strong>Tahun lulus konsisten</strong> antara file dan filter halaman.</li>
  <li><strong>Rombel tidak kosong</strong>.</li>
  <li><strong>Nama</strong> tidak mengandung spasi berlebih.</li>
</ul>
<h3>Kesalahan Umum &amp; Solusinya</h3>
<ul>
  <li><strong>Header mismatch ditolak</strong>: download template baru, salin data ke template itu, jangan ubah header.</li>
  <li><strong>NISN terasa satu tapi direject</strong>: cek duplikat pada <strong>tahun lulus yang sama</strong>, spasi tersembunyi, format string/angka berbeda, atau leading zero hilang.</li>
  <li><strong>Import sukses tapi data tidak muncul</strong>: cek filter <strong>Tahun Lulus</strong>, cek role/sekolah aktif, lalu reload halaman.</li>
</ul>
  $g1$,
  10,
  TRUE
),
(
  'SCHOOL_ADMIN',
  'Siswa: Rencana Lulusan (SNBP/SNBT/Mandiri/Kedinasan/Lainnya)',
  $g2$
<h3>Tujuan Modul</h3>
<p>Modul <strong>Rencana Lulusan</strong> mencatat rencana siswa per jalur. Satu siswa bisa memiliki isian di beberapa tab, dan <strong>Kedinasan</strong> diperlakukan sebagai jalur tersendiri.</p>
<h3>Persiapan Sebelum Input</h3>
<ul>
  <li>Pastikan siswa sudah ada di <strong>Roster Kelas XII</strong> untuk tahun lulus yang sama.</li>
  <li>Siapkan data pilihan kampus/instansi/jurusan dari siswa.</li>
  <li>Samakan ejaan nama kampus/instansi antar operator.</li>
</ul>
<h3>Langkah-Langkah (Step-by-Step)</h3>
<ol>
  <li>Masuk ke <strong>Siswa &rarr; Rencana Lulusan</strong>.</li>
  <li>Pilih siswa, lalu buka halaman/form <strong>kelola</strong>.</li>
  <li>Isi tab <strong>SNBP</strong> (contoh: pilihan PT 1-3 sesuai field yang tersedia).</li>
  <li>Isi tab <strong>SNBT</strong> (contoh: pilihan PT 1-2/3 sesuai field yang tersedia).</li>
  <li>Isi tab <strong>Mandiri</strong> sesuai pilihan siswa.</li>
  <li>Isi tab <strong>Kedinasan</strong>: isi instansi/sekolah kedinasan; jika ada field prodi/jurusan, isi juga. Jika tidak ada, cukup instansi + keterangan.</li>
  <li>Isi tab <strong>Lainnya</strong> (jika tersedia) untuk rencana di luar jalur utama.</li>
  <li><strong>Simpan tiap tab</strong> sebelum pindah tab lain.</li>
</ol>
<h3>Checklist QC (Audit)</h3>
<ul>
  <li>Jangan menaruh pilihan <strong>SNBP</strong> di tab <strong>SNBT</strong>, dan sebaliknya.</li>
  <li>Nama kampus/instansi konsisten ejaannya.</li>
  <li>Periksa siswa yang punya banyak jalur agar tidak tertukar.</li>
  <li><strong>Khusus SMK</strong>: pastikan data UKK terisi/selaras jika modul UKK tersedia.</li>
</ul>
<h3>Kesalahan Umum &amp; Solusinya</h3>
<ul>
  <li><strong>Data &ldquo;hilang&rdquo; setelah isi</strong>: pastikan tab sudah disimpan. Pahami beda <strong>draft/pending</strong> vs <strong>final</strong>; reload halaman bila perlu.</li>
  <li><strong>Siswa tidak ditemukan</strong>: cek roster dan filter tahun lulus aktif.</li>
  <li><strong>Tab jalur tertukar</strong>: buka lagi siswa, perbaiki tab yang benar, simpan ulang.</li>
</ul>
  $g2$,
  20,
  TRUE
),
(
  'SCHOOL_ADMIN',
  'Siswa: Finalisasi, Pending, dan Cara Validasi Data',
  $g3$
<h3>Tujuan Modul</h3>
<p>Panduan ini menjelaskan <strong>Pending</strong> vs <strong>Final</strong>, cara finalisasi, dan validasi data agar <strong>Status Lulusan</strong> serta rekap memakai hasil yang benar.</p>
<h3>Definisi Status</h3>
<ul>
  <li><strong>Pending / Pending Validasi</strong>: belum final, masih menunggu validasi/perbaikan sekolah.</li>
  <li><strong>Final</strong>: hasil akhir siswa yang dipakai untuk rekap.</li>
</ul>
<h3>Persiapan Sebelum Finalisasi</h3>
<ul>
  <li>Pastikan data <strong>Rencana Lulusan</strong> sudah benar.</li>
  <li>Cek <strong>tahun lulus</strong> dan siswa yang akan difinalkan.</li>
  <li>Pastikan sudah ada bukti/konfirmasi internal sekolah.</li>
</ul>
<h3>Langkah-Langkah Finalisasi (Step-by-Step)</h3>
<ol>
  <li>Review data rencana siswa.</li>
  <li>Klik tombol <strong>Finalisasi</strong> (atau tombol setara di UI).</li>
  <li>Pilih <strong>jalur final</strong> (misal: SNBP/SNBT/Mandiri/Kedinasan/Lainnya).</li>
  <li>Konfirmasi tindakan.</li>
  <li>Pastikan <strong>Status Lulusan</strong> berubah dan status tampil sesuai daftar/filter.</li>
</ol>
<h3>Batalkan Rencana / Koreksi</h3>
<ul>
  <li>Jika tersedia, gunakan opsi <strong>batal finalisasi</strong> / <strong>batalkan rencana</strong> untuk koreksi.</li>
  <li><strong>Jangan reset/menghapus histori lama</strong> secara manual; gunakan fitur pembatalan bila ada.</li>
  <li>Setelah koreksi, simpan data yang benar lalu finalisasi ulang bila diperlukan.</li>
</ul>
<h3>Checklist QC (Audit)</h3>
<ul>
  <li>Final hanya setelah bukti/konfirmasi tersedia.</li>
  <li>Cek ulang tahun lulus dan siswa yang benar.</li>
  <li>Pastikan jalur final sesuai hasil akhir yang akan direkap.</li>
</ul>
<h3>Kesalahan Umum &amp; Solusinya</h3>
<ul>
  <li><strong>Final sudah tersimpan tapi status belum muncul</strong>: cek filter tahun, reload halaman, cek daftar Pending/Final sesuai scope.</li>
  <li><strong>Halaman kosong</strong>: jika data belum ada, seharusnya <strong>empty state</strong> (bukan error). Laporkan bila muncul error 500.</li>
  <li><strong>Salah finalisasi</strong>: gunakan pembatalan/perubahan status (jika tersedia), lalu finalisasi ulang.</li>
</ul>
  $g3$,
  30,
  TRUE
),
(
  'SCHOOL_ADMIN',
  'Prestasi: Input, Edit, dan QC',
  $g4$
<h3>Tujuan Modul</h3>
<p>Modul <strong>Prestasi</strong> digunakan untuk pencatatan prestasi siswa yang rapi, terverifikasi, dan siap direkap.</p>
<h3>Persiapan Sebelum Input</h3>
<ul>
  <li>Pastikan siswa sudah terdaftar di roster dan tahun/filter yang dipilih benar.</li>
  <li>Siapkan detail prestasi: siswa, jenis, tingkat, peringkat, nama ajang, tanggal.</li>
  <li>Siapkan file bukti (jika ada) atau keterangan yang jelas.</li>
</ul>
<h3>Langkah-Langkah Input Prestasi (Step-by-Step)</h3>
<ol>
  <li>Masuk menu <strong>Prestasi</strong>.</li>
  <li>Pilih <strong>tahun</strong>.</li>
  <li>Klik <strong>Tambah Prestasi</strong>.</li>
  <li>Isi: siswa, jenis, tingkat, peringkat, nama ajang, tanggal.</li>
  <li>Upload <strong>bukti</strong> (jika ada), atau isi <strong>keterangan</strong> jika bukti belum tersedia.</li>
  <li>Klik <strong>Simpan</strong> dan cek entri muncul di daftar.</li>
</ol>
<h3>Langkah-Langkah Edit / Hapus</h3>
<ol>
  <li>Buka daftar prestasi pada tahun yang sesuai.</li>
  <li>Cari entri yang akan diperbaiki.</li>
  <li>Klik <strong>Edit</strong>, ubah data, lalu simpan.</li>
  <li>Jika entri duplikat/tidak valid, gunakan aksi <strong>Hapus</strong> (sesuai kewenangan/UI yang tersedia), lalu cek daftar lagi.</li>
</ol>
<h3>Checklist QC (Audit)</h3>
<ul>
  <li><strong>Tahun prestasi</strong> benar.</li>
  <li><strong>Siswa terdaftar di roster</strong>.</li>
  <li><strong>Tingkat</strong> dan <strong>peringkat</strong> konsisten dengan bukti.</li>
  <li>Hindari <strong>duplikasi entri</strong>.</li>
</ul>
<h3>Kesalahan Umum &amp; Solusinya</h3>
<ul>
  <li><strong>Siswa tidak muncul</strong>: cek roster dan filter tahun.</li>
  <li><strong>Data tidak tersimpan</strong>: cek kemungkinan <strong>idle logout 10 menit</strong>, login ulang, lalu simpan kembali.</li>
  <li><strong>Bukti belum ada</strong>: simpan dengan keterangan (jika alur UI mengizinkan), lalu update saat bukti tersedia.</li>
</ul>
  $g4$,
  40,
  TRUE
),
(
  'SCHOOL_ADMIN',
  'Troubleshooting Operator Sekolah (Kasus Umum)',
  $g5$
<h3>Tujuan Modul</h3>
<p>Ringkasan masalah umum operator sekolah dan langkah cek awal sebelum eskalasi/lapor bug.</p>
<h3>Persiapan Sebelum Troubleshooting</h3>
<ul>
  <li>Catat menu, tahun/filter aktif, dan langkah terakhir sebelum masalah muncul.</li>
  <li>Pastikan koneksi stabil dan sesi login masih aktif.</li>
  <li>Reload halaman sekali untuk memastikan tampilan terbaru.</li>
</ul>
<h3>Langkah Cek Cepat (Kasus Umum)</h3>
<ol>
  <li><strong>Idle logout 10 menit + sinkron antar tab</strong>: sesi akan logout otomatis setelah idle. Logout/login di satu tab ikut tersinkron ke tab lain.</li>
  <li><strong>Import mismatch</strong>: header harus sama persis dengan template; download template terbaru lalu import ulang.</li>
  <li><strong>Rejection import</strong>: unduh file rejection, baca alasan per baris, perbaiki sumber data, lalu import ulang.</li>
  <li><strong>Data tidak muncul</strong>: cek filter tahun, pencarian, role/sekolah aktif, lalu reload.</li>
  <li><strong>Pending vs Final</strong>: pastikan Anda melihat daftar/status yang sesuai tujuan pengecekan.</li>
</ol>
<h3>Checklist QC Sebelum Melapor</h3>
<ul>
  <li>Sudah cek filter tahun dan reload halaman.</li>
  <li>Sudah cek sesi login (idle logout).</li>
  <li>Sudah cek file rejection bila kasus import.</li>
  <li>Sudah pastikan itu bukan <strong>empty state</strong> normal.</li>
</ul>
<h3>Kesalahan Umum &amp; Solusinya</h3>
<ul>
  <li><strong>Data hilang setelah pindah tab</strong>: pastikan tiap tab/form sudah disimpan.</li>
  <li><strong>Simpan tidak bereaksi</strong>: cek sesi login, lalu ulangi setelah login kembali.</li>
  <li><strong>Halaman kosong/error</strong>: bedakan empty state vs error; jika error (mis. 500) laporkan detailnya.</li>
</ul>
<h3>Cara Melaporkan Bug</h3>
<ul>
  <li>Sertakan <strong>screenshot</strong> (tanpa data sensitif berlebihan).</li>
  <li>Sertakan <strong>NISN</strong> terkait bila perlu.</li>
  <li>Sertakan <strong>waktu kejadian</strong> (tanggal/jam).</li>
  <li>Tulis <strong>langkah reproduksi</strong> singkat (1-2-3).</li>
</ul>
  $g5$,
  90,
  TRUE
);

UPDATE guide_books g
SET content_html = s.content_html,
    sort_order = s.sort_order,
    is_active = s.is_active,
    updated_at = NOW()
FROM tmp_guide_book_seed_school_admin s
WHERE g.role = s.role
  AND g.title = s.title;

INSERT INTO guide_books (role, title, content_html, sort_order, is_active)
SELECT s.role, s.title, s.content_html, s.sort_order, s.is_active
FROM tmp_guide_book_seed_school_admin s
WHERE NOT EXISTS (
  SELECT 1
  FROM guide_books g
  WHERE g.role = s.role
    AND g.title = s.title
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM guide_books
WHERE role = 'SCHOOL_ADMIN'
  AND title IN (
    'Siswa: Roster Kelas XII (Import, Update, Rejection)',
    'Siswa: Rencana Lulusan (SNBP/SNBT/Mandiri/Kedinasan/Lainnya)',
    'Siswa: Finalisasi, Pending, dan Cara Validasi Data',
    'Prestasi: Input, Edit, dan QC',
    'Troubleshooting Operator Sekolah (Kasus Umum)'
  );

UPDATE guide_books
SET is_active = TRUE,
    updated_at = NOW()
WHERE role = 'SCHOOL_ADMIN'
  AND title = 'Panduan Sekolah: Roster Kelas XII, Rencana Lulusan, dan QC';
-- +goose StatementEnd
