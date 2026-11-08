import { proxyToBackend } from "@/lib/api/proxy";

export const runtime = "nodejs";

export const POST = (request: Request) =>
  proxyToBackend(request, "/api/siswa/validation/finalize-bulk");
