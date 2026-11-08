import { proxyToBackend } from "@/lib/api/proxy";

export const runtime = "nodejs";

export const GET = async (request: Request) =>
  proxyToBackend(request, "/api/public/status-lulusan/analytics-pt");
