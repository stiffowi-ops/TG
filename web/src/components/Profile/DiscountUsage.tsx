import {useState} from 'react';
import {BadgePercent, CheckCircle2, Gift} from 'lucide-react';
import type {DiscountWallet} from '../../types';
import ProgressTracker from './ProgressTracker';
import {prettyDate} from '../../demo';
import './discount-usage.css';

export default function DiscountUsage({wallet,onRequest}:{wallet:DiscountWallet|null;onRequest:(offer:string)=>Promise<void>}) {
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState('');
 const latest=wallet?.history?.[0];
 const pending=latest?.status==='pending';
 const used=latest?.status==='used'&&!wallet?.percent;
 const percent=pending||used?latest.percent:wallet?.percent??0;
 async function request(){
  if(!wallet?.offer||busy)return;
  setBusy(true);setError('');
  try{await onRequest(wallet.offer)}catch(e){setError((e as Error).message)}finally{setBusy(false)}
 }
 return <>
  <section className={'discount-balance'+(used?' discount-balance--used':'')} aria-live="polite">
   <div>{used?<CheckCircle2 size={24}/>:<Gift size={24}/>}<span>{used?'Скидка использована':'Скидка на следующую оплату'}</span></div>
   <strong className={percent?'':'discount-balance__empty'}>{percent?percent+'%':'У вас нет скидок'}</strong>
   {pending?<span className="discount-tag">На согласовании</span>:wallet?.offer?
    <button className="profile-cta" type="button" disabled={busy} onClick={request}><BadgePercent size={18}/>{busy?'Отправляем…':'Воспользоваться скидкой'}</button>:
    <small>{used?'Администратор подтвердил использование скидки.':'Приглашайте клиентов по своей ссылке.'}</small>}
   {error&&<small role="alert">{error}</small>}
  </section>
  {latest&&<div className="profile-detail-tracker"><ProgressTracker status={pending?'working':'resolved'} labels={['Отправлено','Согласование','Использовано']} subject="скидки"/></div>}
 </>;
}
export function DiscountHistory({wallet}:{wallet:DiscountWallet|null}){
 const used=wallet?.history?.filter(r=>r.status==='used')||[];
 if(!used.length)return null;
 return <section className="discount-panel"><h2>История скидок</h2>{used.map(r=><div className="discount-history-row" key={r.id}><span>{prettyDate(r.approvedAt||r.createdAt)}</span><b className="discount-tag discount-tag--used">Использовано · {r.percent}%</b></div>)}</section>;
}
