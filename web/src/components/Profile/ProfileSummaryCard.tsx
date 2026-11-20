import type {ClientProfile} from '../../types';
import ProfileGlyph from './ProfileGlyph';

type Props = {profile:ClientProfile|null;fallbackName?:string;onClick:()=>void};
export default function ProfileSummaryCard({profile,fallbackName,onClick}:Props) {
  const hasName=Boolean(profile?.name?.trim());
  return <button type="button" className="profile-summary" onClick={onClick} aria-label="Мои данные. Открыть личную информацию">
    <span className="profile-symbol profile-symbol--user"><ProfileGlyph name="user" size={27}/></span>
    <span className="profile-summary__copy">
      <strong>{hasName?profile!.name:(fallbackName||'Заполните профиль')}</strong>
      <small>{profile?.phone||'Добавьте контактные данные'}</small>
    </span>
    <ProfileGlyph name="chevron" size={20} className="profile-chevron"/>
  </button>;
}
