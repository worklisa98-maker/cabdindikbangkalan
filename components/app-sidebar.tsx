"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import Image from "next/image";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  ChevronDown,
  ExternalLink,
  FileDown,
  LogIn,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
  RefreshCw,
} from "lucide-react";
import { useAuth } from "@/lib/auth/client";
import { apiFetch } from "@/lib/api/client";
import { fetchServiceLinks } from "@/lib/service-links";
import { getErrorMessageFromResponse } from "@/lib/http";
import { hasRoleAccess, normalizeAppRole } from "@/lib/auth/role-scope";

import {
  filterVisibleSections,
  getSectionHref,
  sidebarSections,
  type SidebarSection,
} from "@/lib/navigation/sidebar-menu";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import { cn } from "@/lib/utils";
import type { AuthUser as User } from "@/lib/auth/types";
import LogoSipres from "@/app/assets/logo-sipres.png";
import logojatimcerdas from "@/app/assets/Logo-jatim-cerdas-non-baground.png";

const menuButtonClasses =
  "relative flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-sidebar-foreground transition-colors duration-200 hover:bg-slate-100";
const subMenuButtonClasses =
  "relative flex w-full items-center rounded-xl px-3 py-2.5 text-sm text-sidebar-foreground/80 transition-colors duration-200 hover:bg-slate-100";
const activeMenuClasses =
  "bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] text-white shadow-sm before:absolute before:left-0 before:top-0 before:h-full before:w-1 before:rounded-r-full before:bg-[#1E63D5]";

type ValidationSummary = {
  pending: number;
};

type BundleJobPayload = {
  job_id?: string;
  status?: string;
  progress?: number;
  error_message?: string;
  download_url?: string;
};

type BundleJobState = {
  jobId: string;
  status: string;
  progress: number;
};

const siswaValidationSummaryRoles = [
  "ADMIN",
  "ADMIN_MASTER",
  "SCHOOL_ADMIN",
  "PENGAWAS",
] as const;

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const readNumber = (value: unknown) => {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return 0;
};

const normalizeSummary = (payload: unknown): ValidationSummary => {
  if (!isRecord(payload)) {
    return { pending: 0 };
  }
  const data = isRecord(payload.data) ? payload.data : payload;
  return {
    pending: readNumber(
      data.pending ?? data.pending_count ?? data.pendingCount
    ),
  };
};

const normalizeBundleJob = (payload: unknown): BundleJobPayload => {
  if (!isRecord(payload)) {
    return {};
  }
  const data = isRecord(payload.data) ? payload.data : payload;
  return {
    job_id: typeof data.job_id === "string" ? data.job_id : undefined,
    status: typeof data.status === "string" ? data.status : undefined,
    progress:
      typeof data.progress === "number" && Number.isFinite(data.progress)
        ? data.progress
        : undefined,
    error_message:
      typeof data.error_message === "string" ? data.error_message : undefined,
    download_url:
      typeof data.download_url === "string" ? data.download_url : undefined,
  };
};

const fetchPendingSummary = async (): Promise<ValidationSummary> => {
  const response = await apiFetch("/api/siswa/validation/summary");
  if (!response.ok) {
    throw new Error("summary_failed");
  }
  const payload = await response.json().catch(() => null);
  return normalizeSummary(payload);
};

const normalizePath = (path: string) => path.replace(/\/+$/, "");

const isPathMatch = (pathname: string, href: string) => {
  const normalizedPathname = normalizePath(pathname) || "/";
  const normalizedHref = normalizePath(href) || "/";
  if (normalizedHref === "/") {
    return normalizedPathname === "/";
  }
  return (
    normalizedPathname === normalizedHref ||
    normalizedPathname.startsWith(`${normalizedHref}/`)
  );
};

