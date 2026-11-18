import {useEffect, useMemo, useState} from 'react';
import type {ReactNode} from 'react';
import {
 ArrowLeft, ArrowRight, Building2, Check,
 CheckCircle2, ChevronDown, ChevronRight, CircleHelp, Clock3,
 Compass, ExternalLink, Heart, LayoutGrid, ListFilter, Map as MapIcon,
 MapPin, MessageCircle, Plus, Search, Send, ShieldCheck, SlidersHorizontal,
 Sparkles, X
} from 'lucide-react';
import type {DemoCase, Location, CaseStatus, Tab} from './types';
import {backupLocations, mapLink, MAX_COMPARE, MAX_UPDATES, normalize, prettyDate, seededCases, statusLabel} from './demo';

type WebApp = {
 ready():void; expand():void; close?():void; setHeaderColor?(color:string):void;
 HapticFeedback?:{impactOccurred(style:'light'|'medium'|'heavy'):void};
 initData?:string;
};
declare global {interface Window {Telegram?:{WebApp?:WebApp}}}

const heartKey = 'locations-prototype:favorites:v1';
const casesKey = 'locations-prototype:cases:v1';
function stored<T>(key:string, initial:T):T {
 try{const v=window.localStorage.getItem(key);return v?JSON.parse(v) as T:initial;}catch{return initial;}
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
function BottomNav({tab,onTab,selected}:{tab:Tab;onTab:(t:Tab)=>void;selected:number}){
 const items:{id:Tab;label:string;icon:ReactNode}[]=[
  {id:'catalog',label:'Локации',icon:<Compass size={22}/>},
  {id:'favorites',label:'Избранное',icon:<Heart size={22}/>},
  {id:'requests',label:'Мои заявки',icon:<MessageCircle size={22}/>},
 ];
 return <nav className="bottom-nav" aria-label="Основная навигация">{items.map(i=><button type="button" key={i.id} aria-current={tab===i.id?'page':undefined} className={`nav-item ${tab===i.id?'active':''}`} onClick={()=>{tgHaptic();onTab(i.id)}}>{i.icon}<span>{i.label}</span>{i.id==='favorites'&&selected>0&&<span className="nav-count">{selected}</span>}</button>)}</nav>
}
function Status({status}:{status:CaseStatus}){return <span className={`status status-${status}`}>{statusLabel[status]}</span>}
function Dialog({children,onClose,className=''}:{children:ReactNode;onClose:()=>void;className?:string}){
 useEffect(()=>{const onKey=(e:KeyboardEvent)=>{if(e.key==='Escape')onClose()};document.addEventListener('keydown',onKey);document.body.classList.add('has-dialog');return()=>{document.removeEventListener('keydown',onKey);document.body.classList.remove('has-dialog')}},[onClose]);
 return <div className="dialog-overlay" onMouseDown={e=>{if(e.target===e.currentTarget)onClose()}}><div className={`dialog ${className}`} role="dialog" aria-modal="true"><button className="dialog-close icon-button" onClick={onClose} aria-label="Закрыть"><X size={20}/></button>{children}</div></div>;
}
function LocationCard({l,saved,onDetails,onFavorite,compareMode=false,selected=false,onSelect}:{l:Location;saved:boolean;onDetails:(l:Location)=>void;onFavorite:(id:string)=>void;compareMode?:boolean;selected?:boolean;onSelect?:(id:string)=>void}){
 return <article className={`location-card ${compareMode&&selected?'selected':''}`}>
  <div className="location-art-wrap" onClick={()=>onDetails(l)} role="button" tabIndex={0} onKeyDown={e=>{if(e.key==='Enter')onDetails(l)}}><AreaArt location={l}/><span className="location-tag">{l.category}</span></div>
  <button type="button" className={`card-heart ${saved?'liked':''}`} onClick={()=>{tgHaptic();onFavorite(l.id)}} title={saved?'Удалить из избранного':'В избранное'} aria-label={saved?'Удалить из избранного':'Сохранить в избранное'}><Heart size={19} fill={saved?'currentColor':'none'}/></button>
  <div className="location-body"><div className="location-overline"><MapPin size={13}/>{l.area}<span className="number">№{l.id}</span></div><h3>{l.title}</h3>
   <div className="location-actions"><button className="text-action" onClick={()=>onDetails(l)}>Подробнее <ChevronRight size={16}/></button>{compareMode&&onSelect?<button className={`compare-selector ${selected?'checked':''}`} onClick={()=>onSelect(l.id)}>{selected?<Check size={16}/>:<Plus size={16}/>} {selected?'Выбрано':'Сравнить'}</button>:<button className="map-text-link" onClick={()=>openMap(mapLink(l))}><MapPin size={15}/> Карта</button>}</div>
  </div>
 </article>;
}
function App(){
 const [tab,setTab]=useState<Tab>('catalog');
 const [locations,setLocations]=useState<Location[]>(backupLocations);
 const [loading,setLoading]=useState(true);
 const [query,setQuery]=useState('');
 const [area,setArea]=useState('Все районы');
 const [category,setCategory]=useState('Все');
 const [showMap,setShowMap]=useState(false);
 const [favorites,setFavorites]=useStoredState<string[]>(heartKey,[]);
 const [cases,setCases]=useStoredState<DemoCase[]>(casesKey,seededCases());
 const [compareIds,setCompareIds]=useState<string[]>([]);
 const [compareOpen,setCompareOpen]=useState(false);
 const [detail,setDetail]=useState<Location|null>(null);
 const [caseDetail,setCaseDetail]=useState<number|null>(null);
 const [creating,setCreating]=useState<string|null>(null);
 const [supplement,setSupplement]=useState<number|null>(null);
 const [toast,setToast]=useState('');
 useEffect(()=>{const app=window.Telegram?.WebApp;try{app?.ready();app?.expand();app?.setHeaderColor?.('#f7f9f8')}catch{/* standard browser */}},[]);
 useEffect(()=>{let active=true;fetch('/api/locations').then(async response=>{if(!response.ok)throw new Error('no API');return response.json() as Promise<{locations:Location[]}>}).then(data=>{if(active&&data.locations?.length)setLocations(data.locations)}).catch(()=>{/* standalone UI preview uses bundled demo data */}).finally(()=>{if(active)setLoading(false)});return()=>{active=false}},[]);
 useEffect(()=>{if(!toast)return;const t=setTimeout(()=>setToast(''),3000);return()=>clearTimeout(t)},[toast]);
 const locationById=useMemo(()=>new Map(locations.map(l=>[l.id,l])),[locations]);
 const areas=useMemo(()=>['Все районы',...Array.from(new Set(locations.map(l=>l.area))).sort((a,b)=>a.localeCompare(b,'ru'))],[locations]);
 const filtered=useMemo(()=>locations.filter(l=>(category==='Все'||l.category===category)&&(area==='Все районы'||l.area===area)&&normalize(l.title).includes(normalize(query))),[locations,query,category,area]);
 const favoritesSet=new Set(favorites);
 const favoriteLocations=locations.filter(l=>favoritesSet.has(l.id));
 const activeCase=cases.find(c=>c.number===caseDetail)||null;
 const onTab=(t:Tab)=>{setTab(t);setCompareIds([]);setCompareOpen(false);setShowMap(false);window.scrollTo({top:0,behavior:'smooth'})};
 const favoriteToggle=(id:string)=>{setFavorites(f=>f.includes(id)?f.filter(x=>x!==id):[...f,id]);setToast(favoritesSet.has(id)?'Удалено из избранного':'Добавлено в избранное')};
 const compareToggle=(id:string)=>{setCompareIds(prev=>prev.includes(id)?prev.filter(x=>x!==id):prev.length>=MAX_COMPARE?(setToast('Для сравнения доступны 2–3 локации'),prev):[...prev,id])};
 const submitCase=(id:string)=>{const number=Math.max(44,...cases.map(c=>c.number))+1;const item:DemoCase={number,locationIds:[id],createdAt:new Date().toISOString(),status:'received',updates:[]};setCases(prev=>[item,...prev]);setCreating(null);setCaseDetail(number);setToast('Демо-заявка создана только на этом устройстве')};
 const submitSupplement=(number:number,ids:string[],comment:string)=>{setCases(prev=>prev.map(c=>c.number===number&&c.status!=='resolved'&&c.updates.length<MAX_UPDATES?{...c,locationIds:[...new Set([...c.locationIds,...ids])],updates:[...c.updates,{id:c.updates.length+1,locationIds:ids,comment,createdAt:new Date().toISOString()}]}:c));setSupplement(null);setToast('Демо-дополнение добавлено в историю')};
 const uniqueCategory=Array.from(new Set(locations.map(l=>l.category)));
 const readyForCompare=compareIds.length>=2&&compareIds.length<=3;
 return <div className="app-shell">
  <header className="app-header"><div className="header-inner"><button className="brand" onClick={()=>onTab('catalog')}><span className="brand-icon"><Compass size={23} strokeWidth={2.2}/></span><span className="brand-text">Локации<span className="brand-dot">.</span><small>пространство возможностей</small></span></button><div className="header-actions"><span className="demo-chip"><span/>Демо-версия</span><span className="avatar">Л</span></div></div></header>
  <main className="main-container">
   {tab==='catalog'&&<>
    <section className="hero"><div className="hero-copy"><span className="eyebrow"><Sparkles size={14}/> Возможности для Вашего бизнеса</span><h1>Найдите место,<br/>где начнётся <em>рост.</em></h1><p>Изучайте локации, сравнивайте варианты и выбирайте точки, которые подходят Вашим планам.</p><button className="hero-button" onClick={()=>{document.getElementById('catalog-results')?.scrollIntoView({behavior:'smooth'})}}>Смотреть локации <ArrowRight size={17}/></button></div><div className="hero-visual" aria-hidden="true"><div className="hero-orbit orb-1"/><div className="hero-orbit orb-2"/><div className="hero-map-card card-one"><span className="orbit-dot"/><div><span className="tiny-line"/><span className="tiny-line short"/></div></div><div className="hero-map-card card-two"><MapPin size={20}/><div><span className="tiny-line"/><span className="tiny-line short"/></div></div><div className="hero-pin"><MapPin size={42} strokeWidth={1.7} fill="#fdfefe"/></div><div className="hero-curve"/></div></section>
    <div className="top-metrics"><span><span className="metric-icon"><MapPin size={17}/></span><strong>{locations.length}</strong> локаций</span><span><span className="metric-icon purple"><SlidersHorizontal size={17}/></span>Удобный подбор</span><span><span className="metric-icon orange"><Heart size={17}/></span>Сохраняйте лучшее</span></div>
    <section className="section-block" id="catalog-results"><div className="section-heading"><div><span className="overline">КАТАЛОГ</span><h2>Исследуйте локации</h2><p>Все варианты в одном месте. Найдите подходящий за пару кликов.</p></div><span className="results-counter">Найдено: {filtered.length}</span></div>
     <div className="filters"><label className="search-field"><Search size={19}/><input type="search" placeholder="Название локации или станции" value={query} onChange={e=>setQuery(e.target.value)} aria-label="Поиск локаций"/>{query&&<button onClick={()=>setQuery('')} aria-label="Очистить поиск"><X size={16}/></button>}</label><label className="filter-select"><ListFilter size={17}/><select aria-label="Район" value={area} onChange={e=>setArea(e.target.value)}>{areas.map(a=><option key={a}>{a}</option>)}</select><ChevronDown size={17}/></label></div>
     <div className="catalog-toolbar"><div className="chip-row"><button className={`filter-chip ${category==='Все'?'active':''}`} onClick={()=>setCategory('Все')}>Все локации</button>{uniqueCategory.map(c=><button key={c} className={`filter-chip ${category===c?'active':''}`} onClick={()=>setCategory(c)}>{c}</button>)}</div><div className="view-switch"><button className={!showMap?'active':''} onClick={()=>setShowMap(false)} title="Карточки" aria-label="Карточки"><LayoutGrid size={18}/></button><button className={showMap?'active':''} onClick={()=>setShowMap(true)} title="Карта" aria-label="Карта"><MapIcon size={18}/></button></div></div>
     {showMap?<div className="map-panel"><div className="map-frame"><iframe title="Обзорная карта локаций" loading="lazy" src={`https://yandex.ru/map-widget/v1/?ll=37.6%2C55.65&z=10&pt=${encodeURIComponent(filtered.slice(0,20).map(l=>`${l.longitude},${l.latitude},pm2rdm`).join('~'))}`}/><div className="map-info">Для точной точки откройте её в Яндекс Картах.</div></div><div className="map-aside"><h3>Локации на карте</h3><p>{filtered.length} вариантов</p><div className="map-list">{filtered.map(l=><button key={l.id} onClick={()=>setDetail(l)}><span className="map-aside-pin"><MapPin size={16}/></span><span>{l.title}<small>{l.area}</small></span><ChevronRight size={17}/></button>)}</div></div></div>:
      filtered.length?<div className="card-grid">{filtered.map(l=><LocationCard key={l.id} l={l} saved={favoritesSet.has(l.id)} onDetails={setDetail} onFavorite={favoriteToggle}/>)}</div>:<Empty icon={<Search size={29}/>} title="Ничего не найдено" description="Попробуйте другое название или измените фильтры." action={<button className="btn secondary" onClick={()=>{setQuery('');setArea('Все районы');setCategory('Все')}}>Сбросить фильтры</button>}/>}
     {loading&&<p className="discreet-loading">Загружаем демонстрационный каталог…</p>}
    </section>
   </>}
   {tab==='favorites'&&<section className="section-block tab-section"><div className="section-heading"><div><span className="overline">ВАША ПОДБОРКА</span><h1>Избранное <span className="heading-number">{favoriteLocations.length}</span></h1><p>Сохраняйте интересные варианты и сравнивайте до трёх локаций.</p></div></div>
    {favoriteLocations.length?<><div className="compare-banner"><div className="compare-banner-symbol"><SlidersHorizontal size={22}/></div><div><strong>Сравните варианты между собой</strong><span>Отметьте 2–3 локации. Выбрано: {compareIds.length} из 3.</span></div><button className="btn dark" disabled={!readyForCompare} onClick={()=>setCompareOpen(true)}>Сравнить <ArrowRight size={16}/></button></div><div className="card-grid">{favoriteLocations.map(l=><LocationCard key={l.id} l={l} saved={true} onDetails={setDetail} onFavorite={favoriteToggle} compareMode selected={compareIds.includes(l.id)} onSelect={compareToggle}/>)}</div></>:<Empty icon={<Heart size={30}/>} title="Здесь пока пусто" description="Откройте каталог и нажмите на сердечко у понравившейся локации. Все сохранённые варианты появятся здесь." action={<button className="btn primary" onClick={()=>onTab('catalog')}>Перейти в каталог <ArrowRight size={16}/></button>}/>}
   </section>}
   {tab==='requests'&&<section className="section-block tab-section"><div className="section-heading request-heading"><div><span className="overline">ОБРАЩЕНИЯ</span><h1>Мои заявки</h1><p>Следите за статусом обращения и просматривайте историю в одном месте.</p></div><button className="btn primary" onClick={()=>setCreating('')}><Plus size={17}/> Новая заявка</button></div>
    <div className="demo-notice"><ShieldCheck size={20}/><span><strong>Демонстрационный режим.</strong> Заявки и дополнения не уходят администраторам. Статусы примеров показаны для проверки интерфейса.</span></div>
    <div className="status-journey"><div><span className="journey-icon received"><Send size={18}/></span><b>Получено</b><small>Заявка создана</small></div><span className="journey-line"/><div><span className="journey-icon working"><Clock3 size={18}/></span><b>В работе</b><small>Назначен специалист</small></div><span className="journey-line"/><div><span className="journey-icon resolved"><CheckCircle2 size={18}/></span><b>Решено</b><small>Заявка обработана</small></div></div>
    <div className="request-section-head"><h2>История обращений</h2><span>{cases.length} заявки</span></div>
    <div className="requests-list">{[...cases].sort((a,b)=>b.number-a.number).map(c=><button className="request-card" key={c.number} onClick={()=>setCaseDetail(c.number)}><span className="request-card-icon"><MessageCircle size={22}/></span><span className="request-card-main"><span className="request-id">Заявка №{String(c.number).padStart(4,'0')}</span><span className="request-locations">{c.locationIds.map(id=>locationById.get(id)?.title||`Локация №${id}`).slice(0,2).join(' · ')}</span><small>{prettyDate(c.createdAt)} · Дополнения: {c.updates.length}/{MAX_UPDATES}</small></span><span className="request-card-right"><Status status={c.status}/><ChevronRight size={20}/></span></button>)}</div>
    <p className="requests-footnote">В примерах статусы не обновляются сервером. Для новых демо-заявок используется статус «Получено».</p>
   </section>}
  </main>
  <footer className="site-footer"><span>Подбор локаций · Демонстрационная версия</span><span>Каталог и заявки не связаны с рабочими системами</span></footer>
  <BottomNav tab={tab} onTab={onTab} selected={favorites.length}/>
  {toast&&<div className="toast" role="status"><CheckCircle2 size={18}/>{toast}</div>}
  {detail&&<Dialog onClose={()=>setDetail(null)} className="detail-dialog"><div className="detail-art"><AreaArt location={detail}/></div><div className="detail-padding"><div className="detail-topline"><Pill>{detail.category}</Pill><span>№{detail.id}</span></div><h2>{detail.title}</h2><p className="detail-area"><MapPin size={17}/>{detail.area}</p><div className="detail-facts"><div><span>Широта</span><strong>{detail.latitude.toFixed(4)}</strong></div><div><span>Долгота</span><strong>{detail.longitude.toFixed(4)}</strong></div></div><p className="detail-hint">Здесь появятся реальные фотографии, трафик и описание после подключения материалов. Сейчас показаны проверочные координаты и демонстрационная иллюстрация.</p><div className="detail-buttons"><button className="btn secondary" onClick={()=>favoriteToggle(detail.id)}><Heart size={18} fill={favoritesSet.has(detail.id)?'currentColor':'none'}/>{favoritesSet.has(detail.id)?'В избранном':'В избранное'}</button><button className="btn secondary" onClick={()=>openMap(mapLink(detail!))}><MapPin size={17}/> На карте <ExternalLink size={13}/></button></div><button className="btn primary full" onClick={()=>{setCreating(detail.id);setDetail(null)}}>Оставить заявку по локации <ArrowRight size={18}/></button></div></Dialog>}
  {compareOpen&&<Dialog onClose={()=>setCompareOpen(false)} className="compare-dialog"><div className="dialog-content"><span className="overline">ВАША ПОДБОРКА</span><h2>Сравнение локаций</h2><p className="section-subtitle">Сравнивайте основные данные выбранных вариантов.</p><div className="compare-table"><div className="compare-row compare-top"><span>Параметр</span>{compareIds.map(id=><strong key={id}>{locationById.get(id)?.title}</strong>)}</div>{([{name:'Категория',get:(l:Location)=>l.category},{name:'Район',get:(l:Location)=>l.area},{name:'Координаты',get:(l:Location)=>`${l.latitude.toFixed(3)}, ${l.longitude.toFixed(3)}`}]).map(row=><div className="compare-row" key={row.name}><span>{row.name}</span>{compareIds.map(id=><span key={id}>{locationById.get(id)?row.get(locationById.get(id)!):'—'}</span>)}</div>)}</div><div className="compare-links">{compareIds.map(id=>{const l=locationById.get(id);return l?<button key={id} className="btn secondary" onClick={()=>openMap(mapLink(l))}>{l.title} <ExternalLink size={15}/></button>:null})}</div><p className="muted-note">В этой версии сравниваются только данные тестового каталога. Коммерческие показатели не придуманы и будут добавлены отдельно.</p></div></Dialog>}
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
function CreateCase({locations,selected,onClose,onSubmit}:{locations:Location[];selected:string;onClose:()=>void;onSubmit:(id:string)=>void}){
 const [stage,setStage]=useState(0);
 const [partner,setPartner]=useState<'existing'|'new'|''>('');
 const [name,setName]=useState('');const [phone,setPhone]=useState('');const [company,setCompany]=useState('');const [id,setId]=useState(selected||locations[0]?.id||'');
 const steps=['Сотрудничество','Контакты','Локация','Проверка'];
 const validPhone=/^\+7\d{10}$/.test(phone.replace(/[\s()\-]/g,''));
 const canNext=stage===0?partner!=='' : stage===1?name.trim().length>=2&&validPhone&&(partner==='existing'||company.trim().length>=2):stage===2?!!id:true;
 const next=()=>{if(canNext)setStage(Math.min(3,stage+1))};
 return <Dialog onClose={onClose} className="form-dialog"><div className="dialog-content"><span className="overline">ДЕМОНСТРАЦИЯ</span><h2>Новая заявка</h2><p className="section-subtitle">Пройдите привычные шаги. Данные никуда не отправятся.</p><div className="steps-bar">{steps.map((step,i)=><span key={step} className={`step ${i<=stage?'active':''}`} title={step}/>)}</div><p className="step-caption">Шаг {stage+1} из 4 · {steps[stage]}</p>
 {stage===0&&<div className="form-stage"><h3>Вы уже сотрудничаете с нами?</h3><button className={`choose-option ${partner==='existing'?'checked':''}`} onClick={()=>setPartner('existing')}><Building2 size={21}/><span><b>Уже сотрудничаю</b><small>Укажите контактные данные</small></span>{partner==='existing'&&<Check size={19}/>}</button><button className={`choose-option ${partner==='new'?'checked':''}`} onClick={()=>setPartner('new')}><Sparkles size={21}/><span><b>Хочу сотрудничать</b><small>Расскажите о своей компании</small></span>{partner==='new'&&<Check size={19}/>}</button></div>}
 {stage===1&&<div className="form-stage"><h3>Как к Вам обращаться?</h3><label className="input-label">Ваше имя<input autoComplete="off" placeholder="Имя" value={name} onChange={e=>setName(e.target.value)}/></label><label className="input-label">Номер телефона<input autoComplete="off" type="tel" placeholder="+7 999 000 00 00" value={phone} onChange={e=>setPhone(e.target.value)}/></label>{phone&&!validPhone&&<small className="field-error">Проверьте номер: +7 и десять цифр.</small>}{partner==='new'&&<label className="input-label">Название юридического лица<input autoComplete="off" placeholder="Название компании" value={company} onChange={e=>setCompany(e.target.value)}/></label>}<div className="inline-hint"><ShieldCheck size={17}/>Контактные данные используются только на этом экране и не сохраняются в демонстрационной заявке.</div></div>}
 {stage===2&&<div className="form-stage"><h3>Какая локация Вас интересует?</h3><label className="input-label">Выберите локацию<select value={id} onChange={e=>setId(e.target.value)}>{locations.map(l=><option key={l.id} value={l.id}>{l.title}</option>)}</select></label><div className="selected-location-preview">{locations.find(l=>l.id===id)&&<AreaArt location={locations.find(l=>l.id===id)!} compact/>}<div><strong>{locations.find(l=>l.id===id)?.title}</strong><small>{locations.find(l=>l.id===id)?.area}</small></div></div></div>}
 {stage===3&&<div className="form-stage"><h3>Проверьте данные заявки</h3><div className="review-item"><span>Сотрудничество</span><strong>{partner==='existing'?'Уже сотрудничаю':'Хочу сотрудничать'}</strong></div><div className="review-item"><span>Имя</span><strong>{name}</strong></div><div className="review-item"><span>Телефон</span><strong>{phone}</strong></div>{partner==='new'&&<div className="review-item"><span>Юридическое лицо</span><strong>{company}</strong></div>}<div className="review-item"><span>Локация</span><strong>{locations.find(l=>l.id===id)?.title}</strong></div><div className="demo-notice compact-notice"><CircleHelp size={17}/>Будет создана только демонстрационная заявка со статусом «Получено».</div></div>}
 <div className="wizard-footer">{stage>0?<button className="btn secondary" onClick={()=>setStage(stage-1)}><ArrowLeft size={17}/> Назад</button>:<button className="btn secondary" onClick={onClose}>Отмена</button>}{stage<3?<button className="btn primary" disabled={!canNext} onClick={next}>Продолжить <ArrowRight size={17}/></button>:<button className="btn primary" onClick={()=>onSubmit(id)}><Send size={17}/> Отправить демо-заявку</button>}</div></div></Dialog>;
}
function SupplementDialog({record,locations,onClose,onSubmit}:{record:DemoCase;locations:Location[];onClose:()=>void;onSubmit:(number:number,ids:string[],comment:string)=>void}){
 const [ids,setIds]=useState<string[]>([]);const [comment,setComment]=useState('');
 const available=locations.filter(l=>!record.locationIds.includes(l.id));
 const remaining=MAX_UPDATES-record.updates.length;
 return <Dialog onClose={onClose} className="form-dialog"><div className="dialog-content"><span className="overline">ДОПОЛНЕНИЕ К ОБРАЩЕНИЮ</span><h2>Заявка №{String(record.number).padStart(4,'0')}</h2><p className="section-subtitle">Дополнения: {record.updates.length} из {MAX_UPDATES} использовано. Вы можете добавить локацию, вопрос или оба варианта.</p><div className="form-stage"><h3>Дополнительные локации</h3><div className="check-list">{available.map(l=><label key={l.id}><input type="checkbox" checked={ids.includes(l.id)} onChange={e=>setIds(prev=>e.target.checked?[...prev,l.id].slice(0,10):prev.filter(x=>x!==l.id))}/><span>{l.title}</span></label>)}</div><label className="input-label">Уточнение или вопрос<textarea value={comment} onChange={e=>setComment(e.target.value)} maxLength={1000} rows={4} placeholder="Напишите, что ещё Вас интересует…"/></label><small className="field-meta">{comment.length}/1000 символов · Осталось дополнений: {remaining}</small></div><div className="wizard-footer"><button className="btn secondary" onClick={onClose}>Отменить</button><button className="btn primary" disabled={remaining<=0||(ids.length===0&&!comment.trim())} onClick={()=>onSubmit(record.number,ids,comment.trim())}><Send size={17}/> Отправить дополнение</button></div></div></Dialog>;
}
export default App;
