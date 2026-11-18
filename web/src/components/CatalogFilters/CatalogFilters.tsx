import {Check, RotateCcw, X} from 'lucide-react';
import DistrictFilter from '../DistrictFilter/DistrictFilter';
import CategoryTabs from '../CategoryTabs/CategoryTabs';
import ViewSwitcher from '../ViewSwitcher/ViewSwitcher';
import type {CatalogView} from '../ViewSwitcher/ViewSwitcher';

type Props={
 area:string;areas:string[];onArea:(area:string)=>void;
 category:string;available:string[];onCategory:(category:string)=>void;
 view:CatalogView;onView:(view:CatalogView)=>void;
 activeFilters:number;reset:()=>void;
 open:boolean;onClose:()=>void;
};
export default function CatalogFilters({area,areas,onArea,category,available,onCategory,view,onView,activeFilters,reset,open,onClose}:Props){
 return <>
  <div className="catalog-filter-controls">
   <DistrictFilter value={area} areas={areas} onChange={onArea}/>
   <ViewSwitcher view={view} onChange={onView}/>
  </div>
  <CategoryTabs value={category} onChange={onCategory} available={available}/>
  {open&&<div className="catalog-filter-overlay" role="presentation" onMouseDown={e=>{if(e.target===e.currentTarget)onClose()}}>
   <section className="catalog-filter-sheet" role="dialog" aria-modal="true" aria-label="Фильтры локаций">
    <div className="catalog-filter-sheet__header"><h2>Фильтры</h2><button type="button" aria-label="Закрыть фильтры" onClick={onClose}><X size={23}/></button></div>
    <p>Настройте подбор под Ваши планы.</p>
    <label className="catalog-filter-sheet__label">Район</label>
    <DistrictFilter value={area} areas={areas} onChange={onArea}/>
    <label className="catalog-filter-sheet__label">Тип локации</label>
    <CategoryTabs value={category} onChange={onCategory} available={available}/>
    <div className="catalog-filter-sheet__actions">
     <button type="button" className="catalog-reset" onClick={reset}><RotateCcw size={16}/>Сбросить {activeFilters>0?('('+activeFilters+')'):''}</button>
     <button type="button" className="catalog-apply" onClick={onClose}><Check size={18}/>Показать локации</button>
    </div>
   </section>
  </div>}
 </>;
}