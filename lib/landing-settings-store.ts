import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

import {
  defaultLandingSettings,
  mergeLandingSettings,
  normalizeLandingSettings,
  type LandingSettings,
} from "@/lib/landing-settings";

const DATA_DIR = path.join(process.cwd(), "data");
const LANDING_SETTINGS_PATH = path.join(DATA_DIR, "landing-settings.json");

export const readLandingSettings = async (): Promise<LandingSettings> => {
  try {
    const raw = await readFile(LANDING_SETTINGS_PATH, "utf8");
    return normalizeLandingSettings(JSON.parse(raw));
  } catch (error) {
    const code = (error as NodeJS.ErrnoException | undefined)?.code;
    if (code === "ENOENT") {
      return defaultLandingSettings;
    }
    console.error("[landing-settings] read failed", error);
    return defaultLandingSettings;
  }
};

export const writeLandingSettings = async (
  patch: unknown
): Promise<LandingSettings> => {
  const current = await readLandingSettings();
  const next = mergeLandingSettings(current, patch);
  await mkdir(DATA_DIR, { recursive: true });
  await writeFile(LANDING_SETTINGS_PATH, JSON.stringify(next, null, 2), "utf8");
  return next;
};
