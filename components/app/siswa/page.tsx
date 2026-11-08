"use client";

import Link from "next/link";

import { useAuth } from "@/lib/auth/client";

type SiswaMenuItem = {
  title: string;
  href: string;
  description: string;
  adminOnly?: boolean;
};

const siswaMenuItems: SiswaMenuItem[] = [
  {
    title: "Data Siswa Kelas Akhir",
    href: "/siswa/kelas-xii",
    description: "Kelola data cohort siswa kelas akhir.",
    adminOnly: true,
  },
  {
    title: "Rencana Lulusan",
    href: "/siswa/rencana-lulusan",
    description: "Pantau dan lengkapi rencana lulusan siswa kelas akhir.",
    adminOnly: true,
  },
  {
    title: "Status Lulusan",
    href: "/siswa/status-lulusan",
    description: "Ringkasan status lulusan siswa (sementara).",
    adminOnly: true,
  },
];

export default function SiswaPage() {
  const { status, user } = useAuth();
  const isAdminUser =
    status === "authenticated" &&
    (user?.role === "ADMIN" ||
      user?.role === "ADMIN_MASTER" ||
      user?.role === "SCHOOL_ADMIN");

  const visibleMenus = siswaMenuItems.filter(
    (item) => !item.adminOnly || isAdminUser
  );

  return (
    <section className="space-y-6">
      <header className="space-y-2">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          Menu Siswa
        </p>
        <h1 className="text-3xl font-semibold text-zinc-900">Siswa</h1>
        <p className="text-sm text-zinc-600">
          Pilih modul data siswa dan status lulusan dari daftar berikut.
        </p>
      </header>

      <div className="grid gap-3 sm:grid-cols-2">
        {visibleMenus.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            className="rounded-xl border border-zinc-200 bg-white p-4 transition hover:border-zinc-300 hover:bg-zinc-50"
          >
            <p className="text-sm font-semibold text-zinc-900">{item.title}</p>
            <p className="mt-1 text-sm text-zinc-600">{item.description}</p>
          </Link>
        ))}
      </div>
    </section>
  );
}
