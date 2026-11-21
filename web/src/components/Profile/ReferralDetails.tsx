import {useRef,useState} from 'react';
import {Copy,Share2,Check,Link2,Users} from 'lucide-react';
import type {MiniReferralInfo,DiscountWallet} from '../../types';

import DiscountUsage,{DiscountHistory} from './DiscountUsage';

export default function ReferralDetails({wallet,onRequest,info,loading,error}:{wallet:DiscountWallet|null;onRequest:(offer:string)=>Promise<void>;info:MiniReferralInfo|null;loading:boolean;error:string}){
 const [copied,setCopied]=useState(false);
 const [manualCopy,setManualCopy]=useState(false);
 const linkField=useRef<HTMLInputElement>(null);
 const link=info?.link||'';
 const copy=async()=>{
  if(!link)return;
  try{await navigator.clipboard.writeText(link);setCopied(true);setManualCopy(false)}
  catch{setCopied(false);setManualCopy(true);linkField.current?.focus();linkField.current?.select()}
 };
 const share=()=>{
  if(!link)return;
  const target='https://t.me/share/url?url='+encodeURIComponent(link)+'&text='+encodeURIComponent('Приглашаю посмотреть локации в приложении «Локации»');
  if(window.Telegram?.WebApp?.openTelegramLink){window.Telegram.WebApp.openTelegramLink(target)}
  else{window.open(target,'_blank','noopener,noreferrer')}
 };
 return <div className="profile-discount-screen">
  <DiscountUsage wallet={wallet} onRequest={onRequest}/>
  <div className="profile-referral-share">
   <div className="profile-referral-share__heading"><Users size={22}/><strong>Ваша реферальная ссылка</strong></div>
   {loading?<p>Получаем персональную ссылку…</p>:error?<p role="alert">{error}</p>:link?<>
     <div className="profile-referral-share__link" title={link}><Link2 size={18}/><input ref={linkField} readOnly value={link} aria-label="Персональная реферальная ссылка" onFocus={event=>event.target.select()}/></div>
     <div className="profile-referral-share__actions">
       <button type="button" onClick={copy}><>{copied?<Check size={18}/>:<Copy size={18}/>}</>{copied?'Скопировано':'Скопировать'}</button>
       <button type="button" onClick={share}><Share2 size={18}/> Поделиться</button>
     </div>
     {manualCopy&&<small role="status">Ссылка выделена. Выберите «Копировать» в меню устройства.</small>}
     <div className="profile-referral-share__counts"><span>Приглашений на проверке: <b>{info?.pending||0}</b></span><span>Начислений: <b>{info?.awarded||0}</b></span></div>
   </>:<p>Ссылка пока недоступна. Попробуйте открыть раздел снова.</p>}
  </div>
  <DiscountHistory wallet={wallet}/>
 </div>;
}
