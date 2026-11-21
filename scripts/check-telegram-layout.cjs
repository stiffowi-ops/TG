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
          openTelegramLink(url){window.sharedReferral=url},
          isVerticalSwipesEnabled:true,
          disableVerticalSwipes(){this.isVerticalSwipesEnabled=false},
          enableVerticalSwipes(){this.isVerticalSwipesEnabled=true}
        }};`
      }));
      let grantedDiscount=0;
      let discountPhase='available';
      let mockCases=[];
      const discountRecord={id:701,userId:1,name:'Тестовый клиент',percent:15,createdAt:'2026-11-21T09:00:00Z'};
      const mockReferral='ref_ABCDEFGHIJKLMNOP';
      await page.route('**/api/me', route => route.fulfill({json:{
        user:{id:1,firstName:'Тест'}, admin:fullscreen, favorites:[], cases:mockCases,
        referralCode:mockReferral,
        discountWallet:discountPhase==='pending'
         ?{percent:0,offer:'',history:[{...discountRecord,status:'pending'}]}
         :discountPhase==='used'
          ?{percent:0,offer:'',history:[{...discountRecord,status:'used',approvedAt:'2026-11-21T09:15:00Z'}]}
          :{percent:grantedDiscount,offer:grantedDiscount?'m1':'',history:[]},
        profile:{userId:1,name:'Тестовый клиент',phone:'+79990000000',kind:'existing',company:'',discountPercent:grantedDiscount}
      }}));
      await page.route('**/api/referral', route => route.fulfill({json:{
        code:'ref_MNOPQRSTUVWXYZab',link:'https://t.me/TestLocationBot?startapp=ref_MNOPQRSTUVWXYZab',
        pendingCode:mockReferral,pending:1,awarded:0
      }}));
      await page.route('**/api/referral/validate', route => {
        const valid=route.request().postDataJSON().code===mockReferral;
        return route.fulfill({status:valid?200:400,json:valid?{valid:true,code:mockReferral}:{error:'Реферальная ссылка не найдена'}});
      });
      await page.route('**/api/locations/view', route => route.fulfill({json:{ok:true}}));
      // Admin screens use fixed, non-personal fixture data for reproducible visual captures.
      await page.route('**/api/admin/profiles', route => route.fulfill({json:{profiles:[
        {userId:100001,name:'Мария Смирнова',phone:'+79991112233',kind:'existing',company:'Город',discountPercent:10,manualDiscountPercent:0,referralDiscountPercent:10},
        {userId:100002,name:'Александр Иванов',phone:'+79994445566',kind:'new',company:'Новые точки',discountPercent:0,manualDiscountPercent:0,referralDiscountPercent:0}
      ]}}));
      await page.route('**/api/admin/referrals', route => route.fulfill({json:{referrals:[
        {inviteeId:100002,inviterId:100001,caseNumber:101,inviterName:'Мария Смирнова',inviteeName:'Александр Иванов',status:'pending',createdAt:'2026-11-20T12:30:00Z',ready:true}
      ]}}));
      await page.route('**/api/admin/cases', route => route.fulfill({json:{cases:[
        {number:101,locationIds:['001'],name:'Александр Иванов',createdAt:'2026-11-20T12:30:00Z',status:'working',takenAt:'2026-11-20T13:00:00Z',updates:[]}
      ]}}));
      await page.route('**/api/admin/discount-requests*', route => {
        if(new URL(route.request().url()).searchParams.has('summary'))return route.fulfill({json:{count:1,latest:501}});
        return route.fulfill({json:[{id:501,userId:100001,name:'Мария Смирнова',percent:10,status:'pending',createdAt:'2026-11-21T08:00:00Z'}]});
      });
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
      await sheet.getByRole('button',{name:'Другие',exact:true}).tap();
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
      await nav.getByRole('button',{name:'Профиль'}).tap();
      await page.getByRole('heading',{name:'Мой профиль'}).waitFor();
      assert.ok(await page.getByRole('button',{name:/Данные для заявок/}).isVisible());
      const referral=page.locator('.profile-referral');
      assert.ok(await referral.isVisible());
      assert.match(await referral.getAttribute('aria-label'), /Пригласить новых клиентов/);
      await referral.locator('img').evaluateAll(images=>Promise.all(images.map(img=>img.decode())));
      const bannerBox=await referral.boundingBox();
      assert.ok(Math.abs(bannerBox.height-bannerBox.width*560/1934)<1,'Referral banner lost its reference proportions');
      if(fullscreen){
        for(const width of [320,390,600]){
          await page.setViewportSize({width,height});
          const box=await referral.boundingBox();
          assert.ok(Math.abs(box.height-box.width*560/1934)<1);
          await referral.screenshot({path:`screenshots/referral-banner-${width}.png`});
        }
        await page.setViewportSize({width:390,height});
      }
      assert.ok(await page.getByText('У вас нет скидок',{exact:true}).isVisible());
      assert.ok(await page.getByText('+7 ••• •••-••-00',{exact:true}).isVisible(),'Phone must be masked on the profile summary');
      assert.equal(await page.getByRole('button',{name:/Данные для заявок/}).count(),1,'Contacts must have one entry point');
      assert.equal(await page.locator('.profile-rewards__summary').getAttribute('aria-expanded'),'false','Rewards must start collapsed');
      const order=await page.evaluate(()=>['.profile-summary','.profile-cta--primary-action','.profile-shortcuts','.profile-referral-hub'].map(x=>document.querySelector(x).getBoundingClientRect().top));
      assert.ok(order.every((top,i)=>i===0||top>order[i-1]),'Primary action and requests must precede referrals: '+order);
      if(fullscreen)await page.screenshot({path:'screenshots/07-profile-overview.png'});
      await referral.locator('.profile-referral__cta').tap();
      assert.equal(new URL(await page.evaluate(()=>window.sharedReferral)).searchParams.get('url'),
        'https://t.me/TestLocationBot?startapp=ref_MNOPQRSTUVWXYZab');
      assert.ok(await page.getByRole('heading',{name:'Мой профиль'}).isVisible(),'Sharing must stay on the profile');
      await page.locator('.profile-rewards__summary').tap();
      assert.equal(await page.locator('.profile-rewards__summary').getAttribute('aria-expanded'),'true');
      if(fullscreen)await page.screenshot({path:'screenshots/16-rewards-expanded-empty.png'});
      await page.getByRole('button',{name:/Моя ссылка и история приглашений/}).tap();
      await page.getByText('Ваша реферальная ссылка').waitFor();
      if(fullscreen)await page.screenshot({path:'screenshots/08-referral-details.png'});
      assert.ok(await page.getByRole('button',{name:'Скопировать'}).isVisible());
      await page.getByRole('button',{name:'Поделиться'}).tap();
      assert.equal(new URL(await page.evaluate(()=>window.sharedReferral)).searchParams.get('url'),
        await page.getByLabel('Персональная реферальная ссылка').inputValue());
      await page.getByRole('button',{name:'Назад'}).tap();
      assert.ok(await page.getByRole('button',{name:'Подать заявку'}).isVisible());
      await page.getByRole('button',{name:'Подать заявку'}).tap();
      const form=page.getByRole('dialog');
      assert.equal(await form.getByPlaceholder('Вставьте ссылку или код').inputValue(),mockReferral);
      const refField=form.getByPlaceholder('Вставьте ссылку или код');
      await refField.fill('ref_invalid');
      await form.getByRole('button',{name:'Продолжить'}).tap();
      await form.getByRole('alert').waitFor();
      await refField.fill(mockReferral);
      await form.getByRole('button',{name:'Продолжить'}).tap();
      await form.getByText('Проверьте данные заявки').waitFor();
      await form.getByRole('button',{name:'Закрыть',exact:true}).tap();
      mockCases=[{number:101,locationIds:['001'],createdAt:'2026-11-21T08:00:00Z',status:'working',takenAt:'2026-11-21T08:15:00Z',updates:[]}];
      await nav.getByRole('button',{name:'Локации'}).tap();
      await nav.getByRole('button',{name:'Профиль'}).tap();
      await page.getByRole('button',{name:'Отследить заявку'}).waitFor();
      assert.equal(await page.getByRole('button',{name:'Подать заявку'}).count(),0,'Active request must replace the application CTA');
      assert.ok(await page.getByText(/Специалист взял в работу/).isVisible(),'Active request must show last activity');
      const activeOrder=await page.evaluate(()=>['.profile-summary','.profile-active-card','.profile-cta--primary-action','.profile-shortcuts','.profile-referral-hub'].map(x=>document.querySelector(x).getBoundingClientRect().top));
      assert.ok(activeOrder.every((top,i)=>i===0||top>activeOrder[i-1]),'Active request must precede referrals: '+activeOrder);
      if(fullscreen)await page.screenshot({path:'screenshots/09-active-request.png'});
      await page.getByRole('button',{name:'Отследить заявку'}).tap();
      await page.getByRole('heading',{name:'Заявка Метро Остров Мечты'}).waitFor();
      if(fullscreen)await page.screenshot({path:'screenshots/10-request-tracker.png'});
      await page.getByRole('button',{name:'Назад'}).tap();
      mockCases=[];
      await nav.getByRole('button',{name:'Избранное'}).tap();
      await page.locator('.catalog-page-heading__back').tap();
      grantedDiscount=15; // Simulate an administrator granting a discount while the app remains open.
      await nav.getByRole('button',{name:'Профиль'}).tap();
      await page.getByRole('heading',{name:'Мой профиль'}).waitFor();
      await page.getByText('Ваша скидка: 15%',{exact:true}).waitFor();
      assert.ok(await page.getByText('Скидка доступна',{exact:true}).isVisible());
      assert.equal(await page.getByText('Скидка 15%',{exact:true}).count(),0,'Discount must not appear in the contacts card');
      if(fullscreen)await page.locator('.profile-referral-hub').screenshot({path:'screenshots/18-discount-available-real.png'});
      assert.ok(await page.getByRole('button',{name:'Воспользоваться скидкой'}).isVisible(),'Discount redemption belongs to referral block');
      if(fullscreen)await page.screenshot({path:'screenshots/11-profile-discount.png'});
      await page.locator('.profile-rewards__summary').tap();
      if(fullscreen)await page.screenshot({path:'screenshots/17-profile-discount-expanded.png'});
      await page.getByRole('button',{name:/Моя ссылка и история приглашений/}).tap();
      assert.equal(await page.getByRole('button',{name:'Воспользоваться скидкой'}).count(),1,'The dedicated discount screen must offer redemption');
      await page.getByRole('button',{name:'Назад'}).tap();
      discountPhase='pending';
      await nav.getByRole('button',{name:'Локации'}).tap();
      await nav.getByRole('button',{name:'Профиль'}).tap();
      await page.getByText('Ожидает подтверждения',{exact:true}).waitFor();
      assert.equal(await page.getByRole('button',{name:'Воспользоваться скидкой'}).count(),0,'Cannot redeem while confirmation is pending');
      assert.ok(await page.locator('.profile-rewards--pending').isVisible());
      if(fullscreen)await page.locator('.profile-referral-hub').screenshot({path:'screenshots/19-discount-pending-real.png'});
      await page.locator('.profile-rewards__summary').tap();
      assert.ok(await page.getByText('Запрос на использование скидки отправлен. Мы сообщим о результате.').isVisible());
      assert.equal(await page.getByText('Подтверждение',{exact:true}).count(),1);
      if(fullscreen)await page.locator('.profile-referral-hub').screenshot({path:'screenshots/20-discount-pending-expanded-real.png'});
      discountPhase='used';
      await nav.getByRole('button',{name:'Локации'}).tap();
      await nav.getByRole('button',{name:'Профиль'}).tap();
      await page.getByText('Скидка использована',{exact:true}).waitFor();
      assert.ok(await page.getByText('Применение скидки 15% подтверждено',{exact:true}).isVisible());
      assert.equal(await page.getByRole('button',{name:'Воспользоваться скидкой'}).count(),0);
      if(fullscreen)await page.locator('.profile-referral-hub').screenshot({path:'screenshots/21-discount-used-real.png'});
      await page.locator('.profile-rewards__summary').tap();
      if(fullscreen)await page.locator('.profile-referral-hub').screenshot({path:'screenshots/22-discount-used-expanded-real.png'});
      if(fullscreen){
        const manage=nav.getByRole('button',{name:'Управление'});
        await manage.tap();
        await page.getByRole('heading',{name:'Управление'}).waitFor();
        await page.screenshot({path:'screenshots/12-admin-dashboard.png'});
        await page.locator('.admin-page').getByRole('button',{name:/Клиенты/}).tap();
        await page.getByText('Мария Смирнова').first().waitFor();
        await page.screenshot({path:'screenshots/13-admin-clients.png'});
        await page.getByRole('button',{name:'Назад в управление'}).tap();
        await page.locator('.admin-page').getByRole('button',{name:/Начисления/}).tap();
        await page.getByText('Пригласил').waitFor();
        await page.screenshot({path:'screenshots/14-admin-referrals.png'});
        await page.getByRole('button',{name:'Назад в управление'}).tap();
        await page.locator('.admin-page').getByRole('button',{name:/Запросы на скидку/}).tap();
        await page.getByRole('button',{name:'Согласовать'}).waitFor();
        await page.screenshot({path:'screenshots/15-admin-discounts.png'});
      }
      await nav.getByRole('button',{name:'Локации'}).tap();
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
