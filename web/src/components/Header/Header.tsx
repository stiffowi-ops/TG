type HeaderProps = {onHome:()=>void};

export default function Header({onHome}:HeaderProps){
  return <header className="app-header site-header">
    <div className="header-inner site-header__inner">
      <button type="button" className="brand site-header__brand" onClick={onHome} aria-label="На главную">
        <span className="brand-text site-header__wordmark">Локации<span className="brand-dot">.</span>
          <small>Пространство возможностей</small>
        </span>
      </button>
    </div>
  </header>;
}
