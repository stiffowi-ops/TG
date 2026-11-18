// Exercise a cold catalog and Telegram's overlay with predictable network timing.
const assert = require('node:assert/strict');

module.exports = async function checkMapPhotos(context, target) {
  const page = await context.newPage();
  try {
    await page.route('**/telegram-web-app.js*', route => route.fulfill({
      contentType: 'application/javascript',
      body: `
        const listeners = new Set();
        const back = {
          isVisible: false,
          onClick: fn => listeners.add(fn), offClick: fn => listeners.delete(fn),
          show() { this.isVisible = true }, hide() { this.isVisible = false },
          press() { for (const fn of listeners) fn() }
        };
        window.Telegram = {WebApp: {initData: 'test', isFullscreen: true,
          ready() {}, expand() {}, BackButton: back, contentSafeAreaInset: {top: 50}}};
        document.documentElement.style.setProperty('--tg-safe-area-inset-top', '24px');
        document.documentElement.style.setProperty('--tg-content-safe-area-inset-top', '50px');
      `
    }));
    await page.route('**/api/me', route => route.fulfill({json: {user: {id: 1}, favorites: []}}));
    await page.route('**/api/locations/view', route => route.fulfill({json: {ok: true}}));
    await page.route('https://yandex.ru/map-widget/**', route => route.fulfill({body: '<html></html>', contentType: 'text/html'}));

    const location = {id:'001', title:'Метро Остров Мечты', area:'Москва', category:'Метро',
      latitude:55.69218672, longitude:37.66859046};
    const photos = [1, 2, 3].map(i => '/location-photos/test-' + i + '.png');
    const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4P4EBAAQhAZBKIMbQAAAAAElFTkSuQmCC', 'base64');
    await page.route('**/location-photos/test-*.png', route => route.fulfill({contentType:'image/png', body:png}));
    let photosReady = false;
    await page.route('**/api/locations', route => route.fulfill({json: {
      locations: [{...location, imageUrls: photosReady ? photos : []}], photosSyncing: !photosReady
    }}));

    await page.goto(target);
    await page.getByRole('button',{name:/Смотреть локации/}).click();
    const switcher = page.locator('.catalog-view-switcher');
    await switcher.getByRole('button',{name:'Карта'}).click();
    const pin = page.getByRole('button',{name:'Выбрать локацию '+location.title});
    await pin.tap();
    const oldPin = await pin.elementHandle();
    const preview = page.locator('.geo-map-preview');
    await preview.locator('.location-gallery__fallback').waitFor();
    photosReady = true;
    const photo = preview.locator('.location-gallery__image');
    await photo.waitFor({timeout:35000});
    await photo.evaluate(img => img.decode());
    assert.equal(await oldPin.evaluate(el => el.isConnected), true);
    await preview.getByRole('button',{name:'Следующее фото'}).tap();
    assert.equal(await photo.getAttribute('src'), photos[1]);
    await photo.evaluate(img => img.decode());

    // Emulate the part of the WebView that intercepts touches above its content.
    await page.evaluate(() => {
      const toolbar = document.createElement('div');
      toolbar.style.cssText = 'position:fixed;inset:0 0 auto;height:74px;z-index:2147483647';
      document.body.append(toolbar);
    });
    const viewer = page.getByRole('dialog',{name:/Яндекс Карты/});
    const open = preview.getByRole('button',{name:'Открыть на карте'});
    for (const exit of ['Вернуться в каталог', 'Закрыть карту']) {
      await open.tap();
      const button = viewer.getByRole('button',{name:exit});
      assert.ok((await button.boundingBox()).y >= 74);
      await button.tap();
      await preview.waitFor();
      assert.equal(await photo.getAttribute('src'), photos[1]);
    }
    await photo.tap();
    const details = page.locator('.detail-dialog');
    await details.getByRole('button',{name:'Следующее фото'}).tap();
    const detailPhoto = details.locator('.location-gallery__image');
    await detailPhoto.evaluate(img => img.decode());
    await details.getByRole('button',{name:/На Яндекс Карте/}).tap();
    await viewer.waitFor();
    assert.equal(await page.evaluate(() => Telegram.WebApp.BackButton.isVisible), true);
    await page.evaluate(() => Telegram.WebApp.BackButton.press());
    await details.waitFor();
    assert.equal(await detailPhoto.getAttribute('src'), photos[1]);
    assert.equal(await page.evaluate(() => Telegram.WebApp.BackButton.isVisible), false);
    console.log('Gallery refreshed on the selected marker; both map controls and Telegram Back returned to their origin.');
  } finally {
    await page.close();
  }
};
