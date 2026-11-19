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
    await openCatalog();
    assert.equal(await page.locator('.catalog-view-content').evaluate(el=>getComputedStyle(el).animationDuration),'0.3s');
    await page.waitForSelector('[data-location-id="16"][data-reveal-pending]',{state:'attached'});
    await page.waitForFunction(()=>getComputedStyle(document.querySelector('[data-location-id="1"]')).opacity==='1');

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

    // Navigate again before the first entry animation would finish.
    for(const name of ['Избранное','Мои заявки','Локации']){
      await nav.getByRole('button',{name,exact:true}).evaluate(el=>el.click());
    }
    await openCatalog();
    await page.waitForSelector('[data-location-id="30"][data-reveal-pending]',{state:'attached'});
    assert.equal(await id(16).evaluate(el=>getComputedStyle(el).opacity),'1');

    await page.emulateMedia({reducedMotion:'reduce'});
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
