import { proxyToBackend } from "@/lib/api/proxy";
type Context = { params: Promise<{ id: string }> };
export const GET = async (request: Request, context: Context) => {
  const { id } = await context.params;
  return proxyToBackend(
    request,
    `/api/schools/${encodeURIComponent(id)}/location`
  );
};
export const PUT = async (request: Request, context: Context) => {
  const { id } = await context.params;
  return proxyToBackend(
    request,
    `/api/schools/${encodeURIComponent(id)}/location`
  );
};
