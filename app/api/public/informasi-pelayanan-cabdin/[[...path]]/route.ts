import { proxyToBackend } from "@/lib/api/proxy";

export const runtime = "nodejs";

const toBackendPath = async (params: Promise<{ path?: string[] }>) => {
  const { path } = await params;
  const suffix = (path ?? []).join("/");
  return suffix
    ? `/api/public/informasi-pelayanan-cabdin/${suffix}`
    : "/api/public/informasi-pelayanan-cabdin";
};

export const GET = async (
  request: Request,
  { params }: { params: Promise<{ path?: string[] }> }
) => proxyToBackend(request, await toBackendPath(params));
