import { proxyToBackend } from "@/lib/api/proxy";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/legacy-import/settings");

export const PUT = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/legacy-import/settings");

export const PATCH = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/legacy-import/settings");
