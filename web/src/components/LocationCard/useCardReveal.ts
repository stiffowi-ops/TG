import {useCallback,useRef} from 'react';

// The catalog and favorites share one observer and one history for this App session.
export default function useCardReveal(){
 const seen=useRef(new Set<string>());
 const observer=useRef<IntersectionObserver|null>(null);

 return useCallback((card:HTMLElement|null)=>{
  if(!card || !card.dataset.locationId || seen.current.has(card.dataset.locationId) ||
     typeof IntersectionObserver==='undefined')return;

  const io=observer.current??=new IntersectionObserver((entries,io)=>{
   for(const entry of entries){
    const item=entry.target as HTMLElement;
    if(entry.isIntersecting){
     seen.current.add(item.dataset.locationId!);
     delete item.dataset.revealPending;
     io.unobserve(item);
    }else{
     item.dataset.revealPending='';
    }
   }
  },{root:card.closest('main')});

  io.observe(card);
  return ()=>{
   io.unobserve(card);
   delete card.dataset.revealPending;
  };
 },[]);
}
