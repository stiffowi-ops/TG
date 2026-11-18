import {useEffect, useMemo, useState} from 'react';
import type {ReactNode} from 'react';
import AdminExtras from './AdminExtras';
import PhotoCarousel from './components/PhotoCarousel/PhotoCarousel';
import CatalogPage from './components/CatalogPage/CatalogPage';
import LocationCard from './components/LocationCard/LocationCard';
import YandexLocationViewer from './components/YandexLocationViewer/YandexLocationViewer';
import Header from './components/Header/Header';
import LandingPage from './components/LandingPage/LandingPage';
import BottomNavigation from './components/BottomNavigation/BottomNavigation';
import {
 ArrowLeft, ArrowRight, Building2, Check,
 CheckCircle2, ChevronRight, CircleHelp, Clock3,
 ExternalLink, Heart, MapPin, MessageCircle, Plus, Send, ShieldCheck, SlidersHorizontal, Sparkles, X
} from 'lucide-react';
import type {DemoCase, Location, CaseStatus, Tab, NewCaseInput} from './types';
import {backupLocations, mapLink, MAX_COMPARE, MAX_UPDATES, normalize, prettyDate, statusLabel} from './demo';

type WebApp = {
 ready():void; expand():void; close?():void; setHeaderColor?(color:string):void;
 HapticFeedback?:{impactOccurred(style:'light'|'medium'|'heavy'):void};
 initData?:string;
};
declare global {interface Window {Telegram?:{WebApp?:WebApp}}}

const heartKey = 'locations-prototype:favorites:v1';
type MiniSession={admin:boolean;user:{id:number;username:string;firstName:string};favorites:string[]|null;cases:DemoCase[]|null};
async function miniApi<T>(path:string,options:RequestInit={}):Promise<T>{
 const initData=window.Telegram?.WebApp?.initData;
 if(!initData)throw Error('Откройте приложение через Telegram.');
 const headers=new Headers(options.headers);headers.set('X-Telegram-Init-Data',initData);
 if(options.body)headers.set('Content-Type','application/json');
 const response=await fetch(path,{...options,headers});
 const data=await response.json().catch(()=>({error:'Ошибка соединения'}));
 if(!response.ok)throw Error(data.error||'Ошибка сервера');
 return data as T;
}

