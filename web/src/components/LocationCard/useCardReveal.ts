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
     io.unobserve(item);
     requestAnimationFrame(()=>{if(item.isConnected)delete item.dataset.revealPending});
    }else{
     item.dataset.revealPending='';
    }
   }
  },{root:card.closest('main')});

  // Mark every new, unseen card before paint. Otherwise above-fold cards
  // can render fully visible and the reveal never actually starts.
  card.dataset.revealPending='';
  io.observe(card);
  return ()=>{
   io.unobserve(card);
   delete card.dataset.revealPending;
  };
 },[]);
}
