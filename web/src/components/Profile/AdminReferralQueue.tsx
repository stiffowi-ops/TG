import {useEffect,useState} from 'react';
import {Gift,RefreshCcw} from 'lucide-react';
import './admin-referrals.css';

export type ReferralRecord={
 inviteeId:number;inviterId:number;caseNumber:number;inviterName:string;inviteeName:string;
 status:'pending'|'deferred'|'awarded';createdAt:string;ready:boolean;
};
type Props={caseTitle:(number:number)=>string|undefined;load:()=>Promise<ReferralRecord[]>;decide:(number:number,action:'award'|'defer')=>Promise<void>};
export default function AdminReferralQueue({load,decide,caseTitle}:Props){
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
  <div className="admin-referrals__head"><p>Приглашения, по которым ожидается решение</p><button type="button" onClick={()=>void refresh()} aria-label="Обновить реферальные заявки"><RefreshCcw size={19}/></button></div>
  {error&&<p role="alert" className="admin-referrals__error">{error}</p>}
  {loading?<p className="admin-referrals__empty">Проверяем приглашения…</p>:rows.length===0?<p className="admin-referrals__empty">Нет ожидающих реферальных начислений.</p>:
  <div className="admin-referrals__list">{rows.map(rec=><article key={rec.caseNumber} className="discount-panel">
   <div className="admin-client-head"><span className="profile-symbol"><Gift size={21}/></span><span className="profile-current__copy"><strong>Приглашение клиента</strong><small>Заявка {caseTitle(rec.caseNumber)||'№'+rec.caseNumber}</small></span></div>
   <span className="discount-tag">{rec.status==='deferred'?'Отложено':rec.ready?'Ожидает решения':'Ожидает принятия заявки'}</span>
   <div className="discount-history-row"><span>Пригласил</span><b>{rec.inviterName}</b></div>
   <div className="discount-history-row"><span>Приглашён</span><b>{rec.inviteeName}</b></div>
   <div className="discount-history-row"><span>Вознаграждение</span><b>10%</b></div>
   <div className="admin-referrals__actions">
    <button disabled={busy!==null||!rec.ready} onClick={()=>void action(rec.caseNumber,'award')}>Начислить 10%</button>
    <button disabled={busy!==null||!rec.ready||rec.status==='deferred'} onClick={()=>void action(rec.caseNumber,'defer')}>Назначу позже</button>
   </div>
   <small>Начисление доступно после принятия заявки в работу, в том числе после её завершения.</small>
  </article>)}</div>}
 </section>;
}
