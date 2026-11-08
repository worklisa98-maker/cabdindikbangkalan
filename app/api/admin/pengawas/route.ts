import { proxyToBackend } from "@/lib/api/proxy";

export const GET = (request: Request) =>
  proxyToBackend(request, "/admin/pengawas");

export const POST = (request: Request) =>
  proxyToBackend(request, "/admin/pengawas");
