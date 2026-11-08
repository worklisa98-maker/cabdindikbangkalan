import { proxyToBackend } from "@/lib/api/proxy";

export const PATCH = (request: Request) =>
  proxyToBackend(request, "/api/user/me/password");
