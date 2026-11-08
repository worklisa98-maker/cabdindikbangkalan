import assert from 'node:assert/strict';
import { resolveLocationSchool } from '../lib/school-location-context.ts';
const schools = [{id:21,namaSekolah:'Sekolah A'},{id:22,namaSekolah:'Sekolah B'}];
assert.equal(resolveLocationSchool({role:'SCHOOL_ADMIN',schoolId:21},schools,'22')?.id,21);
assert.equal(resolveLocationSchool({role:'SCHOOL_ADMIN'},[schools[1]],'22'),null);
assert.equal(resolveLocationSchool({role:'SCHOOL_ADMIN',schoolId:99},schools,'21'),null);
assert.equal(resolveLocationSchool({role:'SCHOOL_ADMIN',schoolId:21},[],''),null);
for (const role of ['ADMIN','ADMIN_MASTER','PENGAWAS']) {
  assert.equal(resolveLocationSchool({role},schools,'22')?.id,22);
  assert.equal(resolveLocationSchool({role},schools,'999'),null);
  assert.equal(resolveLocationSchool({role},schools,''),null);
}
console.log('PASS: 13 location-context assertions (own mapping, missing mapping, out-of-scope selection, explicit admin selection).');
