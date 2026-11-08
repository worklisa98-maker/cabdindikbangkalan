"use client";

import { GuideBooksPageContent } from "@/components/guide-books/GuideBooksPageContent";
import AdminRouteGuard from "@/components/siswa/AdminRouteGuard";

export default function GuideBooksPage() {
  return (
    <AdminRouteGuard>
      <GuideBooksPageContent />
    </AdminRouteGuard>
  );
}
