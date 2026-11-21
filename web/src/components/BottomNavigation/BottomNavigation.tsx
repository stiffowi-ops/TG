import {Compass, Heart, Map as MapIcon, UserRound, ShieldCheck} from 'lucide-react';
import type {Tab} from '../../types';

type BottomNavigationProps={
 tab:Tab;
 onNavigate:(tab:Tab)=>void;
 savedCount:number;
 discountCount?:number;
 isAdmin:boolean;
 onHaptic?:()=>void;
};

export default function BottomNavigation({tab,onNavigate,savedCount,discountCount=0,isAdmin}:BottomNavigationProps){
 const items=[
  {id:'home' as const,label:'Локации',icon:Compass},
  {id:'map' as const,label:'Карта',icon:MapIcon},
  {id:'favorites' as const,label:'Избранное',icon:Heart},
  {id:'requests' as const,label:'Профиль',icon:UserRound},
  ...(isAdmin?[{id:'admin' as const,label:'Управление',icon:ShieldCheck}]:[]),
 ];
 return <nav className="bottom-nav bottom-navigation" aria-label="Основная навигация">
  {items.map(item=>{
   const active=tab===item.id||(item.id==='home'&&tab==='catalog');
   const Icon=item.icon;
   return <button type="button" key={item.id} aria-current={active?'page':undefined}
    className={`nav-item bottom-navigation__item ${active?'active':''}`}
    onClick={()=>onNavigate(item.id)}>
    <Icon size={22} strokeWidth={active?2.15:1.85} aria-hidden="true"/>
    <span>{item.label}</span>
    {item.id==='admin'&&discountCount>0&&<span className="nav-count" aria-label={'Запросов на скидку '+discountCount}>{discountCount}</span>}
    {item.id==='favorites'&&savedCount>0&&<span className="nav-count">{savedCount}</span>}
   </button>;
  })}
 </nav>;
}
