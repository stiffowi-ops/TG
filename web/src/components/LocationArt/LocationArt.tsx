import {useEffect,useState} from 'react';
import {MapPin} from 'lucide-react';
import type {Location} from '../../types';

// Independent fallback artwork. Replace it without touching the catalog or gallery.
export default function LocationArt({location,compact=false}:{location:Location;compact?:boolean}){
 const tone=(parseInt(location.id,10)%5)+1;
 const [broken,setBroken]=useState(false);
 useEffect(()=>setBroken(false),[location.imageUrl]);
 const hasPhoto=!!location.imageUrl&&!broken;
 return <div className={`area-art tone-${tone} ${compact?'compact':''}`} aria-label={hasPhoto?`Фото: ${location.title}`:'Демонстрационная иллюстрация локации'}>
  {hasPhoto&&<img className="art-photo" src={location.imageUrl} alt={`Фотография локации ${location.title}`} loading="lazy" onError={()=>setBroken(true)}/> }
  <div className="art-glow"/><div className="art-sun"/>
  <div className="art-buildings"><i/><i/><i/><i/><i/><i/></div>
  <div className="art-line"/><div className="art-road"/>
  <span className="art-mark"><MapPin size={16} strokeWidth={2.6}/></span>
 </div>;
}
