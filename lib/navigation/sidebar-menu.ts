import type { ComponentType } from "react";
import {
  BookOpen,
  BookUser,
  CalendarOff,
  ContactRound,
  ExternalLink,
  Link2,
  FileClock,
  FileText,
  GraduationCap,
  Image,
  Leaf,
  ListChecks,
  MessageSquareWarning,
  Network,
  School,
  Settings,
  DatabaseBackup,
  Trophy,
  Users,
} from "lucide-react";
import { isGaspolSchoolUserAllowedSidebarSection } from "@/lib/auth/gaspol-school-access";
import { getPrimaryAppRole, hasRoleAccess } from "@/lib/auth/role-scope";
import type { AppRole, AuthUser } from "@/lib/auth/types";

export type SidebarUser = Pick<AuthUser, "role"> | null;

export type SidebarRole = AppRole | "PUBLIC";

export type SidebarSection = {
  key: string;
  label: string;
  icon?: ComponentType<{ className?: string }>;
  href?: string;
  external?: boolean;
  children?: SidebarSection[];
  allowedRoles?: SidebarRole[];
  isEnabled?: boolean;
};

export const sidebarSections: SidebarSection[] = [
  {
    key: "mgmp",
    label: "MGMP & JARKOM SMK",
    icon: Network,
    href: "/mgmp",
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_CABDIN",
      "ADMIN_MASTER",
      "USER_MGMP",
    ],
    children: [
      {
        key: "monitoring",
        label: "Monitoring",
        href: "/mgmp",
        allowedRoles: ["ADMIN", "ADMIN_CABDIN", "ADMIN_MASTER", "USER_MGMP"],
      },
      {
        key: "pendaftaran",
        label: "Pendaftaran / Verval",
        href: "/mgmp/pendaftaran",
        allowedRoles: ["ADMIN", "ADMIN_CABDIN", "ADMIN_MASTER"],
      },
      {
        key: "registrasi-publik",
        label: "Daftar MGMP / JARKOM",
        href: "/mgmp/register",
        allowedRoles: ["PUBLIC"],
      },
      {
        key: "cek-status",
        label: "Cek Status Registrasi",
        href: "/mgmp/status",
        allowedRoles: ["PUBLIC"],
      },
      {
        key: "histori-mgmp",
        label: "Histori",
        href: "/mgmp/histori",
        allowedRoles: ["ADMIN", "ADMIN_CABDIN", "ADMIN_MASTER", "USER_MGMP"],
      },
      {
        key: "settings-mgmp",
        label: "Settings",
        icon: Settings,
        href: "/mgmp/settings",
        allowedRoles: ["ADMIN_MASTER"],
      },
    ],
  },
  {
    key: "backup-pemulihan",
    label: "Backup & Pemulihan Data",
    icon: DatabaseBackup,
    href: "/admin/backup",
    allowedRoles: ["ADMIN_MASTER"],
  },
  {
    key: "pengaduan-kritik-saran",
    label: "Kritik, Saran & Pengaduan",
    icon: MessageSquareWarning,
    href: "/pengaduan-kritik-saran",
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_CABDIN",
      "ADMIN_MASTER",
      "SCHOOL_ADMIN",
      "USER_SEKOLAH_GASPOL",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "cek-pengaduan",
    label: "Cek Pengaduan",
    href: "/pengaduan-kritik-saran?tab=LACAK",
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_CABDIN",
      "ADMIN_MASTER",
      "SCHOOL_ADMIN",
      "USER_SEKOLAH_GASPOL",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "akreditasi",
    label: "Akreditasi Sekolah",
    icon: School,
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_MASTER",
      "SCHOOL_ADMIN",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "prestasi",
    label: "Prestasi",
    icon: Trophy,
    href: "/prestasi",
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_MASTER",
      "SCHOOL_ADMIN",
      "PENGAWAS",
    ],
  },
  {
    key: "siswa",
    label: "Siswa",
    icon: BookUser,
    href: "/siswa",
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_MASTER",
      "SCHOOL_ADMIN",
      "PENGAWAS",
    ],
    children: [
      {
        key: "kelas-xii",
        label: "Data Siswa Kelas Akhir",
        icon: Users,
        href: "/siswa/kelas-akhir",
        allowedRoles: ["ADMIN", "ADMIN_MASTER", "SCHOOL_ADMIN", "PENGAWAS"],
      },
      {
        key: "rencana-lulusan",
        label: "Rencana Lulusan",
        icon: ListChecks,
        href: "/siswa/rencana-lulusan",
        allowedRoles: ["ADMIN", "ADMIN_MASTER", "SCHOOL_ADMIN", "PENGAWAS"],
      },
      {
        key: "pengumuman-pt",
        label: "Pengumuman Diterima PT",
        icon: GraduationCap,
        href: "/siswa/pengumuman-pt",
        allowedRoles: ["ADMIN", "ADMIN_MASTER", "SCHOOL_ADMIN", "PENGAWAS"],
      },
      {
        key: "status-lulusan",
        label: "Status Lulusan",
        icon: GraduationCap,
        href: "/siswa/status-lulusan",
        allowedRoles: ["ADMIN", "ADMIN_MASTER", "SCHOOL_ADMIN", "PENGAWAS"],
      },
      {
        key: "pending-validasi",
        label: "Pending Validasi",
        icon: FileClock,
        href: "/siswa/pending-validasi",
        allowedRoles: ["ADMIN", "ADMIN_MASTER", "SCHOOL_ADMIN", "PENGAWAS"],
      },
    ],
  },
  {
    key: "gaspol",
    label: "GASPOL BANG",
    icon: Leaf,
    href: "/gaspol",
    allowedRoles: [
      "PUBLIC",
      "GASPOL_VOTER",
      "ADMIN",
      "ADMIN_CABDIN",
      "SCHOOL_ADMIN",
      "USER_SEKOLAH_GASPOL",
      "ADMIN_MASTER",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "informasi-pelayanan-cabdin",
    label: "Informasi Pelayanan Cabdin",
    icon: FileText,
    href: "/informasi-pelayanan-cabdin",
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_CABDIN",
      "SCHOOL_ADMIN",
      "USER_SEKOLAH_GASPOL",
      "ADMIN_MASTER",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "halo-kepegawaian",
    label: "Halo Kepegawaian",
    icon: ContactRound,
    href: "/halo-kepegawaian",
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_CABDIN",
      "SCHOOL_ADMIN",
      "USER_SEKOLAH_GASPOL",
      "USER_MGMP",
      "ADMIN_MASTER",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "sinom",
    label: "SINOM",
    icon: ExternalLink,
    href: "/sinom",
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_CABDIN",
      "SCHOOL_ADMIN",
      "USER_SEKOLAH_GASPOL",
      "USER_MGMP",
      "ADMIN_MASTER",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "pusat-tautan",
    label: "Pusat Tautan",
    icon: Link2,
    href: "/pusat-tautan",
    allowedRoles: [
      "PUBLIC",
      "ADMIN",
      "ADMIN_CABDIN",
      "SCHOOL_ADMIN",
      "USER_SEKOLAH_GASPOL",
      "USER_MGMP",
      "ADMIN_MASTER",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "lomba-hardiknas",
    label: "Lomba Poster Digital Hari Kartini",
    icon: Image,
    href: "/lomba-hardiknas",
    allowedRoles: [
      "PUBLIC",
      "SCHOOL_ADMIN",
      "PENGAWAS",
      "ADMIN_CABDIN",
      "ADMIN_MASTER",
    ],
  },
  {
    key: "guide-books",
    label: "Panduan",
    icon: BookOpen,
    href: "/guide_books",
    allowedRoles: [
      "ADMIN",
      "ADMIN_MASTER",
      "SCHOOL_ADMIN",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "settings",
    label: "Settings",
    icon: Settings,
    href: "/user/settings",
    children: [
      {
        key: "settings-beranda",
        label: "Beranda",
        href: "/admin/settings/landing",
        allowedRoles: ["ADMIN_MASTER"],
      },
      {
        key: "settings-lokasi",
        label: "Lokasi Sekolah",
        href: "/user/settings#lokasi-sekolah",
        allowedRoles: ["ADMIN_MASTER"],
      },
    ],
    allowedRoles: [
      "ADMIN",
      "ADMIN_MASTER",
      "SCHOOL_ADMIN",
      "USER_SEKOLAH_GASPOL",
      "PENGAWAS",
      "VALIDATOR",
    ],
  },
  {
    key: "cuti",
    label: "Cuti",
    icon: CalendarOff,
    isEnabled: false,
    children: [
      { key: "guru", label: "Guru", href: "/cuti/guru" },
      { key: "tendik", label: "Tendik", href: "/cuti/tendik" },
      { key: "pengawas", label: "Pengawas", href: "/cuti/pengawas" },
      {
        key: "staff-cabang-dinas",
        label: "Staff Cabang Dinas",
        href: "/cuti/staff-cabang-dinas",
      },
    ],
  },
];

const publicRootSectionKeys = new Set([
  "cek-pengaduan",
  "mgmp",
  "pengaduan-kritik-saran",
  "akreditasi",
  "prestasi",
  "siswa",
  "gaspol",
  "informasi-pelayanan-cabdin",
  "halo-kepegawaian",
  "sinom",
  "pusat-tautan",
  "lomba-hardiknas",
]);

const isSectionAllowed = (section: SidebarSection, user: SidebarUser) => {
  if (section.allowedRoles == null || section.allowedRoles.length === 0) {
    return true;
  }
  return hasRoleAccess(getPrimaryAppRole(user?.role), section.allowedRoles);
};

export const getAkreditasiRoute = (currentUser?: SidebarUser) => {
  if (
    currentUser?.role === "SCHOOL_ADMIN" ||
    currentUser?.role === "USER_SEKOLAH_GASPOL"
  ) {
    return "/akreditasi";
  }
  return currentUser ? "/internal/dashboard" : "/akreditasi";
};

export const getSectionHref = (section: SidebarSection, user: SidebarUser) => {
  if (section.key === "akreditasi") {
    return getAkreditasiRoute(user);
  }
  if (section.key === "siswa" && !user) {
    return "/status-lulusan";
  }
  return section.href ?? "#";
};

export const filterVisibleSections = (
  sections: SidebarSection[],
  user: SidebarUser,
  isRoot = true
): SidebarSection[] =>
  sections
    .filter(
      (section) =>
        user?.role !== "USER_MGMP" || !isRoot || section.key === "mgmp"
    )
    .filter((section) =>
      user?.role === "USER_SEKOLAH_GASPOL"
        ? isGaspolSchoolUserAllowedSidebarSection(section.key)
        : true
    )
    .filter((section) =>
      user || !isRoot ? true : publicRootSectionKeys.has(section.key)
    )
    .filter((section) => section.isEnabled !== false)
    .map((section) => {
      const children = section.children
        ? filterVisibleSections(section.children, user, false)
        : undefined;
      return { ...section, children };
    })
    .filter((section) => {
      const hasVisibleChildren =
        section.children != null && section.children.length > 0;
      return isSectionAllowed(section, user) || hasVisibleChildren;
    });
