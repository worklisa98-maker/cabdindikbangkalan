import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import {
  isGaspolSchoolUserAllowedApiRequest,
  isGaspolSchoolUserAllowedPage,
} from "@/lib/auth/gaspol-school-access";

const TOKEN_COOKIE = "cabdin_token";

const PUBLIC_PATHS = [
  "/",
  "/akreditasi",
  "/dashboard",
  "/prestasi",
  "/siswa-perguruanpn-tinggi",
  "/siswa/status",
  "/siswa/status/diterima-perguruan-tinggi",
  "/siswa/status/diterima-dunia-kerja",
  "/siswa/status/berwirausaha",
  "/siswa/status/belum-melanjutkan",
  "/login",
  "/pusat-tautan",
  "/halo-kepegawaian",
  "/sinom",
];

const PUBLIC_API_PATHS = [
  "/api/auth/login",
  "/api/auth/logout",
  "/api/meta/last-updated",
];

const PUBLIC_API_PREFIXES = ["/api/public", "/api/templates/"];

const PROTECTED_PREFIXES = [
  "/admin/settings/landing",
  "/internal",
  "/uploads",
  "/user",
  "/cuti",
  "/siswa/kelas-xii",
  "/siswa/pendaftaran",
  "/siswa/status/pending-validasi",
];

const isPublicPath = (pathname: string) =>
  PUBLIC_PATHS.some((path) => pathname === path);

const isAssetPath = (pathname: string) =>
  pathname.startsWith("/_next") ||
  pathname.startsWith("/favicon") ||
  pathname.startsWith("/icons") ||
  pathname.startsWith("/images") ||
  pathname.startsWith("/public");

const isPublicApiPath = (pathname: string) =>
  PUBLIC_API_PATHS.includes(pathname) ||
  PUBLIC_API_PREFIXES.some((prefix) => pathname.startsWith(prefix));

const isProtectedPath = (pathname: string) =>
  PROTECTED_PREFIXES.some(
    (prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`)
  );

const decodeTokenRole = (token?: string | null) => {
  if (!token) {
    return null;
  }
  const parts = token.split(".");
  if (parts.length < 2) {
    return null;
  }
  try {
    const normalized = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = normalized.padEnd(
      normalized.length + ((4 - (normalized.length % 4)) % 4),
      "="
    );
    const payload = JSON.parse(atob(padded)) as { role?: string } | null;
    return (payload?.role ?? "").trim().toUpperCase() || null;
  } catch {
    return null;
  }
};

export default function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const token = request.cookies.get(TOKEN_COOKIE)?.value;
  const role = decodeTokenRole(token);
  const isGaspolSchoolUser = role === "USER_SEKOLAH_GASPOL";

  if (isAssetPath(pathname) || isPublicPath(pathname)) {
    if (isGaspolSchoolUser && !pathname.startsWith("/api")) {
      if (!isGaspolSchoolUserAllowedPage(pathname)) {
        const gaspolUrl = request.nextUrl.clone();
        gaspolUrl.pathname = "/gaspol";
        gaspolUrl.search = "";
        return NextResponse.redirect(gaspolUrl);
      }
    }
    return NextResponse.next();
  }

  if (pathname.startsWith("/api") && isPublicApiPath(pathname)) {
    if (
      isGaspolSchoolUser &&
      !isGaspolSchoolUserAllowedApiRequest(request.method, pathname)
    ) {
      return NextResponse.json({ error: "FORBIDDEN" }, { status: 403 });
    }
    return NextResponse.next();
  }

  if (pathname.startsWith("/api")) {
    if (!token) {
      return NextResponse.json({ error: "UNAUTHORIZED" }, { status: 401 });
    }
    if (
      isGaspolSchoolUser &&
      !isGaspolSchoolUserAllowedApiRequest(request.method, pathname)
    ) {
      return NextResponse.json({ error: "FORBIDDEN" }, { status: 403 });
    }
    return NextResponse.next();
  }

  if (isGaspolSchoolUser && !isGaspolSchoolUserAllowedPage(pathname)) {
    const gaspolUrl = request.nextUrl.clone();
    gaspolUrl.pathname = "/gaspol";
    gaspolUrl.search = "";
    return NextResponse.redirect(gaspolUrl);
  }

  if (isProtectedPath(pathname)) {
    if (!token) {
      const loginUrl = request.nextUrl.clone();
      loginUrl.pathname = "/login";
      loginUrl.search = "";
      return NextResponse.redirect(loginUrl);
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
