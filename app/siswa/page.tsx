"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";

import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { Card, CardContent } from "@/components/ui/card";
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
    href: "/siswa/kelas-akhir",
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
    title: "Pengumuman Diterima PT",
    href: "/siswa/pengumuman-pt",
    description:
      "Import data multi-jalur pengumuman diterima perguruan tinggi.",
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
  const searchParams = useSearchParams();
  const isExportMode = searchParams.get("export") === "1";
  const { status, user } = useAuth();
  const [isReportReady, setIsReportReady] = useState(false);
  const isAdminUser =
    status === "authenticated" &&
    (user?.role === "ADMIN" ||
      user?.role === "ADMIN_MASTER" ||
      user?.role === "SCHOOL_ADMIN" ||
      user?.role === "PENGAWAS");

  const visibleMenus = siswaMenuItems.filter(
    (item) => !item.adminOnly || isAdminUser
  );
  const isPageLoading = status === "loading";

  useEffect(() => {
    let cancelled = false;
    requestAnimationFrame(() => {
      if (cancelled) {
        return;
      }
      if (isPageLoading) {
        setIsReportReady(false);
        return;
      }
      requestAnimationFrame(() => {
        if (!cancelled) {
          setIsReportReady(true);
        }
      });
    });

    return () => {
      cancelled = true;
    };
  }, [isPageLoading]);

  return (
    <div
      id="report-root"
      data-report-ready={isReportReady ? "1" : "0"}
      className={isExportMode ? "w-full max-w-none" : "w-full"}
    >
      <section className="space-y-6">
        <PageHeader
          eyebrow="Menu Siswa"
          title="Siswa"
          description="Pilih modul data siswa dan status lulusan dari daftar berikut."
        />

        <Card>
          <CardContent className="p-4 sm:p-6">
            {visibleMenus.length === 0 ? (
              <EmptyState
                title="Belum ada modul yang tersedia"
                description="Modul akan tampil sesuai role dan hak akses akun Anda."
              />
            ) : (
              <div className="grid gap-3 sm:grid-cols-2">
                {visibleMenus.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    className="rounded-2xl border border-slate-200/80 bg-white p-4 transition hover:-translate-y-0.5 hover:border-slate-300 hover:bg-slate-50/70"
                  >
                    <p className="text-sm font-semibold text-slate-900">
                      {item.title}
                    </p>
                    <p className="mt-1 text-sm text-slate-600">
                      {item.description}
                    </p>
                  </Link>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
