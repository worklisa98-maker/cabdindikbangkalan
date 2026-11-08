import { proxyToBackend } from "@/lib/api/proxy";

const buildBackendPath = async (paramsPromise: Promise<{ path: string[] }>) => {
  const { path } = await paramsPromise;
  const normalized = Array.isArray(path) ? path.join("/") : "";
  return `/api/gaspol/${normalized}`;
};

export const GET = async (
  request: Request,
  context: { params: Promise<{ path: string[] }> }
) => proxyToBackend(request, await buildBackendPath(context.params));

export const POST = async (
  request: Request,
  context: { params: Promise<{ path: string[] }> }
) => proxyToBackend(request, await buildBackendPath(context.params));

export const PUT = async (
  request: Request,
  context: { params: Promise<{ path: string[] }> }
) => proxyToBackend(request, await buildBackendPath(context.params));

export const PATCH = async (
  request: Request,
  context: { params: Promise<{ path: string[] }> }
) => proxyToBackend(request, await buildBackendPath(context.params));

export const DELETE = async (
  request: Request,
  context: { params: Promise<{ path: string[] }> }
) => proxyToBackend(request, await buildBackendPath(context.params));
