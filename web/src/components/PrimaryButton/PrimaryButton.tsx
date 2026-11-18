import type {ReactNode} from 'react';
import {ArrowRight} from 'lucide-react';

type PrimaryButtonProps={children:ReactNode;onClick:()=>void;className?:string};

export default function PrimaryButton({children,onClick,className=''}:PrimaryButtonProps){
 return <button type="button" className={['landing-cta','primary-action',className].filter(Boolean).join(' ')} onClick={onClick}>
  <span>{children}</span><ArrowRight size={20} aria-hidden="true"/>
 </button>;
}
