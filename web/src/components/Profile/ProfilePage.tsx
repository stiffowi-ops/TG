import {useEffect,useState} from 'react';
import type {DemoCase,Location,ClientProfile,MiniReferralInfo,DiscountWallet} from '../../types';
import {prettyDate,MAX_UPDATES} from '../../demo';
import ProfileGlyph from './ProfileGlyph';
import ProfileSummaryCard from './ProfileSummaryCard';
import ProfileShortcutTile from './ProfileShortcutTile';
import ProfileReferralBanner from './ProfileReferralBanner';
import ActiveRequestPreview from './ActiveRequestPreview';
import ProgressTracker from './ProgressTracker';
import ReferralDetails from './ReferralDetails';
import './profile.css';

type DraftProfile=Pick<ClientProfile,'name'|'phone'|'company'|'kind'>;
type Props={view:ProfileView;setView:(view:ProfileView)=>void;wallet:DiscountWallet|null;onRequest:(offer:string)=>Promise<void>;cases:DemoCase[];locations:Location[];user?:{firstName:string}|null;profile:ClientProfile|null;referral:MiniReferralInfo|null;referralLoading:boolean;referralError:string;onSave:(p:DraftProfile)=>Promise<void>;onCreate:()=>void;onSupplement:(n:number)=>void};
export type ProfileView='home'|'data'|'requests'|'discount'|'case';
const statusTitle:Record<DemoCase['status'],string>={received:'Получено',working:'В работе',resolved:'Решено'};
function StatusTag({status}:{status:DemoCase['status']}) {
 return <span className={'profile-status profile-status--'+status}>{statusTitle[status]}</span>;
}
function ProfileDataForm({profile,onSave}:{profile:ClientProfile|null;onSave:Props['onSave']}) {
 const [name,setName]=useState(profile?.name||'');
 const [phone,setPhone]=useState(profile?.phone||'');
 const [kind,setKind]=useState<'new'|'existing'|''>(profile?.kind||'');
 const [company,setCompany]=useState(profile?.company||'');
 const [busy,setBusy]=useState(false);
 const [message,setMessage]=useState('');
 useEffect(()=>{setName(profile?.name||'');setPhone(profile?.phone||'');setKind(profile?.kind||'');setCompany(profile?.company||'')},[profile?.name,profile?.phone,profile?.kind,profile?.company]);
 const save=async(event:React.FormEvent<HTMLFormElement>)=>{
  event.preventDefault();setMessage('');setBusy(true);
  try{await onSave({name,phone,kind:kind as ClientProfile['kind'],company});setMessage('Данные сохранены')}
  catch(error){setMessage((error as Error).message)}
  finally{setBusy(false)}
 };
 return <form className="profile-data-form" onSubmit={save}>
   <p>Заполни данные один раз, чтобы подавать следующие заявки быстрее.</p>
   <label>Имя или ФИО<input value={name} onChange={e=>setName(e.target.value)} placeholder="Как к вам обращаться" autoComplete="name" maxLength={120} required/></label>
   <label>Номер телефона<input value={phone} onChange={e=>setPhone(e.target.value)} type="tel" placeholder="+7 999 000-00-00" autoComplete="tel" required/></label>
   <label>Тип партнёра<select value={kind} onChange={e=>setKind(e.target.value as 'new'|'existing'|'')} required><option value="">Выберите вариант</option><option value="existing">Уже партнёр</option><option value="new">Хочу стать партнёром</option></select></label>
   <label>Организация<input value={company} onChange={e=>setCompany(e.target.value)} placeholder="Название организации" maxLength={200}/></label>
   {message&&<p className="profile-data-form__feedback" role="status">{message}</p>}
   <button type="submit" className="profile-cta" disabled={busy}><ProfileGlyph name="done" size={20}/>{busy?'Сохраняем…':'Сохранить данные'}</button>
 </form>;
}
export function ProfileEditor({profile,onSave}:{profile:ClientProfile|null;onSave:Props['onSave']}){return <ProfileDataForm profile={profile} onSave={onSave}/>;}
function RequestItem({record,title,onClick}:{record:DemoCase;title:string;onClick:()=>void}) {
 return <button type="button" className="profile-request-item" onClick={onClick}>
  <span className="profile-symbol profile-symbol--application"><ProfileGlyph name="application" size={22}/></span>
  <span className="profile-request-item__copy"><strong>{title}</strong><small>{prettyDate(record.createdAt)}</small></span>
  <StatusTag status={record.status}/>
  <ProfileGlyph name="chevron" size={18} className="profile-chevron"/>
 </button>;
}
export {default as ProfileProgressTracker} from './ProgressTracker';

