import {useState} from 'react';
import {BadgePercent, CheckCircle2, Gift, Clock3, ChevronDown, ChevronRight} from 'lucide-react';
import type {DiscountWallet} from '../../types';
import ProgressTracker from './ProgressTracker';
import {prettyDate} from '../../demo';
import './discount-usage.css';

type Props={wallet:DiscountWallet|null;onRequest:(offer:string)=>Promise<void>;compact?:boolean;onDetails?:()=>void};

export default function DiscountUsage({wallet,onRequest,compact=false,onDetails}:Props) {
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState('');
 const [expanded,setExpanded]=useState(false);
 const latest=wallet?.history?.[0];
 const pending=latest?.status==='pending';
 const used=latest?.status==='used'&&!wallet?.percent;
 const percent=pending||used?latest.percent:wallet?.percent??0;
 const phase=used?'used':pending?'pending':percent?'available':'empty';
 async function request(){
  if(!wallet?.offer||busy)return;
  setBusy(true);setError('');
  try{await onRequest(wallet.offer)}catch(e){setError((e as Error).message)}finally{setBusy(false)}
 }
 if(compact){
  const status=used?'Применение скидки '+percent+'% подтверждено':pending?'Ожидает подтверждения':percent?'Скидка доступна':'Приглашайте клиентов по ссылке';
  const title=used?'Скидка использована':pending?'Скидка '+percent+'%':percent?'Ваша скидка: '+percent+'%':'У вас нет скидок';
  return <section className={'profile-rewards profile-rewards--'+phase} aria-label="Мои скидки и приглашения">
   <button type="button" className="profile-rewards__summary" aria-expanded={expanded} onClick={()=>setExpanded(value=>!value)}>
    <span className="profile-rewards__icon" key={phase}>{used?<CheckCircle2 size={20}/>:pending?<Clock3 size={20}/>:<Gift size={20}/>}</span>
    <span className="profile-rewards__copy" key={'text-'+phase} aria-live="polite"><strong>{title}</strong><small>{status}</small></span>
    <ChevronDown size={20} className={'profile-rewards__arrow'+(expanded?' is-expanded':'')} aria-hidden="true"/>
   </button>
   {wallet?.offer&&!pending&&<button className="profile-cta profile-rewards__redeem" type="button" disabled={busy} onClick={request}><BadgePercent size={18}/>{busy?'Отправляем…':'Воспользоваться скидкой'}</button>}
   {expanded&&<div className="profile-rewards__details">
    {pending&&<small className="profile-rewards__info">Запрос на использование скидки отправлен. Мы сообщим о результате.</small>}
    {latest&&<div className="profile-rewards__tracker"><ProgressTracker status={pending?'working':'resolved'} labels={['Отправлено','Подтверждение','Использовано']} subject="скидки"/></div>}
    {onDetails&&<button type="button" className="profile-rewards__history" onClick={onDetails}>Моя ссылка и история приглашений <ChevronRight size={18}/></button>}
   </div>}
   {error&&<small role="alert" className="profile-rewards__error">{error}</small>}
  </section>;
 }
 return <>
  <section className={'discount-balance'+(used?' discount-balance--used':'')} aria-live="polite">
   <div>{used?<CheckCircle2 size={24}/>:pending?<Clock3 size={24}/>:<Gift size={24}/>}<span>{used?'Скидка использована':pending?'Ожидает подтверждения':'Скидка доступна'}</span></div>
   <strong className={percent?'':'discount-balance__empty'}>{percent?percent+'%':'У вас нет скидок'}</strong>
   {pending?<span className="discount-tag">Запрос на использование скидки отправлен</span>:wallet?.offer?
    <button className="profile-cta" type="button" disabled={busy} onClick={request}><BadgePercent size={18}/>{busy?'Отправляем…':'Воспользоваться скидкой'}</button>:
    <small>{used?'Применение скидки подтверждено.':'Приглашайте клиентов по своей ссылке.'}</small>}
   {error&&<small role="alert">{error}</small>}
  </section>
  {latest&&<div className="profile-detail-tracker"><ProgressTracker status={pending?'working':'resolved'} labels={['Отправлено','Подтверждение','Использовано']} subject="скидки"/></div>}
 </>;
}
export function DiscountHistory({wallet}:{wallet:DiscountWallet|null}){
 const used=wallet?.history?.filter(r=>r.status==='used')||[];
 if(!used.length)return null;
 return <section className="discount-panel"><h2>История скидок</h2>{used.map(r=><div className="discount-history-row" key={r.id}><span>{prettyDate(r.approvedAt||r.createdAt)}</span><b className="discount-tag discount-tag--used">Использовано · {r.percent}%</b></div>)}</section>;
}
