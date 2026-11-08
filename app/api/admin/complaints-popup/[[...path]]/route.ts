import { proxyToBackend } from "@/lib/api/proxy";

const backendPath = async (params: Promise<{ path?: string[] }>) =>
  `/api/admin/complaints-popup/${((await params).path ?? []).join("/")}`.replace(
    /\/$/,
    ""
  );

export const GET = async (
  request: Request,
  context: { params: Promise<{ path?: string[] }> }
) => proxyToBackend(request, await backendPath(context.params));

export const POST = async (
  request: Request,
  context: { params: Promise<{ path?: string[] }> }
) => proxyToBackend(request, await backendPath(context.params));

export const PATCH = async (
  request: Request,
  context: { params: Promise<{ path?: string[] }> }
) => proxyToBackend(request, await backendPath(context.params));
