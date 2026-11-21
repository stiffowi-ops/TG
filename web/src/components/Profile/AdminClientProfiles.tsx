import {useEffect,useMemo,useState} from 'react';
import {BadgePercent,Check,Search,UserRound,ChevronRight,ChevronDown} from 'lucide-react';
import type {ClientProfile} from '../../types';
import './admin-profiles.css';

const manualDiscount=(profile:ClientProfile)=>profile.manualDiscountPercent??profile.discountPercent;

type Props={load:()=>Promise<ClientProfile[]>;save:(userId:number,discountPercent:number)=>Promise<ClientProfile>};
export default function AdminClientProfiles({load,save}:Props){
 const [profiles,setProfiles]=useState<ClientProfile[]>([]);
 const [filter,setFilter]=useState<'all'|'existing'|'new'>('all');
 const [expanded,setExpanded]=useState<number|null>(null);
 const [savedID,setSavedID]=useState<number|null>(null);
 const [search,setSearch]=useState('');
 const [error,setError]=useState('');
 const [busy,setBusy]=useState<number|null>(null);
 const [ready,setReady]=useState(false);
 const [values,setValues]=useState<Record<number,string>>({});
 useEffect(()=>{let mounted=true;load().then(items=>{if(!mounted)return;setProfiles(items);setValues(Object.fromEntries(items.map(p=>[p.userId,String(manualDiscount(p))])));setReady(true)}).catch(e=>{if(mounted){setError((e as Error).message);setReady(true)}});return()=>{mounted=false}},[load]);
 const visible=useMemo(()=>profiles.filter(p=>(filter==='all'||p.kind===filter)&&(p.name.toLocaleLowerCase('ru').includes(search.trim().toLocaleLowerCase('ru'))||p.phone.includes(search)||String(p.userId).includes(search))),[profiles,filter,search]);
 const onSave=async(p:ClientProfile)=>{
  const raw=values[p.userId]?.trim()||'';
  if(!/^\d+$/.test(raw)||Number(raw)>100){setError('Введите целый процент скидки от 0 до 100.');return}
  setBusy(p.userId);setSavedID(null);setError('');
  try{const saved=await save(p.userId,Number(raw));setProfiles(old=>old.map(x=>x.userId===saved.userId?saved:x));setSavedID(p.userId);setValues(old=>({...old,[p.userId]:String(manualDiscount(saved))}))}
  catch(e){setError((e as Error).message)}finally{setBusy(null)}
 };
 return <section className="admin-clients" aria-label="Профили клиентов">
  <label className="admin-clients__search"><Search size={18}/><input value={search} onChange={e=>setSearch(e.target.value)} placeholder="Имя или Telegram ID" aria-label="Поиск клиентов"/></label>
  <div className="admin-clients__filters" role="group" aria-label="Тип партнёра">
   {([['all','Все'],['existing','Действующие'],['new','Потенциальные']] as const).map(([key,label])=><button type="button" key={key} aria-pressed={filter===key} className={filter===key?'active':''} onClick={()=>setFilter(key)}>{label}</button>)}
  </div>
  {error&&<p className="admin-clients__error" role="alert">{error}</p>}
  {!ready?<p className="admin-clients__empty">Загружаем клиентов…</p>:visible.length===0?<p className="admin-clients__empty">Клиентов по выбранному фильтру пока нет.</p>:<div className="admin-clients__list">{visible.map(p=><div className="admin-clients__card" key={p.userId}>
   <button type="button" className="admin-client-toggle admin-client-head" aria-expanded={expanded===p.userId} onClick={()=>setExpanded(expanded===p.userId?null:p.userId)}>
    <span className="profile-symbol"><UserRound size={21}/></span><span className="profile-current__copy"><strong>{p.name||'Без имени'}</strong><small>Telegram ID {p.userId}</small></span>
    {expanded===p.userId?<ChevronDown size={17}/>:<><b>{p.discountPercent}%</b><ChevronRight size={17}/></>}
   </button>
   <span className={'discount-tag '+(p.kind==='existing'?'discount-tag--used':'discount-tag--new')}>{p.kind==='existing'?'Действующий партнёр':p.kind==='new'?'Потенциальный партнёр':'Тип партнёра не указан'}</span>
   {expanded===p.userId&&<div className="admin-client-body">
   <div className="discount-history-row"><span>Действующая скидка</span><strong>{p.discountPercent}%</strong></div>
   <small>По приглашениям · {p.referralDiscountPercent||0}%</small>
   <p className="admin-clients__contact">{p.phone||'Телефон не указан'}{p.company?' · '+p.company:''}</p>
   <div className="admin-clients__discount"><label><BadgePercent size={18}/> Ручная скидка, %<input inputMode="numeric" type="number" min="0" max="100" step="1" value={values[p.userId]??'0'} onChange={e=>{setSavedID(null);setValues(old=>({...old,[p.userId]:e.target.value}))}}/></label><button type="button" disabled={busy!==null||values[p.userId]===String(manualDiscount(p))} onClick={()=>onSave(p)}>{busy===p.userId?'Сохраняем…':<><Check size={17}/> Сохранить</>}</button></div>
   {savedID===p.userId&&<small role="status">Ручная скидка сохранена.</small>}
   </div>}
  </div>)}</div>}
 </section>;
}
