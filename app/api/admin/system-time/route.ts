import { proxyToBackend } from "@/lib/api/proxy";

export const dynamic = "force-dynamic";

export const GET = async (request: Request) =>
  proxyToBackend(request, "/api/admin/system-time");
