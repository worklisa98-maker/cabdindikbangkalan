"use client";

import NextImage from "next/image";

import {
  GUIDE_BOOK_COLUMN_RATIOS,
  GUIDE_BOOK_LAYOUT_TYPES,
  type GuideBookSlideItem,
} from "@/lib/guide-books";

type GuideBookSlideRendererProps = {
  slide: GuideBookSlideItem;
  className?: string;
};

const richTextClasses =
  "max-w-none text-sm leading-6 text-zinc-800 [&_a]:text-blue-700 [&_a]:underline [&_blockquote]:my-3 [&_blockquote]:border-l-4 [&_blockquote]:border-zinc-300 [&_blockquote]:pl-3 [&_code]:rounded [&_code]:bg-zinc-100 [&_code]:px-1 [&_code]:py-0.5 [&_h1]:mt-4 [&_h1]:text-2xl [&_h1]:font-semibold [&_h2]:mt-4 [&_h2]:text-xl [&_h2]:font-semibold [&_h3]:mt-4 [&_h3]:text-lg [&_h3]:font-semibold [&_li]:my-1 [&_ol]:my-2 [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-2 [&_pre]:my-3 [&_pre]:overflow-x-auto [&_pre]:rounded [&_pre]:bg-zinc-900 [&_pre]:p-3 [&_pre]:text-zinc-100 [&_ul]:my-2 [&_ul]:list-disc [&_ul]:pl-6";

const imageFrameClasses =
  "flex min-h-48 items-center justify-center overflow-hidden rounded-2xl border border-zinc-200 bg-zinc-50 p-3";

const getColumnTemplate = (ratio: GuideBookSlideItem["columnRatio"]) => {
  switch (ratio) {
    case GUIDE_BOOK_COLUMN_RATIOS.TEXT_WIDE:
      return "md:grid-cols-[1.2fr_0.8fr]";
    case GUIDE_BOOK_COLUMN_RATIOS.IMAGE_WIDE:
      return "md:grid-cols-[0.8fr_1.2fr]";
    default:
      return "md:grid-cols-2";
  }
};

const TextBlock = ({ html }: { html: string }) => (
  <div className={richTextClasses} dangerouslySetInnerHTML={{ __html: html }} />
);

const ImageBlock = ({
  src,
  alt,
  full = false,
}: {
  src: string;
  alt: string;
  full?: boolean;
}) => (
  <div
    className={
      full ? `${imageFrameClasses} min-h-[24rem] p-4` : imageFrameClasses
    }
  >
    {src ? (
      <NextImage
        unoptimized
        width={1200}
        height={900}
        src={src}
        alt={alt}
        className={
          full
            ? "max-h-[70vh] w-full object-contain"
            : "h-full w-full object-contain"
        }
      />
    ) : (
      <div className="text-sm text-zinc-400">Tidak ada gambar</div>
    )}
  </div>
);

export function GuideBookSlideRenderer({
  slide,
  className = "",
}: GuideBookSlideRendererProps) {
  switch (slide.layoutType) {
    case GUIDE_BOOK_LAYOUT_TYPES.TEXT_LEFT_IMAGE_RIGHT:
      return (
        <div
          className={`grid gap-5 rounded-2xl bg-white p-6 ${getColumnTemplate(
            slide.columnRatio
          )} ${className}`.trim()}
        >
          <TextBlock html={slide.textHtml} />
          <ImageBlock src={slide.image1Url} alt="Juknis slide image" />
        </div>
      );

    case GUIDE_BOOK_LAYOUT_TYPES.IMAGE_LEFT_TEXT_RIGHT:
      return (
        <div
          className={`grid gap-5 rounded-2xl bg-white p-6 ${getColumnTemplate(
            slide.columnRatio
          )} ${className}`.trim()}
        >
          <ImageBlock src={slide.image1Url} alt="Juknis slide image" />
          <TextBlock html={slide.textHtml} />
        </div>
      );

    case GUIDE_BOOK_LAYOUT_TYPES.IMAGE_LEFT_IMAGE_RIGHT:
      return (
        <div
          className={`grid gap-5 rounded-2xl bg-white p-6 md:grid-cols-2 ${className}`.trim()}
        >
          <ImageBlock src={slide.image1Url} alt="Juknis slide image left" />
          <ImageBlock src={slide.image2Url} alt="Juknis slide image right" />
        </div>
      );

    case GUIDE_BOOK_LAYOUT_TYPES.FULL_IMAGE:
      return (
        <div className={`rounded-2xl bg-white p-6 ${className}`.trim()}>
          <ImageBlock src={slide.image1Url} alt="Juknis slide image" full />
        </div>
      );

    case GUIDE_BOOK_LAYOUT_TYPES.TEXT_TOP_TWO_IMAGES:
      return (
        <div
          className={`space-y-5 rounded-2xl bg-white p-6 ${className}`.trim()}
        >
          <TextBlock html={slide.textHtml} />
          <div className="grid gap-5 md:grid-cols-2">
            <ImageBlock src={slide.image1Url} alt="Juknis slide image left" />
            <ImageBlock src={slide.image2Url} alt="Juknis slide image right" />
          </div>
        </div>
      );

    case GUIDE_BOOK_LAYOUT_TYPES.FULL_TEXT:
    default:
      return (
        <div className={`rounded-2xl bg-white p-6 ${className}`.trim()}>
          <TextBlock html={slide.textHtml} />
        </div>
      );
  }
}
