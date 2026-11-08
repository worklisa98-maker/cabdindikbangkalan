import { proxyToBackend } from "@/lib/api/proxy";

export const PUT = async (
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) => {
  const { id } = await params;
  return proxyToBackend(request, `/api/admin/guide-books/${id}`);
};

export const DELETE = async (
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) => {
  const { id } = await params;
  return proxyToBackend(request, `/api/admin/guide-books/${id}`);
};
