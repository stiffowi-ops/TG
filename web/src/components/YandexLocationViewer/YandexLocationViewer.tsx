import {useEffect,useMemo,useState} from 'react';
import {ArrowLeft, ExternalLink, MapPin, X} from 'lucide-react';
import type {Location} from '../../types';
import {mapLink} from '../../demo';

/** Standalone, keyless Yandex Maps iframe for a single catalog coordinate.
 * Not a JavaScript Maps API integration; catalog filters are independent.
 */
export function yandexPointWidgetURL(location:Location):string|null{
 const {latitude,longitude}=location;
 if(latitude===undefined||longitude===undefined||!Number.isFinite(latitude)||!Number.isFinite(longitude))return null;
 if(Math.abs(latitude)>90||Math.abs(longitude)>180)return null;
 const coordinate=longitude.toFixed(8)+','+latitude.toFixed(8);
 const params=new URLSearchParams({ll:coordinate,z:'16',pt:coordinate+',pm2rdm'});
 return 'https://yandex.ru/map-widget/v1/?'+params.toString();
}

export default function YandexLocationViewer({location,onClose}:{
 location:Location;onClose:()=>void;
}){
 const src=useMemo(()=>yandexPointWidgetURL(location),[location.id,location.latitude,location.longitude]);
 const [loaded,setLoaded]=useState(false);
 useEffect(()=>{
  setLoaded(false);
  const previous=document.body.style.overflow;
  document.body.style.overflow='hidden';
  const app=window.Telegram?.WebApp;
  const backButton=app?.initData && app.isVersionAtLeast?.('6.1')!==false ? app.BackButton : undefined;
  const wasVisible=backButton?.isVisible;
  backButton?.onClick(onClose);
  backButton?.show();
  const onEscape=(event:KeyboardEvent)=>{if(event.key==='Escape')onClose()};
  window.addEventListener('keydown',onEscape);
  return()=>{
   document.body.style.overflow=previous;
   window.removeEventListener('keydown',onEscape);
   backButton?.offClick(onClose);
   if(!wasVisible)backButton?.hide();
  };
 },[src,onClose]);
 return <div className="yandex-location-overlay" role="presentation">
  <section className="yandex-location-viewer" role="dialog" aria-modal="true" aria-label={`Яндекс Карты: ${location.title}`}>
   <header className="yandex-location-viewer__header">
    <button type="button" className="yandex-location-viewer__back" onClick={onClose} aria-label="Вернуться в каталог"><ArrowLeft size={22}/></button>
    <div className="yandex-location-viewer__heading"><small>ЯНДЕКС КАРТЫ</small><strong>{location.title}</strong></div>
    <button type="button" className="yandex-location-viewer__close" onClick={onClose} aria-label="Закрыть карту"><X size={22}/></button>
   </header>
   {src?<div className="yandex-location-viewer__frame">
    {!loaded&&<div className="yandex-location-viewer__loading" role="status"><MapPin size={23}/>Открываем выбранную точку…</div>}
    <iframe key={src} title={`Яндекс Карта: ${location.title}`} src={src}
     referrerPolicy="strict-origin-when-cross-origin"
     allow="geolocation" loading="eager" onLoad={()=>setLoaded(true)}/>
   </div>:<div className="yandex-location-viewer__empty"><MapPin size={32}/><h3>Нет координат локации</h3><p>Для этой точки пока не указано точное местоположение.</p></div>}
   <footer className="yandex-location-viewer__footer">
    <span><MapPin size={15}/>{location.area}</span>
    <a href={mapLink(location)} target="_blank" rel="noreferrer noopener">Открыть в Яндексе <ExternalLink size={15}/></a>
   </footer>
  </section>
 </div>;
}
