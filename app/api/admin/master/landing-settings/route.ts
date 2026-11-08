import { proxyToBackend } from "@/lib/api/proxy";

export const dynamic = "force-dynamic";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/landing-settings");

export const PUT = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/landing-settings");
