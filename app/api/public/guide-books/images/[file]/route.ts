import { proxyToBackend } from "@/lib/api/proxy";

export async function GET(
  request: Request,
  context: { params: Promise<{ file: string }> }
) {
  const { file } = await context.params;
  return proxyToBackend(
    request,
    `/api/public/guide-books/images/${encodeURIComponent(file)}`
  );
}
