"use client";
import { useEffect, useRef, useState } from "react";
import type { Map as LeafletMap, LayerGroup } from "leaflet";
import "leaflet/dist/leaflet.css";
import { hasCoordinates, type MapSchool } from "@/lib/school-map";
import { LOCATION_OUTLINES, schoolLevelColor } from "./school-map-style";

export default function SchoolMapCanvas({
  schools,
  selectedId,
  onSelect,
  compact = false,
}: {
  schools: MapSchool[];
  selectedId?: number | null;
  onSelect?: (school: MapSchool) => void;
  compact?: boolean;
}) {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<LeafletMap | null>(null);
  const layer = useRef<LayerGroup | null>(null);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState(false);
  const [tileError, setTileError] = useState(false);
  useEffect(() => {
    let disposed = false;
    let observer: ResizeObserver | undefined;
    import("leaflet")
      .then((L) => {
        if (disposed || !container.current) return;
        const instance = L.map(container.current, {
          scrollWheelZoom: false,
        }).setView([-7.03, 112.9], 10);
        map.current = instance;
        L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
          maxZoom: 19,
          attribution:
            '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
        })
          .on("tileerror", () => setTileError(true))
          .addTo(instance);
        layer.current = L.layerGroup().addTo(instance);
        observer = new ResizeObserver(() => instance.invalidateSize());
        observer.observe(container.current);
        setReady(true);
      })
      .catch(() => {
        if (!disposed) setError(true);
      });
    return () => {
      disposed = true;
      observer?.disconnect();
      map.current?.remove();
      map.current = null;
      layer.current = null;
    };
  }, []);
  useEffect(() => {
    if (!ready) return;
    let disposed = false;
    import("leaflet")
      .then((L) => {
        if (disposed || !map.current || !layer.current) return;
        layer.current.clearLayers();
        const valid = schools.filter(hasCoordinates);
        valid.forEach((school) => {
          const node = document.createElement("div");
          const title = document.createElement("strong");
          title.textContent = school.name;
          node.append(title);
          for (const text of [
            `NPSN ${school.npsn}`,
            `${school.level} · ${school.status}`,
            school.accreditation ? `Akreditasi ${school.accreditation}` : "",
            school.location_verified
              ? "Lokasi terverifikasi"
              : "Lokasi belum terverifikasi",
          ]) {
            if (!text) continue;
            const p = document.createElement("p");
            p.textContent = text;
            node.append(p);
          }
          if (onSelect) {
            const button = document.createElement("button");
            button.type = "button";
            button.textContent = "Lihat Profil Sekolah";
            button.className = "text-blue-700 font-semibold underline";
            button.onclick = () => onSelect(school);
            node.append(button);
          }
          const marker = L.circleMarker([school.latitude, school.longitude], {
            radius: selectedId === school.id ? 10 : 7,
            color: school.location_verified
              ? LOCATION_OUTLINES.verified
              : LOCATION_OUTLINES.unverified,
            fillColor: schoolLevelColor(school.level),
            fillOpacity: 0.85,
            weight: 2,
          })
            .bindPopup(node)
            .addTo(layer.current!);
          if (selectedId === school.id) marker.openPopup();
        });
        const selected = valid.find((s) => s.id === selectedId);
        if (selected)
          map.current.setView([selected.latitude, selected.longitude], 14);
        else if (valid.length)
          map.current.fitBounds(
            L.latLngBounds(valid.map((s) => [s.latitude, s.longitude])),
            { padding: [30, 30], maxZoom: 13 }
          );
      })
      .catch(() => {
        if (!disposed) setError(true);
      });
    return () => {
      disposed = true;
    };
  }, [schools, selectedId, onSelect, ready]);
  return (
    <div className="relative">
      <div
        ref={container}
        role="region"
        aria-label="Peta sebaran sekolah"
        className={`z-0 w-full bg-slate-100 ${compact ? "h-64" : "h-[420px] sm:h-[480px]"}`}
      />
      {error && (
        <p
          role="alert"
          className="absolute inset-x-4 top-4 z-10 rounded-xl bg-white p-4 text-sm"
        >
          Peta belum dapat dimuat. Informasi sekolah tetap tersedia pada daftar.
        </p>
      )}
      {tileError && (
        <p
          role="status"
          className="absolute inset-x-4 bottom-8 z-[500] rounded-lg bg-white/95 p-2 text-xs text-slate-700"
        >
          Latar peta tidak tersedia. Titik dan daftar sekolah tetap dapat
          digunakan.
        </p>
      )}
    </div>
  );
}
