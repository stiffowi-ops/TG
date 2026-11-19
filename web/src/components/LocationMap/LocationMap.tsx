import {useEffect,useMemo,useRef,useState} from 'react';
import {ArrowRight,ChevronLeft,ChevronRight,FileText,Heart,LocateFixed,MapPin,Minus,Plus,X} from 'lucide-react';
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
type UserPoint={latitude:number;longitude:number};
const distanceKm=(from:UserPoint,to:Location)=>{
 const toRadians=(n:number)=>n*Math.PI/180;
 const deltaLat=toRadians(to.latitude!-from.latitude);
 const deltaLng=toRadians(to.longitude!-from.longitude);
 const a=Math.sin(deltaLat/2)**2+Math.cos(toRadians(from.latitude))*Math.cos(toRadians(to.latitude!))*Math.sin(deltaLng/2)**2;
 return 6371*2*Math.atan2(Math.sqrt(a),Math.sqrt(1-a));
};
const readableDistance=(km:number)=>km<1?Math.round(km*1000)+' м':km.toFixed(1).replace('.',',')+' км';
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
 const clusterMarkersRef=useRef<Map<string,Pin>>(new Map());
 const clusterUpdateRef=useRef<()=>void>(()=>{});
 const userMarkerRef=useRef<MapLibreMarker|null>(null);
 const pointsRef=useRef(points);
 pointsRef.current=points;
 const selectedRef=useRef(selectedId);
 selectedRef.current=selectedId;
 const onSelectRef=useRef(onSelect);
 onSelectRef.current=onSelect;
 const swipeRef=useRef<{x:number;y:number}|null>(null);
 const [nearbyPoint,setNearbyPoint]=useState<UserPoint|null>(null);
 const [expandedCluster,setExpandedCluster]=useState<string[]|null>(null);
 const nearby=useMemo(()=>nearbyPoint?points.map(location=>({location,km:distanceKm(nearbyPoint,location)}))
  .sort((a,b)=>a.km-b.km||a.location.title.localeCompare(b.location.title,'ru')):[],[points,nearbyPoint]);
 const ordered=nearbyPoint?nearby.map(item=>item.location):points;
 const selectedIndex=ordered.findIndex(location=>location.id===selectedId);
 const changeSelected=(step:number)=>{
  if(!ordered.length)return;
  const next=(selectedIndex+step+ordered.length)%ordered.length;
  onSelect(ordered[next].id);
 };
 const selectedDistance=nearby.find(item=>item.location.id===selectedId)?.km;
 const [tileFailures,setTileFailures]=useState(0);
 const [mapUnavailable,setMapUnavailable]=useState(false);
 const [geoError,setGeoError]=useState('');

 // Lightweight screen-space clustering. Keep existing K markers mounted so gallery
 // refreshes, selection and favorites never recreate their DOM nodes.
 clusterUpdateRef.current=()=>{
  const map=mapRef.current;
  if(!map)return;
  const locations=pointsRef.current;
  const pins=markersRef.current;
  const cellSize=55;
  const groups=new Map<number,number[]>();
  const parents=locations.map((_,index)=>index);
  const find=(index:number):number=>{
   while(parents[index]!==index){parents[index]=parents[parents[index]];index=parents[index]}
   return index;
  };
  const buckets=new Map<string,Array<{index:number;x:number;y:number}>>();
  const locationsOnScreen:Array<{index:number;x:number;y:number}>=[];
  for(let index=0;index<locations.length;index++){
   const location=locations[index];
   const marker=pins.get(location.id);
   if(marker)marker.element.style.visibility='visible';
   if(!marker||location.id===selectedRef.current)continue;
   const p=map.project([location.longitude!,location.latitude!]);
   if(p.x< -cellSize||p.y< -cellSize||p.x>map.getContainer().clientWidth+cellSize||p.y>map.getContainer().clientHeight+cellSize)continue;
   const cellX=Math.floor(p.x/cellSize),cellY=Math.floor(p.y/cellSize);
   for(let dx=-1;dx<=1;dx++)for(let dy=-1;dy<=1;dy++){
    for(const entry of buckets.get((cellX+dx)+':'+(cellY+dy))??[]){
     if((entry.x-p.x)**2+(entry.y-p.y)**2<=cellSize**2){
      const rootA=find(index),rootB=find(entry.index);
      if(rootA!==rootB)parents[rootA]=rootB;
     }
    }
   }
   const key=cellX+':'+cellY;
   const bucket=buckets.get(key)??[];
   bucket.push({index,x:p.x,y:p.y});
   buckets.set(key,bucket);
   locationsOnScreen.push({index,x:p.x,y:p.y});
  }
  for(const entry of locationsOnScreen){
   const root=find(entry.index);
   const group=groups.get(root)??[];
   group.push(entry.index);
   groups.set(root,group);
  }
  const surviving=new Set<string>();
  for(const indexes of groups.values()){
   if(indexes.length<2)continue;
   const ids=indexes.map(index=>locations[index].id).sort();
   const key=ids.join('|');
   surviving.add(key);
   let x=0,y=0;
   for(const index of indexes){
    const location=locations[index];
    const position=map.project([location.longitude!,location.latitude!]);
    x+=position.x;y+=position.y;
    const pin=pins.get(location.id);
    if(pin)pin.element.style.visibility='hidden';
   }
   const center=map.unproject([x/indexes.length,y/indexes.length]);
   let pin=clusterMarkersRef.current.get(key);
   if(!pin){
    const element=document.createElement('button');
    element.type='button';
    element.className='geo-cluster-marker';
    element.textContent=String(ids.length);
    element.setAttribute('aria-label','Группа из '+ids.length+' локаций');
    element.title=ids.length+' локаций — приблизить';
    element.addEventListener('click',event=>{
     event.stopPropagation();
     const current=mapRef.current;
     if(!current)return;
     if(current.getZoom()>=17.5){
      setExpandedCluster(ids);
     }else{
      const bounds=new LngLatBounds();
      ids.forEach(id=>{
       const location=pointsRef.current.find(l=>l.id===id);
       if(location)bounds.extend([location.longitude!,location.latitude!]);
      });
      const center=current.unproject([
       ids.reduce((sum,id)=>{
        const loc=pointsRef.current.find(l=>l.id===id);
        return sum+(loc?current.project([loc.longitude!,loc.latitude!]).x:0);
       },0)/ids.length,
       ids.reduce((sum,id)=>{
        const loc=pointsRef.current.find(l=>l.id===id);
        return sum+(loc?current.project([loc.longitude!,loc.latitude!]).y:0);
       },0)/ids.length
      ]);
      current.easeTo({center,zoom:Math.min(18,current.getZoom()+2),duration:320,essential:true});
     }
    });
    const marker=new MapLibreMarker({element,anchor:'center'}).setLngLat(center).addTo(map);
    pin={marker,element};
    clusterMarkersRef.current.set(key,pin);
   }else{
    pin.marker.setLngLat(center);
   }
  }
  for(const [key,pin] of clusterMarkersRef.current){
   if(!surviving.has(key)){pin.marker.remove();clusterMarkersRef.current.delete(key)}
  }
 };

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
   map.on('load',()=>{if(active){map?.resize();clusterUpdateRef.current()}});
   map.on('moveend',()=>{if(active)clusterUpdateRef.current()});
   map.on('resize',()=>{if(active)clusterUpdateRef.current()});
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
   clusterMarkersRef.current.forEach(({marker})=>marker.remove());
   clusterMarkersRef.current.clear();
   userMarkerRef.current?.remove();
   userMarkerRef.current=null;
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
  })){clusterUpdateRef.current();return;}
  markersRef.current.forEach(({marker})=>marker.remove());
  markersRef.current.clear();
  for(const location of points){
   const element=document.createElement('button');
   element.type='button';
   element.className='geo-pin-container';
   element.setAttribute('aria-label','Выбрать локацию '+location.title);
   element.title=location.title;
   // Keep the actual geographic anchor at the tip; animate only the SVG,
   // never the positioned MapLibre marker wrapper.
   element.innerHTML=`<span class="geo-pin" aria-hidden="true">
    <svg class="geo-pin__svg" viewBox="0 0 40 53" focusable="false">
     <path class="geo-pin__shape" d="M20 2.5C10.7 2.5 3 9.9 3 19.3c0 8.4 5.6 15.2 14 17.1L20 50l3-13.6c8.4-1.9 14-8.7 14-17.1C37 9.9 29.3 2.5 20 2.5Z"/>
     <text class="geo-pin__letter" x="20" y="26.2" text-anchor="middle">К</text>
    </svg>
    <span class="geo-pin__heart" aria-hidden="true">♥</span>
   </span>`;
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
  clusterUpdateRef.current();
 },[points,hasPoints]);

 useEffect(()=>{
  markersRef.current.forEach(({element},id)=>{
   const active=id===selectedId;
   element.classList.toggle('geo-pin-container--active',active);
   element.querySelector('.geo-pin')?.classList.toggle('geo-pin--active',active);
   element.classList.toggle('geo-pin-container--favorite',favorites.has(id));
   element.setAttribute('aria-pressed',String(active));
  });
  clusterUpdateRef.current();
 },[selectedId,points,favorites]);

 useEffect(()=>{
  const map=mapRef.current;
  if(!map||!nearbyPoint)return;
  if(!userMarkerRef.current){
   const el=document.createElement('span');
   el.className='geo-user-point';
   el.setAttribute('aria-label','Ваше местоположение');
   userMarkerRef.current=new MapLibreMarker({element:el,anchor:'center'}).addTo(map);
  }
  userMarkerRef.current.setLngLat([nearbyPoint.longitude,nearbyPoint.latitude]);
 },[nearbyPoint,hasPoints]);

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
  setGeoError('Определяем местоположение…');
  navigator.geolocation.getCurrentPosition(position=>{
   const user={latitude:position.coords.latitude,longitude:position.coords.longitude};
   setGeoError('');
   setNearbyPoint(user);
   setExpandedCluster(null);
   map.flyTo({center:[user.longitude,user.latitude],zoom:Math.max(11,map.getZoom()),duration:500,essential:true});
  },error=>setGeoError(error.code===1?'Доступ к геолокации запрещён. Разрешите его в браузере.':'Не удалось определить местоположение. Попробуйте ещё раз.'),{enableHighAccuracy:false,timeout:10000,maximumAge:60000});
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
   {expandedCluster&&<div className="geo-cluster-sheet" role="group" aria-label="Локации в одной точке">
    <div className="geo-cluster-sheet__heading"><strong>Локации рядом ({expandedCluster.length})</strong><button type="button" aria-label="Закрыть список локаций" onClick={()=>setExpandedCluster(null)}><X size={17}/></button></div>
    <div className="geo-cluster-sheet__list">{expandedCluster.map(id=>{
     const location=points.find(p=>p.id===id);
     return location?<button type="button" key={id} onClick={()=>{onSelect(id);setExpandedCluster(null)}}>{location.title}<ArrowRight size={15}/></button>:null;
    })}</div>
   </div>}
   {nearbyPoint&&!chosen&&<div className="geo-nearby" role="region" aria-label="Ближайшие локации">
    <div className="geo-nearby__heading"><strong>Рядом со мной</strong><button type="button" onClick={()=>{setNearbyPoint(null);userMarkerRef.current?.remove();userMarkerRef.current=null}} aria-label="Закрыть ближайшие локации"><X size={17}/></button></div>
    <div className="geo-nearby__list">{nearby.slice(0,6).map(({location,km})=><button type="button" key={location.id} onClick={()=>onSelect(location.id)}>
     <span>{location.title}</span><small>{readableDistance(km)}</small><ArrowRight size={15}/>
    </button>)}</div>
   </div>}
   {chosen&&<div className="geo-map-preview" role="group" aria-label={'Выбрана локация '+chosen.title}
    onTouchStart={event=>{swipeRef.current={x:event.touches[0].clientX,y:event.touches[0].clientY}}}
    onTouchEnd={event=>{
     const start=swipeRef.current;
     swipeRef.current=null;
     if(!start)return;
     const dx=event.changedTouches[0].clientX-start.x;
     const dy=event.changedTouches[0].clientY-start.y;
     if(Math.abs(dx)>70&&Math.abs(dx)>Math.abs(dy)*1.3){
      // Photo gallery has its own swipe gesture: only card body swipes navigate locations.
      if(!(event.target instanceof Element && event.target.closest('.geo-map-preview__photo')))changeSelected(dx<0?1:-1);
     }
    }}>
    <button type="button" className="geo-map-preview__close" aria-label="Закрыть карточку" onClick={()=>onSelect(null)}><X size={17}/></button>
    <div className="geo-map-preview__photo"><MapPhoto location={chosen} onOpen={()=>onDetails(chosen)}/></div>
    <div className="geo-map-preview__body">
     <h3 className="geo-map-preview__title">{chosen.title}</h3>
     <div className="geo-map-preview__browse" role="group" aria-label="Листать локации">
      <button type="button" aria-label="Предыдущая локация" onClick={()=>changeSelected(-1)} disabled={ordered.length<2}><ChevronLeft size={17}/></button>
      <span>{selectedIndex+1} из {ordered.length}{selectedDistance!==undefined?' · '+readableDistance(selectedDistance):''}</span>
      <button type="button" aria-label="Следующая локация" onClick={()=>changeSelected(1)} disabled={ordered.length<2}><ChevronRight size={17}/></button>
     </div>
     <button type="button" className="geo-map-preview__description" onClick={()=>onDetails(chosen)}>
      <FileText size={17} aria-hidden="true"/><span>Описание локации</span><ArrowRight className="geo-map-preview__description-arrow" size={18} aria-hidden="true"/>
     </button>
     <div className="geo-map-preview__actions">
      <button type="button" className="geo-map-preview__favorite" onClick={()=>onFavorite(chosen.id)} aria-label={favorites.has(chosen.id)?'Удалить из избранного':'В избранное'} aria-pressed={favorites.has(chosen.id)}>
       <Heart size={19} fill={favorites.has(chosen.id)?'currentColor':'none'}/>
      </button>
      <button type="button" className="geo-map-preview__details" onClick={()=>onOpenYandex(chosen)}><MapPin size={16} aria-hidden="true"/>Открыть на карте <ArrowRight size={16} aria-hidden="true"/></button>
     </div>
    </div>
   </div>}
   <div className="geo-map__attribution"><a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">© OpenStreetMap contributors</a></div>
  </div>:<div className="geo-map-empty"><MapPin size={28}/><h3>Нет точек с координатами</h3><p>Попробуйте изменить фильтры или выбрать другую локацию.</p></div>}
 </div>;
}