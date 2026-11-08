import { proxyToBackend } from "@/lib/api/proxy";

export const GET = async (request: Request) =>
  proxyToBackend(request, "/api/templates/ukk");
