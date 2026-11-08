import { proxyToBackend } from "@/lib/api/proxy";
export const dynamic = "force-dynamic";
export const PUT = (request: Request, context: { params: Promise<{ name: string }> }) =>
  context.params.then(({ name }) => proxyToBackend(request, `/api/admin/master/service-links/external/${name}`));
