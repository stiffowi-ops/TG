import {useEffect,useState} from 'react';
import type {DemoCase} from '../../types';
import ProfileGlyph from './ProfileGlyph';
const states=['received','working','resolved'] as const;
const caseLabels=['Получено','В работе','Решено'];
const glyphs=['sent','clock','done'] as const;
export default function ProgressTracker({status,labels=caseLabels,subject="заявки"}:{status:DemoCase['status'];labels?:readonly string[];subject?:string}){
 const idx=states.indexOf(status);
 const [animate,setAnimate]=useState(false);
 useEffect(()=>{const timer=window.setTimeout(()=>setAnimate(true),100);return()=>window.clearTimeout(timer)},[status]);
 return <div className="profile-tracker" aria-label={'Статус '+subject+': '+labels[idx]}>
   <span className="profile-tracker__line"><span style={{width:animate?idx*50+'%':'0%'}}/></span>
   {states.map((state,i)=><span key={state} className={'profile-tracker__step profile-tracker__step--'+state+(i<=idx?' visited':'')+(i===idx?' current':'')}>
     <span className="profile-tracker__icon"><ProfileGlyph name={glyphs[i]} size={22}/></span>
     <strong>{labels[i]}</strong>
   </span>)}
 </div>;
}
