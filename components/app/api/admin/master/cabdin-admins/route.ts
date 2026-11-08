import { proxyToBackend } from "@/lib/api/proxy";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/cabdin-admins");

export const POST = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/cabdin-admins");
