import {ChevronDown, MapPin} from 'lucide-react';

export default function DistrictFilter({value,areas,onChange}:{
 value:string;areas:string[];onChange:(value:string)=>void
}){
 return <label className="catalog-district-filter">
  <MapPin size={18} aria-hidden="true"/>
  <select value={value} onChange={e=>onChange(e.target.value)} aria-label="Район">
   {areas.map(area=><option key={area} value={area}>{area}</option>)}
  </select>
  <ChevronDown size={17} aria-hidden="true"/>
 </label>;
}
