import { AUTH_MESSAGE_KEY, clearToken, getToken } from "@/lib/auth";
import { LOGOUT_BROADCAST_KEY } from "@/lib/idle-logout";

const readErrorPayload = async (response: Response): Promise<unknown> => {
  const clone = response.clone();
  const contentType = clone.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    try {
      return await clone.json();
    } catch {
      // Fall back to text below.
    }
  }
  try {
    return await clone.text();
  } catch {
    return "";
  }
};

const formatPayloadPreview = (payload: unknown, maxLength = 500) => {
  let preview = "";
  if (typeof payload === "string") {
    preview = payload;
  } else if (payload && typeof payload === "object") {
    try {
      preview = JSON.stringify(payload);
    } catch {
      preview = String(payload);
    }
  } else if (payload != null) {
    preview = String(payload);
  }

  const trimmed = preview.trim();
  if (!trimmed) return "";
  if (trimmed.length <= maxLength) return trimmed;
  return `${trimmed.slice(0, maxLength)}…`;
};

const buildApiPath = (path: string) => {
  if (path.startsWith("http://") || path.startsWith("https://")) {
    return path;
  }
  const normalized = path.startsWith("/") ? path : `/${path}`;
  if (normalized.startsWith("/api/")) {
    return normalized;
  }
  // Route non-/api paths through Next.js API handlers.
  return `/api${normalized}`;
};

export const apiFetch = async (
  path: string,
  init?: RequestInit,
  options?: {
    skipAuthRedirect?: boolean;
    suppressErrorStatuses?: number[];
  }
) => {
  const token = getToken() ?? "";
  const headers = new Headers(init?.headers ?? {});
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  /* lama
  if (!headers.has("Content-Type") && !(init?.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }*/
 /* baru */
 const method = (init?.method ?? "GET").toUpperCase();

  if (
    init?.body &&
    !(init.body instanceof FormData) &&
    !headers.has("Content-Type") &&
    method !== "GET" &&
    method !== "HEAD"
  ) {
    headers.set("Content-Type", "application/json");
  }


  const response = await fetch(buildApiPath(path), {
    ...init,
    headers,
  });

  if (response.status === 401 && !options?.skipAuthRedirect) {
    let errorCode: string | undefined;
    try {
      const payload = (await response.clone().json()) as { error?: string };
      errorCode = payload?.error;
    } catch {
      errorCode = undefined;
    }

    clearToken();
    if (typeof window !== "undefined") {
      if (errorCode === "TOKEN_REVOKED") {
        window.localStorage.setItem(
          AUTH_MESSAGE_KEY,
          "Sesi Anda berakhir. Silakan login ulang."
        );
      }
      window.localStorage.setItem(LOGOUT_BROADCAST_KEY, String(Date.now()));
      window.location.replace("/login");
    }
  }

  if (!response.ok) {
    const shouldSuppress =
      options?.suppressErrorStatuses?.includes(response.status) ?? false;
    if (!shouldSuppress) {
      const payload = await readErrorPayload(response);
      const preview = formatPayloadPreview(payload);
      console.error(
        `[apiFetch] ${path} failed status=${response.status}${
          preview ? ` payload=${preview}` : ""
        }`
      );
    }
  }

  return response;
};
