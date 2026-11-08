import type { NextRequest } from "next/server";
import { proxyToBackend } from "@/lib/api/proxy";

export const GET = async (
  request: NextRequest,
  { params }: { params: Promise<{ type: string }> }
) => {
  const { type } = await params;
  return proxyToBackend(request, `/templates/${type}`);
};
