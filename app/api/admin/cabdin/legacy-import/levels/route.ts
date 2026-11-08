import { proxyToBackend } from "@/lib/api/proxy";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/admin/cabdin/legacy-import/levels");

export const PUT = (request: Request) =>
  proxyToBackend(request, "/api/admin/cabdin/legacy-import/levels");

export const PATCH = (request: Request) =>
  proxyToBackend(request, "/api/admin/cabdin/legacy-import/levels");
