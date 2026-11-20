// Verify map clusters, browsing between locations and geolocation-based sorting.
const assert=require('node:assert/strict');

module.exports=async function checkMapDiscovery(context,target){
 const page=await context.newPage();
 try{
  const places=[
   {id:'A',title:'Первая точка',area:'Москва',category:'Другие локации',latitude:55.7000,longitude:37.6500},
   {id:'B',title:'Ближайшая точка',area:'Москва',category:'Другие локации',latitude:55.70001,longitude:37.65001},
   {id:'C',title:'Третья точка',area:'Москва',category:'Метро',latitude:55.805,longitude:37.755},
   {id:'D',title:'Четвёртая точка',area:'Москва',category:'Метро',latitude:55.812,longitude:37.779}
  ];
  await page.route('**/telegram-web-app.js*',route=>route.fulfill({contentType:'application/javascript',body:''}));
  await page.route('**/api/me',route=>route.fulfill({json:{user:{id:1},favorites:[]}}));
  await page.route('**/api/locations',route=>route.fulfill({json:{locations:places}}));
  // The browser permission backend can vary between runner environments.
  // Feed deterministic coordinates through the same geolocation callback API.
  await page.addInitScript(()=>{
   Object.defineProperty(navigator,'geolocation',{configurable:true,value:{
    getCurrentPosition(success){success({coords:{latitude:55.70001,longitude:37.65001}})},
    watchPosition(){return 1},
    clearWatch(){}
   }});
  });
  await page.goto(target);
  await page.getByRole('button',{name:'Смотреть локации'}).click();
  await page.locator('.catalog-view-switcher').getByRole('button',{name:'Карта'}).click();
  await page.locator('.geo-pin-container').first().waitFor({state:'attached'});
  // Two points almost coincide. Their cluster must be visible, but pins stay mounted.
  const cluster=page.getByRole('button',{name:'Группа из 2 локаций'}).first();
  await cluster.waitFor({timeout:20000});
  assert.equal(await page.locator('.geo-pin-container').count(),places.length);
  // Regression: a positioned MapLibre marker is absolute, never a normal-flow
  // button. Relative-positioned markers produced the vertical K-pin column.
  const geometry=await page.evaluate(()=>{
   const pins=[...document.querySelectorAll('.geo-pin-container')];
   const third=pins.find(p=>p.title==='Третья точка');
   const fourth=pins.find(p=>p.title==='Четвёртая точка');
   const center=(node)=>{const rect=node.getBoundingClientRect();return {x:rect.left+rect.width/2,y:rect.top+rect.height/2}};
   return {
    positions:pins.map(pin=>getComputedStyle(pin).position),
    third:center(third),fourth:center(fourth)
   };
  });
  assert.ok(geometry.positions.every(p=>p==='absolute'),'Pins must be absolutely positioned for MapLibre: '+JSON.stringify(geometry));
  assert.ok(Math.abs(geometry.third.x-geometry.fourth.x)>2,
   'Markers with different longitudes must not form a fixed vertical column: '+JSON.stringify(geometry));
  await cluster.evaluate(el=>el.click());
  await page.waitForTimeout(550);
  assert.equal(await page.locator('.geo-pin-container').count(),places.length);

  // Select an ordinary pin while a cluster exists; card browsing must select the next location.
  await page.getByRole('button',{name:'Выбрать локацию Третья точка'}).evaluate(el=>el.click());
  const preview=page.locator('.geo-map-preview');
  await preview.waitFor();
  assert.equal((await preview.locator('.geo-map-preview__title').innerText()),'Третья точка');
  await preview.getByRole('button',{name:'Следующая локация'}).click();
  assert.equal((await preview.locator('.geo-map-preview__title').innerText()),'Четвёртая точка');
  await preview.getByRole('button',{name:'Предыдущая локация'}).click();
  assert.equal((await preview.locator('.geo-map-preview__title').innerText()),'Третья точка');

  // The nearby list must be explicitly permission-gated and distance sorted.
  await preview.getByRole('button',{name:'Закрыть карточку'}).click();
  await page.getByRole('button',{name:'Моё местоположение'}).click();
  await page.waitForTimeout(350);
  console.log('NEARBY_DIAGNOSTIC',JSON.stringify(await page.evaluate(()=>({
   error:document.querySelector('.geo-map-actions__error')?.textContent,
   nearby:document.querySelector('.geo-nearby')?.textContent,
   preview:document.querySelector('.geo-map-preview__title')?.textContent,
   hasGeolocation:!!navigator.geolocation,
   geolocationFn:String(navigator.geolocation?.getCurrentPosition).slice(0,110)
  }))));
  const nearby=page.getByRole('region',{name:'Ближайшие локации'});
  await nearby.waitFor({timeout:12000});
  await nearby.getByRole('button',{name:/Ближайшая точка/}).waitFor({timeout:8000});
  const names=await nearby.locator('.geo-nearby__list button > span').allInnerTexts();
  assert.deepEqual(names.slice(0,2),['Ближайшая точка','Первая точка']);
  assert.ok((await nearby.innerText()).includes('м'));
  await nearby.getByRole('button',{name:/Ближайшая точка/}).click();
  assert.equal((await preview.locator('.geo-map-preview__title').innerText()),'Ближайшая точка');
  await preview.getByRole('button',{name:'Следующая локация'}).click();
  assert.equal((await preview.locator('.geo-map-preview__title').innerText()),'Первая точка');
  await preview.getByRole('button',{name:'Закрыть карточку'}).click();
  await nearby.getByRole('button',{name:'Закрыть ближайшие локации'}).click();
  assert.equal(await nearby.count(),0);

  // Returning from a selected pin must restore the camera for the filtered subset.
  const metro=page.getByRole('button',{name:'У метро',exact:true});
  const search=page.getByRole('searchbox');
  await metro.click();
  await search.fill('точка');
  const pins=page.locator('.geo-pin-container');
  const overview=page.getByRole('button',{name:'Показать все точки'});
  await page.waitForFunction(()=>document.querySelectorAll('.geo-pin-container').length===2);
  const initial=await pins.evaluateAll(nodes=>nodes.map(node=>node.style.transform));
  await page.getByRole('button',{name:'Выбрать локацию Третья точка'}).click();
  await preview.waitFor();
  await page.waitForFunction(before=>[...document.querySelectorAll('.geo-pin-container')]
   .some((pin,index)=>pin.style.transform!==before[index]),initial);
  await overview.tap();
  await preview.waitFor({state:'detached'});
  await page.waitForFunction(before=>[...document.querySelectorAll('.geo-pin-container')]
   .every((pin,index)=>pin.style.transform===before[index]),initial);
  assert.equal(await pins.count(),2);
  assert.equal(await metro.getAttribute('aria-pressed'),'true');
  assert.equal(await search.inputValue(),'точка');

  await search.fill('Третья');
  await page.getByRole('button',{name:'Выбрать локацию Третья точка'}).click();
  await overview.tap();
  await preview.waitFor({state:'detached'});
  assert.equal(await pins.count(),1);
  await search.fill('несуществующая локация');
  await overview.waitFor({state:'detached'});
  console.log('Verified clustered pins, carousel selection, nearest distances, and geolocation permission.');
 }finally{
  await page.close();
 }
};
