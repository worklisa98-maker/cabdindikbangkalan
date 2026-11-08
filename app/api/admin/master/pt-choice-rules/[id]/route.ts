import { proxyToBackend } from "@/lib/api/proxy";

type RouteContext = {
  params: Promise<{ id: string }>;
};

export async function PATCH(request: Request, context: RouteContext) {
  const { id } = await context.params;
  return proxyToBackend(request, `/api/admin/master/pt-choice-rules/${id}`);
}
