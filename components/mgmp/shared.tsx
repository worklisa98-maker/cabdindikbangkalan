"use client";

import type { ReactNode } from "react";
import Link from "next/link";

export const inputClass =
  "w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100";
export const buttonClass =
  "inline-flex items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-60";
export const secondaryButtonClass =
  "inline-flex items-center justify-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:opacity-60";

export function Panel({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <section
      className={`rounded-2xl border border-slate-200 bg-white p-5 shadow-sm ${className}`}
    >
      {children}
    </section>
  );
}

export function PageTitle({
  title,
  description,
  actions,
}: {
  title: string;
  description: string;
  actions?: ReactNode;
}) {
  return (
    <div className="mb-6 flex flex-col justify-between gap-4 md:flex-row md:items-start">
      <div>
        <p className="text-xs font-bold tracking-[0.2em] text-blue-700 uppercase">
          SIPRES · MGMP &amp; JARKOM SMK
        </p>
        <h1 className="mt-1 text-2xl font-bold text-slate-900">{title}</h1>
        <p className="mt-1 max-w-3xl text-sm text-slate-600">{description}</p>
      </div>
      {actions}
    </div>
  );
}

export function PublicMGMPNavigation({
  active,
}: {
  active: "register" | "status";
}) {
  const items = [
    { key: "register", label: "Daftar MGMP / JARKOM", href: "/mgmp/register" },
    { key: "status", label: "Cek Pendaftaran", href: "/mgmp/status" },
  ] as const;
  return (
    <nav
      aria-label="Navigasi pendaftaran MGMP dan JARKOM"
      className="mb-6 grid grid-cols-2 rounded-xl border border-slate-200 bg-white p-1 shadow-sm"
    >
      {items.map((item) => (
        <Link
          key={item.key}
          href={item.href}
          aria-current={active === item.key ? "page" : undefined}
          className={`rounded-lg px-4 py-2.5 text-center text-sm font-semibold transition ${
            active === item.key
              ? "bg-blue-700 text-white shadow-sm"
              : "text-slate-600 hover:bg-slate-50 hover:text-blue-700"
          }`}
        >
          {item.label}
        </Link>
      ))}
    </nav>
  );
}

export function Modal({
  title,
  children,
  onClose,
  wide = false,
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
}) {
  return (
    <div className="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/50 p-4">
      <div
        className={`max-h-[90vh] w-full overflow-y-auto rounded-2xl bg-white p-5 shadow-2xl ${wide ? "max-w-4xl" : "max-w-xl"}`}
      >
        <div className="mb-4 flex items-center justify-between gap-4">
          <h2 className="text-lg font-bold text-slate-900">{title}</h2>
          <button
            onClick={onClose}
            className="rounded-lg px-3 py-1 text-slate-500 hover:bg-slate-100"
            aria-label="Tutup"
          >
            ✕
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

export function StatusPill({ value }: { value: string }) {
  const normalized =
    {
      PENDING: "Menunggu Verifikasi",
      REVISION_REQUIRED: "Perlu Perbaikan",
      APPROVED: "Disetujui",
      REJECTED: "Ditolak",
    }[value] ?? value.replaceAll("_", " ");
  const color =
    value.includes("ACTIVE") || value === "AKTIF" || value === "APPROVED" || value === "ADA_KEGIATAN"
      ? "bg-emerald-100 text-emerald-700"
      : value === "REVISION_REQUIRED"
        ? "bg-orange-100 text-orange-800"
        : value.includes("PENDING") || value.includes("BELUM")
          ? "bg-amber-100 text-amber-800"
          : value === "INACTIVE" || value === "NONAKTIF" || value === "DIKELUARKAN" || value === "REJECTED"
            ? "bg-red-100 text-red-700"
            : "bg-blue-100 text-blue-700";
  return (
    <span
      className={`inline-flex rounded-full px-2.5 py-1 text-xs font-bold ${color}`}
    >
      {normalized}
    </span>
  );
}

export async function readAPI<T>(response: Response): Promise<T> {
  const payload = (await response.json().catch(() => null)) as {
    data?: T;
    message?: string;
    details?: unknown;
    error?: string;
  } | null;
  if (!response.ok) {
    const detail = Array.isArray(payload?.details)
      ? payload.details.join(" ")
      : typeof payload?.details === "string"
        ? payload.details
        : "";
    throw new Error(
      payload?.message || detail || payload?.error || "Permintaan gagal."
    );
  }
  return (payload?.data ?? payload) as T;
}
