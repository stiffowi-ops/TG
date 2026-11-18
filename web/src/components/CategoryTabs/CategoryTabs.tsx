import {MapPin, TrainFront} from 'lucide-react';

export default function CategoryTabs({value,onChange,available}:{
 value:string;onChange:(category:string)=>void;available:string[]
}){
 const kinds=[
  {key:'Все',label:'Все',icon:null},
  {key:'Метро',label:'Метро',icon:<TrainFront size={17}/>},
  {key:'Другие локации',label:'Другие',icon:<MapPin size={17}/>},
 ].filter(item=>item.key==='Все'||available.includes(item.key));
 return <div className="catalog-category-tabs" aria-label="Категории локаций" role="group">
  {kinds.map(item=><button type="button" key={item.key} className={value===item.key?'active':''}
   onClick={()=>onChange(item.key)} aria-pressed={value===item.key}>
   {item.icon}{item.label}
  </button>)}
 </div>;
}
