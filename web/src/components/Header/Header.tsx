import {ArrowLeft, Heart} from 'lucide-react';

type HeaderProps = {
  onHome:()=>void;
  onFavorites?:()=>void;
};

export default function Header({onHome,onFavorites}:HeaderProps){
  const wordmark = <span className="brand-text site-header__wordmark">
    Локации<span className="brand-dot">.</span>
    <small>Пространство возможностей</small>
  </span>;

  return <header className="app-header site-header">
    <div className="header-inner site-header__inner">
      {onFavorites ? <>
        <button type="button" className="catalog-page-heading__back" onClick={onHome} aria-label="На главную"><ArrowLeft size={22}/></button>
        <h1 className="site-header__brand" aria-label="Локации">{wordmark}</h1>
        <button type="button" className="catalog-page-heading__favorite" onClick={onFavorites} aria-label="Избранное"><Heart size={23}/></button>
      </> : <button type="button" className="brand site-header__brand" onClick={onHome} aria-label="На главную">{wordmark}</button>}
    </div>
  </header>;
}
