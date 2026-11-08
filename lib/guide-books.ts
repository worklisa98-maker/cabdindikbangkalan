export const GUIDE_BOOK_PLACEMENTS = {
  NONE: "",
  LOMBA_POSTER_KARTINI: "LOMBA_POSTER_KARTINI",
} as const;

export const GUIDE_BOOK_LAYOUT_TYPES = {
  TEXT_LEFT_IMAGE_RIGHT: "TEXT_LEFT_IMAGE_RIGHT",
  IMAGE_LEFT_TEXT_RIGHT: "IMAGE_LEFT_TEXT_RIGHT",
  IMAGE_LEFT_IMAGE_RIGHT: "IMAGE_LEFT_IMAGE_RIGHT",
  FULL_TEXT: "FULL_TEXT",
  FULL_IMAGE: "FULL_IMAGE",
  TEXT_TOP_TWO_IMAGES: "TEXT_TOP_TWO_IMAGES",
} as const;

export const GUIDE_BOOK_COLUMN_RATIOS = {
  EQUAL: "1:1",
  TEXT_WIDE: "3:2",
  IMAGE_WIDE: "2:3",
} as const;

export type GuideBookPlacementKey =
  (typeof GUIDE_BOOK_PLACEMENTS)[keyof typeof GUIDE_BOOK_PLACEMENTS];

export type GuideBookLayoutType =
  (typeof GUIDE_BOOK_LAYOUT_TYPES)[keyof typeof GUIDE_BOOK_LAYOUT_TYPES];

export type GuideBookColumnRatio =
  (typeof GUIDE_BOOK_COLUMN_RATIOS)[keyof typeof GUIDE_BOOK_COLUMN_RATIOS];

export type GuideBookSlideItem = {
  id: number;
  layoutType: GuideBookLayoutType;
  textHtml: string;
  image1Url: string;
  image2Url: string;
  columnRatio: GuideBookColumnRatio;
  sortOrder: number;
  createdAt?: string;
  updatedAt?: string;
};

export type GuideBookItem = {
  id: number;
  role: string;
  title: string;
  contentHtml: string;
  sortOrder: number;
  placementKey: GuideBookPlacementKey;
  isPublic: boolean;
  slides: GuideBookSlideItem[];
  updatedAt: string;
  isActive?: boolean;
  createdAt?: string;
};

export const guideBookPlacementOptions: Array<{
  label: string;
  value: GuideBookPlacementKey;
}> = [
  { label: "Tanpa placement", value: GUIDE_BOOK_PLACEMENTS.NONE },
  {
    label: "Lomba Poster Digital Hari Kartini",
    value: GUIDE_BOOK_PLACEMENTS.LOMBA_POSTER_KARTINI,
  },
];

export const guideBookLayoutOptions: Array<{
  label: string;
  value: GuideBookLayoutType;
}> = [
  {
    label: "Teks kiri, gambar kanan",
    value: GUIDE_BOOK_LAYOUT_TYPES.TEXT_LEFT_IMAGE_RIGHT,
  },
  {
    label: "Gambar kiri, teks kanan",
    value: GUIDE_BOOK_LAYOUT_TYPES.IMAGE_LEFT_TEXT_RIGHT,
  },
  {
    label: "Dua gambar berdampingan",
    value: GUIDE_BOOK_LAYOUT_TYPES.IMAGE_LEFT_IMAGE_RIGHT,
  },
  {
    label: "Teks penuh",
    value: GUIDE_BOOK_LAYOUT_TYPES.FULL_TEXT,
  },
  {
    label: "Gambar penuh",
    value: GUIDE_BOOK_LAYOUT_TYPES.FULL_IMAGE,
  },
  {
    label: "Teks atas, dua gambar bawah",
    value: GUIDE_BOOK_LAYOUT_TYPES.TEXT_TOP_TWO_IMAGES,
  },
];

export const guideBookColumnRatioOptions: Array<{
  label: string;
  value: GuideBookColumnRatio;
}> = [
  { label: "50 : 50", value: GUIDE_BOOK_COLUMN_RATIOS.EQUAL },
  { label: "60 : 40", value: GUIDE_BOOK_COLUMN_RATIOS.TEXT_WIDE },
  { label: "40 : 60", value: GUIDE_BOOK_COLUMN_RATIOS.IMAGE_WIDE },
];

export const guideBookPlacementLabels: Record<string, string> = {
  [GUIDE_BOOK_PLACEMENTS.NONE]: "Tanpa placement",
  [GUIDE_BOOK_PLACEMENTS.LOMBA_POSTER_KARTINI]:
    "Lomba Poster Digital Hari Kartini",
};

export const guideBookLayoutUsesText = (layoutType: GuideBookLayoutType) =>
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.TEXT_LEFT_IMAGE_RIGHT ||
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.IMAGE_LEFT_TEXT_RIGHT ||
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.FULL_TEXT ||
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.TEXT_TOP_TWO_IMAGES;

export const guideBookLayoutUsesImage1 = (layoutType: GuideBookLayoutType) =>
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.TEXT_LEFT_IMAGE_RIGHT ||
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.IMAGE_LEFT_TEXT_RIGHT ||
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.IMAGE_LEFT_IMAGE_RIGHT ||
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.FULL_IMAGE ||
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.TEXT_TOP_TWO_IMAGES;

export const guideBookLayoutUsesImage2 = (layoutType: GuideBookLayoutType) =>
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.IMAGE_LEFT_IMAGE_RIGHT ||
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.TEXT_TOP_TWO_IMAGES;

export const guideBookLayoutUsesColumnRatio = (
  layoutType: GuideBookLayoutType
) =>
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.TEXT_LEFT_IMAGE_RIGHT ||
  layoutType === GUIDE_BOOK_LAYOUT_TYPES.IMAGE_LEFT_TEXT_RIGHT;
