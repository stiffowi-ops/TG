import {useState} from 'react';
import type {DiscountUse} from '../../types';
import ProfileGlyph from '../Profile/ProfileGlyph';
import {prettyDate} from '../../demo';

export default function DiscountInbox({rows,approve}:{rows:DiscountUse[]|null;approve:(id:number)=>Promise<void>}){
 const [busy,setBusy]=useState<number|null>(null);
 const [error,setError]=useState('');
 async function confirm(id:number){
  setBusy(id);setError('');
  try{await approve(id)}catch(e){setError((e as Error).message)}finally{setBusy(null)}
 }
 if(rows===null)return <p className="profile-muted">Загружаем запросы…</p>;
 if(!rows.length)return <p className="profile-empty">Запросов на использование скидки пока нет.</p>;
 const sorted=[...rows].sort((a,b)=>Number(b.status==='pending')-Number(a.status==='pending')||b.id-a.id);
 return <div className="discount-inbox">{error&&<p role="alert">{error}</p>}
  {sorted.map(r=><article key={r.id} className="discount-panel">
   <div className="admin-client-head"><span className="profile-symbol"><ProfileGlyph name="user" size={21}/></span><span className="profile-current__copy"><strong>{r.name}</strong><small>Telegram ID {r.userId}</small></span></div>
   <div className="discount-history-row"><strong>Скидка {r.percent}%</strong><span className={'discount-tag'+(r.status==='used'?' discount-tag--used':'')}>{r.status==='used'?'Согласовано':'На согласовании'}</span></div>
   <small>{prettyDate(r.approvedAt||r.createdAt)}</small>
   {r.status==='pending'&&<button className="profile-cta" disabled={busy!==null} onClick={()=>confirm(r.id)}><ProfileGlyph name="done" size={18}/>{busy===r.id?'Согласовываем…':'Согласовать'}</button>}
  </article>)}
 </div>;
}
