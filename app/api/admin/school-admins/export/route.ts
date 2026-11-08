/* awal
import { proxyToBackend } from "@/lib/api/proxy";

export const POST = (request: Request) =>
  proxyToBackend(request, "/admin/school-admins/export");
*/

import { proxyToBackend } from "@/lib/api/proxy";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function POST(request: Request) {
  return proxyToBackend(request, "/admin/school-admins/export");
}
