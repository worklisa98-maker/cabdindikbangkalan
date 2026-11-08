/*
//const TOKEN_COOKIE = "cabdin_token";

const HOP_BY_HOP_HEADERS = new Set([
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
  "host",
  "content-length",
]);

const parseTokenFromCookie = (cookieHeader: string) => {
  if (!cookieHeader) return null;
  const match = cookieHeader.match(
    new RegExp(`(?:^|; )${TOKEN_COOKIE}=([^;]*)`)
  );
  return match ? decodeURIComponent(match[1]) : null;
};

const buildForwardHeaders = (request: Request) => {
  const headers = new Headers();
  request.headers.forEach((value, key) => {
    if (HOP_BY_HOP_HEADERS.has(key.toLowerCase())) {
      return;
    }
    headers.set(key, value);
  });

  if (!headers.has("authorization")) {
    const token = parseTokenFromCookie(request.headers.get("cookie") ?? "");
    if (token) {
      headers.set("authorization", `Bearer ${token}`);
    }
  }

  return headers;
};

export const runtime = "nodejs";

export const GET = async (request: Request) => {
  const baseUrl = process.env.BACKEND_URL ?? "";
  const normalizedBaseUrl = baseUrl.trim().replace(/\/$/, "");
  if (!normalizedBaseUrl) {
    return new Response(
      JSON.stringify({
        error: "BACKEND_URL_NOT_CONFIGURED",
        message: "BACKEND_URL belum dikonfigurasi.",
      }),
      {
        status: 500,
        headers: { "Content-Type": "application/json" },
      }
    );
  }

  try {
    const url = new URL(request.url);
    const searchParams = new URLSearchParams(url.search);
    if (!searchParams.get("page")) {
      searchParams.set("page", "1");
    }
    if (!searchParams.get("pageSize")) {
      searchParams.set("pageSize", "10");
    }

    const targetUrl = `${normalizedBaseUrl}/api/histori?${searchParams.toString()}`;
    const response = await fetch(targetUrl, {
      method: "GET",
      headers: buildForwardHeaders(request),
      cache: "no-store",
    });
    
    return new Response(response.body, {
      status: response.status,
      headers: response.headers,
    }); 
    // baru
    const outHeaders = new Headers();
    response.headers.forEach((value, key) => {
      if (HOP_BY_HOP_HEADERS.has(key.toLowerCase())) return;
      // Kadang Next sensitif dengan header ini juga saat streaming
      if (key.toLowerCase() === "content-length") return;
      outHeaders.set(key, value);
    });

    // Pastikan content-type ada agar client gak bingung
    if (!outHeaders.has("content-type")) {
      outHeaders.set("content-type", "application/json");
    }

    const buf = await response.arrayBuffer();
    return new Response(buf, {
      status: response.status,
      headers: outHeaders,
    });


  } catch (error) {
    console.error("[api/histori] proxy failed", error);
    return new Response(
      JSON.stringify({
        error: "BACKEND_REQUEST_FAILED",
        message: "Gagal menghubungi backend.",
      }),
      {
        status: 502,
        headers: { "Content-Type": "application/json" },
      }
    );
  }
};*/
import { proxyToBackend } from "@/lib/api/proxy";

export const runtime = "nodejs";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/histori");

