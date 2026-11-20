import {designAssets} from '../../design/assets';
type Props={percent:number;onClick:()=>void};
export default function ProfileReferralBanner({percent,onClick}:Props){
 return <button type="button" className="profile-referral" onClick={onClick} aria-label="Бонусы. Приглашайте новых клиентов и получайте скидку на следующую оплату">
   <span className="profile-referral__copy">
     <strong>Приглашайте<br/>новых клиентов</strong>
     <span>и получайте скидку<br/>на следующую оплату</span>
     {percent>0&&<em className="profile-referral__discount">Ваша скидка {percent}%</em>}
   </span>
   <img className="profile-referral__art" src={designAssets.profileReferralGift} alt="" width="175" height="156" loading="eager" decoding="async"/>
 </button>;
}
