import {List, Map as MapIcon} from 'lucide-react';

export type CatalogView='list'|'map';
export default function ViewSwitcher({view,onChange}:{
 view:CatalogView;onChange:(view:CatalogView)=>void
}){
 return <div className="catalog-view-switcher" role="group" aria-label="Режим просмотра">
  <button type="button" className={view==='list'?'active':''} aria-pressed={view==='list'} onClick={()=>onChange('list')}><List size={18}/>Список</button>
  <button type="button" className={view==='map'?'active':''} aria-pressed={view==='map'} onClick={()=>onChange('map')}><MapIcon size={18}/>Карта</button>
 </div>;
}
