const TOKEN_COOKIE = "cabdin_token";

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

const getBackendBaseUrl = () => {
  const baseUrl = process.env.BACKEND_URL ?? "";
  return baseUrl.replace(/\/$/, "");
};

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

export const proxyToBackend = async (
  request: Request,
  backendPath: string,
  options?: { body?: BodyInit | null }
) => {
  const baseUrl = getBackendBaseUrl();
  if (!baseUrl) {
    console.error(
      "[proxyToBackend] BACKEND_URL_NOT_CONFIGURED",
      request.method,
      new URL(request.url).pathname
    );
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

  const normalizedPath = backendPath.startsWith("/")
    ? backendPath
    : `/${backendPath}`;
  const { search } = new URL(request.url);
  const targetUrl = `${baseUrl}${normalizedPath}${search}`;
  console.log(
    "[proxyToBackend]",
    request.method,
    new URL(request.url).pathname,
    "->",
    targetUrl
  );

  const init: RequestInit & { duplex?: "half" } = {
    method: request.method,
    headers: buildForwardHeaders(request),
  };
  if (request.method !== "GET" && request.method !== "HEAD") {
    if (options?.body !== undefined) {
      init.body = options.body;
    } else {
      init.body = request.body;
      init.duplex = "half";
    }
  }

  try {
    return await fetch(targetUrl, init);
  } catch (error) {
    console.error("[proxyToBackend] failed", error);
    const payload: Record<string, unknown> = { error: "BACKEND_REQUEST_FAILED" };
    if (process.env.NODE_ENV !== "production") {
      payload.details = String(error);
      payload.targetUrl = targetUrl;
    }
    return new Response(JSON.stringify(payload), {
      status: 502,
      headers: { "Content-Type": "application/json" },
    });
  }
};
