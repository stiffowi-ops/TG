import {Search, SlidersHorizontal, X} from 'lucide-react';

export default function CatalogSearch({value,onChange,onOpenFilters,activeFilters}:{
 value:string;onChange:(value:string)=>void;onOpenFilters:()=>void;activeFilters:number
}){
 return <div className="catalog-search-row">
  <label className="catalog-search" htmlFor="catalog-search">
   <Search size={21} aria-hidden="true"/>
   <input id="catalog-search" type="search" autoComplete="off" placeholder="Поиск локаций..." value={value} onChange={e=>onChange(e.target.value)}/>
   {value&&<button type="button" className="catalog-search__clear" aria-label="Очистить поиск" onClick={()=>onChange('')}><X size={17}/></button>}
  </label>
  <button type="button" onClick={onOpenFilters} className={`catalog-filter-trigger ${activeFilters?'is-active':''}`}
   aria-label="Настроить фильтры" aria-haspopup="dialog">
   <SlidersHorizontal size={22}/>{activeFilters>0&&<span className="catalog-filter-trigger__count">{activeFilters}</span>}
  </button>
 </div>;
}
