import {Heart, MapPin, SlidersHorizontal} from 'lucide-react';
import Hero from '../Hero/Hero';
import ShortcutCard from '../ShortcutCard/ShortcutCard';
import {designAssets} from '../../design/assets';

type LandingPageProps={
 onCatalog:()=>void;
 onSearch:()=>void;
 onFavorites:()=>void;
};

export default function LandingPage({onCatalog,onSearch,onFavorites}:LandingPageProps){
 return <div className="landing-home" aria-label="Главная страница">
  <Hero onBrowse={onCatalog}/>
  <nav className="landing-shortcuts" aria-label="Возможности сервиса">
   <ShortcutCard label="Большое разнообразие локаций" icon={<MapPin size={25}/>} artwork={designAssets.shortcutBuilding} tone="locations" onClick={onCatalog}/>
   <ShortcutCard label="Удобный подбор" icon={<SlidersHorizontal size={25}/>} tone="selection" onClick={onSearch}/>
   <ShortcutCard label="Сохраняйте лучшее" icon={<Heart size={25}/>} artwork={designAssets.shortcutFavorite} tone="favorites" onClick={onFavorites}/>
  </nav>
 </div>;
}
