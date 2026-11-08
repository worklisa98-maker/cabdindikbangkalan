import { proxyToBackend } from "@/lib/api/proxy";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/user/settings");

export const PUT = (request: Request) =>
  proxyToBackend(request, "/api/user/settings");
