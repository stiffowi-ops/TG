import {ArrowLeft, Heart} from 'lucide-react';

type HeaderProps = {
  onHome:()=>void;
  onBack?:()=>void;
  backLabel?:string;
  onFavorites?:()=>void;
};

export default function Header({onHome,onBack,backLabel='Назад',onFavorites}:HeaderProps){
  const wordmark = <span className="brand-text site-header__wordmark">
    Локации<span className="brand-dot">.</span>
    <small>Пространство возможностей</small>
  </span>;

  return <header className={onBack?'app-header site-header site-header--section':'app-header site-header'}>
    <div className="header-inner site-header__inner">
      {onBack&&<button type="button" className="catalog-page-heading__back" onClick={onBack} aria-label={backLabel}><ArrowLeft size={22}/></button>}
      {onFavorites
        ? <h1 className="site-header__brand" aria-label="Локации">{wordmark}</h1>
        : <button type="button" className="brand site-header__brand" onClick={onHome} aria-label="На главную">{wordmark}</button>}
      {onFavorites&&<button type="button" className="catalog-page-heading__favorite" onClick={onFavorites} aria-label="Избранное"><Heart size={23}/></button>}
    </div>
  </header>;
}
