import { proxyToBackend } from "@/lib/api/proxy";

export const runtime = "nodejs";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/public/siswa/analytics/summary");
