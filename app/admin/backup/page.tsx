"use client";
import { useEffect, useState } from "react";
import { DatabaseBackup, Download, KeyRound } from "lucide-react";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
type BackupStatus = {
  lastBackupAt?: string;
  due: boolean;
  configured: boolean;
  history: Array<{
    startedAt: string;
    status: string;
    sizeBytes?: number;
    durationMs?: number;
    checksum?: string;
    message?: string;
  }>;
};
export default function BackupPage() {
  const { user, status } = useAuth();
  const [data, setData] = useState<BackupStatus>();
  const [busy, setBusy] = useState(false);
  const load = () =>
    apiFetch("/api/admin/backups")
      .then((r) => r.json())
      .then((x) => setData(x.data));
  useEffect(() => {
    if (user?.role === "ADMIN_MASTER") load();
  }, [user]);
  async function download() {
    setBusy(true);
    const r = await apiFetch("/api/admin/backups/download", { method: "POST" });
    if (!r.ok) {
      const x = await r.json();
      alert(x.error);
      setBusy(false);
      return;
    }
    const blob = await r.blob();
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    const cd = r.headers.get("content-disposition") || "";
    a.download =
      cd.match(/filename="([^"]+)"/)?.[1] || "SIPRES-BACKUP.sipres-backup";
    a.click();
    URL.revokeObjectURL(a.href);
    setBusy(false);
    load();
  }
  if (status === "loading") return <p>Memuat...</p>;
  if (user?.role !== "ADMIN_MASTER")
    return (
      <p className="rounded-xl border p-6">Halaman ini khusus ADMIN_MASTER.</p>
    );
  return (
    <div className="mx-auto w-full max-w-5xl space-y-6">
      <header className="rounded-3xl bg-slate-900 p-7 text-white">
        <DatabaseBackup className="mb-3 h-9 w-9" />
        <h1 className="text-2xl font-bold">Backup & Pemulihan Data</h1>
        <p className="text-slate-300">
          Backup manual terenkripsi untuk disimpan di laptop. Pemulihan hanya
          melalui CLI pada server maintenance.
        </p>
      </header>
      <section className="grid gap-4 md:grid-cols-3">
        <div className="rounded-2xl border bg-white p-5">
          <p className="text-sm text-slate-500">Backup terakhir</p>
          <b>
            {data?.lastBackupAt
              ? new Date(data.lastBackupAt).toLocaleString("id-ID")
              : "Belum pernah"}
          </b>
        </div>
        <div className="rounded-2xl border bg-white p-5">
          <p className="text-sm text-slate-500">Status pengingat</p>
          <b>{data?.due ? "Sudah jatuh tempo" : "Belum jatuh tempo"}</b>
        </div>
        <div className="rounded-2xl border bg-white p-5">
          <p className="text-sm text-slate-500">Enkripsi</p>
          <b>{data?.configured ? "Siap" : "Belum dikonfigurasi"}</b>
        </div>
      </section>
      <button
        disabled={busy || !data?.configured}
        onClick={download}
        className="flex w-full items-center justify-center gap-2 rounded-xl bg-blue-700 p-4 font-semibold text-white disabled:bg-slate-400"
      >
        <Download />
        {busy ? "Membuat dan mengenkripsi..." : "Buat & Unduh Backup"}
      </button>
      <div className="rounded-2xl border border-amber-200 bg-amber-50 p-5 text-sm text-amber-950">
        <KeyRound className="mb-2" />
        <b>Simpan BACKUP_ENCRYPTION_KEY secara terpisah.</b>
        <p>
          Key tidak ikut di dalam backup dan tidak dapat dipulihkan. Server
          hanya dapat memastikan file selesai dikirim ke browser, bukan bahwa
          file benar-benar disimpan pengguna.
        </p>
      </div>
      <section className="overflow-x-auto rounded-2xl border bg-white p-5">
        <h2 className="mb-3 font-bold">Riwayat backup</h2>
        <table className="w-full text-left text-sm">
          <thead>
            <tr>
              <th>Waktu</th>
              <th>Status</th>
              <th>Ukuran</th>
              <th>Durasi</th>
              <th>Checksum</th>
            </tr>
          </thead>
          <tbody>
            {data?.history?.map((x, i) => (
              <tr key={i} className="border-t">
                <td className="py-3">
                  {new Date(x.startedAt).toLocaleString("id-ID")}
                </td>
                <td>{x.status}</td>
                <td>
                  {x.sizeBytes
                    ? `${(x.sizeBytes / 1048576).toFixed(2)} MB`
                    : "-"}
                </td>
                <td>
                  {x.durationMs
                    ? `${(x.durationMs / 1000).toFixed(1)} dtk`
                    : "-"}
                </td>
                <td className="max-w-52 truncate font-mono">
                  {x.checksum || x.message || "-"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  );
}
