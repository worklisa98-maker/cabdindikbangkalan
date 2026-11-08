"use client";

import { useEffect } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import gaspolBackground from "@/app/assets/Baground-page-gaspol-bang.png";
import { PageSectionNavigation } from "@/components/portal/page-section-navigation";
import { PublicNavbar } from "@/components/portal/public-navbar";
import { ModuleNavigation } from "@/components/portal/module-navigation";
import {
  filterVisibleSections,
  sidebarSections,
} from "@/lib/navigation/sidebar-menu";
import { SystemTimeIndicator } from "@/components/system-time/SystemTimeIndicator";
import { isGaspolSchoolUserAllowedPage } from "@/lib/auth/gaspol-school-access";
import { useAuth } from "@/lib/auth/client";
import { cn } from "@/lib/utils";

export function AppShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname() ?? "";
  const searchParams = useSearchParams();
  const { user, status } = useAuth();
  const isReportRoute = pathname.startsWith("/reports");
  const isExportMode = searchParams.get("export") === "1";
  const isGaspolRoute = pathname === "/gaspol";
  const isGaspolSchoolUser = user?.role === "USER_SEKOLAH_GASPOL";
  const isMGMPUser = user?.role === "USER_MGMP";
  const isGaspolSchoolUserPathAllowed = isGaspolSchoolUserAllowedPage(pathname);
  const shouldRedirectGaspolSchoolUser =
    status === "authenticated" &&
    isGaspolSchoolUser &&
    !isGaspolSchoolUserPathAllowed;
  const shouldRedirectMGMPUser =
    status === "authenticated" &&
    isMGMPUser &&
    !pathname.startsWith("/mgmp") &&
    pathname !== "/login";
  const gaspolMainStyle = isGaspolRoute
    ? {
        backgroundColor: "rgba(248, 250, 252, 0.94)",
        backgroundImage: `linear-gradient(rgba(248, 250, 252, 0.82), rgba(248, 250, 252, 0.9)), url(${gaspolBackground.src})`,
        backgroundPosition: "center",
        backgroundRepeat: "no-repeat",
        backgroundSize: "cover",
        backgroundAttachment: "fixed" as const,
      }
    : undefined;

  useEffect(() => {
    const root = document.documentElement;
    if (isExportMode) {
      root.dataset.exportMode = "1";
      return () => {
        delete root.dataset.exportMode;
      };
    }

    delete root.dataset.exportMode;
    return undefined;
  }, [isExportMode]);

  useEffect(() => {
    if (!shouldRedirectGaspolSchoolUser) {
      return;
    }
    router.replace("/gaspol");
  }, [router, shouldRedirectGaspolSchoolUser]);

  useEffect(() => {
    if (shouldRedirectMGMPUser) router.replace("/mgmp");
  }, [router, shouldRedirectMGMPUser]);

  if (isReportRoute || isExportMode) {
    return (
      <main className="text-foreground min-h-screen w-full max-w-none bg-white px-0 py-0">
        {children}
      </main>
    );
  }

  if (
    pathname === "/" &&
    !shouldRedirectGaspolSchoolUser &&
    !shouldRedirectMGMPUser
  ) {
    return (
      <div className="min-h-screen bg-slate-50 text-slate-950">
        <SystemTimeIndicator />
        {children}
      </div>
    );
  }

  return (
    <div className="text-foreground flex min-h-screen min-w-0 flex-col bg-slate-50">
      <PublicNavbar />
      <ModuleNavigation
        sections={filterVisibleSections(sidebarSections, user).filter(
          (section) =>
            section.children?.length &&
            section.href &&
            (pathname === section.href ||
              pathname.startsWith(`${section.href}/`))
        )}
        user={user}
      />
      <SystemTimeIndicator />
      <main
        id="main-content"
        className={cn(
          "portal-container flex w-full min-w-0 flex-1 flex-col gap-6 py-6 md:py-8",
          isGaspolRoute && "relative isolate"
        )}
        style={gaspolMainStyle}
      >
        {shouldRedirectGaspolSchoolUser || shouldRedirectMGMPUser ? (
          <div className="rounded-xl border border-slate-200 bg-white px-4 py-6 text-sm text-slate-600">
            Mengalihkan akun ke workspace yang sesuai...
          </div>
        ) : (
          <>
            {(pathname.startsWith("/gaspol") ||
              pathname.startsWith("/mgmp") ||
              pathname === "/user/settings") && <PageSectionNavigation />}
            {children}
          </>
        )}
      </main>
    </div>
  );
}
