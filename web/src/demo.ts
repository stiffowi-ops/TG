import type {DemoCase, Location} from './types';
export const backupLocations:Location[] = [
 {id:'001',title:'Метро Остров Мечты',category:'Метро',area:'Юг Москвы',latitude:55.6921867213,longitude:37.668590455},
 {id:'002',title:'Метро Кленовый бульвар',category:'Метро',area:'Юг Москвы',latitude:55.674692,longitude:37.681568},
 {id:'009',title:'Метро Бирюлёво',category:'Метро',area:'Юг Москвы',latitude:55.5799345286,longitude:37.6588481032},
 {id:'012',title:'Метро Сосенки',category:'Метро',area:'Новая Москва',latitude:55.5517485464,longitude:37.4363464259},
 {id:'017',title:'Метро Троицк',category:'Метро',area:'Новая Москва',latitude:55.4566596493,longitude:37.276769664},
 {id:'033',title:'Метро Сколково',category:'Метро',area:'Запад Москвы',latitude:55.6994740533,longitude:37.3501576403},
 {id:'036',title:'Ивантеевка',category:'Другие локации',area:'Подмосковье',latitude:55.9754911228,longitude:37.9095597672},
];
const categoryLabels=new Map([
 ['Метро','У метро'],
 ['Другие локации','Другие'],
]);
export function categoryLabel(category:string){return categoryLabels.get(category)??category;}
export const statusLabel = {received:'📨 Получено',working:'🛠 В работе',resolved:'✅ Решено'} as const;
const relative=(days:number,hours=0)=>new Date(Date.now()+(days*24+hours)*3600000).toISOString();
export function seededCases():DemoCase[]{return [
 {number:42,locationIds:['017','020'],createdAt:relative(-2),status:'resolved',takenAt:relative(-2,1),resolvedAt:relative(-2,2),updates:[{id:1,locationIds:['020'],comment:'Уточнить условия по второй точке',createdAt:relative(-2,1.2),acceptedAt:relative(-2,1.5)}]},
 {number:43,locationIds:['001'],createdAt:relative(-1),status:'working',takenAt:relative(-1,0.1),updates:[{id:1,locationIds:['002'],comment:'Интересует также соседняя локация',createdAt:relative(-1,0.2)}]},
 {number:44,locationIds:['033'],createdAt:relative(0,-1),status:'received',updates:[]},
 ];}
export const MAX_UPDATES=2;
export const MAX_COMPARE=3;
export const MAP_BASE='https://yandex.ru/maps/';
export function mapLink(l:Location){if(l.mapUrl)return l.mapUrl;if(l.latitude===undefined||l.longitude===undefined)return MAP_BASE;const q=new URLSearchParams({ll:`${l.longitude},${l.latitude}`,z:'15',pt:`${l.longitude},${l.latitude},pm2rdm`});return `${MAP_BASE}?${q.toString()}`;}
export function normalize(s:string){return s.toLowerCase().replace(/ё/g,'е').replace(/метро\s+/g,'').trim();}
export function prettyDate(s:string){return new Intl.DateTimeFormat('ru-RU',{day:'2-digit',month:'short',hour:'2-digit',minute:'2-digit',timeZone:'Europe/Moscow'}).format(new Date(s))+' МСК';}
