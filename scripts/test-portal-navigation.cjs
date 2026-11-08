/* eslint-disable @typescript-eslint/no-require-imports -- CommonJS test loader for TypeScript navigation modules. */
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const Module = require('node:module');
const ts = require('typescript');
const root = path.resolve(__dirname, '..');
const cache = new Map();
function load(relative) {
  const file = path.join(root, relative + '.ts');
  if (cache.has(file)) return cache.get(file).exports;
  const mod = new Module(file, module);
  mod.filename = file;
  mod.paths = Module._nodeModulePaths(path.dirname(file));
  const original = mod.require.bind(mod);
  mod.require = id => id.startsWith('@/') ? load(id.slice(2)) : original(id);
  cache.set(file, mod);
  mod._compile(ts.transpileModule(fs.readFileSync(file, 'utf8'), {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,file);
  return mod.exports;
}
const {sidebarSections,filterVisibleSections,getSectionHref}=load('lib/navigation/sidebar-menu');
const roles=['SCHOOL_ADMIN','ADMIN','ADMIN_CABDIN','ADMIN_MASTER','USER_MGMP','PENGAWAS','VALIDATOR','USER_SEKOLAH_GASPOL'];
for(const role of roles) {
  const user={role};
  const visible=filterVisibleSections(sidebarSections,user);
  assert(!visible.some(s=>s.key==='cuti'),`${role}: disabled module`);
  assert.equal(visible.some(s=>s.key==='backup-pemulihan'),role==='ADMIN_MASTER',`${role}: backup access`);
  assert.equal(visible.some(s=>s.key==='mgmp'),['ADMIN','ADMIN_CABDIN','ADMIN_MASTER','USER_MGMP'].includes(role),`${role}: MGMP access`);
  for(const section of visible) assert.notEqual(getSectionHref(section,user),'#',`${role}: root destination`);
  console.log(`PASS ${role}: ${visible.length} existing role-filtered modules`);
}
const publicSections=filterVisibleSections(sidebarSections,null);
assert(!publicSections.some(s=>['settings','backup-pemulihan','guide-books'].includes(s.key)));
assert.equal(getSectionHref(publicSections.find(s=>s.key==='siswa'),null),'/status-lulusan');
console.log('PASS PUBLIC: protected menu hidden; graduate route preserved.');

assert(publicSections.some(s => s.key === 'cek-pengaduan'));
assert.deepEqual(publicSections.find(s => s.key === 'mgmp').children.map(s => s.key), ['registrasi-publik','cek-status']);
assert(!filterVisibleSections(sidebarSections,{role:'USER_MGMP'}).some(s => s.key === 'cek-pengaduan'));
console.log('PASS public nested registration links and complaint tracking permissions.');

for (const role of roles) {
  const settings = filterVisibleSections(sidebarSections, {role}).find(s => s.key === 'settings');
  const children = settings?.children ?? [];
  assert.equal(children.some(s => s.key === 'settings-beranda'), role === 'ADMIN_MASTER');
  assert.equal(children.some(s => s.key === 'settings-lokasi'), role === 'ADMIN_MASTER');
  if (role === 'ADMIN_MASTER') {
    assert.equal(children.find(s => s.key === 'settings-beranda').href, '/admin/settings/landing');
    assert.equal(children.find(s => s.key === 'settings-lokasi').href, '/user/settings#lokasi-sekolah');
  }
}
console.log('PASS Settings: Beranda and location seed navigation restricted to ADMIN_MASTER.');
