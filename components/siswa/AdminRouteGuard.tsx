"use client";

import { useEffect, useSyncExternalStore, type ReactNode } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { useAuth } from "@/lib/auth/client";

type AdminRouteGuardProps = {
  children: ReactNode;
  allowedRoles?: string[];
};

const subscribeToHydration = () => () => {};
const clientSnapshot = () => true;
const serverSnapshot = () => false;

const defaultRoles = [
  "ADMIN",
  "ADMIN_CABDIN",
  "ADMIN_MASTER",
  "SCHOOL_ADMIN",
  "PENGAWAS",
];

export default function AdminRouteGuard({
  children,
  allowedRoles = defaultRoles,
}: AdminRouteGuardProps) {
  const router = useRouter();
  const { status, user } = useAuth();
  const mounted = useSyncExternalStore(subscribeToHydration, clientSnapshot, serverSnapshot);

  useEffect(() => {
    if (mounted && status === "unauthenticated") {
      router.replace("/login");
    }
  }, [mounted, router, status]);

  const loadingSection = (
    <section className="rounded-xl border border-dashed border-zinc-200 bg-white p-6 text-sm text-zinc-500">
      Memuat...
    </section>
  );

  if (!mounted) {
    return loadingSection;
  }

  if (status === "loading" || status === "unauthenticated") {
    return loadingSection;
  }

  if (!user || !allowedRoles.includes(user.role)) {
    return (
      <section className="rounded-xl border border-dashed border-red-200 bg-white p-6 text-sm text-red-600">
        <p className="font-semibold">403 - Akses Ditolak</p>
        <p className="mt-2">
          Anda tidak memiliki izin untuk membuka halaman ini.
        </p>
        <Link
          href="/"
          className="mt-4 inline-flex rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
        >
          Kembali ke Dashboard
        </Link>
      </section>
    );
  }

  return <>{children}</>;
}
