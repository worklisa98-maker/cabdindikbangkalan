export function getFilenameFromContentDisposition(
  contentDisposition: string | null,
  fallback = "download.xlsx"
) {
  if (!contentDisposition) return fallback;

  const utf8Match = contentDisposition.match(/filename\*\s*=\s*UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1].trim());
    } catch {
      return utf8Match[1].trim();
    }
  }

  const quotedMatch = contentDisposition.match(/filename\s*=\s*"([^"]+)"/i);
  if (quotedMatch?.[1]) return quotedMatch[1].trim();

  const plainMatch = contentDisposition.match(/filename\s*=\s*([^;]+)/i);
  if (plainMatch?.[1]) return plainMatch[1].trim();

  return fallback;
}

const readString = (value: unknown) => {
  if (typeof value === "string") return value.trim();
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return "";
};

export async function getErrorMessageFromResponse(
  response: Response,
  fallback = "Terjadi kesalahan saat memproses permintaan."
) {
  const clone = response.clone();
  const contentType = clone.headers.get("content-type") ?? "";

  if (contentType.includes("application/json")) {
    const payload = (await clone.json().catch(() => null)) as
      | Record<string, unknown>
      | null;
    if (payload) {
      return (
        readString(payload.message) ||
        readString(payload.error_description) ||
        readString(payload.error) ||
        readString(payload.details) ||
        fallback
      );
    }
  }

  const text = await clone.text().catch(() => "");
  return text.trim() || fallback;
}
