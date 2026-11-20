import {useEffect,useState} from 'react';
import {Clock3,Gift,RefreshCcw} from 'lucide-react';
import './admin-referrals.css';

export type ReferralRecord={
 inviteeId:number;inviterId:number;caseNumber:number;inviterName:string;inviteeName:string;
 status:'pending'|'deferred'|'awarded';createdAt:string;
};
type Props={load:()=>Promise<ReferralRecord[]>;decide:(number:number,action:'award'|'defer')=>Promise<void>};
export default function AdminReferralQueue({load,decide}:Props){
 const [rows,setRows]=useState<ReferralRecord[]>([]);
 const [loading,setLoading]=useState(true);
 const [error,setError]=useState('');
 const [busy,setBusy]=useState<number|null>(null);
 const refresh=async()=>{
  setError('');setLoading(true);
  try{setRows(await load())}catch(e){setError((e as Error).message)}
  finally{setLoading(false)}
 };
 useEffect(()=>{void refresh()},[load]);
 const action=async(number:number,kind:'award'|'defer')=>{
  setBusy(number);setError('');
  try{await decide(number,kind);await refresh()}
  catch(e){setError((e as Error).message)}
  finally{setBusy(null)}
 };
 return <section className="admin-referrals" aria-label="Реферальные начисления">
  <div className="admin-referrals__head"><div><h2><Gift size={23}/> Реферальные начисления</h2><p>Заявки, по которым решение ещё не принято</p></div><button type="button" onClick={()=>void refresh()} aria-label="Обновить реферальные заявки"><RefreshCcw size={19}/></button></div>
  {error&&<p role="alert" className="admin-referrals__error">{error}</p>}
  {loading?<p className="admin-referrals__empty">Проверяем приглашения…</p>:rows.length===0?<p className="admin-referrals__empty">Нет ожидающих реферальных начислений.</p>:
  <div className="admin-referrals__list">{rows.map(rec=><article key={rec.caseNumber} className="admin-referrals__item">
   <div className="admin-referrals__status"><Clock3 size={16}/>{rec.status==='deferred'?'Назначу позже':'Ожидает решения'}</div>
   <strong>Заявка №{rec.caseNumber}</strong>
   <small>Клиент: {rec.inviteeName}</small><small>Пригласил: {rec.inviterName} · ID {rec.inviterId}</small>
   <div className="admin-referrals__actions">
    <button disabled={busy!==null} onClick={()=>void action(rec.caseNumber,'award')}>Начислить 10%</button>
    <button disabled={busy!==null||rec.status==='deferred'} onClick={()=>void action(rec.caseNumber,'defer')}>Назначу позже</button>
   </div>
   <small>Начисление доступно после принятия заявки в работу, в том числе после её завершения.</small>
  </article>)}</div>}
 </section>;
}
