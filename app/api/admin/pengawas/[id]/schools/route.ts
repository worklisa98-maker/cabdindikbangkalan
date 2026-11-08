import type { NextRequest } from "next/server";
import { proxyToBackend } from "@/lib/api/proxy";

type Params = {
  params: Promise<{ id: string }>;
};

export const PUT = async (request: NextRequest, context: Params) => {
  const { id } = await context.params;
  return proxyToBackend(request, `/admin/pengawas/${id}/schools`);
};
