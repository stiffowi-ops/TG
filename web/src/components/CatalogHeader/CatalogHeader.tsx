import Header from '../Header/Header';

export default function CatalogHeader({onBack,onFavorites}:{
 onBack:()=>void;onFavorites:()=>void
}){
 return <>
  <Header onHome={onBack} onFavorites={onFavorites}/>
  <p className="catalog-intro">Найдите подходящее место для Вашего бизнеса</p>
 </>;
}
