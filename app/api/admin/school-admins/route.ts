import { proxyToBackend } from "@/lib/api/proxy";

export const GET = (request: Request) =>
  proxyToBackend(request, "/admin/school-admins");

export const POST = (request: Request) =>
  proxyToBackend(request, "/admin/school-admins");
