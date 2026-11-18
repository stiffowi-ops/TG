import {ArrowUpRight, Check, ChevronRight, Heart, MapPin, Plus} from 'lucide-react';
import type {Location} from '../../types';
import PhotoCarousel from '../PhotoCarousel/PhotoCarousel';
import LocationArt from '../LocationArt/LocationArt';

export type LocationCardProps={
 location:Location;
 saved:boolean;
 onDetails:(location:Location)=>void;
 onFavorite:(id:string)=>void;
 onMap:(location:Location)=>void;
 compareMode?:boolean;
 selected?:boolean;
 onSelect?:(id:string)=>void;
};

export default function LocationCard({
 location,saved,onDetails,onFavorite,onMap,compareMode=false,selected=false,onSelect
}:LocationCardProps){
 const photos=location.imageUrls?.length?location.imageUrls:location.imageUrl?[location.imageUrl]:[];
 return <article className={`location-card location-card--editorial ${selected?'selected':''}`}>
  <div className="location-art-wrap location-card__media">
   <PhotoCarousel photos={photos} fallback={<LocationArt location={location}/>} label={location.title} onOpen={()=>onDetails(location)}/>
   <span className="location-tag location-card__tag">{location.category}</span>
   <button type="button" className={`card-heart location-card__heart ${saved?'liked':''}`}
    onClick={()=>onFavorite(location.id)} aria-label={saved?'Удалить из избранного':'Добавить в избранное'}>
    <Heart size={22} fill={saved?'currentColor':'none'}/>
   </button>
  </div>
  <div className="location-body location-card__body">
   <div className="location-card__heading">
    <button type="button" className="location-card__title" onClick={()=>onDetails(location)}>{location.title}</button>
    <button type="button" className="location-card__details" aria-label={`Подробнее: ${location.title}`} onClick={()=>onDetails(location)}><ChevronRight size={21}/></button>
   </div>
   <div className="location-card__meta">
    <span><MapPin size={15}/>{location.area}</span>
    <span className="location-card__category">{location.category}</span>
    <span className="location-card__number">№{location.id}</span>
   </div>
   {compareMode&&onSelect?
    <button type="button" className={`location-card__compare ${selected?'is-selected':''}`} onClick={()=>onSelect(location.id)}>
     {selected?<Check size={16}/>:<Plus size={16}/>}
     {selected?'Выбрано для сравнения':'Добавить к сравнению'}
    </button>:
    <button type="button" className="location-card__map-link" onClick={()=>onMap(location)}>
     На карте <ArrowUpRight size={16}/>
    </button>}
  </div>
 </article>;
}
