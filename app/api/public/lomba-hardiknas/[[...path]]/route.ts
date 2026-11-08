import { proxyToBackend } from "@/lib/api/proxy";

const buildBackendPath = async (
  paramsPromise: Promise<{ path?: string[] }>
) => {
  const { path } = await paramsPromise;
  const normalized = Array.isArray(path) && path.length > 0 ? path.join("/") : "";
  return normalized
    ? `/api/public/lomba-hardiknas/${normalized}`
    : "/api/public/lomba-hardiknas";
};

export const GET = async (
  request: Request,
  context: { params: Promise<{ path?: string[] }> }
) => proxyToBackend(request, await buildBackendPath(context.params));

export const POST = async (
  request: Request,
  context: { params: Promise<{ path?: string[] }> }
) => proxyToBackend(request, await buildBackendPath(context.params));
