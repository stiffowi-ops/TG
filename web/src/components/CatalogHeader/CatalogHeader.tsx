import {ArrowLeft, Heart} from 'lucide-react';

export default function CatalogHeader({onBack,onFavorites}:{
 onBack:()=>void;onFavorites:()=>void
}){
 return <div className="catalog-page-heading catalog-page-heading--modern">
  <div className="catalog-page-heading__row">
   <button type="button" className="catalog-page-heading__back" onClick={onBack} aria-label="На главную"><ArrowLeft size={22}/></button>
   <div><span className="catalog-page-heading__eyebrow">ПРОСТРАНСТВО ВОЗМОЖНОСТЕЙ</span><h1>Локации<span>.</span></h1></div>
   <button type="button" className="catalog-page-heading__favorite" onClick={onFavorites} aria-label="Избранное"><Heart size={23}/></button>
  </div>
  <p>Найдите подходящее место для Вашего бизнеса</p>
 </div>;
}
