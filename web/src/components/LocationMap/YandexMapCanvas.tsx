import {useEffect,useRef,useState} from 'react';
import type {Location} from '../../types';

type Point = [number,number];
type MapEntity = {destroy:()=>void;addChild:(child:unknown)=>void;update:(change:unknown)=>void};
type Api={
 ready:Promise<unknown>;
 YMap:new (container:HTMLElement,props:unknown)=>MapEntity;
 YMapDefaultSchemeLayer:new (props?:unknown)=>unknown;
 YMapDefaultFeaturesLayer:new (props?:unknown)=>unknown;
 YMapMarker:new (props:unknown,element:HTMLElement)=>unknown;
};
declare global{interface Window{ymaps3?:Api}}
let pending:Promise<Api>|null=null;
function loadYandex(key:string):Promise<Api>{
 if(window.ymaps3)return window.ymaps3.ready.then(()=>window.ymaps3!);
 if(pending)return pending;
 pending=new Promise<Api>((resolve,reject)=>{
  const script=document.createElement('script');
  script.src='https://api-maps.yandex.ru/v3/?apikey='+encodeURIComponent(key)+'&lang=ru_RU';
  script.async=true;script.referrerPolicy='strict-origin-when-cross-origin';
  const timer=window.setTimeout(()=>{script.remove();reject(Error('Время подключения Яндекс Карт истекло'))},18000);
  script.onload=()=>{window.clearTimeout(timer);if(!window.ymaps3){reject(Error('Библиотека Яндекс Карт не найдена'));return}window.ymaps3.ready.then(()=>resolve(window.ymaps3!),reject)};
  script.onerror=()=>{window.clearTimeout(timer);reject(Error('Не удалось загрузить Яндекс Карты'))};
  document.head.appendChild(script);
 }).catch(error=>{pending=null;throw error});
 return pending;
}
function calculateLocation(points:Location[]){
 const xs=points.map(l=>l.longitude!),ys=points.map(l=>l.latitude!);
 const minX=Math.min(...xs),maxX=Math.max(...xs),minY=Math.min(...ys),maxY=Math.max(...ys);
 const span=Math.max((maxX-minX)*.6,maxY-minY);
 const zoom=span>.8?8:span>.4?9:span>.22?10:span>.1?11:span>.045?12:13;
 return {center:[(minX+maxX)/2,(minY+maxY)/2] as Point,zoom};
}
type Props={locations:Location[];selectedId:string|null;onSelect:(id:string)=>void;apiKey:string;onFailure:()=>void};
export default function YandexMapCanvas({locations,selectedId,onSelect,apiKey,onFailure}:Props){
 const ref=useRef<HTMLDivElement>(null);
 const mapRef=useRef<MapEntity|null>(null);
 const domMarkers=useRef<Map<string,HTMLElement>>(new Map());
 const latestSelect=useRef(onSelect);latestSelect.current=onSelect;
 const [ready,setReady]=useState(false);
 const signature=locations.map(l=>l.id+':'+l.longitude+':'+l.latitude).join('|');
 useEffect(()=>{
  let cancelled=false;let map:MapEntity|null=null;
  setReady(false);
  const host=ref.current;if(!host)return;
  loadYandex(apiKey).then(api=>{
   if(cancelled)return;
   const loc=calculateLocation(locations);
   map=new api.YMap(host,{location:loc});
   mapRef.current=map;
   map.addChild(new api.YMapDefaultSchemeLayer({}));
   map.addChild(new api.YMapDefaultFeaturesLayer({}));
   const markers=new Map<string,HTMLElement>();
   for(const location of locations){
    const el=document.createElement('button');
    el.type='button';el.className='yandex-location-pin';
    el.setAttribute('aria-label','Показать '+location.title);
    el.setAttribute('data-location-id',location.id);
    el.textContent='●';
    el.addEventListener('click',()=>latestSelect.current(location.id));
    map.addChild(new api.YMapMarker({coordinates:[location.longitude!,location.latitude!]},el));
    markers.set(location.id,el);
   }
   domMarkers.current=markers;
   setReady(true);
  }).catch(error=>{if(!cancelled){console.warn('Yandex Maps API unavailable',error);onFailure()}});
  return()=>{cancelled=true;try{map?.destroy()}catch{};mapRef.current=null;domMarkers.current.clear();void 0};
 },[apiKey,signature]);
 useEffect(()=>{
  for(const [id,node] of domMarkers.current){node.classList.toggle('is-selected',id===selectedId)}
  if(!selectedId||!mapRef.current)return;
  const chosen=locations.find(l=>l.id===selectedId);
  if(chosen)mapRef.current.update({location:{center:[chosen.longitude!,chosen.latitude!],zoom:14,duration:420}});
 },[selectedId,ready]);
 return <div className="geo-map__tiles yandex-map-canvas" data-testid="yandex-map-canvas" aria-label="Яндекс Карты">
  <div ref={ref} className="yandex-map-host"/>
  {!ready&&<span className="yandex-map-loading">Подключаем Яндекс Карты…</span>}
 </div>;
}
