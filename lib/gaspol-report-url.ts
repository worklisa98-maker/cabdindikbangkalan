export function normalizeGaspolReportURL(raw: string): string {
  const value = raw.trim();
  if (!value) return "";
  const candidate =
    !value.includes(":") && value.split("/")[0].includes(".")
      ? `https://${value}`
      : value;
  try {
    const u = new URL(candidate);
    if (
      !["http:", "https:"].includes(u.protocol) ||
      !u.hostname ||
      u.username ||
      u.password ||
      /\s/.test(candidate)
    )
      return "";
    return candidate;
  } catch {
    return "";
  }
}
