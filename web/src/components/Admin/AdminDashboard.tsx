import ProfileGlyph from '../Profile/ProfileGlyph';
import ProfileShortcutTile from '../Profile/ProfileShortcutTile';
import './admin-dashboard.css';

export const adminTitles = {home:'Управление',clients:'Клиенты',cases:'Заявки',awards:'Начисления',analytics:'Аналитика',approvals:'Запросы на скидку',services:'Сервисы'};
export type AdminView=keyof typeof adminTitles;
export default function AdminDashboard({open,count,error}:{open:(view:AdminView)=>void;count:number|null;error:boolean}){
 const caption=error?'Не удалось обновить запросы':count===null?'Загружаем запросы…':count?'На согласовании · '+count:'Нет запросов на согласовании';
 return <>
  <button type="button" className="profile-current admin-inbox-link" onClick={()=>open('approvals')}>
   <span className="profile-symbol"><ProfileGlyph name="discount"/></span><span className="profile-current__copy"><strong>Запросы на скидку</strong><small aria-live="polite">{caption}</small></span><ProfileGlyph name="chevron" size={18}/>
  </button>
  <div className="profile-shortcuts">
   <ProfileShortcutTile title="Клиенты" subtitle="Профили и скидки" icon="clients" theme="warm" onClick={()=>open('clients')}/>
   <ProfileShortcutTile title="Заявки" subtitle="Обращения и статусы" icon="requests" theme="blue" onClick={()=>open('cases')}/>
   <ProfileShortcutTile title="Начисления" subtitle="Реферальные скидки" icon="gift" theme="warm" onClick={()=>open('awards')}/>
   <ProfileShortcutTile title="Аналитика" subtitle="Показатели локаций" icon="analytics" theme="blue" onClick={()=>open('analytics')}/>
  </div>
  <button type="button" className="profile-current" onClick={()=>open('services')}>
   <span className="profile-symbol"><ProfileGlyph name="services" size={22}/></span><span className="profile-current__copy"><strong>Сервисы</strong><small>Посетители, локации, фото и бэкапы</small></span><ProfileGlyph name="chevron" size={18}/>
  </button>
 </>;
}
