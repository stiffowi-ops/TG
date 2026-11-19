import type {RefCallback} from 'react';
import {Search} from 'lucide-react';
import type {Location} from '../../types';
import CatalogSearch from '../CatalogSearch/CatalogSearch';
import CatalogFilters from '../CatalogFilters/CatalogFilters';
import LocationCard from '../LocationCard/LocationCard';
import LocationMap from '../LocationMap/LocationMap';
import type {CatalogView} from '../ViewSwitcher/ViewSwitcher';

type Props={
 filtersOpen:boolean;setFiltersOpen:(open:boolean)=>void;
 observeCard:RefCallback<HTMLElement>;
 locations:Location[];areas:string[];availableCategories:string[];
 query:string;onQuery:(query:string)=>void;
 area:string;onArea:(area:string)=>void;
 category:string;onCategory:(category:string)=>void;
 view:CatalogView;onView:(view:CatalogView)=>void;
 selectedId:string|null;onSelect:(id:string|null)=>void;
 favorites:Set<string>;onFavorite:(id:string)=>void;
 onDetails:(location:Location)=>void;onMap:(location:Location)=>void;
 loading:boolean;
};

export default function CatalogPage({
 filtersOpen,setFiltersOpen,locations,observeCard,areas,availableCategories,query,onQuery,area,onArea,category,onCategory,
 view,onView,selectedId,onSelect,favorites,onFavorite,onDetails,onMap,
 loading
}:Props){
 const activeFilters=Number(area!=='Все районы')+Number(category!=='Все');
 const reset=()=>{onArea('Все районы');onCategory('Все')};
 return <section className="section-block catalog-page catalog-page--redesigned" id="catalog-results">
  <p className="catalog-intro">Найдите подходящее место для Вашего бизнеса</p>
  <div className="catalog-page__controls">
   <CatalogSearch value={query} onChange={onQuery} onOpenFilters={()=>setFiltersOpen(true)} activeFilters={activeFilters}/>
   <CatalogFilters area={area} areas={areas} onArea={onArea} category={category}
    available={availableCategories} onCategory={onCategory} view={view}
    onView={onView} activeFilters={activeFilters} reset={reset}
    open={filtersOpen} onClose={()=>setFiltersOpen(false)}/>
  </div>
  <div key={view} className="catalog-view-content">
   <p className="catalog-view-hint">{view==='map'?'Выберите метку на карте':'Выберите подходящую локацию'}</p>
   {view==='map'?
    <LocationMap locations={locations} selectedId={selectedId} onSelect={onSelect}
     favorites={favorites} onFavorite={onFavorite} onDetails={onDetails} onOpenYandex={onMap}/>:
    locations.length?<div className="card-grid catalog-location-grid">
     {locations.map(location=><LocationCard key={location.id} location={location} observeCard={observeCard}
      saved={favorites.has(location.id)} onFavorite={onFavorite}
      onDetails={onDetails} onMap={onMap}/>)}
    </div>:<div className="catalog-no-results">
     <Search size={30}/><h3>Локации не найдены</h3>
     <p>Попробуйте изменить запрос или параметры поиска.</p>
     <button type="button" onClick={()=>{onQuery('');reset()}}>Сбросить фильтры</button>
    </div>}
   {loading&&<p className="discreet-loading">Обновляем каталог…</p>}
  </div>
 </section>;
}
