import {useEffect,useState} from 'react';
import type {ReactNode} from 'react';
import {ChevronLeft, ChevronRight} from 'lucide-react';

export type PhotoCarouselProps = {
 photos?:string[];
 fallback:ReactNode;
 label:string;
 onOpen?:()=>void;
 compact?:boolean;
};
export default function PhotoCarousel({photos=[],fallback,label,onOpen,compact=false}:PhotoCarouselProps){
 const images=photos.filter((photo):photo is string=>typeof photo==='string'&&photo.startsWith('/'));
 const [current,setCurrent]=useState(0);
 const [broken,setBroken]=useState<string[]>([]);
 const [touch,setTouch]=useState<number|null>(null);
 useEffect(()=>{setCurrent(0);setBroken([])},[images.join('|')]);
 const available=images.filter(image=>!broken.includes(image)).slice(0,3);
 const active=Math.min(current,Math.max(0,available.length-1));
 const go=(offset:number)=>setCurrent(index=>(index+offset+available.length)%available.length);
 const open=()=>onOpen?.();
 return <div className={`location-gallery ${compact?'location-gallery--compact':''}`}>
  {available.length>0
   ?<button type="button" className="location-gallery__image-button" onClick={open} aria-label={`Подробнее: ${label}`}
     onTouchStart={e=>setTouch(e.touches[0]?.clientX??null)}
     onTouchEnd={e=>{if(touch===null)return;const delta=(e.changedTouches[0]?.clientX??touch)-touch;setTouch(null);if(Math.abs(delta)>45){e.preventDefault();e.stopPropagation();go(delta>0?-1:1)}}}>
     <img className="location-gallery__image" src={available[active]} alt={`Фото ${active+1} из ${available.length}: ${label}`} loading="lazy"
      onError={()=>setBroken(list=>[...list,available[active]])}/>
    </button>
   : <button type="button" className="location-gallery__fallback" onClick={open} aria-label={`Подробнее: ${label}`}>{fallback}</button>}
  {available.length>1&&<>
   <button type="button" className="location-gallery__arrow location-gallery__arrow--prev" aria-label="Предыдущее фото" onClick={e=>{e.stopPropagation();go(-1)}}><ChevronLeft size={17}/></button>
   <button type="button" className="location-gallery__arrow location-gallery__arrow--next" aria-label="Следующее фото" onClick={e=>{e.stopPropagation();go(1)}}><ChevronRight size={17}/></button>
   <div className="location-gallery__pagination" aria-label={`Фото ${active+1} из ${available.length}`}>
    {available.map((_,i)=><button type="button" key={i} aria-label={`Фото ${i+1}`} aria-current={active===i?'true':undefined} onClick={()=>setCurrent(i)} className={active===i?'active':''}/>)}
   </div>
   <span className="location-gallery__count">{active+1}/{available.length}</span>
  </>}
 </div>;
}
