"use client";
import Link from "next/link";
import Image from "next/image";
import { usePathname, useRouter } from "next/navigation";
import { Menu, X, ChevronDown, Search, Bell } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  filterVisibleSections,
  getSectionHref,
  type SidebarSection,
  sidebarSections,
} from "@/lib/navigation/sidebar-menu";
import { fetchServiceLinks } from "@/lib/service-links";
import { useAuth } from "@/lib/auth/client";
import { apiFetch } from "@/lib/api/client";
import { hasRoleAccess } from "@/lib/auth/role-scope";
import { ReportDownload } from "./report-download";
import type { ReactNode } from "react";

export function PublicNavbar({ account }: { account?: ReactNode }) {
  const pathname = usePathname();
  const { user, status, logout } = useAuth();
  const router = useRouter();
  const pending = useQuery({
    queryKey: ["siswa-status-pending-summary", user?.id],
    enabled:
      status === "authenticated" &&
      hasRoleAccess(user?.role, [
        "ADMIN",
        "ADMIN_MASTER",
        "SCHOOL_ADMIN",
        "PENGAWAS",
      ]),
    queryFn: async () => {
      const response = await apiFetch("/api/siswa/validation/summary");
      if (!response.ok) throw new Error("Notifikasi gagal dimuat");
      const payload = await response.json();
      const data = payload.data ?? payload;
      return Number(
        data.pending ?? data.pending_count ?? data.pendingCount ?? 0
      );
    },
  });
  const signOut = async () => {
    try {
      await apiFetch("/auth/logout", { method: "POST" });
    } finally {
      logout();
      router.push("/login");
    }
  };
  const homeHref =
    user?.role === "USER_MGMP"
      ? "/mgmp"
      : user?.role === "USER_SEKOLAH_GASPOL"
        ? "/gaspol"
        : "/";
  const scope = status === "authenticated" && user ? "user" : "public";
  const [open, setOpen] = useState(false);
  const drawer = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (open) drawer.current?.showModal();
    else drawer.current?.close();
  }, [open]);
  const { data } = useQuery({
    queryKey: ["service-links", scope, scope === "user" ? user?.id : "anon"],
    queryFn: () => fetchServiceLinks(scope),
    enabled: status !== "loading",
  });
  const sections = (
    status === "loading" ? [] : filterVisibleSections(sidebarSections, user)
  )
    .filter((s) => s.key !== "halo-kepegawaian" || data?.halo.is_active)
    .filter((s) => s.key !== "sinom" || data?.sinom.is_active);
  const groups = [
    { label: "Data Pendidikan", keys: ["akreditasi", "prestasi", "siswa"] },
    {
      label: "Program & Layanan",
      keys: ["gaspol", "mgmp", "halo-kepegawaian", "sinom", "lomba-hardiknas"],
    },
    { label: "Informasi Cabdin", keys: ["informasi-pelayanan-cabdin"] },
    {
      label: "Layanan Masyarakat",
      keys: ["pengaduan-kritik-saran", "cek-pengaduan"],
    },
  ];
  const isActive = (href: string) =>
    pathname === href || (href !== "/" && pathname.startsWith(`${href}/`));
  const expandSections = (sections: SidebarSection[]) =>
    sections.flatMap((section) => [section, ...(section.children ?? [])]);
  const navigation = (mobile: boolean) => (
    <nav
      onClick={(event) => {
        if ((event.target as HTMLElement).closest("a")) setOpen(false);
      }}
      aria-label={mobile ? "Navigasi mobile" : "Navigasi utama"}
      className={
        mobile
          ? "flex flex-col gap-2"
          : "hidden shrink-0 items-center gap-0 min-[1440px]:flex"
      }
    >
      <Link
        href={homeHref}
        onClick={() => setOpen(false)}
        aria-current={pathname === homeHref ? "page" : undefined}
        className={`rounded-lg px-3 py-2 text-sm font-semibold ${pathname === homeHref ? "text-primary bg-blue-50" : "text-slate-700 hover:bg-blue-50"}`}
      >
        Beranda
      </Link>
      {groups
        .filter((group) =>
          sections.some((section) => group.keys.includes(section.key))
        )
        .map((group) => (
          <details
            key={group.label}
            className="group relative"
            onKeyDown={(e) => {
              if (e.key === "Escape") e.currentTarget.open = false;
            }}
          >
            <summary
              className={`flex cursor-pointer list-none items-center gap-1 rounded-lg px-3 py-3 text-sm hover:bg-slate-50 ${sections.some((section) => group.keys.includes(section.key) && isActive(getSectionHref(section, user))) ? "bg-blue-50 font-semibold text-blue-800" : "text-slate-700"}`}
            >
              {group.label}
              <ChevronDown className="size-3" />
            </summary>
            <div
              className={`space-y-1 rounded-xl border border-slate-200 bg-white p-2 ${mobile ? "" : "absolute top-full right-0 w-72 shadow-lg"}`}
            >
              {expandSections(
                sections.filter((s) => group.keys.includes(s.key))
              ).map((s) => (
                <Link
                  key={s.key}
                  href={
                    s.key === "sinom" && data?.sinom.url
                      ? data.sinom.url
                      : getSectionHref(s, user)
                  }
                  target={
                    s.external || (s.key === "sinom" && user)
                      ? "_blank"
                      : undefined
                  }
                  rel={
                    s.external || (s.key === "sinom" && user)
                      ? "noopener noreferrer"
                      : undefined
                  }
                  onClick={(e) => {
                    setOpen(false);
                    e.currentTarget.closest("details")?.removeAttribute("open");
                  }}
                  aria-current={
                    isActive(getSectionHref(s, user)) ? "page" : undefined
                  }
                  className="block rounded-lg px-3 py-3 text-sm text-slate-700 hover:bg-blue-50 aria-[current=page]:bg-blue-50 aria-[current=page]:text-blue-800"
                >
                  {s.label}
                </Link>
              ))}
            </div>
          </details>
        ))}
      {sections.some((s) => s.key === "pusat-tautan") && (
        <Link
          className="rounded-lg px-3 py-2 text-sm"
          href="/pusat-tautan"
          aria-current={pathname === "/pusat-tautan" ? "page" : undefined}
        >
          Pusat Tautan
        </Link>
      )}
      {homeHref === "/" && (
        <Link
          href="/#portal-search"
          aria-label="Cari sekolah, NPSN, atau layanan"
          onClick={() => setOpen(false)}
          className="rounded-full border border-slate-200 p-2 text-slate-600 hover:bg-blue-50"
        >
          <Search className="size-4" />
        </Link>
      )}

      {pending.data != null && (
        <Link
          href="/siswa/pending-validasi"
          className="relative rounded-full border border-slate-200 p-2"
          aria-label={`${pending.data} menunggu validasi`}
        >
          <Bell className="size-4" />
          {pending.data > 0 && (
            <span className="absolute -top-2 -right-2 rounded-full bg-amber-100 px-1 text-xs">
              {pending.data}
            </span>
          )}
        </Link>
      )}
      {account ??
        (status === "loading" ? (
          <span className="px-3 text-sm">Memuat akun�</span>
        ) : user ? (
          <details
            className="relative"
            onKeyDown={(event) => {
              if (event.key === "Escape") event.currentTarget.open = false;
            }}
          >
            <summary
              className="max-w-44 cursor-pointer truncate rounded-lg bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-900"
              aria-label="Menu akun"
            >
              {user.namaInstansi || user.username}
            </summary>
            <div
              className={`${mobile ? "" : "absolute top-full right-0 w-64 shadow-lg"} space-y-1 rounded-xl border border-slate-200 bg-white p-3`}
            >
              <p className="px-2 text-xs break-words text-slate-500">
                {user.role}
              </p>
              {sections
                .filter((section) =>
                  ["settings", "guide-books", "backup-pemulihan"].includes(
                    section.key
                  )
                )
                .map((section) => (
                  <Link
                    key={section.key}
                    href={getSectionHref(section, user)}
                    className="block rounded-lg p-2 text-sm hover:bg-blue-50"
                  >
                    {section.label}
                  </Link>
                ))}
              <ReportDownload />
              <button
                type="button"
                className="w-full rounded-lg p-2 text-left text-sm text-red-700 hover:bg-red-50"
                onClick={() => void signOut()}
              >
                Keluar
              </button>
            </div>
          </details>
        ) : (
          <Link
            href="/login"
            onClick={() => setOpen(false)}
            className="portal-button"
          >
            Masuk
          </Link>
        ))}
    </nav>
  );
  return (
    <header className="sticky top-0 z-30 border-b border-slate-200 bg-white">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:bg-white focus:p-4"
      >
        Lewati ke konten
      </a>
      <div className="portal-container flex min-h-18 items-center justify-between gap-4 py-3">
        <Link
          href={homeHref}
          className="flex min-w-0 items-center gap-3 text-slate-950"
        >
          <Image
            src="/logo-prov-jawa-timur.png"
            alt="Lambang Pemerintah Provinsi Jawa Timur"
            width={42}
            height={52}
            className="h-12 w-10 shrink-0 object-contain"
          />
          <span className="max-w-[220px] text-[13px] leading-[1.35] font-bold text-slate-900">
            Cabang Dinas Pendidikan
            <span className="block">Wilayah Kabupaten Bangkalan</span>
            <span className="mt-1 block text-[10px] font-medium tracking-wide text-slate-500">
              PEMERINTAH PROVINSI JAWA TIMUR
            </span>
          </span>
        </Link>
        <button
          type="button"
          aria-label="Buka navigasi"
          aria-expanded={open}
          aria-controls="public-drawer"
          onClick={() => setOpen(true)}
          className="shrink-0 rounded-lg border p-2 min-[1440px]:hidden"
        >
          <Menu />
        </button>
        {navigation(false)}
        <dialog
          id="public-drawer"
          ref={drawer}
          onCancel={() => setOpen(false)}
          onClose={() => setOpen(false)}
          aria-label="Menu portal"
          className="fixed inset-y-0 right-0 left-auto m-0 h-dvh max-h-none w-[min(22rem,90vw)] max-w-none border-0 bg-white p-5 text-slate-900 shadow-xl backdrop:bg-slate-950/40"
        >
          <div className="mb-5 flex items-center justify-between">
            <p className="font-semibold">Menu portal</p>
            <button
              type="button"
              onClick={() => setOpen(false)}
              aria-label="Tutup navigasi"
              className="rounded-lg border p-2"
            >
              <X />
            </button>
          </div>
          {navigation(true)}
        </dialog>
      </div>
    </header>
  );
}
