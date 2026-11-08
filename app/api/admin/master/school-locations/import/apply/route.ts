import { proxyToBackend } from "@/lib/api/proxy";
export const dynamic = "force-dynamic";
export const POST = (request: Request) =>
  proxyToBackend(request, "/api/admin/master/school-locations/import/apply");
