import { proxyToBackend } from "@/lib/api/proxy";

export const dynamic = "force-dynamic";

export const POST = async (request: Request) =>
  proxyToBackend(request, "/api/admin/system-time/reset");
