import { proxyToBackend } from "@/lib/api/proxy";

export const runtime = "nodejs";

export const GET = async (
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) => {
  const { id } = await params;
  return proxyToBackend(request, `/api/uploads/${id}/rejected.xlsx`);
};
