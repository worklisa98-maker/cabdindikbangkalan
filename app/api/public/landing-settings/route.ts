import { proxyToBackend } from "@/lib/api/proxy";

export const dynamic = "force-dynamic";

export const GET = (request: Request) =>
  proxyToBackend(request, "/api/public/landing-settings");
