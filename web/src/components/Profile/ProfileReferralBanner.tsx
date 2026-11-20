import {designAssets} from '../../design/assets';
import ProfileGlyph from './ProfileGlyph';

type Props={percent:number;onClick:()=>void};
export default function ProfileReferralBanner({percent,onClick}:Props){
 return <button type="button" className="profile-referral profile-referral--invitation" onClick={onClick}
  aria-label={percent>0?`Ваша скидка ${percent}%. Открыть раздел бонусов`:'Приглашайте новых клиентов. Открыть раздел бонусов'}>
  <span className="profile-referral__copy">
   <strong>Приглашайте<br/>новых клиентов</strong>
   <span>Получайте скидку на<br/>следующую оплату</span>
   <span className="profile-referral__cta" aria-hidden="true">Пригласить <ProfileGlyph name="chevron" size={19}/></span>
  </span>
  <img className="profile-referral__art" src={designAssets.profileReferralMegaphone}
   alt="" width="196" height="165" decoding="async" loading="eager"/>
  <span className="profile-referral__arrow" aria-hidden="true"><ProfileGlyph name="chevron" size={23}/></span>
  {percent>0&&<span className="profile-referral__balance">Скидка {percent}%</span>}
 </button>;
}
