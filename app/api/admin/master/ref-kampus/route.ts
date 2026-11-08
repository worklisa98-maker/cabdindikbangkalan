import { proxyToBackend } from "@/lib/api/proxy";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/ref-kampus");

export const POST = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/ref-kampus");
