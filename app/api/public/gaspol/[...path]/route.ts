import { proxyToBackend } from "@/lib/api/proxy";

const buildBackendPath = async (paramsPromise: Promise<{ path: string[] }>) => {
  const { path } = await paramsPromise;
  const normalized = Array.isArray(path) ? path.join("/") : "";
  return `/api/public/gaspol/${normalized}`;
};

export const GET = async (
  request: Request,
  context: { params: Promise<{ path: string[] }> }
) => proxyToBackend(request, await buildBackendPath(context.params));
