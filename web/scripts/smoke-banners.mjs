// Optional live smoke check. Requires a running local API and Google Chrome.
// Unlike deterministic e2e tests this deliberately uses the real upstream cache.
import { chromium, expect } from '@playwright/test';

const base = process.env.BANNER_SMOKE_URL || 'http://127.0.0.1:8080';
const games = [
  ['genshin', 'Genshin Impact'],
  ['hsr', 'Honkai: Star Rail'],
  ['zzz', 'Zenless Zone Zero'],
  ['wuwa', 'Wuthering Waves'],
];
const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  for (const [label, viewport] of [
    ['desktop', { width: 1440, height: 1100 }],
    ['tablet', { width: 1000, height: 1000 }],
    ['phone', { width: 390, height: 844 }],
  ]) {
    const page = await browser.newPage({ viewport, timezoneId: 'Europe/Istanbul' });
    const errors = [];
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('requestfailed', (request) => console.log('Request failed:', request.url(), request.failure()));
    await page.goto(`${base}/banners`);
    for (const [id, name] of games) {
      await page.locator('.game-tabs').getByRole('button', { name }).click();
      await expect(page.locator('.schedule-status')).toBeVisible({ timeout: 20000 });
      const data = await (await page.request.get(`${base}/api/v1/banners?game=${id}`)).json();
      expect(data.game).toBe(id);
      const displayed = id === 'zzz' ? data.items.flatMap((banner) => {
        const characters = banner.items.filter((item) => item.kind === 'character' && item.rarity === 5);
        return banner.pool_family === 'limited_character' && characters.length > 1
          ? characters.map((item) => ({ ...banner, title: item.name, items: [item] }))
          : [banner];
      }) : data.items;
      const current = displayed.filter((b) => b.status === 'current');
      expect(current.length).toBeGreaterThan(0);
      await expect(page.locator('.wish-banner h2')).toHaveText(current[0].title);
      const art = page.locator('.wish-art');
      await expect(art).toBeVisible();
      await page.screenshot({ path: `test-results/live-${id}-${label}.png`, fullPage: true });
      await expect.poll(() => art.evaluate((img) => img.complete && img.naturalWidth > 0), {
        timeout: 20000,
      }).toBeTruthy();
      if (id !== 'wuwa') {
        await expect(page.locator('.wish-banner')).toHaveClass(/image-splash/);
        const dimensions = await art.evaluate((img) => ({ width: img.naturalWidth, height: img.naturalHeight }));
        expect(Math.max(dimensions.width, dimensions.height)).toBeGreaterThanOrEqual(1000);
        console.log('Full art:', id, dimensions);
      }
      if (id === 'zzz') {
        await expect(page.getByLabel('Арт персонажа', { exact: true })).toHaveCount(0);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
      // Also exercise every current banner, including the longer weapon titles.
      const selectors = page.locator('.banner-selector button');
      for (let index = 0; index < current.length; index++) {
        await selectors.nth(index).click();
        await expect(page.locator('.wish-banner h2')).toHaveText(current[index].title);
        if (id === 'zzz' && current[index].pool_family === 'limited_character') {
          await expect(art).toHaveAttribute('alt', current[index].title);
          await expect(art).toHaveAttribute('src', current[index].items[0].art_url);
        }
        const copy = await page.locator('.wish-copy').boundingBox();
        const card = await page.locator('.wish-banner').boundingBox();
        expect(copy.y + copy.height).toBeLessThanOrEqual(card.y + card.height);
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
      }
      await selectors.first().click();
      if (id === 'wuwa') {
        const box = await page.locator('.wish-banner').boundingBox();
        const imageBox = await art.boundingBox();
        expect(Math.abs(box.width - imageBox.width)).toBeLessThanOrEqual(1);
        expect(Math.abs(box.height - imageBox.height)).toBeLessThanOrEqual(1);
        await expect(page.locator('.art-fallback-note')).toHaveCount(0);
      }
      await expect.poll(() => art.evaluate((img) => img.complete && img.naturalWidth > 0), { timeout: 20000 }).toBeTruthy();
      await page.locator('.wish-banner').screenshot({ path: `test-results/hero-${id}-${label}.png` });
      await page.screenshot({ path: `test-results/live-${id}-${label}.png`, fullPage: true });
      await page.getByRole('button', { name: 'Календарь', exact: true }).click();
      await expect(page.locator('.calendar-row')).toHaveCount(displayed.length);
      await page.screenshot({ path: `test-results/live-${id}-calendar-${label}.png`, fullPage: true });
      await page.getByRole('button', { name: 'Баннеры', exact: true }).click();
      console.log(JSON.stringify({ viewport: label, game: id, sourceCount: data.items.length, cardCount: displayed.length, stale: data.stale, image: 'loaded' }));
    }
    expect(errors).toEqual([]);
    await page.close();
  }
} finally {
  await browser.close();
}
