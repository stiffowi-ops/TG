import {designAssets} from '../../design/assets';
import PrimaryButton from '../PrimaryButton/PrimaryButton';

type HeroProps={onBrowse:()=>void};

export default function Hero({onBrowse}:HeroProps){
 return <section className="landing-hero hero-composition" aria-labelledby="hero-heading">
  <div className="landing-copy hero-composition__copy">
   <span className="landing-eyebrow">ВОЗМОЖНОСТИ ДЛЯ ВАШЕГО БИЗНЕСА</span>
   <h1 id="hero-heading">Найдите место,<br/>где начнётся <em>рост.</em></h1>
   <p>Изучайте локации, сравнивайте варианты и выбирайте точки, которые подходят Вашим планам.</p>
   <PrimaryButton onClick={onBrowse}>Смотреть локации</PrimaryButton>
  </div>
  <div className="landing-scene hero-composition__scene" aria-hidden="true">
   <picture>
    <source type="image/avif" srcSet={designAssets.hero.modern}/>
    <img className="landing-illustration" src={designAssets.hero.fallback} alt="" loading="eager" decoding="async" fetchPriority="high"/>
   </picture>
  </div>
 </section>;
}
