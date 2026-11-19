// Verify map clusters, browsing between locations and geolocation-based sorting.
const assert=require('node:assert/strict');

module.exports=async function checkMapDiscovery(context,target){
 const page=await context.newPage();
 const origin=new URL(target).origin;
 try{
  const places=[
   {id:'A',title:'Первая точка',area:'Москва',category:'Метро',latitude:55.7000,longitude:37.6500},
   {id:'B',title:'Ближайшая точка',area:'Москва',category:'Метро',latitude:55.70001,longitude:37.65001},
   {id:'C',title:'Третья точка',area:'Москва',category:'Метро',latitude:55.805,longitude:37.755},
   {id:'D',title:'Четвёртая точка',area:'Москва',category:'Метро',latitude:55.812,longitude:37.779}
  ];
  await page.route('**/telegram-web-app.js*',route=>route.fulfill({contentType:'application/javascript',body:''}));
  await page.route('**/api/me',route=>route.fulfill({json:{user:{id:1},favorites:[]}}));
  await page.route('**/api/locations',route=>route.fulfill({json:{locations:places}}));
  await context.grantPermissions(['geolocation'],{origin});
  await context.setGeolocation({latitude:55.70001,longitude:37.65001});
  await page.goto(target);
  await page.getByRole('button',{name:'Смотреть локации'}).click();
  await page.locator('.catalog-view-switcher').getByRole('button',{name:'Карта'}).click();
  await page.locator('.geo-pin-container').first().waitFor({state:'attached'});
  // Two points almost coincide. Their cluster must be visible, but pins stay mounted.
  const cluster=page.getByRole('button',{name:'Группа из 2 локаций'}).first();
  await cluster.waitFor({timeout:20000});
  assert.equal(await page.locator('.geo-pin-container').count(),places.length);
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
  console.log('Verified clustered pins, carousel selection, nearest distances, and geolocation permission.');
 }finally{
  await page.close();
  await context.clearPermissions();
 }
};