function stored<T>(key:string, initial:T):T {
 try{const v=window.localStorage.getItem(key);if(!v)return initial;const parsed:unknown=JSON.parse(v);if(Array.isArray(initial))return (Array.isArray(parsed)?parsed.filter(x=>typeof x==='string'):initial) as T;return parsed as T;}catch{return initial;}
}
function useStoredState<T>(key:string,initial:T){
 const [value,setValue]=useState<T>(()=>stored(key,initial));
 useEffect(()=>{try{localStorage.setItem(key,JSON.stringify(value));}catch{/* disabled storage */}},[key,value]);
 return [value,setValue] as const;
}
function tgHaptic(){try{window.Telegram?.WebApp?.HapticFeedback?.impactOccurred('light');}catch{/* browser mode */}}
function openMap(url:string){const app=window.Telegram?.WebApp as WebApp & {openLink?:(url:string,options?:{try_instant_view?:boolean})=>void};if(app?.openLink){app.openLink(url);}else{window.open(url,'_blank','noopener,noreferrer');}}
function AreaArt({location,compact=false}:{location:Location;compact?:boolean}){
 const tone=(parseInt(location.id,10)%5)+1;
 const [broken,setBroken]=useState(false);
 useEffect(()=>setBroken(false),[location.imageUrl]);
 const hasPhoto=!!location.imageUrl&&!broken;
 return <div className={`area-art tone-${tone} ${compact?'compact':''}`} aria-label={hasPhoto?`Фото: ${location.title}`:'Демонстрационная иллюстрация локации'}>
  {hasPhoto&&<img className="art-photo" src={location.imageUrl} alt={`Фотография локации ${location.title}`} loading="lazy" onError={()=>setBroken(true)}/> }
  <div className="art-glow"/><div className="art-sun"/>
  <div className="art-buildings"><i/><i/><i/><i/><i/><i/></div>
  <div className="art-line"/><div className="art-road"/>
  <span className="art-mark"><MapPin size={16} strokeWidth={2.6}/></span>
  {!compact&&!hasPhoto&&<span className="image-disclaimer">Демо-иллюстрация · ожидается фото</span>}
 </div>;
}
function Pill({children,kind='neutral'}:{children:ReactNode;kind?:string}){return <span className={`pill ${kind}`}>{children}</span>}
function Empty({icon,title,description,action}:{icon:ReactNode;title:string;description:string;action?:ReactNode}){
 return <div className="empty"><div className="empty-symbol">{icon}</div><h3>{title}</h3><p>{description}</p>{action}</div>;
}
function Status({status}:{status:CaseStatus}){return <span className={`status status-${status}`}>{statusLabel[status]}</span>}
function Dialog({children,onClose,className=''}:{children:ReactNode;onClose:()=>void;className?:string}){
 useEffect(()=>{const onKey=(e:KeyboardEvent)=>{if(e.key==='Escape')onClose()};document.addEventListener('keydown',onKey);document.body.classList.add('has-dialog');return()=>{document.removeEventListener('keydown',onKey);document.body.classList.remove('has-dialog')}},[onClose]);
 return <div className="dialog-overlay" onMouseDown={e=>{if(e.target===e.currentTarget)onClose()}}><div className={`dialog ${className}`} role="dialog" aria-modal="true"><button className="dialog-close icon-button" onClick={onClose} aria-label="Закрыть"><X size={20}/></button>{children}</div></div>;
}
function App(){
 const [tab,setTab]=useState<Tab>('home');
 const [focusSearch,setFocusSearch]=useState(false);
 const [locations,setLocations]=useState<Location[]>(backupLocations);
 const [loading,setLoading]=useState(true);
 const [query,setQuery]=useState('');
 const [area,setArea]=useState('Все районы');
 const [category,setCategory]=useState('Все');
 const [showMap,setShowMap]=useState(false);
 const [mapSelectedId,setMapSelectedId]=useState<string|null>(null);
 const [yandexLocation,setYandexLocation]=useState<Location|null>(null);
 const [favorites,setFavorites]=useStoredState<string[]>(heartKey,[]);
 const [cases,setCases]=useState<DemoCase[]>([]);
  const [session,setSession]=useState<MiniSession|null>(null);
  const [adminCases,setAdminCases]=useState<DemoCase[]>([]);
  const [adminBusy,setAdminBusy]=useState(false);
 const [compareIds,setCompareIds]=useState<string[]>([]);
 const [compareOpen,setCompareOpen]=useState(false);
 const [detail,setDetail]=useState<Location|null>(null);
 const [caseDetail,setCaseDetail]=useState<number|null>(null);
 const [creating,setCreating]=useState<string|null>(null);
 const [supplement,setSupplement]=useState<number|null>(null);
 const [toast,setToast]=useState('');
 useEffect(()=>{
  const app=window.Telegram?.WebApp as (WebApp & {
   contentSafeAreaInset?:{top?:number};
   onEvent?:(event:string,callback:()=>void)=>void;
   offEvent?:(event:string,callback:()=>void)=>void;
  })|undefined;
  const updateSafeArea=()=>{
   const inTelegram=Boolean(app?.initData);
   document.documentElement.classList.toggle('inside-telegram-miniapp',inTelegram);
   // Respect the actual Telegram content inset, without a fabricated 76px gap.
   // A number of Android Telegram WebViews already position their web content
   // below the native toolbar, so reserving another 76px creates blank scroll.
   const explicit=Number(app?.contentSafeAreaInset?.top)||0;
   const cssInset=parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--tg-content-safe-area-inset-top'))||0;
   document.documentElement.style.setProperty('--telegram-top-space',Math.max(0,explicit,cssInset)+'px');
  };
  try{app?.ready();app?.expand();app?.setHeaderColor?.('#ffffff')}catch{/* standard browser */}
  updateSafeArea();
  app?.onEvent?.('contentSafeAreaChanged',updateSafeArea);
  return()=>{app?.offEvent?.('contentSafeAreaChanged',updateSafeArea)};
 },[]);
 useEffect(()=>{
  if(tab!=='catalog'||!focusSearch)return;
  document.getElementById('catalog-search')?.focus();
  setFocusSearch(false);
 },[tab,focusSearch]);
 useEffect(()=>{let active=true;fetch('/api/locations').then(async response=>{if(!response.ok)throw new Error('no API');return response.json() as Promise<{locations:Location[]}>}).then(data=>{if(active&&data.locations?.length)setLocations(data.locations)}).catch(()=>{/* standalone UI preview uses bundled demo data */}).finally(()=>{if(active)setLoading(false)});return()=>{active=false}},[]);
 useEffect(()=>{if(!toast)return;const t=setTimeout(()=>setToast(''),3000);return()=>clearTimeout(t)},[toast]);
 useEffect(()=>{if(!window.Telegram?.WebApp?.initData)return;let active=true;
  miniApi<MiniSession>('/api/me').then(data=>{if(active){setSession(data);setCases(Array.isArray(data.cases)?data.cases:[]);setFavorites(Array.isArray(data.favorites)?data.favorites:[])}}).catch(err=>{if(active)setToast(err.message)});
  return()=>{active=false};},[]);
 useEffect(()=>{if(tab!=='requests'||!session)return;
 miniApi<MiniSession>('/api/me').then(data=>setCases(Array.isArray(data.cases)?data.cases:[])).catch(err=>setToast(err.message));
 },[tab,session?.user.id]);
 useEffect(()=>{if(tab!=='admin'||!session?.admin)return;setAdminBusy(true);
  miniApi<{cases:DemoCase[]|null}>('/api/admin/cases').then(data=>setAdminCases(Array.isArray(data.cases)?data.cases:[])).catch(err=>setToast(err.message)).finally(()=>setAdminBusy(false));
 },[tab,session?.admin]);
 const refreshCatalog=()=>fetch('/api/locations').then(async r=>r.json()).then((data:{locations:Location[]})=>{if(data.locations?.length)setLocations(data.locations)}).catch(()=>setToast('Каталог не обновился'));
 useEffect(()=>{if(!detail||!session)return;miniApi('/api/locations/view',{method:'POST',body:JSON.stringify({id:detail.id})}).catch(()=>{});},[detail?.id,session?.user.id]);
 const adminAction=async(number:number,action:'take'|'ack'|'resolve',updateId=0)=>{
  try{await miniApi('/api/admin/action',{method:'POST',body:JSON.stringify({number,action,updateId})});
  const data=await miniApi<{cases:DemoCase[]}>('/api/admin/cases');setAdminCases(data.cases);setToast('Изменения сохранены');}
  catch(err){setToast((err as Error).message)}
 };
 const locationById=useMemo(()=>new Map(locations.map(l=>[l.id,l])),[locations]);
 const areas=useMemo(()=>['Все районы',...Array.from(new Set(locations.map(l=>l.area))).sort((a,b)=>a.localeCompare(b,'ru'))],[locations]);
 const filtered=useMemo(()=>locations.filter(l=>(category==='Все'||l.category===category)&&(area==='Все районы'||l.area===area)&&normalize(l.title).includes(normalize(query))),[locations,query,category,area]);
 const favoritesSet=new Set(favorites);
 const favoriteLocations=locations.filter(l=>favoritesSet.has(l.id));
 const showYandexLocation=(location:Location)=>{setYandexLocation(location)};
 const activeCase=cases.find(c=>c.number===caseDetail)||null;
 const onTab=(t:Tab,openSearch=false)=>{if(t==='admin'&&!session?.admin)return;setTab(t);setFocusSearch(openSearch&&t==='catalog');setCompareIds([]);setCompareOpen(false);setShowMap(t==='map');document.querySelector<HTMLElement>('.main-container')?.scrollTo({top:0,behavior:'instant' as ScrollBehavior})};
 const favoriteToggle=async(id:string)=>{try{
  const data=await miniApi<{favorites:string[]|null}>('/api/favorites/toggle',{method:'POST',body:JSON.stringify({id})});
  const next=Array.isArray(data.favorites)?data.favorites:[];
  setFavorites(next);setToast(next.includes(id)?'Добавлено в избранное':'Удалено из избранного')
  }catch(err){setToast((err as Error).message)}};
 const compareToggle=(id:string)=>{setCompareIds(prev=>prev.includes(id)?prev.filter(x=>x!==id):prev.length>=MAX_COMPARE?(setToast('Для сравнения доступны 2–3 локации'),prev):[...prev,id])};
 const submitCase=async(data:NewCaseInput)=>{try{
 const result=await miniApi<{case:DemoCase}>('/api/cases',{method:'POST',body:JSON.stringify(data)});
 setCases(prev=>[result.case,...prev.filter(c=>c.number!==result.case.number)]);
 setCreating(null);setCaseDetail(result.case.number);setToast('Заявка отправлена специалистам');
 }catch(err){setToast((err as Error).message)}};
 const submitSupplement=async(number:number,ids:string[],comment:string)=>{try{
 const result=await miniApi<{case:DemoCase}>('/api/cases/supplement',{method:'POST',body:JSON.stringify({number,supplement:{locationIds:ids,comment}})});
 setCases(prev=>prev.map(c=>c.number===number?result.case:c));setSupplement(null);setToast('Дополнение отправлено');
 }catch(err){setToast((err as Error).message)}};
 const uniqueCategory=Array.from(new Set(locations.map(l=>l.category)));
 const readyForCompare=compareIds.length>=2&&compareIds.length<=3;
 return <div className="app-shell">
  {(tab!=='catalog'&&tab!=='map')&&<Header onHome={()=>onTab('home')}/>}
  <main className="main-container">
   {tab==='home'&&<LandingPage
     onCatalog={()=>onTab('catalog')}
     onSearch={()=>onTab('catalog',true)}
     onFavorites={()=>onTab('favorites')}
   />}
   {(tab==='catalog'||tab==='map')&&<CatalogPage
    locations={filtered} areas={areas} availableCategories={uniqueCategory}
    query={query} onQuery={setQuery} area={area} onArea={setArea}
    category={category} onCategory={setCategory}
    view={showMap?'map':'list'} onView={view=>onTab(view==='map'?'map':'catalog')}
    selectedId={mapSelectedId} onSelect={setMapSelectedId}
    favorites={favoritesSet} onFavorite={favoriteToggle}
    onDetails={setDetail} onMap={showYandexLocation}
    onBack={()=>onTab('home')} onFavorites={()=>onTab('favorites')}
    loading={loading}
   />}
   {tab==='favorites'&&<section className="section-block tab-section"><div className="section-heading"><div><span className="overline">ВАША ПОДБОРКА</span><h1>Избранное <span className="heading-number">{favoriteLocations.length}</span></h1><p>Сохраняйте интересные варианты и сравнивайте до трёх локаций.</p></div></div>
    {favoriteLocations.length?<><div className="compare-banner"><div className="compare-banner-symbol"><SlidersHorizontal size={22}/></div><div><strong>Сравните варианты между собой</strong><span>Отметьте 2–3 локации. Выбрано: {compareIds.length} из 3.</span></div><button className="btn dark" disabled={!readyForCompare} onClick={()=>setCompareOpen(true)}>Сравнить <ArrowRight size={16}/></button></div><div className="card-grid">{favoriteLocations.map(l=><LocationCard key={l.id} location={l} saved={true} onDetails={setDetail} onFavorite={favoriteToggle} onMap={showYandexLocation} compareMode selected={compareIds.includes(l.id)} onSelect={compareToggle}/>)}</div></>:<Empty icon={<Heart size={30}/>} title="Здесь пока пусто" description="Откройте каталог и нажмите на сердечко у понравившейся локации. Все сохранённые варианты появятся здесь." action={<button className="btn primary" onClick={()=>onTab('catalog')}>Перейти в каталог <ArrowRight size={16}/></button>}/>}
   </section>}
   {tab==='requests'&&<section className="section-block tab-section"><div className="section-heading request-heading"><div><span className="overline">ОБРАЩЕНИЯ</span><h1>Мои заявки</h1><p>Следите за статусом обращения и просматривайте историю в одном месте.</p></div><button className="btn primary" onClick={()=>setCreating('')}><Plus size={17}/> Новая заявка</button></div>
    {!session&&<div className="demo-notice"><ShieldCheck size={20}/><span>Для отправки заявки откройте приложение из Telegram.</span></div>}
    <div className="status-journey"><div><span className="journey-icon received"><Send size={18}/></span><b>Получено</b><small>Заявка создана</small></div><span className="journey-line"/><div><span className="journey-icon working"><Clock3 size={18}/></span><b>В работе</b><small>Назначен специалист</small></div><span className="journey-line"/><div><span className="journey-icon resolved"><CheckCircle2 size={18}/></span><b>Решено</b><small>Заявка обработана</small></div></div>
    <div className="request-section-head"><h2>История обращений</h2><span>{cases.length} заявки</span></div>
    <div className="requests-list">{[...cases].sort((a,b)=>b.number-a.number).map(c=><button className="request-card" key={c.number} onClick={()=>setCaseDetail(c.number)}><span className="request-card-icon"><MessageCircle size={22}/></span><span className="request-card-main"><span className="request-id">Заявка №{String(c.number).padStart(4,'0')}</span><span className="request-locations">{c.locationIds.map(id=>locationById.get(id)?.title||`Локация №${id}`).slice(0,2).join(' · ')}</span><small>{prettyDate(c.createdAt)} · Дополнения: {c.updates.length}/{MAX_UPDATES}</small></span><span className="request-card-right"><Status status={c.status}/><ChevronRight size={20}/></span></button>)}</div>
    <p className="requests-footnote">История и статусы обращений сохраняются на сервере.</p>
   </section>}
  {tab==='admin'&&session?.admin&&<section className="section-block tab-section">
   <div className="section-heading"><div><span className="overline">ДОСТУП АДМИНИСТРАТОРА</span><h1>Панель управления</h1><p>Заявки, ответственные и дополнения. Только для подтверждённых администраторов.</p></div></div>
   <div className="admin-stats">
    <div><small>Новые заявки</small><strong>{adminCases.filter(c=>c.status==='received').length}</strong></div>
    <div><small>В работе</small><strong>{adminCases.filter(c=>c.status==='working').length}</strong></div>
    <div><small>Обработаны</small><strong>{adminCases.filter(c=>c.status==='resolved').length}</strong></div>
   </div>
   <AdminExtras locations={locations} refresh={refreshCatalog}/><h2>Банк заявок</h2>{adminBusy&&<p>Загружаем обращения…</p>}
   <div className="admin-request-list">{adminCases.map(c=><article className="admin-request" key={c.number}>
    <div className="admin-request-head"><strong>Заявка №{String(c.number).padStart(4,'0')}</strong><Status status={c.status}/></div>
    <p><b>{c.name||'Клиент'}</b>{c.company?' · '+c.company:''}</p>
    <p>Телефон: <a href={'tel:'+c.phone}>{c.phone}</a>{c.username?' · @'+c.username:''}</p>
    <p>Локации: {c.locationIds.map(id=>locationById.get(id)?.title||id).join(', ')}</p>
    <small>Отправлена: {prettyDate(c.createdAt)} · Ответственный: {c.takenBy||'не назначен'}</small>
    {c.updates.map(u=><div className="admin-update" key={u.id}>
      <b>Дополнение №{u.id} {u.acceptedAt?'· принято':'· ожидает'}</b>
      <p>{u.locationIds.map(id=>locationById.get(id)?.title||id).join(', ')}</p>{u.comment&&<p>{u.comment}</p>}
      {!u.acceptedAt&&c.status!=='resolved'&&<button className="btn secondary" onClick={()=>adminAction(c.number,'ack',u.id)}>Принять дополнение</button>}
    </div>)}
    <div className="admin-actions">
     {c.status==='received'&&<button className="btn primary" onClick={()=>adminAction(c.number,'take')}>Взять в работу</button>}
     {c.status==='working'&&<button className="btn primary" disabled={c.updates.some(u=>!u.acceptedAt)} onClick={()=>adminAction(c.number,'resolve')}>Завершить заявку</button>}
    </div>
   </article>)}</div>
  </section>}
  </main>
  <footer className="site-footer"><span>Подбор локаций</span><span>Доступ к личным данным через Telegram</span></footer>
  <BottomNavigation tab={tab} onNavigate={onTab} savedCount={favorites.length} isAdmin={!!session?.admin} onHaptic={tgHaptic}/>
  {toast&&<div className="toast" role="status"><CheckCircle2 size={18}/>{toast}</div>}
  {detail&&<Dialog onClose={()=>setDetail(null)} className="detail-dialog"><div className="detail-art"><PhotoCarousel photos={detail.imageUrls?.length?detail.imageUrls:detail.imageUrl?[detail.imageUrl]:[]} fallback={<AreaArt location={detail}/>} label={detail.title} onOpen={()=>{}}/></div><div className="detail-padding"><div className="detail-topline"><Pill>{detail.category}</Pill><span>№{detail.id}</span></div><h2>{detail.title}</h2><p className="detail-area"><MapPin size={17}/>{detail.area}</p><div className="detail-facts"><div><span>Широта</span><strong>{detail.latitude?.toFixed(4)??'не указана'}</strong></div><div><span>Долгота</span><strong>{detail.longitude?.toFixed(4)??'не указана'}</strong></div></div><div className="location-description">
 {([{title:'Ориентиры и транспорт',body:[detail.lineStatus,detail.landmarks].filter(Boolean).join(' · ')},{title:'Трафик и локация',body:detail.traffic},{title:'Кто платит',body:detail.payers},{title:'Окно возможностей',body:detail.opportunity},{title:'Почему именно здесь',body:detail.conclusion}]).map(item=><details key={item.title}><summary>{item.title}</summary><p>{item.body||'Информация пока не добавлена'}</p></details>)}
 </div><div className="detail-buttons"><button className="btn secondary" onClick={()=>favoriteToggle(detail.id)}><Heart size={18} fill={favoritesSet.has(detail.id)?'currentColor':'none'}/>{favoritesSet.has(detail.id)?'В избранном':'В избранное'}</button><button className="btn secondary" onClick={()=>{setYandexLocation(detail);setDetail(null)}}><MapPin size={17}/> На Яндекс Карте <ExternalLink size={13}/></button></div><button className="btn primary full" onClick={()=>{setCreating(detail.id);setDetail(null)}}>Оставить заявку по локации <ArrowRight size={18}/></button></div></Dialog>}
  {yandexLocation&&<YandexLocationViewer location={yandexLocation} onClose={()=>setYandexLocation(null)}/>}
  {compareOpen&&<Dialog onClose={()=>setCompareOpen(false)} className="compare-dialog"><div className="dialog-content"><span className="overline">ВАША ПОДБОРКА</span><h2>Сравнение локаций</h2><p className="section-subtitle">Сравнивайте основные данные выбранных вариантов.</p><div className="compare-table"><div className="compare-row compare-top"><span>Параметр</span>{compareIds.map(id=><strong key={id}>{locationById.get(id)?.title}</strong>)}</div>{([{name:'Категория',get:(l:Location)=>l.category},{name:'Район',get:(l:Location)=>l.area},{name:'Координаты',get:(l:Location)=>l.latitude===undefined||l.longitude===undefined?'Нет данных':`${l.latitude.toFixed(3)}, ${l.longitude.toFixed(3)}`}]).map(row=><div className="compare-row" key={row.name}><span>{row.name}</span>{compareIds.map(id=><span key={id}>{locationById.get(id)?row.get(locationById.get(id)!):'—'}</span>)}</div>)}</div><div className="compare-links">{compareIds.map(id=>{const l=locationById.get(id);return l?<button key={id} className="btn secondary" onClick={()=>openMap(mapLink(l))}>{l.title} <ExternalLink size={15}/></button>:null})}</div><p className="muted-note">Показаны только подтверждённые данные.</p></div></Dialog>}
  {activeCase&&<Dialog onClose={()=>setCaseDetail(null)} className="case-dialog"><div className="dialog-content"><span className="overline">ВАШЕ ОБРАЩЕНИЕ</span><h2>Заявка №{String(activeCase.number).padStart(4,'0')}</h2><Status status={activeCase.status}/><div className="case-details"><div><span>Создана</span><strong>{prettyDate(activeCase.createdAt)}</strong></div><div><span>Локаций</span><strong>{activeCase.locationIds.length}</strong></div><div><span>Дополнения</span><strong>{activeCase.updates.length} из 2</strong></div></div><h3>Интересующие локации</h3><div className="case-location-list">{activeCase.locationIds.map(id=><div key={id}><MapPin size={16}/>{locationById.get(id)?.title||`Локация №${id}`}</div>)}</div><h3>История обращения</h3><CaseTimeline record={activeCase}/>{activeCase.status!=='resolved'&&<div className="case-supplement-actions">{activeCase.updates.length<MAX_UPDATES?<><button className="btn secondary" onClick={()=>{setSupplement(activeCase.number);setCaseDetail(null)}}><Plus size={17}/> Дополнить заявку</button><span>Доступно дополнений: {MAX_UPDATES-activeCase.updates.length} из {MAX_UPDATES}</span></>:<div className="quota-message"><CheckCircle2 size={18}/>Вы уже отправили два дополнения. Пожалуйста, дождитесь ответа специалиста.</div>}</div>}<div className="dialog-footer"><button className="btn dark full" onClick={()=>setCaseDetail(null)}>Вернуться к заявкам</button></div></div></Dialog>}
  {creating!==null&&<CreateCase locations={locations} selected={creating} onClose={()=>setCreating(null)} onSubmit={submitCase}/>}
  {supplement!==null&&<SupplementDialog onClose={()=>setSupplement(null)} record={cases.find(c=>c.number===supplement)!} locations={locations} onSubmit={submitSupplement}/>}
 </div>
}
function CaseTimeline({record}:{record:DemoCase}){
 const events:{at:string;icon:ReactNode;title:string;description?:string}[]=[{at:record.createdAt,icon:<Send size={16}/>,title:'Заявка получена'}];
 if(record.takenAt)events.push({at:record.takenAt,icon:<Clock3 size={16}/>,title:'Специалист взял заявку в работу'});
 for(const u of record.updates){events.push({at:u.createdAt,icon:<Plus size={16}/>,title:`Дополнение №${u.id} отправлено`,description:u.comment||`${u.locationIds.length} локаций`});if(u.acceptedAt)events.push({at:u.acceptedAt,icon:<Check size={16}/>,title:`Дополнение №${u.id} принято`})}
 if(record.resolvedAt)events.push({at:record.resolvedAt,icon:<CheckCircle2 size={16}/>,title:'Заявка обработана'});
 events.sort((a,b)=>a.at.localeCompare(b.at));
 return <div className="timeline">{events.map((event,i)=><div className="timeline-event" key={`${event.title}-${i}`}><span className={`timeline-dot ${i===events.length-1?'last':''}`}>{event.icon}</span><div><strong>{event.title}</strong>{event.description&&<p>{event.description}</p>}<small>{prettyDate(event.at)}</small></div></div>)}</div>;
}
function CreateCase({locations,selected,onClose,onSubmit}:{locations:Location[];selected:string;onClose:()=>void;onSubmit:(data:NewCaseInput)=>void}){
 const [stage,setStage]=useState(0);
 const [partner,setPartner]=useState<'existing'|'new'|''>('');
 const [name,setName]=useState('');const [phone,setPhone]=useState('');const [company,setCompany]=useState('');const [id,setId]=useState(selected||locations[0]?.id||'');
 const steps=['Сотрудничество','Контакты','Локация','Проверка'];
 const validPhone=/^\+7\d{10}$/.test(phone.replace(/[\s()\-]/g,''));
 const canNext=stage===0?partner!=='' : stage===1?name.trim().length>=2&&validPhone&&(partner==='existing'||company.trim().length>=2):stage===2?!!id:true;
 const next=()=>{if(canNext)setStage(Math.min(3,stage+1))};
 return <Dialog onClose={onClose} className="form-dialog"><div className="dialog-content"><span className="overline">НОВОЕ ОБРАЩЕНИЕ</span><h2>Новая заявка</h2><p className="section-subtitle">Укажите данные для связи со специалистом.</p><div className="steps-bar">{steps.map((step,i)=><span key={step} className={`step ${i<=stage?'active':''}`} title={step}/>)}</div><p className="step-caption">Шаг {stage+1} из 4 · {steps[stage]}</p>
 {stage===0&&<div className="form-stage"><h3>Вы уже сотрудничаете с нами?</h3><button className={`choose-option ${partner==='existing'?'checked':''}`} onClick={()=>setPartner('existing')}><Building2 size={21}/><span><b>Уже сотрудничаю</b><small>Укажите контактные данные</small></span>{partner==='existing'&&<Check size={19}/>}</button><button className={`choose-option ${partner==='new'?'checked':''}`} onClick={()=>setPartner('new')}><Sparkles size={21}/><span><b>Хочу сотрудничать</b><small>Расскажите о своей компании</small></span>{partner==='new'&&<Check size={19}/>}</button></div>}
 {stage===1&&<div className="form-stage"><h3>Как к Вам обращаться?</h3><label className="input-label">Ваше имя<input autoComplete="off" placeholder="Имя" value={name} onChange={e=>setName(e.target.value)}/></label><label className="input-label">Номер телефона<input autoComplete="off" type="tel" placeholder="+7 999 000 00 00" value={phone} onChange={e=>setPhone(e.target.value)}/></label>{phone&&!validPhone&&<small className="field-error">Проверьте номер: +7 и десять цифр.</small>}{partner==='new'&&<label className="input-label">Название юридического лица<input autoComplete="off" placeholder="Название компании" value={company} onChange={e=>setCompany(e.target.value)}/></label>}<div className="inline-hint"><ShieldCheck size={17}/>Данные будут переданы специалисту для обработки заявки.</div></div>}
 {stage===2&&<div className="form-stage"><h3>Какая локация Вас интересует?</h3><label className="input-label">Выберите локацию<select value={id} onChange={e=>setId(e.target.value)}>{locations.map(l=><option key={l.id} value={l.id}>{l.title}</option>)}</select></label><div className="selected-location-preview">{locations.find(l=>l.id===id)&&<AreaArt location={locations.find(l=>l.id===id)!} compact/>}<div><strong>{locations.find(l=>l.id===id)?.title}</strong><small>{locations.find(l=>l.id===id)?.area}</small></div></div></div>}
 {stage===3&&<div className="form-stage"><h3>Проверьте данные заявки</h3><div className="review-item"><span>Сотрудничество</span><strong>{partner==='existing'?'Уже сотрудничаю':'Хочу сотрудничать'}</strong></div><div className="review-item"><span>Имя</span><strong>{name}</strong></div><div className="review-item"><span>Телефон</span><strong>{phone}</strong></div>{partner==='new'&&<div className="review-item"><span>Юридическое лицо</span><strong>{company}</strong></div>}<div className="review-item"><span>Локация</span><strong>{locations.find(l=>l.id===id)?.title}</strong></div><div className="demo-notice compact-notice"><CircleHelp size={17}/>Заявка будет зарегистрирована и отправлена специалистам.</div></div>}
 <div className="wizard-footer">{stage>0?<button className="btn secondary" onClick={()=>setStage(stage-1)}><ArrowLeft size={17}/> Назад</button>:<button className="btn secondary" onClick={onClose}>Отмена</button>}{stage<3?<button className="btn primary" disabled={!canNext} onClick={next}>Продолжить <ArrowRight size={17}/></button>:<button className="btn primary" onClick={()=>onSubmit({locationId:id,kind:partner as "new"|"existing",company,name,phone})}><Send size={17}/> Отправить заявку</button>}</div></div></Dialog>;
}
function SupplementDialog({record,locations,onClose,onSubmit}:{record:DemoCase;locations:Location[];onClose:()=>void;onSubmit:(number:number,ids:string[],comment:string)=>void}){
 const [ids,setIds]=useState<string[]>([]);const [comment,setComment]=useState('');
 const available=locations.filter(l=>!record.locationIds.includes(l.id));
 const remaining=MAX_UPDATES-record.updates.length;
 return <Dialog onClose={onClose} className="form-dialog"><div className="dialog-content"><span className="overline">ДОПОЛНЕНИЕ К ОБРАЩЕНИЮ</span><h2>Заявка №{String(record.number).padStart(4,'0')}</h2><p className="section-subtitle">Дополнения: {record.updates.length} из {MAX_UPDATES} использовано. Вы можете добавить локацию, вопрос или оба варианта.</p><div className="form-stage"><h3>Дополнительные локации</h3><div className="check-list">{available.map(l=><label key={l.id}><input type="checkbox" checked={ids.includes(l.id)} onChange={e=>setIds(prev=>e.target.checked?[...prev,l.id].slice(0,10):prev.filter(x=>x!==l.id))}/><span>{l.title}</span></label>)}</div><label className="input-label">Уточнение или вопрос<textarea value={comment} onChange={e=>setComment(e.target.value)} maxLength={1000} rows={4} placeholder="Напишите, что ещё Вас интересует…"/></label><small className="field-meta">{comment.length}/1000 символов · Осталось дополнений: {remaining}</small></div><div className="wizard-footer"><button className="btn secondary" onClick={onClose}>Отменить</button><button className="btn primary" disabled={remaining<=0||(ids.length===0&&!comment.trim())} onClick={()=>onSubmit(record.number,ids,comment.trim())}><Send size={17}/> Отправить дополнение</button></div></div></Dialog>;
}
export default App;
