import { proxyToBackend } from "@/lib/api/proxy";

export const GET = async (
  request: Request,
  { params }: { params: Promise<{ file: string }> }
) => {
  const { file } = await params;
  return proxyToBackend(request, `/api/guide-books/images/${file}`);
};