export default function ProfilePage({view,setView,wallet,onRequest,cases,locations,user,profile,referral,referralLoading,referralError,onSave,onCreate,onSupplement}:Props){
 const [caseNumber,setCaseNumber]=useState<number|null>(null);
 const names=new Map(locations.map(l=>[l.id,l.title]));
 const sorted=[...cases].sort((a,b)=>b.number-a.number);
 const lastUse=wallet?.history?.[0];
 const active=sorted.find(c=>c.status==='received'||c.status==='working');
 const current=sorted.find(c=>c.number===caseNumber);
 const caseName=(c:DemoCase)=>'Заявка '+(names.get(c.locationIds[0]||'')||'Локация');
 const openCase=(number:number)=>{setCaseNumber(number);setView('case')};
 if(view==='home')return <section className="profile-page profile-page--home">
  <div className="profile-page__title"><h1>Мой профиль</h1><button className="profile-settings" aria-label="Настройки профиля" onClick={()=>setView('data')}><ProfileGlyph name="settings" size={21}/></button></div>
  <ProfileSummaryCard profile={profile} fallbackName={user?.firstName} onClick={()=>setView('data')}/>
  <ProfileReferralBanner onClick={()=>setView('discount')}/>
  {lastUse&&<button type="button" className="profile-current" onClick={()=>setView('discount')}>
   <span className="profile-symbol"><ProfileGlyph name={lastUse.status==='pending'?'clock':'done'} size={22}/></span>
   <span className="profile-current__copy"><strong>Скидка {lastUse.percent}% {lastUse.status==='pending'?'на согласовании':'использована'}</strong><small>{lastUse.status==='pending'?'Запрос у администратора':'Посмотреть историю'}</small></span><ProfileGlyph name="chevron" size={18}/>
  </button>}
  <div className="profile-shortcuts">
   <ProfileShortcutTile title="Мои данные" subtitle="Контакты и реквизиты" icon="user" theme="warm" onClick={()=>setView('data')}/>
   <ProfileShortcutTile title="Мои заявки" subtitle="История и статус" icon="requests" theme="blue" onClick={()=>setView('requests')}/>
  </div>
  {active&&<ActiveRequestPreview record={active} title={names.get(active.locationIds[0]||'')||'Локация'} onClick={()=>openCase(active.number)}/>}
  <button type="button" className="profile-cta" onClick={onCreate}><ProfileGlyph name="plus" size={23}/> Подать заявку</button>
 </section>;
 return <section className="profile-page profile-page--subview">
  {view==='data'&&<><h1>Мои данные</h1><ProfileDataForm profile={profile} onSave={onSave}/></>}
  {view==='discount'&&<><h1>Бонусы</h1><ReferralDetails wallet={wallet} onRequest={onRequest} info={referral} loading={referralLoading} error={referralError}/></>}
  {view==='requests'&&<><h1>Мои заявки</h1>{sorted.length?sorted.map(c=><RequestItem key={c.number} record={c} title={caseName(c)} onClick={()=>openCase(c.number)}/>):<div className="profile-empty">Пока нет заявок. После отправки они появятся здесь.</div>}<button className="profile-cta" onClick={onCreate}><ProfileGlyph name="plus" size={22}/> Подать заявку</button></>}
  {view==='case'&&current&&<><h1>{caseName(current)}</h1><p className="profile-muted">{prettyDate(current.createdAt)}</p><div className="profile-detail-tracker"><ProgressTracker status={current.status}/></div><h2>История заявки</h2><div className="profile-history">
    <div><span className="profile-history__dot"/><strong>Заявка получена</strong><small>{prettyDate(current.createdAt)}</small></div>
    {current.takenAt&&<div><span className="profile-history__dot"/><strong>Специалист взял в работу</strong><small>{prettyDate(current.takenAt)}</small></div>}
    {current.updates.map(u=><div key={u.id}><span className="profile-history__dot"/><strong>Дополнение №{u.id} отправлено</strong><small>{prettyDate(u.createdAt)}</small></div>)}
    {current.resolvedAt&&<div><span className="profile-history__dot"/><strong>Заявка обработана</strong><small>{prettyDate(current.resolvedAt)}</small></div>}
   </div>{current.status!=='resolved'&&(current.updates.length<MAX_UPDATES?<button className="profile-cta profile-cta--outline" onClick={()=>onSupplement(current.number)}>Дополнить заявку</button>:<p className="profile-muted">Лимит дополнений исчерпан. Дождитесь ответа специалиста.</p>)}</>}
 </section>;
}
