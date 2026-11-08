import { proxyToBackend } from "@/lib/api/proxy";
const path = async (p: Promise<{ path?: string[] }>) =>
  `/api/admin/complaints/${((await p).path ?? []).join("/")}`.replace(
    /\/$/,
    ""
  );
export const GET = async (
  r: Request,
  c: { params: Promise<{ path?: string[] }> }
) => proxyToBackend(r, await path(c.params));
export const POST = async (
  r: Request,
  c: { params: Promise<{ path?: string[] }> }
) => proxyToBackend(r, await path(c.params));
export const PATCH = async (
  r: Request,
  c: { params: Promise<{ path?: string[] }> }
) => proxyToBackend(r, await path(c.params));
