import { proxyToBackend } from "@/lib/api/proxy";

export const PUT = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/legacy-import/levels");

export const PATCH = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/legacy-import/levels");
