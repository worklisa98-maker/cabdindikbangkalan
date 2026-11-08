import Image from "next/image";
import Link from "next/link";
import type { LandingSettings } from "@/lib/landing-settings";
export function PortalFooter({ settings }: { settings?: LandingSettings }) {
  const clean = (value?: string) =>
    value && !/xxxx|example\./i.test(value) ? value : null;
  const contacts = [
    clean(settings?.contact_address),
    clean(settings?.contact_phone),
    clean(settings?.contact_whatsapp),
    clean(settings?.contact_email),
    clean(settings?.contact_hours),
  ].filter(Boolean);
  return (
    <footer className="mt-4 bg-[#092640] text-white">
      <div className="portal-container grid gap-7 py-8 sm:grid-cols-2 lg:grid-cols-[1.4fr_1fr_1fr_1.25fr]">
        <div>
          <div className="flex items-center gap-3">
            <Image
              src="/logo-prov-jawa-timur.png"
              width={42}
              height={52}
              alt="Lambang Jawa Timur"
              className="h-12 w-10 object-contain"
            />
            <p className="text-sm leading-5 font-bold">
              Cabang Dinas Pendidikan
              <br />
              Wilayah Kabupaten Bangkalan
            </p>
          </div>
          <p className="mt-3 text-xs leading-5 text-blue-200">
            Pemerintah Provinsi Jawa Timur
          </p>
          <p className="mt-2 text-xs leading-5 text-blue-200 italic">
            Bersama sekolah, menuju generasi emas Bangkalan.
          </p>
        </div>
        <div>
          <h2 className="text-sm font-semibold">Navigasi</h2>
          <div className="mt-3 flex flex-col gap-2 text-xs text-blue-100">
            <Link href="/">Beranda</Link>
            <Link href="/akreditasi">Data pendidikan</Link>
            <Link href="/informasi-pelayanan-cabdin">Informasi pelayanan</Link>
            <Link href="/pusat-tautan">Pusat tautan</Link>
          </div>
        </div>
        <div>
          <h2 className="text-sm font-semibold">Program & Layanan</h2>
          <div className="mt-3 flex flex-col gap-2 text-xs text-blue-100">
            <Link href="/prestasi">SIPRES / Prestasi</Link>
            <Link href="/gaspol">GASPOL BANG</Link>
            <Link href="/mgmp">MGMP & JARKOM SMK</Link>
            <Link href="/pengaduan-kritik-saran">
              Kritik, saran & pengaduan
            </Link>
            {settings?.guidebook_url && (
              <Link href={settings.guidebook_url}>Panduan layanan</Link>
            )}
          </div>
        </div>
        <div>
          <h2 className="text-sm font-semibold">Kontak Kami</h2>
          <div className="mt-3 space-y-2 text-xs leading-5 text-blue-100">
            {contacts.length ? (
              contacts.map((value, index) => (
                <p key={index} className="break-words">
                  {value}
                </p>
              ))
            ) : (
              <p>Informasi kontak belum tersedia.</p>
            )}
          </div>
        </div>
      </div>
      <div className="border-t border-white/10">
        <div className="portal-container py-4 text-[11px] text-blue-200">
          © {new Date().getFullYear()} Cabang Dinas Pendidikan Wilayah Kabupaten
          Bangkalan. Seluruh hak dilindungi.
        </div>
      </div>
    </footer>
  );
}
