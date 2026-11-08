import { NextRequest } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const XLSX_CONTENT_TYPE =
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";

export async function POST(request: NextRequest) {
  const backendBaseUrl = (process.env.BACKEND_URL ?? "").replace(/\/$/, "");
  if (!backendBaseUrl) {
    return new Response(
      JSON.stringify({ error: "BACKEND_URL_NOT_CONFIGURED" }),
      { status: 500, headers: { "Content-Type": "application/json" } }
    );
  }

  let body: unknown;
  try {
    body = await request.json();
  } catch {
    body = {};
  }

  const headers = new Headers();
  const authHeader = request.headers.get("authorization");
  if (authHeader) {
    headers.set("authorization", authHeader);
  }
  headers.set("content-type", "application/json");

  const targetUrl = `${backendBaseUrl}/admin/school-admins/bulk-generate?download=1`;

  const backendResponse = await fetch(targetUrl, {
    method: "POST",
    headers,
    body: JSON.stringify(body ?? {}),
  });

  const contentType = backendResponse.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    const json = await backendResponse.text();
    return new Response(json, {
      status: backendResponse.status,
      headers: { "Content-Type": "application/json" },
    });
  }

  const buffer = await backendResponse.arrayBuffer();
  return new Response(buffer, {
    status: backendResponse.status,
    headers: {
      "Content-Type": XLSX_CONTENT_TYPE,
      "Content-Disposition": 'attachment; filename="akun-admin-sekolah.xlsx"',
      "Cache-Control": "no-store",
    },
  });
}
