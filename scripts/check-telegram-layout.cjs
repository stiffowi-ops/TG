const assert = require('node:assert/strict');

// Telegram can place the WebView underneath its toolbar or start it below the entire toolbar.
module.exports = async function checkTelegramLayout(context, target) {
  for (const fullscreen of [true, false]) {
    const page = await context.newPage();
    try {
      const height = fullscreen ? 844 : 752;
      await page.setViewportSize({width:390, height});
      const errors = [];
      page.on('pageerror', err => errors.push(err.message));
      await page.route('**/telegram-web-app.js*', route => route.fulfill({
        contentType:'application/javascript',
        body:`window.Telegram = {WebApp:{
          platform:'android', initData:'', isFullscreen:${fullscreen}, isExpanded:true,
          isVersionAtLeast:()=>true, ready(){}, expand(){}, setHeaderColor(){},
          isVerticalSwipesEnabled:true,
          disableVerticalSwipes(){this.isVerticalSwipesEnabled=false},
          enableVerticalSwipes(){this.isVerticalSwipesEnabled=true}
        }};`
      }));
      const station = {id:'001', title:'Метро Остров Мечты', area:'Москва', category:'Метро',
        latitude:55.69218672, longitude:37.66859046};
      await page.route('**/api/locations', route => route.fulfill({json:{locations:[station]}}));
      await page.goto(target);
      const nav = page.getByRole('navigation',{name:'Основная навигация'});
      const home = page.locator('.home-viewport');
      async function viewport(stableHeight, systemTop, toolbarTop, bottom, left = 0) {
        await page.evaluate(({stableHeight,systemTop,toolbarTop,bottom,left})=>{
          const css = document.documentElement.style;
          css.setProperty('--tg-viewport-stable-height',stableHeight+'px');
          css.setProperty('--tg-safe-area-inset-top',systemTop+'px');
          css.setProperty('--tg-content-safe-area-inset-top',toolbarTop+'px');
          css.setProperty('--tg-safe-area-inset-bottom',bottom+'px');
          css.setProperty('--tg-safe-area-inset-left',left+'px');
          const cover = document.getElementById('native-toolbar') || document.createElement('div');
          cover.id = 'native-toolbar';
          cover.style.cssText = 'position:fixed;inset:0 0 auto;z-index:2147483647;height:'+(systemTop+toolbarTop)+'px';
          document.body.append(cover);
        },{stableHeight,systemTop,toolbarTop,bottom,left});
      }
      async function homeFits() {
        await home.waitFor();
        const scroll = await home.evaluate(el => ({
          main:el.scrollHeight-el.clientHeight,
          page:document.documentElement.scrollHeight-document.documentElement.clientHeight
        }));
        assert.ok(scroll.main<=1 && scroll.page<=1,'Home scrolls '+JSON.stringify(scroll));
        const mainBox = await home.boundingBox();
        for (const button of await home.getByRole('button').all()) {
          const b = await button.boundingBox();
          assert.ok(b && b.y>=mainBox.y && b.y+b.height<=mainBox.y+mainBox.height+1,'Clipped home button');
        }
        assert.equal(await page.evaluate(()=>Telegram.WebApp.isVerticalSwipesEnabled),false);
      }
      // The SDK may deliver insets after first paint and change its live height while expanding.
      await viewport(height,fullscreen?24:0,fullscreen?48:0,fullscreen?20:0);
      await homeFits();
      const firstNav = await nav.boundingBox();
      assert.equal(firstNav.y + firstNav.height, height - (fullscreen?20:0));
      await page.evaluate(()=>document.documentElement.style.setProperty('--tg-viewport-height','418px'));
      assert.deepEqual(await nav.boundingBox(),firstNav);

      await home.getByRole('button',{name:'Смотреть локации'}).tap();
      const back = page.locator('.catalog-page-heading__back');
      assert.ok((await back.boundingBox()).y >= (fullscreen?72:0));
      await page.getByRole('button',{name:'Настроить фильтры'}).tap();
      await page.getByRole('button',{name:'Закрыть фильтры'}).tap();
      await page.locator('.location-card__title').first().tap();
      await page.locator('.detail-dialog .dialog-close').tap();
      await back.tap();
      await nav.getByRole('button',{name:'Карта'}).tap();
      await back.tap();
      await nav.getByRole('button',{name:'Мои заявки'}).tap();
      await page.getByRole('button',{name:'Новая заявка'}).tap();
      await page.getByRole('dialog').getByRole('button',{name:'Закрыть',exact:true}).tap();
      await nav.getByRole('button',{name:'Избранное'}).tap();
      await page.getByRole('button',{name:'На главную'}).tap();

      await page.setViewportSize({width:320,height:568});
      await viewport(568,fullscreen?24:0,fullscreen?48:0,fullscreen?16:0);
      await homeFits();
      await page.setViewportSize({width:780,height:390});
      await viewport(390,0,fullscreen?48:0,0,fullscreen?24:0);
      await homeFits();
      assert.deepEqual(errors,[]);
      console.log('Telegram layout and touch navigation passed:',fullscreen?'fullscreen':'expanded chat');
    } finally {
      await page.close();
    }
  }
};
