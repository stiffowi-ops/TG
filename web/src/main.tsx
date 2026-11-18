import React from 'react';
import {createRoot} from 'react-dom/client';
import App from './App';
import './styles.css';

// A rendering error should never strand Telegram users on a completely white screen.
class AppErrorBoundary extends React.Component<React.PropsWithChildren, {failed:boolean}> {
 state={failed:false};
 static getDerivedStateFromError():{failed:boolean}{return {failed:true};}
 componentDidCatch(error:Error, info:React.ErrorInfo){
  console.error('Mini App rendering error',error,info.componentStack);
 }
 render(){
  if(this.state.failed){
   return <main className="miniapp-recovery" role="alert">
    <div className="miniapp-recovery__mark">!</div>
    <h1>Не удалось открыть приложение</h1>
    <p>Возникла ошибка отображения. Попробуйте обновить страницу.</p>
    <button type="button" onClick={()=>window.location.reload()}>Обновить приложение</button>
   </main>;
  }
  return this.props.children;
 }
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><AppErrorBoundary><App/></AppErrorBoundary></React.StrictMode>);
