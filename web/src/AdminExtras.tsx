import {useEffect,useState} from 'react';
import type {ReactNode} from 'react';
import {BarChart3,Users,MapPin,Save,Plus,Download,RefreshCw,ShieldCheck} from 'lucide-react';
import type {Location} from './types';

type Metric={id:string;name:string;views:number;unique:number;additions:number;saved:number;leads:number};
type Visitor={userId:number;name:string;username:string;firstAt:string;lastAt:string;visits:number;hasLead:boolean;favorites:string[]};
type Section={id:string;title:string;body:string;enabled:boolean};
type Revision={title:string;category:string;description:string;rating:string;lat?:number;lon?:number;line_status:string;landmarks:string;map_url:string;sections:Section[]};
type AdminSection='analytics'|'visitors'|'locations'|'backup'|'photos';
type PhotoReport={enabled:boolean;running?:boolean;lastAttempt?:string;lastSuccess?:string;matchedLocations?:number;imagesAvailable?:number;downloaded?:number;unchanged?:number;unmatched?:string[];ambiguous?:string[];failures?:string[];error?:string};
const emptyRevision=():Revision=>({title:'',category:'Другие локации',description:'',rating:'',line_status:'',landmarks:'',map_url:'',sections:[
 {id:'traffic',title:'Трафик и локация',body:'',enabled:true},
 {id:'payers',title:'Кто платит',body:'',enabled:true},
 {id:'opportunity',title:'Окно возможностей',body:'',enabled:true},
 {id:'conclusion',title:'Почему именно здесь',body:'',enabled:true},
]});
async function adminFetch<T>(path:string,options:RequestInit={}):Promise<T>{
 const init=(window as typeof window & {Telegram?:{WebApp?:{initData?:string}}}).Telegram?.WebApp?.initData;
 if(!init)throw Error('Требуется вход через Telegram');
 const headers=new Headers(options.headers);
 headers.set('X-Telegram-Init-Data',init);
 if(options.body)headers.set('Content-Type','application/json');
 const res=await fetch(path,{...options,headers});
 if(!res.ok){const error=await res.json().catch(()=>({error:'Ошибка сервера'}));throw Error(error.error||'Ошибка сервера')}
 if(res.headers.get('Content-Type')?.includes('application/zip'))return (await res.blob()) as T;
 return await res.json() as T;
}
export default function AdminExtras({locations,refresh}:{locations:Location[];refresh:()=>void}){
 const [section,setSection]=useState<AdminSection>('analytics');
 const [metrics,setMetrics]=useState<Metric[]>([]);
 const [visitors,setVisitors]=useState<Visitor[]>([]);
 const [metric,setMetric]=useState('views');
 const [days,setDays]=useState(30);
 const [selectedId,setSelectedId]=useState('');
 const [revision,setRevision]=useState<Revision|null>(null);
 const [hash,setHash]=useState('');
 const [create,setCreate]=useState(false);
 const [busy,setBusy]=useState(false);
 const [message,setMessage]=useState('');
 const [photoReport,setPhotoReport]=useState<PhotoReport|null>(null);
 const [photoBusy,setPhotoBusy]=useState(false);
 const readPhotoReport=async()=>{
  setPhotoBusy(true);
  try{const report=await adminFetch<PhotoReport>('/api/admin/photos/status');setPhotoReport(report);setMessage('')}
  catch(error){setMessage((error as Error).message)}
  finally{setPhotoBusy(false)}
 };
 const syncPhotos=async()=>{
  setPhotoBusy(true);
  try{
   await adminFetch('/api/admin/photos/sync',{method:'POST'});
   setMessage('Синхронизация фотографий запущена. Обновите статус через несколько минут.');
   const report=await adminFetch<PhotoReport>('/api/admin/photos/status');setPhotoReport(report);
  }catch(error){setMessage((error as Error).message)}
  finally{setPhotoBusy(false)}
 };
 useEffect(()=>{let live=true;
  if(section!=='analytics'&&section!=='visitors')return;
  setBusy(true);
  const promise=section==='analytics'?adminFetch<{metrics:Metric[]}>('/api/admin/analytics?metric='+metric+'&days='+days)
   :adminFetch<{visitors:Visitor[]}>('/api/admin/visitors');
  promise.then(data=>{if(!live)return;if('metrics' in data)setMetrics(data.metrics);else setVisitors(data.visitors)})
   .catch(e=>{if(live)setMessage(e.message)}).finally(()=>{if(live)setBusy(false)});
  return()=>{live=false}
 },[section,metric,days]);
 useEffect(()=>{if(section==='photos')void readPhotoReport()},[section]);
 const openLocation=async(id:string)=>{
  if(!id)return;
  setSelectedId(id);setCreate(false);setBusy(true);
  try{const data=await adminFetch<{revision:Revision;hash:string}>('/api/admin/location?id='+encodeURIComponent(id));
    setRevision(data.revision);setHash(data.hash);setMessage('');
  }catch(e){setMessage((e as Error).message)}finally{setBusy(false)}
 };
 const saveLocation=async()=>{
  if(!revision)return;
  setBusy(true);
  try{
   if(create){
    const result=await adminFetch<{id:string}>('/api/admin/location/create',{method:'POST',body:JSON.stringify(revision)});
    setMessage('Локация создана: '+result.id);setSelectedId(result.id);setCreate(false);
   }else{
    await adminFetch('/api/admin/location',{method:'POST',body:JSON.stringify({id:selectedId,hash,revision})});
    setMessage('Карточка сохранена');
   }
   refresh();
   if(!create){const data=await adminFetch<{revision:Revision;hash:string}>('/api/admin/location?id='+encodeURIComponent(selectedId));setHash(data.hash);setRevision(data.revision)}
  }catch(e){setMessage((e as Error).message)}finally{setBusy(false)}
 };
 const downloadBackup=async()=>{
  setBusy(true);
  try{
   const file=await adminFetch<Blob>('/api/admin/backup');
   const url=URL.createObjectURL(file);
   const a=document.createElement('a');a.href=url;a.download='miniapp-backup.zip';document.body.append(a);a.click();a.remove();
   URL.revokeObjectURL(url);setMessage('Резервная копия подготовлена');
  }catch(e){setMessage((e as Error).message)}finally{setBusy(false)}
 };
 const field=(name:keyof Revision,label:string,area=false)=><label className="admin-field">{label}
  {area?<textarea value={String(revision?.[name]||'')} rows={4} onChange={e=>setRevision(r=>r?{...r,[name]:e.target.value}:r)}/>
  :<input value={String(revision?.[name]??'')} onChange={e=>setRevision(r=>r?{...r,[name]:e.target.value}:r)}/>}
 </label>;
 const menu:{id:AdminSection;label:string;icon:ReactNode}[]=[
  {id:'analytics',label:'Аналитика',icon:<BarChart3 size={18}/>},
  {id:'visitors',label:'Посетители',icon:<Users size={18}/>},
  {id:'locations',label:'Локации',icon:<MapPin size={18}/>},
  {id:'backup',label:'Бэкапы',icon:<ShieldCheck size={18}/>},
  {id:'photos',label:'Фото',icon:<RefreshCw size={18}/>},
 ];
 return <section className="admin-extras">
  <div className="admin-extras-tabs">{menu.map(item=><button key={item.id} type="button" className={section===item.id?'active':''} onClick={()=>{setSection(item.id);setMessage('')}}>{item.icon}{item.label}</button>)}</div>
  {message&&<p role="status" className="admin-message">{message}</p>}
  {busy&&<p className="admin-wait"><RefreshCw size={16}/> Загружаем…</p>}
  {section==='analytics'&&<div>
   <h2>Аналитика локаций</h2>
   <div className="admin-inline-fields"><label>Показатель<select value={metric} onChange={e=>setMetric(e.target.value)}>
    <option value="views">Просмотры</option><option value="unique">Уникальные</option><option value="added">Добавления в избранное</option>
    <option value="saved">Сейчас в избранном</option><option value="leads">Заявки</option>
   </select></label><label>Период<select value={days} onChange={e=>setDays(Number(e.target.value))}>
    <option value={7}>7 дней</option><option value={30}>30 дней</option><option value={0}>Всё время</option>
   </select></label></div>
   <div className="admin-ranking">{metrics.map((m,i)=><div key={m.id} className="admin-ranking-row">
    <span className="admin-rank">{i+1}</span><span className="admin-rank-name">{m.name}<small>№{m.id}</small></span>
    <strong>{metric==='views'?m.views:metric==='unique'?m.unique:metric==='added'?m.additions:metric==='saved'?m.saved:m.leads}</strong>
   </div>)}</div>
   <p className="admin-explain">Учитываются открытия карточек, сохранения и заявки. Это не подсчёт физических посещений локаций.</p>
  </div>}
  {section==='visitors'&&<div>
   <h2>Посетители</h2><p className="section-subtitle">Активность пользователей без заявки и с заявкой.</p>
   <div className="admin-visitor-list">{visitors.map(p=><article key={p.userId} className="admin-visitor">
    <strong>{p.name.trim()||'Посетитель'} {p.username&&<small>@{p.username}</small>}</strong>
    <small>Telegram ID: {p.userId}</small>
    <div>Последнее действие: {new Date(p.lastAt).toLocaleString('ru-RU')}</div>
    <div>Посещений: {p.visits} · Заявка: {p.hasLead?'есть':'нет'}</div>
    <div>Избранное: {p.favorites.map(id=>locations.find(l=>l.id===id)?.title||id).join(', ')||'пусто'}</div>
   </article>)}</div>
  </div>}
  {section==='locations'&&<div>
   <h2>Редактор локаций</h2>
   <p className="section-subtitle">Изменения сохраняются в отдельном хранилище и не теряются при обновлении кода.</p>
   <div className="admin-inline-fields">
    <label>Локация<select value={selectedId} onChange={e=>void openLocation(e.target.value)}>
     <option value="">Выберите локацию</option>{locations.map(l=><option key={l.id} value={l.id}>{l.id} · {l.title}</option>)}
    </select></label>
    <button className="btn secondary" onClick={()=>{setSelectedId('');setRevision(emptyRevision());setCreate(true);setMessage('')}}><Plus size={17}/> Новая точка</button>
   </div>
   {revision&&<div className="admin-editor">
    {field('title','Название')}{field('category','Категория')}{field('line_status','Линия и статус')}
    {field('landmarks','Ориентиры',true)}{field('description','Краткое описание',true)}
    {field('map_url','Ссылка Яндекс Карт')}{field('rating','Оценка')}
    <div className="admin-inline-fields"><label>Широта<input type="number" step="any" value={revision.lat??''} onChange={e=>setRevision(r=>r?{...r,lat:e.target.value?Number(e.target.value):undefined}:r)}/></label>
     <label>Долгота<input type="number" step="any" value={revision.lon??''} onChange={e=>setRevision(r=>r?{...r,lon:e.target.value?Number(e.target.value):undefined}:r)}/></label></div>
    <h3>Разделы карточки</h3>
    {revision.sections?.map((part,i)=><label className="admin-field" key={part.id}>{part.title}
     <textarea rows={4} value={part.body} onChange={e=>setRevision(r=>r?{...r,sections:r.sections.map((x,j)=>j===i?{...x,body:e.target.value}:x)}:r)}/></label>)}
    <button className="btn primary" disabled={busy} onClick={saveLocation}><Save size={17}/>{create?'Создать локацию':'Сохранить изменения'}</button>
   </div>}
  </div>}
  {section==='photos'&&<div className="admin-photo-sync">
    <h2>Фотографии локаций</h2>
    <p className="section-subtitle">Источник: публичная папка Яндекс Диска. До 3 фотографий на локацию, сопоставление по названию.</p>
    <div className="admin-stats">
     <div><small>Локаций с фото</small><strong>{photoReport?.matchedLocations??'—'}</strong></div>
     <div><small>Доступно фотографий</small><strong>{photoReport?.imagesAvailable??'—'}</strong></div>
     <div><small>Загружено в последней проверке</small><strong>{photoReport?.downloaded??'—'}</strong></div>
    </div>
    <p className="admin-explain">Состояние: {photoReport?.running?'Синхронизация выполняется':photoReport?.lastSuccess?'Синхронизация завершена':photoReport?.error?'Ошибка загрузки':'Ожидание первого запуска'}. Последнее успешное обновление: {photoReport?.lastSuccess?new Date(photoReport.lastSuccess).toLocaleString('ru-RU'):'ещё не выполнялось'}.</p>
    <div className="admin-inline-fields">
     <button className="btn secondary" onClick={()=>void readPhotoReport()} disabled={photoBusy}><RefreshCw size={17}/> Обновить статус</button>
     <button className="btn primary" onClick={()=>void syncPhotos()} disabled={photoBusy||!!photoReport?.running}><RefreshCw size={17}/> Синхронизировать фото</button>
    </div>
    {photoReport?.error&&<p className="admin-explain" role="alert">Ошибка: {photoReport.error}</p>}
    {!!photoReport?.unmatched?.length&&<details><summary>Не удалось сопоставить ({photoReport.unmatched.length} в отчёте)</summary><ul>{photoReport.unmatched.slice(0,50).map(name=><li key={name}>{name}</li>)}</ul></details>}
    {!!photoReport?.ambiguous?.length&&<details><summary>Неоднозначные совпадения ({photoReport.ambiguous.length})</summary><ul>{photoReport.ambiguous.slice(0,50).map(name=><li key={name}>{name}</li>)}</ul></details>}
    {!!photoReport?.failures?.length&&<details><summary>Ошибки загрузки ({photoReport.failures.length})</summary><ul>{photoReport.failures.slice(0,50).map(name=><li key={name}>{name}</li>)}</ul></details>}
    <p className="admin-explain">Фотографии хранятся отдельно от карточек и заявок. Синхронизация не меняет названия, номера и описания локаций.</p>
   </div>}
  {section==='backup'&&<div>
    <h2>Резервные копии</h2>
    <p className="section-subtitle">ZIP содержит каталог, историю заявок, избранное, аналитику и данные посетителей. Храни файл в безопасном месте.</p>
    <button className="btn primary" disabled={busy} onClick={downloadBackup}><Download size={18}/> Скачать резервную копию</button>
    <p className="admin-explain">Восстановление из ZIP пока выполняется через существующие административные инструменты бота. Загрузка архива здесь не включена, чтобы избежать случайной перезаписи данных.</p>
  </div>}
 </section>;
}
