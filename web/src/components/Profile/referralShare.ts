// Keep Telegram sharing identical from the profile banner and referral details.
export function shareReferralLink(link: string | undefined | null): boolean {
 if(!link)return false;
 const target='https://t.me/share/url?url='+encodeURIComponent(link)+'&text='+encodeURIComponent('Приглашаю посмотреть локации в приложении «Локации»');
 if(window.Telegram?.WebApp?.openTelegramLink)window.Telegram.WebApp.openTelegramLink(target);
 else window.open(target,'_blank','noopener,noreferrer');
 return true;
}
