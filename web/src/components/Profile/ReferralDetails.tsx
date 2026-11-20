import {useState} from 'react';
import {Copy,Share2,Check,Link2,Users,Gift} from 'lucide-react';
import type {MiniReferralInfo} from '../../types';

export default function ReferralDetails({percent,info,loading,error}:{percent:number;info:MiniReferralInfo|null;loading:boolean;error:string}){
 const [copied,setCopied]=useState(false);
 const link=info?.link||'';
 const copy=async()=>{
  if(!link)return;
  try{await navigator.clipboard.writeText(link);setCopied(true)}
  catch{setCopied(false)}
 };
 const share=()=>{
  if(!link)return;
  const target='https://t.me/share/url?url='+encodeURIComponent(link)+'&text='+encodeURIComponent('Приглашаю посмотреть локации в приложении «Локации»');
  window.Telegram?.WebApp?.openTelegramLink?.(target) || window.open(target,'_blank','noopener,noreferrer');
 };
 return <div className="profile-discount-screen">
  <div className="profile-discount-screen__balance"><Gift size={28}/><span>Скидка на следующую оплату</span><strong>{percent>0?percent+'%':'У вас нет скидок'}</strong></div>
  <p>Приглашайте новых клиентов и получайте скидку на следующую оплату. Вознаграждение подтверждает администратор после принятия заявки в работу.</p>
  <div className="profile-referral-share">
   <div className="profile-referral-share__heading"><Users size={22}/><strong>Ваша реферальная ссылка</strong></div>
   {loading?<p>Получаем персональную ссылку…</p>:error?<p role="alert">{error}</p>:link?<>
     <div className="profile-referral-share__link" title={link}><Link2 size={18}/><span>{link}</span></div>
     <div className="profile-referral-share__actions">
       <button type="button" onClick={copy}><>{copied?<Check size={18}/>:<Copy size={18}/>}</>{copied?'Скопировано':'Скопировать'}</button>
       <button type="button" onClick={share}><Share2 size={18}/> Поделиться</button>
     </div>
     <div className="profile-referral-share__counts"><span>Приглашений на проверке: <b>{info?.pending||0}</b></span><span>Начислений: <b>{info?.awarded||0}</b></span></div>
   </>:<p>Ссылка пока недоступна. Попробуйте открыть раздел снова.</p>}
  </div>
 </div>;
}
