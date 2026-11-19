import {useEffect,useMemo,useRef,useState} from 'react';
import {ArrowRight,Heart,LocateFixed,MapPin,Minus,Plus,X} from 'lucide-react';
import {LngLatBounds,Map as MapLibreMap,Marker as MapLibreMarker} from 'maplibre-gl';
import type {StyleSpecification} from 'maplibre-gl';
import type {Location} from '../../types';
import PhotoCarousel from '../PhotoCarousel/PhotoCarousel';
import LocationArt from '../LocationArt/LocationArt';

type Props={
 locations:Location[];selectedId:string|null;onSelect:(id:string|null)=>void;
 favorites:Set<string>;onFavorite:(id:string)=>void;onDetails:(location:Location)=>void;onOpenYandex:(location:Location)=>void;
};
type Pin={marker:MapLibreMarker;element:HTMLButtonElement};
const validPoint=(l:Location)=>l.latitude!==undefined&&l.longitude!==undefined&&Number.isFinite(l.latitude)&&Number.isFinite(l.longitude)&&Math.abs(l.latitude)<=90&&Math.abs(l.longitude)<=180;
const tileURL=import.meta.env.VITE_MAP_TILE_URL?.trim()||'https://tile.openstreetmap.org/{z}/{x}/{y}.png';
const mapStyle:StyleSpecification={version:8,sources:{osm:{type:'raster',tiles:[tileURL],tileSize:256,minzoom:0,maxzoom:19,attribution:'© OpenStreetMap contributors'}},layers:[{id:'osm-base',type:'raster',source:'osm'}]};
function MapPhoto({location,onOpen}:{location:Location;onOpen:()=>void}){
 const photos=location.imageUrls?.length?location.imageUrls:location.imageUrl?[location.imageUrl]:[];
 return <PhotoCarousel photos={photos} fallback={<LocationArt location={location} compact/>} label={location.title} onOpen={onOpen} compact/>;
}
export default function LocationMap({locations,selectedId,onSelect,favorites,onFavorite,onDetails,onOpenYandex}:Props){
 const points=useMemo(()=>locations.filter(validPoint),[locations]);
 const hasPoints=points.length>0;
 const chosen=points.find(l=>l.id===selectedId)??null;
 const containerRef=useRef<HTMLDivElement|null>(null);
 const mapRef=useRef<MapLibreMap|null>(null);
 const markersRef=useRef<Map<string,Pin>>(new Map());
 const onSelectRef=useRef(onSelect);
 onSelectRef.current=onSelect;
 const [tileFailures,setTileFailures]=useState(0);
 const [mapUnavailable,setMapUnavailable]=useState(false);
 const [geoError,setGeoError]=useState('');

 useEffect(()=>{
  if(!hasPoints||!containerRef.current)return;
  let map:MapLibreMap|null=null;
  let active=true;
  setMapUnavailable(false);
  setTileFailures(0);
  try{
   map=new MapLibreMap({container:containerRef.current,style:mapStyle,center:[37.6184,55.7512],
    zoom:11,minZoom:3,maxZoom:18,attributionControl:false,scrollZoom:false,
    dragRotate:false,touchPitch:false,renderWorldCopies:false});
   mapRef.current=map;
   map.on('load',()=>{if(active)map?.resize()});
   map.on('error',event=>{
    if(!active)return;
    const detail=String(event.error?.message||'');
    if(/tile|source|request|fetch|network|403|404/i.test(detail))setTileFailures(n=>Math.min(5,n+1));
   });
   map.on('sourcedata',event=>{
    if(active&&event.sourceId==='osm'&&event.isSourceLoaded)setTileFailures(0);
   });
  }catch(error){
   console.error('MapLibre initialization failed',error);
   setMapUnavailable(true);
   if(map){try{map.remove()}catch{/* WebGL context may be unavailable */}}
   mapRef.current=null;
  }
  return()=>{
   active=false;
   markersRef.current.forEach(({marker})=>marker.remove());
   markersRef.current.clear();
   if(mapRef.current===map)mapRef.current=null;
   if(map){try{map.remove()}catch{/* Browser may have lost WebGL context */}}
  };
 },[hasPoints]);

 useEffect(()=>{
  const map=mapRef.current;
  if(!map||!points.length)return;
  // Gallery refreshes must keep the viewport and the marker under the user's finger.
  const pins=markersRef.current;
  if(pins.size===points.length&&points.every(location=>{
   const pin=pins.get(location.id);
   const coords=pin?.marker.getLngLat();
   return pin?.element.title===location.title&&coords?.lat===location.latitude&&coords?.lng===location.longitude;
  }))return;
  markersRef.current.forEach(({marker})=>marker.remove());
  markersRef.current.clear();
  for(const location of points){
   const element=document.createElement('button');
   element.type='button';
   element.className='geo-pin-container';
   element.setAttribute('aria-label','Выбрать локацию '+location.title);
   element.title=location.title;
   element.innerHTML='<span class="geo-pin"><span></span></span>';
   element.addEventListener('click',event=>{event.stopPropagation();onSelectRef.current(location.id)});
   const marker=new MapLibreMarker({element,anchor:'bottom'})
    .setLngLat([location.longitude!,location.latitude!]).addTo(map);
   markersRef.current.set(location.id,{marker,element});
  }
  if(points.length===1)map.jumpTo({center:[points[0].longitude!,points[0].latitude!],zoom:14});
  else{
   const bounds=new LngLatBounds();
   points.forEach(location=>bounds.extend([location.longitude!,location.latitude!]));
   map.fitBounds(bounds,{padding:45,maxZoom:12,duration:0});
  }
 },[points,hasPoints]);

 useEffect(()=>{
  markersRef.current.forEach(({element},id)=>{
   const active=id===selectedId;
   element.classList.toggle('geo-pin-container--active',active);
   element.querySelector('.geo-pin')?.classList.toggle('geo-pin--active',active);
   element.setAttribute('aria-pressed',String(active));
  });
 },[selectedId,points]);

 const latitude=chosen?.latitude;
 const longitude=chosen?.longitude;
 useEffect(()=>{
  const map=mapRef.current;
  if(latitude!==undefined&&longitude!==undefined&&map)map.flyTo({center:[longitude,latitude],
   zoom:Math.max(13,map.getZoom()),duration:450,essential:true});
 },[selectedId,latitude,longitude]);

 const locate=()=>{
  const map=mapRef.current;
  if(!map){setGeoError('Карта недоступна на устройстве');return}
  if(!navigator.geolocation){setGeoError('Геолокация недоступна');return}
  navigator.geolocation.getCurrentPosition(position=>{
   setGeoError('');
   map.flyTo({center:[position.coords.longitude,position.coords.latitude],zoom:13,duration:500,essential:true});
  },()=>setGeoError('Проверьте разрешение на геолокацию'),{enableHighAccuracy:false,timeout:8000});
 };
 return <div className="geo-map-layout">
  {hasPoints?<div className={`geo-map${chosen?' geo-map--preview-open':''}`} role="region" aria-label="Интерактивная карта локаций">
   <div ref={containerRef} className="geo-map__tiles"/>
   {mapUnavailable?<div className="geo-map__fallback" role="status">
    <MapPin size={27}/><strong>Карта недоступна на этом устройстве</strong>
    <p>Локации остаются доступны для выбора и просмотра в Яндекс Картах.</p>
    <div className="geo-map__fallback-list">
     {points.map(l=><button type="button" key={l.id} onClick={()=>onSelect(l.id)}>{l.title}</button>)}
    </div>
   </div>:<div className="geo-map-actions">
    <button type="button" aria-label="Приблизить карту" onClick={()=>mapRef.current?.zoomIn()}><Plus size={22}/></button>
    <button type="button" aria-label="Отдалить карту" onClick={()=>mapRef.current?.zoomOut()}><Minus size={22}/></button>
    <button type="button" aria-label="Моё местоположение" onClick={locate}><LocateFixed size={21}/></button>
    {geoError&&<span className="geo-map-actions__error" role="status">{geoError}</span>}
   </div>}
   {!mapUnavailable&&tileFailures>=4&&<div className="geo-map__tile-notice" role="status">Подложка карты временно недоступна. Метки сохраняют свои координаты.</div>}
   {chosen&&<div className="geo-map-preview" role="group" aria-label={'Выбрана локация '+chosen.title}>
    <button type="button" className="geo-map-preview__close" aria-label="Закрыть карточку" onClick={()=>onSelect(null)}><X size={17}/></button>
    <div className="geo-map-preview__photo"><MapPhoto location={chosen} onOpen={()=>onDetails(chosen)}/></div>
    <div className="geo-map-preview__body">
     <button type="button" className="geo-map-preview__title geo-map-preview__title-button" onClick={()=>onDetails(chosen)}>{chosen.title}</button>
     {chosen.area&&<div className="geo-map-preview__area"><MapPin size={14}/>{chosen.area}</div>}
     <div className="geo-map-preview__actions">
      <button type="button" className="geo-map-preview__favorite" onClick={()=>onFavorite(chosen.id)} aria-label={favorites.has(chosen.id)?'Удалить из избранного':'В избранное'}>
       <Heart size={18} fill={favorites.has(chosen.id)?'currentColor':'none'}/>
      </button>
      <button type="button" className="geo-map-preview__details" onClick={()=>onOpenYandex(chosen)}>Открыть на карте <ArrowRight size={17}/></button>
     </div>
    </div>
   </div>}
   <div className="geo-map__attribution"><a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">© OpenStreetMap contributors</a></div>
  </div>:<div className="geo-map-empty"><MapPin size={28}/><h3>Нет точек с координатами</h3><p>Попробуйте изменить фильтры или выбрать другую локацию.</p></div>}
 </div>;
}