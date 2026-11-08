export const SYSTEM_MAX_PT_CHOICES = 4;

export const PT_JALUR_ORDER = [
  "SNBP",
  "SNBT",
  "MANDIRI",
  "KEDINASAN",
  "LAINNYA",
] as const;

export type PtJalurKey = (typeof PT_JALUR_ORDER)[number];

export const normalizePtJalurKey = (value: string): PtJalurKey | "" => {
  const upper = value.trim().toUpperCase();
  if (!upper) return "";
  if (upper.includes("SNBP")) return "SNBP";
  if (upper.includes("SNBT")) return "SNBT";
  if (upper.includes("MANDIRI") || upper.includes("SNBM")) return "MANDIRI";
  if (upper.includes("KEDINAS")) return "KEDINASAN";
  if (upper.includes("LAIN")) return "LAINNYA";
  return "";
};

export const isValidPtChoiceSlot = (slot: number) =>
  slot >= 1 && slot <= SYSTEM_MAX_PT_CHOICES;

export const defaultPtChoiceRuleMap = (): Record<PtJalurKey, number> => ({
  SNBP: SYSTEM_MAX_PT_CHOICES,
  SNBT: SYSTEM_MAX_PT_CHOICES,
  MANDIRI: SYSTEM_MAX_PT_CHOICES,
  KEDINASAN: SYSTEM_MAX_PT_CHOICES,
  LAINNYA: SYSTEM_MAX_PT_CHOICES,
});
