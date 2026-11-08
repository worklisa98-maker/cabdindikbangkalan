import { proxyToBackend } from "@/lib/api/proxy";

export const runtime = "nodejs";

export const GET = (request: Request) => proxyToBackend(request, "/api/uploads");

export const POST = (request: Request) =>
  proxyToBackend(request, "/api/uploads");
