import {useEffect,useMemo,useState} from 'react';
import {ArrowRight, Heart, LocateFixed, MapPin, Minus, Plus, X} from 'lucide-react';
import {AttributionControl,MapContainer,Marker,TileLayer,useMap} from 'react-leaflet';
import {divIcon,latLngBounds} from 'leaflet';
import type {Location} from '../../types';
import PhotoCarousel from '../PhotoCarousel/PhotoCarousel';
import LocationArt from '../LocationArt/LocationArt';


type Props={
 locations:Location[];selectedId:string|null;onSelect:(id:string|null)=>void;
 favorites:Set<string>;onFavorite:(id:string)=>void;onDetails:(location:Location)=>void;onOpenYandex:(location:Location)=>void;
};
const point=(location:Location)=>location.latitude!==undefined&&location.longitude!==undefined&&
 Number.isFinite(location.latitude)&&Number.isFinite(location.longitude)&&
 Math.abs(location.latitude)<=90&&Math.abs(location.longitude)<=180;
const pinIcon=(selected:boolean)=>divIcon({
 className: 'geo-pin-container',
 html:'<div class="geo-pin '+(selected?'geo-pin--active':'')+'"><span></span></div>',
 iconSize:selected?[51,63]:[36,46],iconAnchor:selected?[25,57]:[18,42]
});
function MapSync({locations,selected}: {locations:Location[];selected:Location|null}){
 const map=useMap();
 const signature=locations.map(l=>l.id).join(',');
 useEffect(()=>{
  map.invalidateSize();
  const points=locations.filter(point);
  if(points.length===0)return;
  if(points.length===1){
   map.setView([points[0].latitude!,points[0].longitude!],14,{animate:false});
  }else{
   const bounds=latLngBounds(points.map(l=>[l.latitude!,l.longitude!]));
   map.fitBounds(bounds,{padding:[45,45],maxZoom:12,animate:false});
  }
 },[map,signature]);
 useEffect(()=>{
  if(selected&&point(selected))map.flyTo([selected.latitude!,selected.longitude!],Math.max(13,map.getZoom()),{duration:.45});
 },[selected?.id,map]);
 return null;
}
function MapActions(){
 const map=useMap();
 const [geoError,setGeoError]=useState('');
 const locate=()=>{
  if(!navigator.geolocation){setGeoError('Геолокация недоступна');return}
  navigator.geolocation.getCurrentPosition(position=>{
   setGeoError('');
   map.flyTo([position.coords.latitude,position.coords.longitude],13,{duration:.5});
  },()=>setGeoError('Проверьте разрешение на геолокацию'),{enableHighAccuracy:false,timeout:8000});
 };
 return <div className="geo-map-actions">
  <button type="button" aria-label="Приблизить карту" onClick={()=>map.zoomIn()}><Plus size={22}/></button>
  <button type="button" aria-label="Отдалить карту" onClick={()=>map.zoomOut()}><Minus size={22}/></button>
  <button type="button" aria-label="Моё местоположение" onClick={locate}><LocateFixed size={21}/></button>
  {geoError&&<span className="geo-map-actions__error" role="status">{geoError}</span>}
 </div>;
}
function MapPhoto({location}:{location:Location}){
 const photos=location.imageUrls?.length?location.imageUrls:location.imageUrl?[location.imageUrl]:[];
 return <PhotoCarousel photos={photos} fallback={<LocationArt location={location} compact/>} label={location.title} compact/>;
}
export default function LocationMap({locations,selectedId,onSelect,favorites,onFavorite,onDetails,onOpenYandex}:Props){
 const points=useMemo(()=>locations.filter(point),[locations]);
 const [tileFailures,setTileFailures]=useState(0);
 const tileUnavailable=tileFailures>=4;
 const chosen=points.find(l=>l.id===selectedId)??null;
 const mapCenter:[number,number]=points.length?[points[0].latitude!,points[0].longitude!]:[55.7512,37.6184];
 return <div className="geo-map-layout">
  {points.length?
   <div className="geo-map" role="region" aria-label="Интерактивная карта локаций">
    <MapContainer center={mapCenter} zoom={11} scrollWheelZoom={false} zoomControl={false} attributionControl={false}
     className="geo-map__tiles">
     <AttributionControl position="bottomright" prefix={false}/>
     <TileLayer attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
      url="https://tile.openstreetmap.org/{z}/{x}/{y}.png" maxZoom={18}
      eventHandlers={{
       tileerror:()=>setTileFailures(v=>Math.min(5,v+1)),
       tileload:()=>setTileFailures(0)
      }}/>
     <MapSync locations={points} selected={chosen}/>
     {points.map(l=><Marker key={l.id} position={[l.latitude!,l.longitude!]}
      icon={pinIcon(l.id===selectedId)} zIndexOffset={l.id===selectedId?1000:0}
      eventHandlers={{click:()=>onSelect(l.id)}}/>)}
     <MapActions/>
    </MapContainer>
    <div className="geo-map__count"><MapPin size={15}/>{points.length} на карте</div>
    {tileUnavailable&&<div className="geo-map__tile-notice" role="status">Подложка карты недоступна. Метки показаны по координатам.</div>}
    {chosen&&<div className="geo-map-preview" role="group" aria-label={`Выбрана локация ${chosen.title}`}>
     <button type="button" className="geo-map-preview__close" aria-label="Закрыть карточку" onClick={()=>onSelect(null)}><X size={17}/></button>
     <div className="geo-map-preview__photo"><MapPhoto location={chosen}/></div>
     <div className="geo-map-preview__body">
      <button type="button" className="geo-map-preview__title geo-map-preview__title-button" onClick={()=>onDetails(chosen)}>{chosen.title}</button>
      <div className="geo-map-preview__area"><MapPin size={14}/>{chosen.area}</div>
      <div className="geo-map-preview__actions">
       <button type="button" className="geo-map-preview__favorite" onClick={()=>onFavorite(chosen.id)} aria-label={favorites.has(chosen.id)?'Удалить из избранного':'В избранное'}>
        <Heart size={18} fill={favorites.has(chosen.id)?'currentColor':'none'}/>
       </button>
       <button type="button" className="geo-map-preview__details" onClick={()=>onOpenYandex(chosen)}>Открыть на карте <ArrowRight size={17}/></button>
      </div>
     </div>
    </div>}
   </div>
   :<div className="geo-map-empty"><MapPin size={28}/><h3>Нет точек с координатами</h3><p>Попробуйте изменить фильтры или выбрать другую локацию.</p></div>}
  {points.length<locations.length&&<p className="geo-map-notice">На карте отображаются только локации с подтверждёнными координатами. Остальные доступны в списке.</p>}
 </div>;
}
