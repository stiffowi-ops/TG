import {useEffect,useState} from 'react';
import {ArrowLeft,ChevronRight,CheckCircle2,ClipboardList,Gift,MapPin,Plus,Send,UserRound,Clock3,Link2} from 'lucide-react';
import type {DemoCase,Location} from '../../types';
import {prettyDate,MAX_UPDATES} from '../../demo';
import type {ClientProfile} from '../../types';
import './profile.css';

type Props={cases:DemoCase[];locations:Location[];user?:{firstName:string}|null;profile:ClientProfile|null;onSave:(p:Pick<ClientProfile,'name'|'phone'|'company'|'kind'>)=>Promise<void>;onCreate:()=>void;onSupplement:(n:number)=>void};
const states=['received','working','resolved'] as const;
const titles=['Получено','В работе','Решено'];
function StatusTag({status}:{status:DemoCase['status']}){return <span className={'profile-status profile-status--'+status}>{titles[states.indexOf(status)]}</span>}
export function ProgressTracker({status,compact=false}:{status:DemoCase['status'];compact?:boolean}){
 const idx=states.indexOf(status);
 const [animated,setAnimated]=useState(false);
 useEffect(()=>{setAnimated(false);const timer=window.setTimeout(()=>setAnimated(true),100);return()=>window.clearTimeout(timer)},[status]);
 return <div className={'profile-tracker '+(compact?'profile-tracker--compact':'')} aria-label={'Статус заявки: '+titles[idx]}>
  <div className="profile-tracker__line"><span style={{width:((animated?idx:0)*50)+'%'}}/></div>
  {states.map((key,i)=>{const Icon=i===0?Send:i===1?Clock3:CheckCircle2;return <div key={key} className={'profile-tracker__step profile-tracker__step--'+key+(i===idx?' current':'')+(i<=idx?' visited':'')}><span className="profile-tracker__icon"><Icon size={22}/></span><strong>{titles[i]}</strong></div>})}
 </div>;
}
export function ProfileEditor({profile,user,onSave}:{profile:ClientProfile|null;user?:{firstName:string}|null;onSave:Props['onSave']}){
 const [open,setOpen]=useState(false);
 const [name,setName]=useState(profile?.name||'');
 const [phone,setPhone]=useState(profile?.phone||'');
 const [company,setCompany]=useState(profile?.company||'');
 const [kind,setKind]=useState<'new'|'existing'|''>(profile?.kind||'');
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState('');
 useEffect(()=>{setName(profile?.name||'');setPhone(profile?.phone||'');setCompany(profile?.company||'');setKind(profile?.kind||'')},[profile]);
 const save=async()=>{setError('');setBusy(true);try{await onSave({name,phone,company,kind:kind as 'new'|'existing'});setOpen(false)}catch(e){setError((e as Error).message)}finally{setBusy(false)}};
 return <div className="profile-editor">
  <button type="button" className="profile-editor__top" onClick={()=>setOpen(!open)} aria-expanded={open}>
   <span className="profile-icon profile-icon--warm"><UserRound size={24}/></span><span className="profile-editor__text"><strong>Данные для заявок</strong><small>{profile?.name||'Заполните профиль'}{profile?.phone?' · •• '+profile.phone.slice(-2):' · телефон не указан'}</small></span><ChevronRight size={20} className={open?'profile-rotated':''}/>
  </button>
  {open&&<div className="profile-editor__fields">
   <p>Заполните один раз, чтобы следующие заявки были короче.</p>
   <label>Имя или ФИО<input value={name} onChange={e=>setName(e.target.value)} placeholder={user?.firstName||'Ваше имя'} maxLength={120}/></label>
   <label>Телефон<input value={phone} onChange={e=>setPhone(e.target.value)} type="tel" placeholder="+7 999 000 00 00"/></label>
   <label>Тип партнёра<select value={kind} onChange={e=>setKind(e.target.value as 'new'|'existing'|'')}><option value="">Выберите тип</option><option value="existing">Действующий</option><option value="new">Потенциальный</option></select></label>
   <label>Организация<input value={company} onChange={e=>setCompany(e.target.value)} placeholder="Название компании" maxLength={200}/></label>
   {error&&<p className="profile-error" role="alert">{error}</p>}
   <button type="button" className="profile-btn profile-btn--primary" disabled={busy} onClick={save}>{busy?'Сохраняем…':'Сохранить данные'}</button>
  </div>}
 </div>;
}
export function DiscountPanel({percent}:{percent:number}){
 const [opened,setOpened]=useState(false);
 return opened?<section className="profile-discount-view">
  <button className="profile-text-back" onClick={()=>setOpened(false)}><ArrowLeft size={18}/> К профилю</button>
  <h2>Бонусы</h2><p>Реферальная программа</p>
  <div className="profile-discount-view__status"><span>Ваша скидка на следующую оплату</span><strong>{percent>0?percent+'%':'У Вас нет скидок'}</strong></div>
  <div className="profile-discount-view__info"><Gift size={26}/><span>Приглашайте новых клиентов и получайте скидку на следующую оплату</span></div>
  <div className="profile-discount-view__placeholder"><Link2 size={19}/> Реферальная ссылка скоро появится</div>
 </section>:<button className="profile-discount-strip" type="button" onClick={()=>setOpened(true)}>
  <span className="profile-icon profile-icon--warm"><Gift size={25}/></span>
  <span className="profile-discount-strip__copy"><strong>{percent>0?'Ваша скидка '+percent+'%':'Бонусы'}</strong><small>{percent>0?'На следующую оплату':'Приглашайте новых клиентов и получайте скидку на следующую оплату'}</small></span><ChevronRight size={20}/>
 </button>;
}
function RequestItem({record,lookup,onClick}:{record:DemoCase;lookup:Map<string,string>;onClick:()=>void}){
 const label=lookup.get(record.locationIds[0]||'')||'Локация';
 return <button className="profile-request-item" onClick={onClick}><span className="profile-icon profile-icon--warm"><MapPin size={20}/></span><span className="profile-request-item__name"><strong>Заявка {label}</strong><small>{prettyDate(record.createdAt)}</small></span><StatusTag status={record.status}/><ChevronRight size={18}/></button>;
}
export default function ProfilePage({cases,locations,user,profile,onSave,onCreate,onSupplement}:Props){
 const [selected,setSelected]=useState<number|null>(null);
 const [all,setAll]=useState(false);
 const lookup=new Map(locations.map(l=>[l.id,l.title]));
 const sorted=[...cases].sort((a,b)=>b.number-a.number);
 const active=sorted.find(c=>c.status==='received'||c.status==='working');
 const chosen=sorted.find(c=>c.number===selected);
 const label=(c:DemoCase)=>'Заявка '+(lookup.get(c.locationIds[0]||'')||'Локация');
 if(chosen)return <section className="profile-page"><button className="profile-text-back" onClick={()=>setSelected(null)}><ArrowLeft size={19}/> К профилю</button><h1>{label(chosen)}</h1><p className="profile-subtitle">{prettyDate(chosen.createdAt)}</p><div className="profile-active"><ProgressTracker status={chosen.status}/></div><h2>История заявки</h2><div className="profile-history"><div><span className="profile-history__dot"/><strong>Заявка получена</strong><small>{prettyDate(chosen.createdAt)}</small></div>{chosen.takenAt&&<div><span className="profile-history__dot"/><strong>Специалист взял в работу</strong><small>{prettyDate(chosen.takenAt)}</small></div>}{chosen.updates.map(u=><div key={u.id}><span className="profile-history__dot"/><strong>Дополнение №{u.id} отправлено</strong><small>{prettyDate(u.createdAt)}</small></div>)}{chosen.resolvedAt&&<div><span className="profile-history__dot"/><strong>Заявка обработана</strong><small>{prettyDate(chosen.resolvedAt)}</small></div>}</div>{chosen.status!=='resolved'&&(chosen.updates.length<MAX_UPDATES?<button className="profile-btn profile-btn--outline" onClick={()=>onSupplement(chosen.number)}>Дополнить заявку</button>:<p>Лимит дополнений исчерпан. Пожалуйста, дождитесь ответа.</p>)}</section>;
 return <section className="profile-page">
  {all?<><button className="profile-text-back" onClick={()=>setAll(false)}><ArrowLeft size={19}/> Мой профиль</button><h1>Мои заявки</h1></>:<><h1>Мой профиль</h1><ProfileEditor profile={profile} user={user} onSave={onSave}/><DiscountPanel percent={profile?.discountPercent||0}/>{active&&<div className="profile-active"><button className="profile-active__header" onClick={()=>setSelected(active.number)}><span className="profile-icon profile-icon--warm"><ClipboardList size={22}/></span><span><small>Активная заявка</small><strong>{label(active)}</strong></span><StatusTag status={active.status}/><ChevronRight size={18}/></button><button className="profile-active__tracker" onClick={()=>setSelected(active.number)}><ProgressTracker status={active.status} compact/></button></div>}</>}
  <button className="profile-btn profile-btn--primary" onClick={onCreate}><Plus size={20}/> Подать заявку</button>
  <div className="profile-list-heading"><h2>Мои заявки</h2>{!all&&sorted.length>3&&<button onClick={()=>setAll(true)}>Все заявки <ChevronRight size={16}/></button>}</div>
  {sorted.length?<div className="profile-request-list">{(all?sorted:sorted.slice(0,3)).map(c=><RequestItem record={c} lookup={lookup} onClick={()=>setSelected(c.number)} key={c.number}/>)}</div>:<p className="profile-placeholder">Заявок пока нет. После отправки они появятся здесь.</p>}
  {!all&&sorted.length>0&&<button className="profile-all" onClick={()=>setAll(true)}>История всех заявок <ChevronRight size={17}/></button>}
 </section>;
}
