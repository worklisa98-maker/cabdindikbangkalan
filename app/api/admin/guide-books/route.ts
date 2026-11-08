import { proxyToBackend } from "@/lib/api/proxy";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/admin/guide-books");

export const POST = (request: Request) =>
  proxyToBackend(request, "/api/admin/guide-books");
