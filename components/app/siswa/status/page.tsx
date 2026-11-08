import Link from "next/link";

const statusRoutes = [
  {
    title: "Diterima Perguruan Tinggi",
    href: "/siswa/status-lulusan/pt",
  },
  {
    title: "Diterima Dunia Kerja",
    href: "/siswa/status-lulusan/kerja",
  },
  {
    title: "Berwirausaha",
    href: "/siswa/status-lulusan/wirausaha",
  },
  {
    title: "Belum Melanjutkan Studi/Kerja",
    href: "/siswa/status-lulusan/belum",
  },
  {
    title: "Pending Validasi",
    href: "/siswa/pending-validasi",
  },
];

export default function SiswaStatusPage() {
  return (
    <section className="space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          Siswa
        </p>
        <h1 className="text-3xl font-semibold text-zinc-900">Status Lulusan</h1>
        <p className="text-sm text-zinc-600">
          Pilih kategori status lulusan untuk melihat ringkasan data.
        </p>
      </header>

      <div className="grid gap-3 sm:grid-cols-2">
        {statusRoutes.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className="rounded-xl border border-zinc-200 bg-white p-4 text-sm font-semibold text-zinc-900 transition hover:border-zinc-300 hover:bg-zinc-50"
          >
            {item.title}
          </Link>
        ))}
      </div>
    </section>
  );
}
