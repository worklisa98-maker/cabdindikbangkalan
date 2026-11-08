import type { NextRequest } from "next/server";

import { proxyToBackend } from "@/lib/api/proxy";

export const runtime = "nodejs";

export const DELETE = async (
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) => {
  const { id } = await params;
  return proxyToBackend(request, `/api/histori/${id}`);
};
