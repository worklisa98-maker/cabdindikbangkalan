const normalizePathname = (pathname: string) => {
  const trimmed = pathname.trim();
  if (!trimmed) {
    return "/";
  }

  const normalized = trimmed.replace(/\/+$/, "");
  return normalized || "/";
};

const gaspolSchoolAllowedPagePaths = new Set([
  "/gaspol",
  "/gaspol/voting",
  "/user/settings",
  "/login",
  "/pusat-tautan",
  "/halo-kepegawaian",
  "/sinom",
]);

const gaspolSchoolAllowedSidebarSections = new Set([
  "gaspol",
  "halo-kepegawaian",
  "pusat-tautan",
  "sinom",
  "settings",
]);

const gaspolSchoolAllowedApiExactPaths = new Set([
  "/api/auth/me",
  "/api/auth/logout",
  "/api/user/me/password",
]);

const gaspolSchoolReadOnlyApiExactPaths = new Set([
  "/api/system-time",
  "/api/system-time/status",
  "/api/service-links",
]);

const gaspolSchoolAllowedApiPrefixes = ["/api/gaspol/", "/api/public/gaspol/"];

export const isGaspolSchoolUserAllowedPage = (pathname: string) =>
  gaspolSchoolAllowedPagePaths.has(normalizePathname(pathname));

export const isGaspolSchoolUserAllowedSidebarSection = (sectionKey: string) =>
  gaspolSchoolAllowedSidebarSections.has(sectionKey);

export const isGaspolSchoolUserAllowedApiRequest = (
  method: string,
  pathname: string
) => {
  const normalizedPathname = normalizePathname(pathname);
  const normalizedMethod = method.trim().toUpperCase() || "GET";

  if (gaspolSchoolAllowedApiExactPaths.has(normalizedPathname)) {
    return true;
  }

  if (
    normalizedMethod === "GET" &&
    gaspolSchoolReadOnlyApiExactPaths.has(normalizedPathname)
  ) {
    return true;
  }

  return gaspolSchoolAllowedApiPrefixes.some(
    (prefix) =>
      normalizedPathname === prefix.slice(0, -1) ||
      normalizedPathname.startsWith(prefix)
  );
};

export const isGaspolSchoolUserAllowedApiPath = (pathname: string) => {
  return isGaspolSchoolUserAllowedApiRequest("GET", pathname);
};
