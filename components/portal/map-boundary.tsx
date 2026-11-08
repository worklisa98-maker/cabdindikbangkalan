"use client";
import { Component, type ReactNode } from "react";
export class MapBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    return this.state.failed ? (
      <p
        role="status"
        className="rounded-xl bg-slate-50 p-6 text-sm text-slate-600"
      >
        Peta belum dapat ditampilkan. Informasi sekolah dan layanan lainnya
        tetap tersedia.
      </p>
    ) : (
      this.props.children
    );
  }
}
