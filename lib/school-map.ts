export type MapSchool = {
  id: number;
  name: string;
  npsn: string;
  level: string;
  status: string;
  accreditation: string;
  latitude: number | null;
  longitude: number | null;
  location_source: string;
  location_verified: boolean;
  location_updated_at?: string;
  location_updated_by?: string;
};
export function hasCoordinates<
  T extends Pick<MapSchool, "latitude" | "longitude">,
>(s: T): s is T & { latitude: number; longitude: number } {
  return (
    typeof s.latitude === "number" &&
    typeof s.longitude === "number" &&
    Number.isFinite(s.latitude) &&
    Number.isFinite(s.longitude) &&
    s.latitude >= -90 &&
    s.latitude <= 90 &&
    s.longitude >= -180 &&
    s.longitude <= 180
  );
}
