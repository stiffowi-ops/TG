import type {ReactNode} from 'react';
import {ChevronRight} from 'lucide-react';

type ShortcutCardProps={
 label:string;
 icon:ReactNode;
 artwork?:string;
 onClick:()=>void;
 tone:'locations'|'selection'|'favorites';
};

export default function ShortcutCard({label,icon,artwork,onClick,tone}:ShortcutCardProps){
 return <button type="button" className={`landing-shortcut shortcut-card shortcut-card--${tone}`} onClick={onClick}>
  <span className="shortcut-icon" aria-hidden="true">{icon}</span>
  <span className="shortcut-label">{label}</span>
  {artwork&&<img className="shortcut-card__art" src={artwork} alt="" loading="lazy"/>}
  <ChevronRight size={20} className="shortcut-chevron" aria-hidden="true"/>
 </button>;
}
