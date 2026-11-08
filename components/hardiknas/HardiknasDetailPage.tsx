"use client";

import NextImage from "next/image";

import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Download, ImageIcon, Loader2 } from "lucide-react";

import { EmptyState } from "@/components/empty-state";
import { PageHeader } from "@/components/page-header";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";

type HardiknasPublicDetail = {
  id: number;
  judulPoster: string;
  namaSiswa: string;
  namaSekolah: string;
  rombel: string;
  tanggalInput: string;
  imageUrl: string;
  downloadUrl: string;
  mediaAvailable?: boolean;
  mediaStatusLabel?: string;
};

type ApiEnvelope<T> = {
  data: T;
  error?: string;
  message?: string;
};

const fetchHardiknasDetail = async (
  entryId: number
): Promise<HardiknasPublicDetail | null> => {
  const response = await apiFetch(
    `/public/lomba-hardiknas/karya/${entryId}`,
    undefined,
    {
      skipAuthRedirect: true,
    }
  );
  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    throw new Error("Gagal memuat detail karya.");
  }
  const payload = (await response.json()) as ApiEnvelope<HardiknasPublicDetail>;
  return payload.data ?? null;
};

export function HardiknasDetailPage({ entryId }: { entryId: number }) {
  return <HardiknasDetailContent key={entryId} entryId={entryId} />;
}

function HardiknasDetailContent({ entryId }: { entryId: number }) {
  const [imageFailed, setImageFailed] = useState(false);
  const isValidId = Number.isInteger(entryId) && entryId > 0;
  const {
    data: detail,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ["hardiknas-detail", entryId],
    queryFn: () => fetchHardiknasDetail(entryId),
    enabled: isValidId,
  });

  if (!isValidId) {
    return (
      <EmptyState
        icon={<ImageIcon className="h-5 w-5" />}
        title="Karya tidak ditemukan."
        description="Tautan preview tidak valid atau karya sudah tidak tersedia."
      />
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <PageHeader
          eyebrow="Lomba Hari Kartini"
          title="Detail Karya Poster"
          description="Memuat detail karya..."
        />
        <div className="flex min-h-[40vh] items-center justify-center rounded-2xl border border-slate-200 bg-white">
          <Loader2 className="h-6 w-6 animate-spin text-slate-500" />
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <EmptyState
        icon={<ImageIcon className="h-5 w-5" />}
        title="Gagal memuat detail karya."
        description="Silakan coba lagi untuk membuka preview poster ini."
        action={
          <button
            type="button"
            onClick={() => refetch()}
            className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700"
          >
            Coba lagi
          </button>
        }
      />
    );
  }

  if (!detail) {
    return (
      <EmptyState
        icon={<ImageIcon className="h-5 w-5" />}
        title="Karya tidak ditemukan."
        description="Karya ini belum tersedia atau sudah dihapus dari sistem."
      />
    );
  }

  const mediaAvailable = detail.mediaAvailable !== false;
  const missingMediaMessage =
    detail.mediaStatusLabel ||
    "File poster tidak tersedia di storage. Peserta perlu mengunggah ulang.";

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow="Lomba Hari Kartini"
        title={detail.judulPoster}
        description="Halaman detail karya poster dapat diakses publik tanpa login."
        actions={
          <div className="flex flex-wrap gap-2">
            <Link
              href="/lomba-hardiknas"
              className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700"
            >
              Kembali ke Gallery
            </Link>
            <a
              href={detail.downloadUrl}
              download={mediaAvailable}
              aria-disabled={!mediaAvailable}
              onClick={(event) => {
                if (!mediaAvailable) {
                  event.preventDefault();
                }
              }}
              className={`inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold ${
                mediaAvailable
                  ? "bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] text-white"
                  : "cursor-not-allowed border border-slate-200 bg-slate-100 text-slate-500"
              }`}
            >
              <Download className="h-4 w-4" />
              Download Poster
            </a>
          </div>
        }
      />

      <div className="grid gap-6 xl:grid-cols-[1.5fr_0.8fr]">
        <Card className="overflow-hidden bg-white">
          <CardContent className="flex min-h-[60vh] items-center justify-center bg-slate-50 p-5">
            {!mediaAvailable || imageFailed ? (
              <div className="flex min-h-[30vh] w-full items-center justify-center rounded-xl border border-dashed border-slate-300 bg-white px-6 text-center text-sm text-slate-500">
                {missingMediaMessage}
              </div>
            ) : (
              <NextImage
                unoptimized
                width={1200}
                height={900}
                src={detail.imageUrl}
                alt={detail.judulPoster}
                className="max-h-[80vh] w-full object-contain"
                onError={() => setImageFailed(true)}
              />
            )}
          </CardContent>
        </Card>

        <Card className="bg-white">
          <CardContent className="space-y-4 p-5">
            {!mediaAvailable ? (
              <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm font-medium text-amber-900">
                {missingMediaMessage}
              </div>
            ) : null}
            <div className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                Judul Poster
              </p>
              <p className="text-base font-semibold text-slate-950">
                {detail.judulPoster}
              </p>
            </div>
            <div className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                Nama Siswa
              </p>
              <p className="text-sm text-slate-700">{detail.namaSiswa}</p>
            </div>
            <div className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                Nama Sekolah
              </p>
              <p className="text-sm text-slate-700">{detail.namaSekolah}</p>
            </div>
            <div className="space-y-1">
              <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                Rombel
              </p>
              <p className="text-sm text-slate-700">{detail.rombel || "-"}</p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
