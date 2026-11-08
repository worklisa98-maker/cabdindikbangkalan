import type { NextRequest } from "next/server";
import { proxyToBackend } from "@/lib/api/proxy";

export const GET = async (
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) => {
  const { id } = await params;
  return proxyToBackend(request, `/admin/school-admins/${id}/initial-password`);
};
