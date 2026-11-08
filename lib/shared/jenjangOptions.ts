export const jenjangOptions = ["SMA", "SMK", "SLB"] as const;
export type JenjangOption = (typeof jenjangOptions)[number];
