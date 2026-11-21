import type {DemoCase} from '../../types';
import {prettyDate} from '../../demo';
import ProfileGlyph from './ProfileGlyph';

const labels:Record<DemoCase['status'],string>={received:'Получено',working:'В работе',resolved:'Решено'};
export default function ActiveRequestPreview({record,title,onClick}:{record:DemoCase;title:string;onClick:()=>void}){
 const events=[
  {date:record.createdAt,label:'Заявка получена'},
  ...(record.takenAt?[{date:record.takenAt,label:'Специалист взял в работу'}]:[]),
  ...record.updates.map(update=>({date:update.createdAt,label:'Заявка дополнена'}))
 ];
 const latest=events.reduce((current,next)=>new Date(next.date).getTime()>new Date(current.date).getTime()?next:current,events[0]);
 const progress=record.status==='received'?1:record.status==='working'?2:3;
 return <button type="button" className="profile-active-card" onClick={onClick} aria-label={'Открыть активную заявку '+title}>
  <span className="profile-active-card__heading"><small>Активная заявка</small><span className={'profile-status profile-status--'+record.status}>{labels[record.status]}</span></span>
  <span className="profile-active-card__title">Заявка {title}<ProfileGlyph name="chevron" size={18} className="profile-chevron"/></span>
  <span className="profile-active-card__progress" aria-hidden="true">{[1,2,3].map(step=><span key={step} className={step<=progress?'is-done':''}/>)}</span>
  <span className="profile-active-card__updated">{latest.label} · {prettyDate(latest.date)}</span>
 </button>;
}
