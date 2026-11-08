"use client";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import { fetchServiceLinks, type ExternalService, type LinkSettings, type ServiceLink } from "@/lib/service-links";

const emptyLinks: LinkSettings = { categories:[], links:[], version:"" };
const button = "rounded-lg bg-blue-700 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50";
const input = "w-full rounded-lg border border-slate-300 px-3 py-2 text-sm";
const newID = () => crypto.randomUUID();

export default function LayananTautanSettingsPage() {
 const router = useRouter();
 const { user, status } = useAuth();
 const client = useQueryClient();
 const { data, isLoading, isError, refetch } = useQuery({ queryKey:["service-links","admin"], queryFn:()=>fetchServiceLinks("admin"), enabled:status==="authenticated" && user?.role==="ADMIN_MASTER", staleTime:0 });
 const [tab,setTab] = useState<"halo"|"sinom"|"links">("halo");
 const [halo,setHalo] = useState<ExternalService | null>(null);
 const [sinom,setSinom] = useState<ExternalService | null>(null);
 const [links,setLinks] = useState<LinkSettings>(emptyLinks);
 const [message,setMessage] = useState("");
 const [saving,setSaving] = useState(false);
 useEffect(()=>{ if (status==="unauthenticated" || (status==="authenticated" && user?.role!=="ADMIN_MASTER")) router.replace("/user/settings"); },[status,user,router]);
 useEffect(()=>{ if(data){setHalo(data.halo);setSinom(data.sinom);setLinks(data.pusat_tautan);} },[data]);
 const saveExternal = async (name:"halo"|"sinom") => {
  const value = name==="halo" ? halo : sinom; if(!value) return;
  setSaving(true);setMessage("");
  try { const response = await apiFetch(`/admin/master/service-links/external/${name}`,{method:"PUT",body:JSON.stringify(value)}); if(!response.ok) throw new Error("Gagal menyimpan layanan."); await refetch(); await client.invalidateQueries({queryKey:["service-links"]}); setMessage("Pengaturan tersimpan."); }
  catch(error){setMessage(String(error));} finally {setSaving(false);}
 };
 const saveLinks = async () => {
  setSaving(true);setMessage("");
  try { const response=await apiFetch("/admin/master/service-links/links",{method:"PUT",body:JSON.stringify(links)}); if(response.status===409) throw new Error("Data berubah pada tab lain. Muat ulang sebelum menyimpan lagi."); if(!response.ok) throw new Error("Gagal menyimpan tautan. Periksa kategori, judul, dan URL aktif."); const payload=await response.json();setLinks(payload.data.pusat_tautan);await client.invalidateQueries({queryKey:["service-links"]});setMessage("Pusat Tautan tersimpan."); }
  catch(error){setMessage(String(error));} finally {setSaving(false);}
 };
 const updateLink=(id:string,patch:Partial<ServiceLink>)=>setLinks(current=>({...current,links:current.links.map(link=>link.id===id?{...link,...patch}:link)}));
 const current=tab==="halo"?halo:sinom;
 const setCurrent=(patch:Partial<ExternalService>)=>{if(tab==="halo")setHalo(value=>value?{...value,...patch}:value);else setSinom(value=>value?{...value,...patch}:value);};
 return <main className="mx-auto max-w-5xl space-y-6 p-6 md:p-10"><div><h1 className="text-3xl font-bold">Layanan & Tautan</h1><p className="mt-2 text-slate-600">Kelola layanan publik dan Pusat Tautan Sipres.</p></div>
  <nav className="flex flex-wrap gap-2" aria-label="Bagian pengaturan">{([ ["halo","Halo Kepegawaian"],["sinom","SINOM"],["links","Pusat Tautan"]] as const).map(([key,label])=><button key={key} type="button" onClick={()=>{setTab(key);setMessage("");}} className={`rounded-lg px-4 py-2 ${tab===key?"bg-blue-700 text-white":"border bg-white"}`}>{label}</button>)}</nav>
  {isLoading ? <p>Memuat pengaturan...</p> : isError ? <p role="alert">Gagal memuat pengaturan.</p> : tab!=="links" && current ? <section className="space-y-4 rounded-xl border bg-white p-5"><h2 className="text-xl font-semibold">{tab==="halo"?"Halo Kepegawaian":"SINOM"}</h2><label className="block space-y-1"><span>URL tujuan</span><input className={input} value={current.url} onChange={event=>setCurrent({url:event.target.value})} placeholder="https://..." /></label><label className="block space-y-1"><span>Deskripsi</span><textarea className={input} value={current.description} onChange={event=>setCurrent({description:event.target.value})} /></label><label className="flex items-center gap-2"><input type="checkbox" checked={current.is_active} onChange={event=>setCurrent({is_active:event.target.checked})} /> Aktif</label>{tab==="halo"?<><label className="block space-y-1"><span>Judul dialog</span><input className={input} value={current.dialog_title??""} onChange={event=>setCurrent({dialog_title:event.target.value})} /></label><label className="block space-y-1"><span>Isi dialog Gmail</span><textarea className={input} rows={4} value={current.dialog_body??""} onChange={event=>setCurrent({dialog_body:event.target.value})} /></label><label className="block space-y-1"><span>Label tombol</span><input className={input} value={current.dialog_button??""} onChange={event=>setCurrent({dialog_button:event.target.value})} /></label></>:null}<button type="button" disabled={saving} className={button} onClick={()=>saveExternal(tab)}>Simpan {tab==="halo"?"Halo":"SINOM"}</button></section> : tab==="links" ? <section className="space-y-6"><div className="rounded-xl border bg-white p-5"><div className="flex justify-between gap-3"><h2 className="text-xl font-semibold">Kategori</h2><button type="button" className={button} onClick={()=>setLinks(value=>({...value,categories:[...value.categories,{id:newID(),title:"Kategori baru",sort_order:value.categories.length}]}))}>Tambah kategori</button></div><div className="mt-4 space-y-3">{links.categories.map(category=><div key={category.id} className="flex gap-2"><input aria-label="Nama kategori" className={input} value={category.title} onChange={event=>setLinks(value=>({...value,categories:value.categories.map(item=>item.id===category.id?{...item,title:event.target.value}:item)}))} /><input aria-label="Urutan kategori" type="number" className="w-20 rounded-lg border px-2" value={category.sort_order} onChange={event=>setLinks(value=>({...value,categories:value.categories.map(item=>item.id===category.id?{...item,sort_order:Number(event.target.value)}:item)}))} /><button type="button" onClick={()=>setLinks(value=>({...value,categories:value.categories.filter(item=>item.id!==category.id),links:value.links.filter(item=>item.category_id!==category.id)}))} className="text-red-700">Hapus</button></div>)}</div></div>
  <div className="rounded-xl border bg-white p-5"><div className="flex justify-between gap-3"><h2 className="text-xl font-semibold">Tautan</h2><button type="button" disabled={!links.categories.length} className={button} onClick={()=>setLinks(value=>({...value,links:[...value.links,{id:newID(),category_id:value.categories[0].id,title:"",description:"",url:"",icon:"🔗",sort_order:value.links.length,is_active:false,is_public:true}]}))}>Tambah tautan</button></div><div className="mt-4 space-y-5">{links.links.map(link=><div key={link.id} className="grid gap-3 rounded-lg border p-4 md:grid-cols-2"><label>Nama<input className={input} value={link.title} onChange={event=>updateLink(link.id,{title:event.target.value})} /></label><label>Kategori<select className={input} value={link.category_id} onChange={event=>updateLink(link.id,{category_id:event.target.value})}>{links.categories.map(category=><option key={category.id} value={category.id}>{category.title}</option>)}</select></label><label>URL<input className={input} value={link.url} onChange={event=>updateLink(link.id,{url:event.target.value})} placeholder="Kosong untuk draft" /></label><label>Ikon<input className={input} value={link.icon} onChange={event=>updateLink(link.id,{icon:event.target.value})} /></label><label className="md:col-span-2">Deskripsi<textarea className={input} value={link.description} onChange={event=>updateLink(link.id,{description:event.target.value})} /></label><label>Urutan<input type="number" className={input} value={link.sort_order} onChange={event=>updateLink(link.id,{sort_order:Number(event.target.value)})} /></label><div className="flex flex-wrap items-center gap-4"><label className="flex gap-2"><input type="checkbox" checked={link.is_active} onChange={event=>updateLink(link.id,{is_active:event.target.checked})} /> Aktif</label><label className="flex gap-2"><input type="checkbox" checked={link.is_public} onChange={event=>updateLink(link.id,{is_public:event.target.checked})} /> Boleh diakses publik</label><span className="rounded bg-slate-100 px-2 py-1 text-xs">{link.is_public?"Publik":"Login saja"}</span></div><button type="button" className="text-left text-red-700" onClick={()=>setLinks(value=>({...value,links:value.links.filter(item=>item.id!==link.id)}))}>Hapus tautan</button></div>)}</div></div><button type="button" disabled={saving} className={button} onClick={saveLinks}>Simpan Pusat Tautan</button></section> : null}
  {message?<p role="status" className="rounded-lg bg-slate-100 p-3">{message}</p>:null}
 </main>;
}
