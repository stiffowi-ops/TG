import {ArrowRight, ChevronRight} from 'lucide-react';
import {designAssets} from '../../design/assets';

export default function ProfileReferralBanner({onClick}: {onClick: () => void}) {
  return <button type="button" className="profile-referral" onClick={onClick}
    aria-label="Пригласить новых клиентов. Поделиться ссылкой в Telegram">
    <span className="profile-referral__inner">
      <img className="profile-referral__art" src={designAssets.profileReferral.megaphone} alt="" />
      <strong className="profile-referral__title">Приглашайте<br/>новых клиентов</strong>
      <span className="profile-referral__caption">Получайте скидку на следующую оплату</span>
      <span className="profile-referral__cta">Пригласить <ArrowRight/></span>
      <span className="profile-referral__arrow"><ChevronRight/></span>
    </span>
  </button>;
}
