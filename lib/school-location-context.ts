type LocationAccount = { role: string; schoolId?: number | null };
type ScopedSchool = { id: number; namaSekolah: string };

/** Resolve only an explicitly mapped school inside the server-provided scope. */
export function resolveLocationSchool(user: LocationAccount | null, schools: ScopedSchool[], selected: string) {
  if (user?.role === "SCHOOL_ADMIN") {
    return schools.find(school => school.id === user.schoolId) ?? null;
  }
  if (selected) return schools.find(school => String(school.id) === selected) ?? null;
  return schools.length === 1 ? schools[0] : null;
}
