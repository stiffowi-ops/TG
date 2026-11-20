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
          platform:'android', initData:'layout-test', isFullscreen:${fullscreen}, isExpanded:true,
          isVersionAtLeast:()=>true, ready(){}, expand(){}, setHeaderColor(){},
          isVerticalSwipesEnabled:true,
          disableVerticalSwipes(){this.isVerticalSwipesEnabled=false},
          enableVerticalSwipes(){this.isVerticalSwipesEnabled=true}
        }};`
      }));
      await page.route('**/api/me', route => route.fulfill({json:{
        user:{id:1}, admin:fullscreen, favorites:[], cases:[]
      }}));
      await page.route('**/api/locations/view', route => route.fulfill({json:{ok:true}}));
      const station = {id:'001', title:'Метро Остров Мечты', area:'Москва', category:'Метро',
        latitude:55.69218672, longitude:37.66859046};
      const town = {...station,id:'036',title:'Ивантеевка',area:'Другие локации',category:'Другие локации'};
      await page.route('**/api/locations', route => route.fulfill({json:{locations:[station,town]}}));
      await page.goto(target);
      const nav = page.getByRole('navigation',{name:'Основная навигация'});
      const home = page.locator('.home-viewport');
      async function checkActiveButton(){
        assert.equal(await nav.getByRole('button').count(),fullscreen?5:4);
        await page.waitForFunction(()=>{
          const button=document.querySelector('.bottom-navigation [aria-current="page"]');
          return button&&Number(getComputedStyle(button,'::before').opacity)>.99;
        });
        const metrics = await nav.locator('[aria-current="page"]').evaluate(button => {
          const box=button.getBoundingClientRect();
          const icon=button.querySelector('svg').getBoundingClientRect();
          const label=button.querySelector('span:not(.nav-count)').getBoundingClientRect();
          const highlight=getComputedStyle(button,'::before');
          const width=parseFloat(highlight.width);
          const translation=new DOMMatrix(highlight.transform).m41;
          const highlightCenter=parseFloat(highlight.left)+translation+width/2;
          const center=rect=>rect.left+rect.width/2;
          return {opacity:highlight.opacity,
            iconOffset:center(icon)-center(box),
            labelOffset:center(label)-center(box),
            highlightOffset:highlightCenter-box.width/2};
        });
        assert.ok(Number(metrics.opacity)>.99,'Highlight not visible: '+JSON.stringify(metrics));
        for(const offset of [metrics.iconOffset,metrics.labelOffset,metrics.highlightOffset])
          assert.ok(Math.abs(offset)<1,'Off-center active highlight: '+JSON.stringify(metrics));
      }
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
          page:document.documentElement.scrollHeight-document.documentElement.clientHeight,
          boxes:Object.fromEntries(['.home-viewport','.landing-home','.landing-hero','.landing-copy','.landing-scene','.landing-shortcuts'].map(selector=>{
            const node=document.querySelector(selector), rect=node.getBoundingClientRect();
            return [selector,[rect.width,rect.height,getComputedStyle(node).minHeight,getComputedStyle(node).display]];
          }))
        }));
        assert.ok(scroll.main<=1 && scroll.page<=1,'Home scrolls '+JSON.stringify(scroll));
        const mainBox = await home.boundingBox();
        for (const button of await home.getByRole('button').all()) {
          const b = await button.boundingBox();
          assert.ok(b && b.y>=mainBox.y && b.y+b.height<=mainBox.y+mainBox.height+1,'Clipped home button');
        }
        assert.equal(await page.evaluate(()=>Telegram.WebApp.isVerticalSwipesEnabled),false);
      }
      // Compare the rendered letters, not the width of their containing element.
      const brandMetrics = () => page.locator('.site-header__wordmark').evaluate(mark =>
        [mark,mark.querySelector('small')].map(el => {
          const text = document.createRange();
          text.selectNodeContents(el.firstChild);
          const rect = text.getBoundingClientRect();
          const style = getComputedStyle(el);
          return {center:Math.round(rect.x+rect.width/2), top:Math.round(rect.y),
            font:style.fontSize, family:style.fontFamily, weight:style.fontWeight,
            tracking:style.letterSpacing, color:style.color};
        })
      );
      // The SDK may deliver insets after first paint and change its live height while expanding.
      await viewport(height,fullscreen?24:0,fullscreen?48:0,fullscreen?20:0);
      await homeFits();
      const homeBrand = await brandMetrics();
      const firstNav = await nav.boundingBox();
      assert.equal(firstNav.y + firstNav.height, height - (fullscreen?20:0));
      await page.evaluate(()=>document.documentElement.style.setProperty('--tg-viewport-height','418px'));
      assert.deepEqual(await nav.boundingBox(),firstNav);

      await home.getByRole('button',{name:'Смотреть локации'}).tap();
      assert.deepEqual(await brandMetrics(),homeBrand,'List header differs from home');
      const back = page.locator('.catalog-page-heading__back');
      assert.ok((await back.boundingBox()).y >= (fullscreen?72:0));
      const catalogBackBox = await back.boundingBox();
      const catalogHeaderBox = await page.locator('.catalog-page .site-header__inner').boundingBox();
      const cards = page.locator('.catalog-location-grid article');
      const categories = page.getByRole('group',{name:'Категории локаций'});
      await cards.filter({hasText:town.title}).waitFor();
      await categories.getByRole('button',{name:'У метро',exact:true}).tap();
      assert.deepEqual(await cards.locator('.location-card__title').allTextContents(),[station.title]);
      await page.setViewportSize({width:320,height});
      await page.getByRole('button',{name:'Настроить фильтры'}).tap();
      const sheet = page.getByRole('dialog',{name:'Фильтры локаций'});
      await sheet.getByRole('button',{name:'Без привязки к метро',exact:true}).tap();
      for(const group of await categories.all()){
        const fits=await group.getByRole('button').evaluateAll(buttons=>buttons.every(button=>{
          const label=button.querySelector('span').getBoundingClientRect();
          const bounds=button.getBoundingClientRect();
          return bounds.left>=0 && bounds.right<=innerWidth && label.left>=bounds.left && label.right<=bounds.right;
        }));
        assert.ok(fits,'Category label extends outside its button on a 320px screen');
      }
      await page.getByRole('button',{name:'Закрыть фильтры'}).tap();
      assert.deepEqual(await cards.locator('.location-card__title').allTextContents(),[town.title]);
      await categories.getByRole('button',{name:'Все',exact:true}).tap();
      await page.setViewportSize({width:390,height});
      await cards.filter({hasText:town.title}).waitFor();
      const cardLines = async card => (await card.innerText()).split(/\n+/).map(line=>line.trim()).filter(Boolean);
      assert.deepEqual(await cardLines(cards.first()),[station.title,station.area,'На карте']);
      assert.deepEqual(await cardLines(cards.last()),[town.title,'На карте']);
      await cards.last().getByRole('button',{name:town.title,exact:true}).tap();
      const details = page.locator('.detail-dialog');
      assert.equal(await details.getByText(town.category,{exact:true}).count(),0);
      assert.ok(!(await details.innerText()).includes('№'+town.id));
      await page.locator('.detail-dialog .dialog-close').tap();
      await back.tap();
      await nav.getByRole('button',{name:'Карта'}).tap();
      await checkActiveButton();
      assert.deepEqual(await brandMetrics(),homeBrand,'Map header differs from home');
      await page.locator('.catalog-page-heading__favorite').tap();
      await page.getByRole('heading',{name:/Избранное/}).waitFor();
      const favoriteBack = page.locator('.favorites-page-header .catalog-page-heading__back');
      const favoriteBackBox = await favoriteBack.boundingBox();
      const favoriteHeaderBox = await page.locator('.favorites-page-header .site-header__inner').boundingBox();
      for(const key of ['x','y','width','height']){
        assert.ok(Math.abs(favoriteBackBox[key]-catalogBackBox[key])<1,
          'Favorites back arrow differs from catalog '+key+': '+JSON.stringify({favorites:favoriteBackBox,catalog:catalogBackBox}));
      }
      for(const key of ['x','y','width','height']){
        assert.ok(Math.abs(favoriteHeaderBox[key]-catalogHeaderBox[key])<1,
          'Favorites header differs from catalog '+key);
      }
      const favoriteBrand = page.locator('.site-header__wordmark');
      const favoriteHeader = await page.evaluate(() => {
        const header = document.querySelector('.site-header__inner').getBoundingClientRect();
        const brand = document.querySelector('.site-header__wordmark').getBoundingClientRect();
        const left = document.querySelector('.catalog-page-heading__back').getBoundingClientRect();
        const right = document.querySelector('.catalog-page-heading__spacer').getBoundingClientRect();
        return {headerCenter:header.left+header.width/2,brandCenter:brand.left+brand.width/2,
          leftGap:left.left-header.left,rightGap:header.right-right.right,mirror:Math.abs(left.width-right.width)};
      });
      assert.ok(Math.abs(favoriteHeader.headerCenter-favoriteHeader.brandCenter)<1,'Favorites logo not centered');
      assert.ok(favoriteHeader.mirror<1 && Math.abs(favoriteHeader.leftGap-favoriteHeader.rightGap)<1,'Favorites arrows are not symmetric');
      assert.equal(await favoriteBack.getAttribute('aria-label'),'На главную');
      assert.ok(await favoriteBrand.isVisible());
      await favoriteBack.tap();
      await home.waitFor();
      await nav.getByRole('button',{name:'Карта'}).tap();
      await back.tap();
      await nav.getByRole('button',{name:'Мои заявки'}).tap();
      await page.getByRole('heading',{name:'Мои заявки'}).waitFor();
      const requestsBack = page.locator('.requests-page-header .catalog-page-heading__back');
      const requestsBackBox = await requestsBack.boundingBox();
      const requestsHeaderBox = await page.locator('.requests-page-header .site-header__inner').boundingBox();
      for(const key of ['x','y','width','height']){
        assert.ok(Math.abs(requestsBackBox[key]-catalogBackBox[key])<1,
          'Requests back arrow differs from catalog '+key+': '+JSON.stringify({requests:requestsBackBox,catalog:catalogBackBox}));
        assert.ok(Math.abs(requestsHeaderBox[key]-catalogHeaderBox[key])<1,
          'Requests header differs from catalog '+key);
      }
      assert.deepEqual(await brandMetrics(),homeBrand,'Requests header differs from home');
      await page.getByRole('button',{name:'Новая заявка'}).tap();
      await page.getByRole('dialog').getByRole('button',{name:'Закрыть',exact:true}).tap();
      await nav.getByRole('button',{name:'Избранное'}).tap();
      await page.locator('.catalog-page-heading__back').tap();
      await nav.getByRole('button',{name:'Мои заявки'}).tap();
      await page.locator('.requests-page-header .catalog-page-heading__back').tap();
      await home.waitFor();

      await page.setViewportSize({width:320,height:568});
      await viewport(568,fullscreen?24:0,fullscreen?48:0,fullscreen?16:0);
      await homeFits();
      await checkActiveButton();
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
