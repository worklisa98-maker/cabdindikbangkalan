export type LandingSettings = {
  hero_title: string;
  hero_subtitle: string;
  hero_image_url: string;
  hero_image_alt: string;
  contact_whatsapp: string;
  contact_phone: string;
  contact_email: string;
  contact_address: string;
  contact_hours: string;
  guidebook_url: string;
};

export const defaultLandingSettings: LandingSettings = {
  hero_title: "Satu Sistem. Satu Data. Prestasi Terintegrasi.",
  hero_subtitle:
    "SIPRES membantu sekolah dan Cabdin memantau prestasi, rencana lulusan, dan analitik secara terstruktur.",
  hero_image_url: "",
  hero_image_alt: "Visual SIPRES",
  contact_whatsapp: "0812-xxxx-xxxx",
  contact_phone: "(031) xxx xxxx",
  contact_email: "cabdin.bangkalan@example.go.id",
  contact_address: "Cabang Dinas Pendidikan Wilayah Bangkalan",
  contact_hours: "Senin - Jumat, 08.00 - 15.00 WIB",
  guidebook_url: "",
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const readString = (
  source: Record<string, unknown>,
  key: keyof LandingSettings,
  fallback: string
) => {
  const value = source[key];
  if (typeof value === "string") {
    return value;
  }
  return fallback;
};

export const normalizeLandingSettings = (value: unknown): LandingSettings => {
  const source = isRecord(value) ? value : {};
  return {
    hero_title: readString(source, "hero_title", defaultLandingSettings.hero_title),
    hero_subtitle: readString(
      source,
      "hero_subtitle",
      defaultLandingSettings.hero_subtitle
    ),
    hero_image_url: readString(
      source,
      "hero_image_url",
      defaultLandingSettings.hero_image_url
    ),
    hero_image_alt: readString(
      source,
      "hero_image_alt",
      defaultLandingSettings.hero_image_alt
    ),
    contact_whatsapp: readString(
      source,
      "contact_whatsapp",
      defaultLandingSettings.contact_whatsapp
    ),
    contact_phone: readString(
      source,
      "contact_phone",
      defaultLandingSettings.contact_phone
    ),
    contact_email: readString(
      source,
      "contact_email",
      defaultLandingSettings.contact_email
    ),
    contact_address: readString(
      source,
      "contact_address",
      defaultLandingSettings.contact_address
    ),
    contact_hours: readString(
      source,
      "contact_hours",
      defaultLandingSettings.contact_hours
    ),
    guidebook_url: readString(
      source,
      "guidebook_url",
      defaultLandingSettings.guidebook_url
    ),
  };
};

export const mergeLandingSettings = (
  current: LandingSettings,
  patch: unknown
): LandingSettings => {
  const source = isRecord(patch) ? patch : {};
  return normalizeLandingSettings({
    ...current,
    ...source,
  });
};
