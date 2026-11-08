import { proxyToBackend } from "@/lib/api/proxy";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/user/me/ops-monitoring-schools");
