import type {DemoCase} from '../../types';
import ProfileGlyph from './ProfileGlyph';

const labels:Record<DemoCase['status'],string>={received:'Получено',working:'В работе',resolved:'Решено'};
export default function ActiveRequestPreview({record,title,onClick}:{record:DemoCase;title:string;onClick:()=>void}){
 return <button type="button" className="profile-current" onClick={onClick}>
   <span className="profile-symbol profile-symbol--application"><ProfileGlyph name="application" size={23}/></span>
   <span className="profile-current__copy"><small>Активная заявка</small><strong>{title}</strong></span>
   <span className={'profile-status profile-status--'+record.status}>{labels[record.status]}</span>
   <ProfileGlyph name="chevron" size={18} className="profile-chevron"/>
 </button>;
}
