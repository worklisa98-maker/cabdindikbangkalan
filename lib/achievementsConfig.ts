import { z } from "zod";
import { jenjangOptions } from "@/lib/shared/jenjangOptions";

export const achievementCategories = ["Siswa", "Guru", "Tendik"] as const;
export type AchievementCategory = (typeof achievementCategories)[number];

export { jenjangOptions };
export type JenjangOption = (typeof jenjangOptions)[number];

export const tingkatOptions = [
  "Kabupaten",
  "Provinsi",
  "Nasional",
  "Internasional",
] as const;
export type TingkatOption = (typeof tingkatOptions)[number];

export const achievementChartColors: Record<AchievementCategory, string> = {
  Siswa: "#1d4ed8",
  Guru: "#0f766e",
  Tendik: "#b45309",
};

export const achievementSchema = z
  .object({
    kategoryDataPrestasi: z.enum(achievementCategories),
    tingkatDataPrestasi: z.string(),
  })
  .passthrough();

export type Achievement = z.infer<typeof achievementSchema>;
