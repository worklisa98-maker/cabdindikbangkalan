import { proxyToBackend } from "@/lib/api/proxy";
export const dynamic = "force-dynamic";
export const PUT = (request: Request) => proxyToBackend(request, "/api/admin/master/service-links/links");
