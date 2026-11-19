const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
(async () => {
  const target=process.env.APP_URL || 'http://127.0.0.1:8080/';
  fs.mkdirSync('screenshots',{recursive:true});
  const browser=await chromium.launch({headless:true,args:['--no-sandbox','--enable-unsafe-swiftshader']});
  const context=await browser.newContext({viewport:{width:390,height:844},deviceScaleFactor:2,isMobile:true,hasTouch:true,locale:'ru-RU'});
  const page=await context.newPage();
  page.on('pageerror',e=>console.error('SCREENSHOT_JS_ERROR:',e.message));
  await page.goto(target,{waitUntil:'networkidle',timeout:40000});
  await page.locator('.landing-home').waitFor();
  await page.locator('.landing-illustration').evaluate(async img=>{if(img instanceof HTMLImageElement && !img.complete) await new Promise(resolve=>{img.onload=resolve;img.onerror=resolve;});});
  const heroQuality=await page.locator('.landing-illustration').evaluate(img=>({loaded:img instanceof HTMLImageElement && img.naturalWidth>0,width:(img instanceof HTMLImageElement?img.naturalWidth:0),currentSrc:(img instanceof HTMLImageElement?img.currentSrc:'')}));
  if(!heroQuality.loaded||heroQuality.width<700)throw Error('Hero illustration did not load at 2x quality: '+JSON.stringify(heroQuality));
  await page.getByRole('button',{name:'Большое разнообразие локаций'}).count().then(n=>{if(n!==1)throw Error('Shortcut missing')});
  await page.screenshot({path:'screenshots/01-home-real.png'});
  await page.getByRole('button',{name:/Смотреть локации/}).click();
  await page.locator('.catalog-page').waitFor();
  await page.screenshot({path:'screenshots/02-catalog-real.png'});
  await page.locator('.catalog-view-switcher').getByRole('button',{name:'Карта'}).click();
  await page.locator('.geo-map .maplibregl-map').waitFor({timeout:20000});
  const pins=page.locator('.geo-pin-container');
  await pins.first().waitFor({timeout:20000});
  // Some real locations share coordinates; DOM click verifies the MapLibre click handler
  // without another marker occluding the tested one in the screenshot viewport.
  await pins.first().evaluate(marker=>marker.click());
  await page.locator('.geo-map-preview').waitFor();
  await page.screenshot({path:'screenshots/05-map-selected-real.png'});
  // Attribution must retain OpenStreetMap copyright without unwanted map engine branding.
  const attribution=await page.locator('.geo-map__attribution').innerText();
  if(!attribution.includes('OpenStreetMap'))throw Error('Missing OSM attribution: '+attribution);
  // Attribution moves to the upper corner so it does not cover the selected location card.
  const mapLayers = await page.evaluate(()=>{
    const rect = selector=>document.querySelector(selector).getBoundingClientRect();
    const credit=rect('.geo-map__attribution'),preview=rect('.geo-map-preview');
    const overlap=!(credit.right<=preview.left||credit.left>=preview.right||credit.bottom<=preview.top||credit.top>=preview.bottom);
    return {overlap,creditTop:credit.top,previewTop:preview.top};
  });
  if(mapLayers.overlap)throw Error('OpenStreetMap attribution overlaps the preview: '+JSON.stringify(mapLayers));
  if(await page.locator('.geo-map .leaflet-container').count())throw Error('Legacy map engine remains in the DOM');
  await page.locator('.geo-map-preview__title-button').click();
  const detailOverlay=page.locator('.detail-dialog');
  await detailOverlay.waitFor();
  const modalAboveMap=await page.evaluate(()=>{
    const dialog=document.querySelector('.detail-dialog').getBoundingClientRect();
    const x=dialog.left+dialog.width/2,y=dialog.top+Math.min(90,dialog.height/2);
    return Boolean(document.elementFromPoint(x,y)?.closest('.detail-dialog'));
  });
  if(!modalAboveMap)throw Error('Location modal is hidden below map controls');
  await detailOverlay.getByRole('button',{name:'Закрыть'}).click();
  await page.locator('.geo-map-preview').getByRole('button',{name:/Открыть на карте/}).click();
  await page.locator('.yandex-location-viewer iframe').waitFor();
  const widget=await page.locator('.yandex-location-viewer iframe').getAttribute('src');
  if(!widget?.includes('/map-widget/v1/')||!widget.includes('pt='))throw Error('Point widget URL missing: '+widget);
  const viewerAboveMap=await page.evaluate(()=>{
    const element=document.querySelector('.yandex-location-viewer__frame');
    if(!element)return false;
    const rect=element.getBoundingClientRect();
    return document.elementFromPoint(rect.x+rect.width/2,rect.y+rect.height/2)?.closest('.yandex-location-viewer')!==null;
  });
  if(!viewerAboveMap)throw Error('Map viewer is hidden below map overlays');
  await page.waitForTimeout(3500);
  await page.screenshot({path:'screenshots/06-yandex-point-inside-app.png'});
  await page.getByRole('button',{name:'Вернуться в каталог'}).click();
  await page.locator('.geo-map-preview').waitFor();
  await page.locator('.catalog-view-switcher').getByRole('button',{name:'Список'}).click();
  await page.locator('.catalog-location-grid .location-card__map-link').first().click();
  await page.locator('.yandex-location-viewer iframe').waitFor();
  await page.getByRole('button',{name:'Вернуться в каталог'}).click();
  await page.locator('.catalog-location-grid').waitFor();
  await page.locator('.catalog-location-grid').waitFor();

  await page.locator('.main-container').evaluate(el=>{el.scrollTop=el.scrollHeight;});
  await page.waitForTimeout(300);
  await page.screenshot({path:'screenshots/03-catalog-bottom-real.png'});
  await page.getByRole('button',{name:/Избранное/}).first().click();
  await page.locator('.tab-section').waitFor();
  await page.screenshot({path:'screenshots/04-favorites-real.png'});
  await page.getByRole('button',{name:/Локации/}).first().click();
  await page.locator('.landing-home').waitFor();
  const info=await page.evaluate(()=>{
    const m=document.querySelector('.main-container'),n=document.querySelector('.bottom-nav'),h=document.querySelector('.app-header');
    return {width:innerWidth,height:innerHeight,documentScrollHeight:document.documentElement.scrollHeight,mainHeight:m.clientHeight,mainScrollHeight:m.scrollHeight,headerHeight:h.getBoundingClientRect().height,navHeight:n.getBoundingClientRect().height,heroImageLoaded:document.querySelector('.landing-illustration')?.complete};
  });
  fs.writeFileSync('screenshots/layout-metrics.json',JSON.stringify(info,null,2));
  console.log('CAPTURE:',JSON.stringify(info));
  await require('./check-map-photos.cjs')(context,target);
  await require('./check-telegram-layout.cjs')(context,target);
  await require('./check-motion.cjs')(context,target);
  await browser.close();
})().catch(e=>{console.error(e);process.exit(1)});