const isSectionActive = (
  section: SidebarSection,
  pathname: string,
  user: User | null
): boolean => {
  if (section.external) {
    return false;
  }
  const href = getSectionHref(section, user);
  if (href !== "#" && isPathMatch(pathname, href)) {
    return true;
  }
  return (
    section.children?.some((child) => isSectionActive(child, pathname, user)) ??
    false
  );
};

export function AppSidebar() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const { user, status, logout } = useAuth();
  const canAccessSiswaValidationSummary = hasRoleAccess(
    user?.role,
    siswaValidationSummaryRoles
  );
  const { data: pendingSummary } = useQuery({
    queryKey: ["siswa-status-pending-summary"],
    queryFn: fetchPendingSummary,
    enabled:
      status === "authenticated" && canAccessSiswaValidationSummary,
  });
  const pendingCount = pendingSummary?.pending ?? 0;
  const [openSections, setOpenSections] = useState<Set<string>>(new Set());
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [isMobileOpen, setIsMobileOpen] = useState(false);
  const [bundleJob, setBundleJob] = useState<BundleJobState | null>(null);
  const [bundleError, setBundleError] = useState("");
  const normalizedRole = normalizeAppRole(user?.role);
  const linkScope = status === "authenticated" && user ? "user" : "public";
  const { data: servicesData, isFetching: servicesFetching } = useQuery({
    queryKey: ["service-links", linkScope, linkScope === "user" ? user?.id : "anon"],
    queryFn: () => fetchServiceLinks(linkScope),
    enabled: status !== "loading",
    staleTime: 0,
    refetchOnMount: "always",
  });
  const visibleSections = useMemo(
    () =>
      status === "loading" ? [] : filterVisibleSections(sidebarSections, user)
        .filter(section => section.key !== "halo-kepegawaian" || (!servicesFetching && servicesData?.halo.is_active === true))
        .filter(section => section.key !== "sinom" || (!servicesFetching && servicesData?.sinom.is_active === true))
        .map(section => section.key === "sinom" && servicesData?.sinom.url
          ? {...section, href: servicesData.sinom.url, external: true} : section),
    [status, user, servicesData, servicesFetching]
  );

  useEffect(() => {
    const activeKeys = new Set<string>();
    const walk = (sections: SidebarSection[], parentKey = "") => {
      for (const section of sections) {
        const sectionKey = parentKey
          ? `${parentKey}/${section.key}`
          : section.key;
        if (
          section.children &&
          section.children.length > 0 &&
          section.children.some((child) =>
            isSectionActive(child, pathname ?? "", user)
          )
        ) {
          activeKeys.add(sectionKey);
        }
        if (section.children && section.children.length > 0) {
          walk(section.children, sectionKey);
        }
      }
    };
    walk(visibleSections);
    setOpenSections((prev) => {
      const next = new Set(prev);
      activeKeys.forEach((key) => next.add(key));
      return next;
    });
  }, [pathname, user, visibleSections]);

  useEffect(() => {
    setIsMobileOpen(false);
  }, [pathname]);

  const toggleSection = (sectionKey: string) => {
    setOpenSections((prev) => {
      const next = new Set(prev);
      if (next.has(sectionKey)) {
        next.delete(sectionKey);
      } else {
        next.add(sectionKey);
      }
      return next;
    });
  };

  const displayName = user?.namaInstansi ?? user?.username ?? "";
  const handleLogout = async () => {
    try {
      await apiFetch("/auth/logout", { method: "POST" });
    } finally {
      logout();
      router.push("/login");
    }
  };
  const canDownloadBundleReport = Boolean(
    user &&
    (normalizedRole === "ADMIN" ||
      user.role === "ADMIN_MASTER" ||
      user.role === "SCHOOL_ADMIN" ||
      user.role === "PENGAWAS")
  );
  const isBundleProcessing = Boolean(
    bundleJob &&
    (bundleJob.status === "queued" || bundleJob.status === "running")
  );
  const handleDownloadBundleReport = async (forceRegenerate = false) => {
    if (isBundleProcessing) {
      return;
    }
    setBundleError("");

    const query = new URLSearchParams();
    const allowedKeys = [
      "tab",
      "status",
      "year",
      "jenjang",
      "status_sekolah",
      "school_id",
      "jenis_lomba",
      "jalur",
      "q",
      "start_year",
      "end_year",
    ];
    for (const key of allowedKeys) {
      const value = searchParams?.get(key);
      if (value) {
        query.set(key, value);
      }
    }
    if (forceRegenerate) {
      query.set("force", "1");
    }

    try {
      const createEndpoint = `/api/reports/bundle/jobs${
        query.toString() ? `?${query.toString()}` : ""
      }`;
      const response = await apiFetch(createEndpoint, { method: "POST" });
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(
            response,
            "Gagal membuat job laporan lengkap."
          )
        );
      }
      const createdPayload = normalizeBundleJob(
        await response.json().catch(() => null)
      );
      const jobId = createdPayload.job_id ?? "";
      const status = createdPayload.status ?? "queued";
      const progress = Math.max(0, Math.min(100, createdPayload.progress ?? 0));

      if (!jobId) {
        throw new Error("Job ID tidak ditemukan.");
      }
      setBundleJob({ jobId, status, progress });

      if (status === "failed") {
        throw new Error(
          createdPayload.error_message || "Proses generate PDF gagal."
        );
      }
      if (status === "done") {
        window.location.assign(
          createdPayload.download_url || `/api/reports/jobs/${jobId}/download`
        );
        setBundleJob(null);
        return;
      }

      while (true) {
        await new Promise((resolve) => setTimeout(resolve, 2000));
        const statusResponse = await apiFetch(`/api/reports/jobs/${jobId}`);
        if (!statusResponse.ok) {
          throw new Error(
            await getErrorMessageFromResponse(
              statusResponse,
              "Gagal membaca status job laporan."
            )
          );
        }

        const statusPayload = normalizeBundleJob(
          await statusResponse.json().catch(() => null)
        );
        const polledStatus = statusPayload.status ?? "running";
        const polledProgress = Math.max(
          0,
          Math.min(100, statusPayload.progress ?? 0)
        );
        setBundleJob({ jobId, status: polledStatus, progress: polledProgress });

        if (polledStatus === "failed") {
          throw new Error(
            statusPayload.error_message || "Proses generate PDF gagal."
          );
        }
        if (polledStatus === "done") {
          window.location.assign(
            statusPayload.download_url || `/api/reports/jobs/${jobId}/download`
          );
          setBundleJob(null);
          return;
        }
      }
    } catch (error) {
      setBundleError(
        error instanceof Error
          ? error.message
          : "Gagal mengunduh laporan lengkap."
      );
      setBundleJob(null);
    }
  };

  const renderSidebarContent = (collapsed: boolean, isMobile = false) => (
    <>
      <SidebarHeader
        className={cn(
          "border-sidebar-border/70 flex items-center justify-between gap-3 border-b pb-4",
          collapsed && "px-2"
        )}
      >
        <Link
          href="/"
          className="focus-visible:ring-primary flex items-center justify-center rounded-xl focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none"
          aria-label="Kembali ke halaman utama"
        >
          <div
            className={cn(
              "flex items-center gap-3",
              collapsed && "justify-center"
            )}
          >
            <Image
              src={logojatimcerdas}
              alt="Logo Jatim"
              width={90}
              height={90}
              className="h-11 w-11 rounded-full object-cover"
              priority
            />
            {!collapsed ? (
              <div>
                <p className="text-sidebar-foreground text-sm leading-none font-semibold">
                  Cabang Dinas Pendidikan
                </p>
                <p className="text-sidebar-foreground/60 text-xs">
                  Wilayah Bangkalan
                </p>
              </div>
            ) : null}
          </div>
        </Link>
        <button
          type="button"
          className={cn(
            "text-sidebar-foreground/70 hover:text-sidebar-foreground rounded-md p-1 transition-colors duration-200 hover:bg-slate-100",
            isMobile ? "md:hidden" : "hidden md:inline-flex"
          )}
          onClick={() => {
            if (isMobile) {
              setIsMobileOpen(false);
              return;
            }
            setIsCollapsed((prev) => !prev);
          }}
          aria-label={
            isMobile
              ? "Tutup sidebar"
              : collapsed
                ? "Expand sidebar"
                : "Collapse sidebar"
          }
        >
          {isMobile ? (
            <PanelLeftClose className="h-4 w-4" />
          ) : collapsed ? (
            <PanelLeftOpen className="h-4 w-4" />
          ) : (
            <PanelLeftClose className="h-4 w-4" />
          )}
        </button>
      </SidebarHeader>
      <SidebarContent className={cn(collapsed && "px-1")}>
        <SidebarGroup>
          <SidebarMenu>
            {status === "loading"
              ? Array.from({ length: 4 }, (_, index) => (
                  <SidebarMenuItem key={`sidebar-skeleton-${index}`}>
                    <div className="flex animate-pulse items-center gap-3 rounded-xl px-3 py-2.5">
                      <div className="h-4 w-4 rounded bg-slate-200" />
                      {!collapsed ? (
                        <div className="h-4 w-28 rounded bg-slate-200" />
                      ) : null}
                    </div>
                  </SidebarMenuItem>
                ))
              : visibleSections.map((section) => {
                  const renderSection = (
                    item: SidebarSection,
                    depth: number,
                    parentKey: string
                  ): ReactNode => {
                    const sectionKey = parentKey
                      ? `${parentKey}/${item.key}`
                      : item.key;
                    const sectionHref = getSectionHref(item, user);
                    const hasChildren = Boolean(
                      item.children && item.children.length > 0
                    );
                    const isOpen = openSections.has(sectionKey);
                    const isActive = isSectionActive(
                      item,
                      pathname ?? "",
                      user
                    );
                    const isTopLevel = depth === 0;
                    const wrapperKey = `${sectionKey}-wrapper`;
                    const showPendingBadge =
                      item.key === "pending-validasi" &&
                      canAccessSiswaValidationSummary &&
                      pendingCount > 0 &&
                      !collapsed;

                    const iconClasses = "h-4 w-4 shrink-0 text-current";
                    const linkLabel = item.external
                      ? `${item.label} (dibuka di tab baru)`
                      : collapsed
                        ? item.label
                        : undefined;
                    const linkContent = (
                      <>
                        {item.icon ? (
                          <item.icon className={iconClasses} />
                        ) : null}
                        <span
                          className={cn(
                            "text-left",
                            !collapsed && "flex-1",
                            isTopLevel && !collapsed && "flex-1"
                          )}
                        >
                          {item.label}
                        </span>
                        {item.external && !collapsed ? (
                          <ExternalLink
                            className="h-3.5 w-3.5 shrink-0 text-current opacity-70"
                            aria-hidden="true"
                          />
                        ) : null}
                        {showPendingBadge ? (
                          <span className="ml-auto rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-700">
                            {pendingCount}
                          </span>
                        ) : null}
                      </>
                    );
                    const content = (
                      <>
                        {collapsed || !hasChildren ? (
                          item.external ? (
                            <a
                              href={sectionHref}
                              target="_blank"
                              rel="noopener noreferrer"
                              onClick={() => {
                                if (isMobile) setIsMobileOpen(false);
                              }}
                              className={cn(
                                isTopLevel
                                  ? menuButtonClasses
                                  : subMenuButtonClasses,
                                isTopLevel &&
                                  collapsed &&
                                  "justify-center gap-0 px-2",
                                !isTopLevel && item.icon && "gap-2",
                                !isTopLevel && depth > 1 && "pl-6"
                              )}
                              aria-label={linkLabel}
                              title={collapsed ? item.label : undefined}
                            >
                              {linkContent}
                            </a>
                          ) : (
                            <Link
                              href={sectionHref}
                              className={cn(
                                isTopLevel
                                  ? menuButtonClasses
                                  : subMenuButtonClasses,
                                isActive && activeMenuClasses,
                                isTopLevel &&
                                  collapsed &&
                                  "justify-center gap-0 px-2",
                                !isTopLevel && item.icon && "gap-2",
                                !isTopLevel && depth > 1 && "pl-6"
                              )}
                              aria-label={linkLabel}
                            >
                              {linkContent}
                            </Link>
                          )
                        ) : (
                          <div
                            className={cn(
                              isTopLevel
                                ? menuButtonClasses
                                : subMenuButtonClasses,
                              isActive && activeMenuClasses,
                              !isTopLevel && depth > 1 && "pl-6"
                            )}
                          >
                            <Link
                              href={sectionHref}
                              className={cn(
                                "flex flex-1 items-center text-left",
                                isTopLevel ? "gap-3" : "gap-2"
                              )}
                            >
                              {item.icon ? (
                                <item.icon className={iconClasses} />
                              ) : null}
                              <span className="flex-1">{item.label}</span>
                              {showPendingBadge ? (
                                <span className="ml-auto rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-700">
                                  {pendingCount}
                                </span>
                              ) : null}
                            </Link>
                            <button
                              type="button"
                              onClick={() => toggleSection(sectionKey)}
                              aria-label={`${item.label} submenu`}
                              className="flex items-center"
                            >
                              <ChevronDown
                                className={cn(
                                  "h-4 w-4 text-current transition",
                                  isOpen && "rotate-180"
                                )}
                              />
                            </button>
                          </div>
                        )}
                        {isOpen && !collapsed && hasChildren ? (
                          <SidebarMenuSub>
                            {item.children?.map((child) =>
                              renderSection(child, depth + 1, sectionKey)
                            )}
                          </SidebarMenuSub>
                        ) : null}
                      </>
                    );

                    if (isTopLevel) {
                      return (
                        <SidebarMenuItem key={wrapperKey}>
                          {content}
                        </SidebarMenuItem>
                      );
                    }
                    return (
                      <SidebarMenuSubItem key={wrapperKey}>
                        {content}
                      </SidebarMenuSubItem>
                    );
                  };

                  return renderSection(section, 0, "");
                })}
          </SidebarMenu>
        </SidebarGroup>
      </SidebarContent>
      <div
        className={cn(
          "border-sidebar-border/70 border-t px-4 py-3",
          collapsed && "px-2"
        )}
      >
        {canDownloadBundleReport ? (
          <div
            className={cn(
              "mb-2",
              collapsed
                ? "flex items-center justify-center gap-1"
                : "space-y-1.5"
            )}
          >
            <button
              type="button"
              onClick={() => void handleDownloadBundleReport(false)}
              disabled={isBundleProcessing}
              className={cn(
                "flex items-center gap-2 rounded-md bg-[#0F3D8F] px-3 py-2 text-xs font-semibold text-white transition hover:bg-[#12336e] disabled:cursor-not-allowed disabled:opacity-60",
                collapsed && "px-2"
              )}
              aria-label="Download PDF"
              title="Download PDF"
            >
              <FileDown className="h-4 w-4" />
              {!collapsed ? <span>Download PDF</span> : null}
            </button>
            <button
              type="button"
              onClick={() => void handleDownloadBundleReport(true)}
              disabled={isBundleProcessing}
              className={cn(
                "flex items-center gap-2 rounded-md border border-transparent px-2.5 py-1.5 text-[11px] font-medium text-slate-600 transition hover:bg-slate-100 hover:text-slate-800 disabled:cursor-not-allowed disabled:opacity-60",
                collapsed && "px-2"
              )}
              aria-label="Regenerate PDF"
              title="Generate ulang (bypass cache 1 jam)"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              {!collapsed ? <span>Regenerate</span> : null}
            </button>
          </div>
        ) : null}
        {isBundleProcessing && !collapsed ? (
          <p className="mb-2 text-[11px] text-slate-500">
            Membuat PDF... ({bundleJob?.progress ?? 0}%)
          </p>
        ) : null}
        {bundleError && !collapsed ? (
          <p className="mb-2 text-[11px] text-red-600">{bundleError}</p>
        ) : null}
        {status === "loading" ? (
          <div
            className={cn(
              "space-y-2",
              collapsed && "flex w-full flex-col items-center"
            )}
          >
            <div className="h-4 w-24 animate-pulse rounded bg-slate-200" />
            {!collapsed ? (
              <div className="h-3 w-16 animate-pulse rounded bg-slate-200" />
            ) : null}
          </div>
        ) : user ? (
          <div
            className={cn(
              "flex items-center justify-between gap-2",
              collapsed && "justify-center"
            )}
          >
            {!collapsed ? (
              <div className="min-w-0">
                <p className="text-sidebar-foreground truncate text-sm font-semibold">
                  {displayName || "User"}
                </p>
                <p className="text-sidebar-foreground/60 text-xs">
                  {user.role}
                </p>
              </div>
            ) : null}
            <div
              className={cn("flex items-center gap-1", collapsed && "flex-col")}
            >
              <button
                type="button"
                onClick={handleLogout}
                className={cn(
                  "text-sidebar-foreground/70 hover:text-sidebar-foreground flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors duration-200 hover:bg-slate-100",
                  collapsed && "p-2"
                )}
                aria-label="Logout"
              >
                <LogOut className="h-4 w-4" />
                {!collapsed ? <span>Logout</span> : null}
              </button>
            </div>
          </div>
        ) : (
          <Link
            href="/login"
            className={cn(
              menuButtonClasses,
              "justify-center",
              !collapsed && "justify-start",
              collapsed && "gap-0 px-2"
            )}
          >
            <LogIn className="text-sidebar-foreground/70 h-4 w-4" />
            {!collapsed ? <span>Login</span> : null}
          </Link>
        )}
      </div>
    </>
  );

  return (
    <>
      <div className="flex items-center justify-between border-b border-slate-200 bg-white px-4 py-3 md:hidden">
        <Link href="/" className="flex items-center gap-2">
          <Image
            src={LogoSipres}
            alt="Logo SIPRES"
            width={72}
            height={72}
            className="h-9 w-auto object-contain"
            priority
          />
          <div>
            <p className="text-sidebar-foreground text-sm leading-none font-semibold">
              SIPRES
            </p>
            <p className="text-sidebar-foreground/60 text-[11px]">
              Cabdin Bangkalan
            </p>
          </div>
        </Link>
        <button
          type="button"
          onClick={() => setIsMobileOpen(true)}
          className="text-sidebar-foreground/70 hover:text-sidebar-foreground rounded-md p-2 transition-colors duration-200 hover:bg-slate-100"
          aria-label="Buka menu"
        >
          <PanelLeftOpen className="h-5 w-5" />
        </button>
      </div>

      {isMobileOpen ? (
        <div className="fixed inset-0 z-50 md:hidden">
          <button
            type="button"
            onClick={() => setIsMobileOpen(false)}
            className="absolute inset-0 bg-black/40"
            aria-label="Tutup menu"
          />
          <Sidebar className="absolute inset-y-0 left-0 z-50 h-svh w-72 max-w-[85vw]">
            {renderSidebarContent(false, true)}
          </Sidebar>
        </div>
      ) : null}

      <Sidebar
        className={cn(
          "hidden transition-[width] duration-200 md:sticky md:top-0 md:flex",
          isCollapsed ? "md:w-20" : "md:w-72"
        )}
      >
        {renderSidebarContent(isCollapsed)}
      </Sidebar>
    </>
  );
}
