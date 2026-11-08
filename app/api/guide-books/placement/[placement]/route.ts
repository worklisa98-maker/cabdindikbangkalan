import { proxyToBackend } from "@/lib/api/proxy";

export async function GET(
  request: Request,
  context: { params: Promise<{ placement: string }> }
) {
  const { placement } = await context.params;
  return proxyToBackend(
    request,
    `/api/guide-books/placement/${encodeURIComponent(placement)}`
  );
}
