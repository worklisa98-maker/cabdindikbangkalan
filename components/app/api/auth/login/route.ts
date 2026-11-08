import { proxyToBackend } from "@/lib/api/proxy";

export const POST = (request: Request) =>
  proxyToBackend(request, "/auth/login");
