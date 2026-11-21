import type {ClientProfile} from '../../types';
import ProfileGlyph from './ProfileGlyph';
import {Pencil} from 'lucide-react';

function maskedPhone(phone:string):string {
 const numbers=phone.replace(/\D/g,'');
 return numbers.length>=10?'+7 ••• •••-••-'+numbers.slice(-2):'Телефон указан';
}

type Props = {profile:ClientProfile|null;fallbackName?:string;onClick:()=>void};
export default function ProfileSummaryCard({profile,fallbackName,onClick}:Props) {
  const hasName=Boolean(profile?.name?.trim());
  return <button type="button" className="profile-summary" onClick={onClick} aria-label="Данные для заявок. Изменить личные данные">
    <span className="profile-symbol profile-symbol--user"><ProfileGlyph name="user" size={27}/></span>
    <span className="profile-summary__copy">
      <strong>{hasName?profile!.name:(fallbackName||'Заполните профиль')}</strong>
      <small>{profile?.phone?maskedPhone(profile.phone):'Добавьте контактные данные'}</small>
    </span>
    <span className="profile-summary__edit"><Pencil size={15} aria-hidden="true"/> Изменить</span>
  </button>;
}
