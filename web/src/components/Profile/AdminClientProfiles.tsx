import {useEffect,useMemo,useState} from 'react';
import {BadgePercent,Check,Search,Users} from 'lucide-react';
import type {ClientProfile} from '../../types';
import './admin-profiles.css';

type Props={load:()=>Promise<ClientProfile[]>;save:(userId:number,discountPercent:number)=>Promise<ClientProfile>};
export default function AdminClientProfiles({load,save}:Props){
 const [profiles,setProfiles]=useState<ClientProfile[]>([]);
 const [filter,setFilter]=useState<'all'|'existing'|'new'>('all');
 const [search,setSearch]=useState('');
 const [error,setError]=useState('');
 const [busy,setBusy]=useState<number|null>(null);
 const [ready,setReady]=useState(false);
 const [values,setValues]=useState<Record<number,string>>({});
 useEffect(()=>{let mounted=true;load().then(items=>{if(!mounted)return;setProfiles(items);setValues(Object.fromEntries(items.map(p=>[p.userId,String(p.discountPercent||0)])));setReady(true)}).catch(e=>{if(mounted){setError((e as Error).message);setReady(true)}});return()=>{mounted=false}},[load]);
 const visible=useMemo(()=>profiles.filter(p=>(filter==='all'||p.kind===filter)&&(p.name.toLowerCase().includes(search.toLowerCase())||p.phone.includes(search)||String(p.userId).includes(search))),[profiles,filter,search]);
 const onSave=async(p:ClientProfile)=>{
  const raw=values[p.userId]?.trim()||'';
  if(!/^\d+$/.test(raw)||Number(raw)>100){setError('Введите целый процент скидки от 0 до 100.');return}
  setBusy(p.userId);setError('');
  try{const saved=await save(p.userId,Number(raw));setProfiles(old=>old.map(x=>x.userId===saved.userId?saved:x))}
  catch(e){setError((e as Error).message)}finally{setBusy(null)}
 };
 return <section className="admin-clients" aria-label="Профили клиентов">
  <div className="admin-clients__intro"><span className="admin-clients__icon"><Users size={24}/></span><div><h2>Профили клиентов</h2><p>Ручное начисление скидки на следующую оплату</p></div></div>
  <div className="admin-clients__filters" role="group" aria-label="Тип партнёра">
   {([['all','Все'],['existing','Действующие'],['new','Потенциальные']] as const).map(([key,label])=><button type="button" key={key} className={filter===key?'active':''} onClick={()=>setFilter(key)}>{label}</button>)}
  </div>
  <label className="admin-clients__search"><Search size={18}/><input value={search} onChange={e=>setSearch(e.target.value)} placeholder="Поиск по имени, телефону или Telegram ID" aria-label="Поиск клиентов"/></label>
  {error&&<p className="admin-clients__error" role="alert">{error}</p>}
  {!ready?<p className="admin-clients__empty">Загружаем клиентов…</p>:visible.length===0?<p className="admin-clients__empty">Клиентов по выбранному фильтру пока нет.</p>:<div className="admin-clients__list">{visible.map(p=><div className="admin-clients__card" key={p.userId}>
   <div className="admin-clients__identity"><div><strong>{p.name||'Без имени'}</strong><small>{p.kind==='existing'?'Действующий партнёр':'Потенциальный партнёр'}</small></div><span>Telegram ID: {p.userId}</span></div>
   <p className="admin-clients__contact">{p.phone||'Телефон не указан'}{p.company?' · '+p.company:''}</p>
   <div className="admin-clients__discount"><label><BadgePercent size={18}/> Скидка, %<input inputMode="numeric" type="number" min="0" max="100" step="1" value={values[p.userId]??'0'} onChange={e=>setValues(old=>({...old,[p.userId]:e.target.value}))}/></label><button type="button" disabled={busy!==null||values[p.userId]===String(p.discountPercent||0)} onClick={()=>onSave(p)}>{busy===p.userId?'Сохраняем…':<><Check size={17}/> Сохранить</>}</button></div>
  </div>)}</div>}
 </section>;
}
