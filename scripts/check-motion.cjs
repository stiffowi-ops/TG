const assert = require('node:assert/strict');

module.exports = async function checkMotion(context, target) {
  const page = await context.newPage();
  try {
    const locations = Array.from({length:30}, (_,i) => ({
      id:String(i+1), title:'Локация '+(i+1), area:'Москва', category:'Другие локации'
    }));
    await page.route('**/telegram-web-app.js*', r => r.fulfill({contentType:'application/javascript',body:''}));
    await page.route('**/api/locations', r => r.fulfill({json:{locations}}));
    await page.emulateMedia({reducedMotion:'no-preference'});
    await page.goto(target);
    const nav = page.getByRole('navigation',{name:'Основная навигация'});
    const openCatalog = () => page.getByRole('button',{name:'Смотреть локации'}).tap();
    const id = n => page.locator('[data-location-id="'+n+'"]');
    const scrollTo = card => card.evaluate(el=>el.scrollIntoView({block:'center',behavior:'instant'}));
    const nextPaint = () => page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
    async function checkButtonPress(reduced){
      const button=nav.getByRole('button',{name:'Локации',exact:true});
      const box=await button.boundingBox();
      await page.mouse.move(box.x+box.width/2,box.y+box.height/2);
      await page.mouse.down();
      try {
        await page.waitForFunction(reduced=>{
          const button=document.querySelector('.bottom-nav button:active');
          if(!button)return false;
          const style=getComputedStyle(button);
          return style.filter==='brightness(0.9)' &&
            (reduced?style.scale==='none':Math.abs(parseFloat(style.scale)-0.965)<0.001);
        },reduced);
      } finally {
        await page.mouse.move(0,0);
        await page.mouse.up();
      }
      await page.waitForFunction(()=>getComputedStyle(document.querySelector('.bottom-nav button')).scale==='none');
    }
    await checkButtonPress(false);
    await openCatalog();
    assert.equal(await page.locator('.catalog-view-content').evaluate(el=>getComputedStyle(el).animationDuration),'0.19s');
    await page.waitForSelector('[data-location-id="16"][data-reveal-pending]',{state:'attached'});
    assert.equal(await id(1).evaluate(el=>getComputedStyle(el).opacity),'1');

    // Capture actual opacity transitions, including any replay when scrolling back.
    await id(16).evaluate(card=>{
      window.reveals=0;
      card.addEventListener('transitionrun', e=>{
        if(e.target===card && e.propertyName==='opacity') window.reveals++;
      });
    });
    await scrollTo(id(16));
    await page.waitForFunction(()=>window.reveals===1 && getComputedStyle(document.querySelector('[data-location-id="16"]')).opacity==='1');
    await scrollTo(id(1));
    await nextPaint();
    await scrollTo(id(16));
    await nextPaint();
    assert.equal(await page.evaluate(()=>window.reveals),1);

    const scrollTop = () => page.locator('main').evaluate(el=>el.scrollTop);
    const catalogTop = await scrollTop();
    assert.ok(catalogTop>0);
    await nav.getByRole('button',{name:'Избранное',exact:true}).tap();
    await page.getByRole('button',{name:'Назад',exact:true}).tap();
    assert.ok(Math.abs(await scrollTop()-catalogTop)<=1,'Lost catalog scroll position');

    // Navigate again before the first entry animation would finish.
    for(const name of ['Избранное','Мои заявки','Локации']){
      await nav.getByRole('button',{name,exact:true}).evaluate(el=>el.click());
    }
    await openCatalog();
    await page.waitForSelector('[data-location-id="30"][data-reveal-pending]',{state:'attached'});
    assert.equal(await id(16).evaluate(el=>getComputedStyle(el).opacity),'1');

    await page.emulateMedia({reducedMotion:'reduce'});
    await checkButtonPress(true);
    assert.equal(await id(30).evaluate(el=>getComputedStyle(el).opacity),'1');
    await scrollTo(id(30));
    await nextPaint();
    assert.equal(await id(30).evaluate(el=>el.getAnimations().length),0);
    await nav.getByRole('button',{name:'Избранное',exact:true}).tap();
    assert.equal(await page.locator('.tab-section').evaluate(el=>getComputedStyle(el).animationName),'none');
    console.log('Verified section entry, one-time card reveal, rapid navigation and reduced motion.');
  } finally {
    await page.close();
  }
};
