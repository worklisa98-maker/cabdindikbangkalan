"use client";

import Image from "next/image";
import Link from "next/link";
import { ArrowLeft, ImagePlus, Power } from "lucide-react";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";

type PopupImage = {
  id: number;
  title: string;
  altText: string;
  imageUrl: string;
  sizeBytes: number;
  isActive: boolean;
  createdAt: string;
};

export default function ComplaintPopupSettingsPage() {
  const { user, status } = useAuth();
  const [images, setImages] = useState<PopupImage[]>([]);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");

  const load = useCallback(async () => {
    const response = await apiFetch("/api/admin/complaints-popup");
    const payload = await response.json();
    if (response.ok) setImages(payload.data ?? []);
    else setMessage(payload.error ?? "Daftar gambar tidak dapat dimuat.");
  }, []);

  useEffect(() => {
    if (status !== "authenticated" || user?.role !== "ADMIN_MASTER") return;
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load, status, user]);

  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setMessage("");
    const response = await apiFetch("/api/admin/complaints-popup", {
      method: "POST",
      body: new FormData(event.currentTarget),
    });
    const payload = await response.json();
    setMessage(payload.message ?? payload.error ?? "Permintaan selesai.");
    if (response.ok) {
      event.currentTarget.reset();
      await load();
    }
    setBusy(false);
  }

  async function setActive(id: number, active: boolean) {
    setBusy(true);
    const response = await apiFetch(`/api/admin/complaints-popup/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ active }),
    });
    const payload = await response.json();
    setMessage(payload.message ?? payload.error ?? "Permintaan selesai.");
    if (response.ok) await load();
    setBusy(false);
  }

  if (status === "loading")
    return <main className="p-8">Memeriksa akses…</main>;
  if (!user || user.role !== "ADMIN_MASTER") {
    return (
      <main className="mx-auto max-w-xl p-8 text-center">
        <h1 className="text-2xl font-bold">Akses ditolak</h1>
        <p className="mt-2 text-slate-600">
          Settings informasi popup hanya tersedia untuk Admin Master.
        </p>
        <Link
          href="/pengaduan-kritik-saran"
          className="mt-5 inline-block text-blue-700"
        >
          Kembali ke Pengaduan/Kritik/Saran
        </Link>
      </main>
    );
  }

  const active = images.find((item) => item.isActive);
  return (
    <main className="mx-auto max-w-6xl space-y-6 p-4 sm:p-8">
      <Link
        href="/pengaduan-kritik-saran"
        className="inline-flex items-center gap-2 text-blue-700"
      >
        <ArrowLeft className="h-4 w-4" /> Kembali
      </Link>
      <div>
        <h1 className="text-3xl font-bold">Informasi Popup</h1>
        <p className="text-slate-600">
          Kelola gambar informasi yang tampil pada halaman layanan masyarakat.
        </p>
      </div>
      {message && (
        <p className="rounded-xl bg-blue-50 p-3 text-sm text-blue-900">
          {message}
        </p>
      )}
      <div className="grid gap-6 lg:grid-cols-2">
        <section className="rounded-3xl border bg-white p-6 shadow-sm">
          <h2 className="text-lg font-bold">Preview aktif</h2>
          {active ? (
            <div className="mt-4">
              <Image
                src={active.imageUrl}
                alt={active.altText}
                width={960}
                height={640}
                unoptimized
                className="max-h-80 w-full rounded-2xl bg-slate-50 object-contain"
              />
              <p className="mt-3 font-semibold">{active.title}</p>
              <p className="text-sm text-slate-600">{active.altText}</p>
              <button
                disabled={busy}
                onClick={() => setActive(active.id, false)}
                className="mt-4 rounded-xl border px-4 py-2 text-sm disabled:opacity-50"
              >
                Nonaktifkan popup
              </button>
            </div>
          ) : (
            <p className="mt-4 rounded-xl bg-slate-50 p-5 text-slate-500">
              Tidak ada popup aktif.
            </p>
          )}
        </section>
        <form
          onSubmit={upload}
          className="rounded-3xl border bg-white p-6 shadow-sm"
        >
          <h2 className="flex items-center gap-2 text-lg font-bold">
            <ImagePlus className="h-5 w-5" /> Upload baru
          </h2>
          <label className="mt-4 block text-sm font-medium">
            Judul gambar
            <input
              name="title"
              required
              maxLength={200}
              className="mt-1 w-full rounded-xl border p-3"
            />
          </label>
          <label className="mt-4 block text-sm font-medium">
            Teks alternatif
            <textarea
              name="altText"
              required
              maxLength={300}
              className="mt-1 w-full rounded-xl border p-3"
            />
          </label>
          <label className="mt-4 block text-sm font-medium">
            Gambar JPG, PNG, atau WebP (maks. 5 MB)
            <input
              name="image"
              type="file"
              required
              accept=".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp"
              className="mt-1 w-full rounded-xl border p-3"
            />
          </label>
          <label className="mt-4 flex items-center gap-2 text-sm">
            <input
              name="isActive"
              value="true"
              type="checkbox"
              defaultChecked
            />{" "}
            Langsung jadikan popup aktif
          </label>
          <button
            disabled={busy}
            className="mt-5 w-full rounded-xl bg-blue-700 p-3 font-semibold text-white disabled:opacity-50"
          >
            {busy ? "Menyimpan…" : "Simpan gambar"}
          </button>
        </form>
      </div>
      <section className="rounded-3xl border bg-white p-6 shadow-sm">
        <h2 className="text-lg font-bold">Riwayat gambar</h2>
        <div className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {images.map((item) => (
            <article key={item.id} className="rounded-2xl border p-3">
              <Image
                src={item.imageUrl}
                alt={item.altText}
                width={480}
                height={320}
                unoptimized
                className="h-40 w-full rounded-xl bg-slate-50 object-contain"
              />
              <p className="mt-2 font-semibold">{item.title}</p>
              <p className="line-clamp-2 text-xs text-slate-500">
                {item.altText}
              </p>
              <button
                disabled={busy || item.isActive}
                onClick={() => setActive(item.id, true)}
                className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-xl border p-2 text-sm disabled:opacity-50"
              >
                <Power className="h-4 w-4" />{" "}
                {item.isActive ? "Sedang aktif" : "Jadikan aktif"}
              </button>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}
