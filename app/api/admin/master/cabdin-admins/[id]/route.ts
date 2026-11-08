import type { NextRequest } from "next/server";
import { proxyToBackend } from "@/lib/api/proxy";

export const PATCH = async (
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) => {
  const { id } = await params;
  return proxyToBackend(request, `/api/admin/master/cabdin-admins/${id}`);
};
