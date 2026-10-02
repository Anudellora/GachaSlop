import { test, expect, type Page } from '@playwright/test';

test('real Go server: SPA, empty collection, validation and background job', async ({
  page,
  request,
}) => {
  expect((await request.get('/readyz')).ok()).toBeTruthy();
  expect((await request.get('/api/v1/accounts')).ok()).toBeTruthy();
  expect((await request.get('/api/v1/unknown')).status()).toBe(404);
  await page.goto('/history');
  await expect(page.getByRole('heading', { name: 'У вашей истории будет свой дом' })).toBeVisible();
  await page.getByRole('button', { name: '+ Импорт истории' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Ссылка на историю круток').fill('not-a-url');
  await dialog.getByRole('button', { name: 'Загрузить историю' }).click();
  await expect(dialog.getByRole('alert')).toContainText('HTTPS');
  await dialog
    .getByLabel('Ссылка на историю круток')
    .fill('https://example.invalid/?authkey=FAKE_TEST_KEY');
  await dialog.getByRole('button', { name: 'Загрузить историю' }).click();
  await expect(dialog.getByRole('heading', { name: 'Не удалось завершить импорт' })).toBeVisible({
    timeout: 15_000,
  });
  expect(
    await page.evaluate(() =>
      JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }),
    ),
  ).not.toContain('FAKE_TEST_KEY');
  await dialog.getByRole('button', { name: 'Вставить новую ссылку' }).click();
  await expect(dialog.getByLabel('Ссылка на историю круток')).toHaveValue('');
  await page.keyboard.press('Escape');
  await expect(dialog).not.toBeVisible();
});

const account = {
  id: 'test-account',
  game: 'genshin',
  uid: '700000001',
  region: 'os_euro',
  language: 'ru-ru',
  timezone_offset: 1,
  created_at: '2026-09-14T12:00:00Z',
  updated_at: '2026-09-14T12:00:00Z',
};
const group = {
  pity_group: 'genshin:301',
  pool_family: 'limited_character',
  total: 1248,
  five_star_pity: 67,
  five_star_certainty: 'exact',
  four_star_pity: 3,
  four_star_certainty: 'exact',
  guarantee_state: 'unknown',
};
const pull = {
  id: 'pull-1',
  account_id: account.id,
  external_id: '1',
  gacha_type: '301',
  source_pool: '301',
  pool_family: 'limited_character',
  pity_group: 'genshin:301',
  name: 'Кэ Цин',
  item_type: 'Персонаж',
  rarity: 5,
  pulled_at: '2026-09-14 20:16:00',
  created_at: account.created_at,
};
const complete = {
  id: 'test-job',
  game: 'genshin',
  account_id: account.id,
  status: 'completed',
  fetched_count: 24,
  imported_count: 24,
  created_at: account.created_at,
};

async function populatedAPI(page: Page) {
  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url());
    let body: unknown;
    if (url.pathname.endsWith('/accounts')) body = { items: [account] };
    else if (url.pathname.endsWith('/pity'))
      body = {
        items: [
          group,
          {
            ...group,
            pity_group: 'genshin:302',
            pool_family: 'limited_weapon',
            five_star_pity: 28,
          },
          {
            ...group,
            pity_group: 'genshin:200',
            pool_family: 'standard',
            five_star_pity: 41,
            five_star_certainty: 'at_least',
          },
        ],
      };
    else if (url.pathname.endsWith('/summary'))
      body = { total: 1248, five_star_count: 18, four_star_count: 142, last_import: complete };
    else if (url.pathname.endsWith('/pulls'))
      body = url.searchParams.has('cursor')
        ? { items: [{ ...pull, id: 'pull-2', name: 'Сахароза', rarity: 4 }] }
        : { items: [pull], next_cursor: 'next-page' };
    else if (url.pathname.endsWith('/sync-jobs')) body = { ...complete, status: 'queued' };
    else body = complete;
    await route.fulfill({ json: body, status: route.request().method() === 'POST' ? 202 : 200 });
  });
}

test('populated overview, server filters, pagination, successful import and no overflow', async ({
  page,
}, info) => {
  await populatedAPI(page);
  await page.goto('/');
  await expect(page.getByText('67', { exact: true })).toBeVisible();
  await expect(page.getByText('≥ 41', { exact: true })).toBeVisible();
  await page.screenshot({ path: `test-results/overview-${info.project.name}.png`, fullPage: true });
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
  ).toBeTruthy();
  await page.getByRole('link', { name: 'История круток', exact: true }).click();
  const filtered = page.waitForRequest(
    (r) => r.url().includes('/pulls?') && r.url().includes('rarity=5'),
  );
  await page.getByRole('combobox', { name: 'Редкость', exact: true }).selectOption('5');
  await filtered;
  await expect(page.getByText('Кэ Цин', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Показать ещё 50' }).click();
  await expect(page.getByText('Сахароза', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Показать ещё 50' })).not.toBeVisible();
  await page.getByRole('button', { name: '+ Импорт истории' }).click();
  const dialog = page.getByRole('dialog');
  await dialog
    .getByLabel('Ссылка на историю круток')
    .fill('https://example.invalid/?authkey=FAKE_TEST_KEY');
  await dialog.getByRole('button', { name: 'Загрузить историю' }).click();
  await expect(dialog.getByRole('heading', { name: 'История на своём месте' })).toBeVisible();
  await dialog.getByRole('button', { name: 'К коллекции' }).click();
  await expect(page).toHaveURL('/');
  await page.getByRole('button', { name: 'Honkai: Star Rail' }).click();
  await expect(page.getByRole('heading', { name: 'У вашей истории будет свой дом' })).toBeVisible();
  await expect(page.getByText('67', { exact: true })).not.toBeVisible();
});

test('background import survives page reload without reposting credentials', async ({ page }) => {
  let done = false;
  let posts = 0;
  await populatedAPI(page);
  await page.route('**/api/v1/sync-jobs', async (route) => {
    posts++;
    await route.fulfill({ status: 202, json: { ...complete, status: 'queued' } });
  });
  await page.route('**/api/v1/sync-jobs/test-job', (route) =>
    route.fulfill({ json: { ...complete, status: done ? 'completed' : 'running' } }),
  );
  await page.goto('/');
  await page.getByRole('button', { name: '+ Импорт истории' }).click();
  const dialog = page.getByRole('dialog');
  await dialog
    .getByLabel('Ссылка на историю круток')
    .fill('https://example.invalid/?authkey=FAKE_TEST_KEY');
  await dialog.getByRole('button', { name: 'Загрузить историю' }).click();
  await expect(dialog.getByRole('button', { name: 'Свернуть' })).toBeVisible();
  await dialog.getByRole('button', { name: 'Свернуть' }).click();
  await expect(page.getByRole('button', { name: 'Показать статус импорта' })).toBeVisible();
  await page.reload();
  await page.getByRole('button', { name: 'Показать статус импорта' }).click();
  await expect(dialog.getByRole('heading', { name: 'Собираем историю' })).toBeVisible();
  done = true;
  await expect(dialog.getByRole('heading', { name: 'История на своём месте' })).toBeVisible();
  expect(posts).toBe(1);
  expect(await page.evaluate(() => sessionStorage.getItem('gachaslop.active-job'))).toBeNull();
});

test('banner API renders all games, phases, countdown and approved layout', async ({
  page,
}, info) => {
  const start = new Date(Date.now() - 86400000).toISOString();
  const end = new Date(Date.now() + 86400000).toISOString();
  await page.route('**/api/v1/banners?*', (route) => {
    const game = new URL(route.request().url()).searchParams.get('game');
    const current = {
      id: `${game}-now`,
      title: `Текущий ${game}`,
      pool_family: 'limited_character',
      image_kind: 'icon',
      starts_at: start,
      ends_at: end,
      status: 'current',
      items: [{ id: '1', name: `Герой ${game}`, rarity: 5, kind: 'character' }],
    };
    return route.fulfill({
      json: {
        game,
        source: 'Test calendar',
        source_url: 'https://example.invalid',
        fetched_at: start,
        stale: false,
        time_note: 'Test UTC',
        items: [
          current,
          {
            ...current,
            id: `${game}-next`,
            title: `Следующий ${game}`,
            starts_at: end,
            ends_at: new Date(Date.now() + 2 * 86400000).toISOString(),
            status: 'upcoming',
          },
        ],
      },
    });
  });
  await page.goto('/banners');
  await expect(page.getByText('Расписание подключено', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Следующие · 1', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Следующий genshin' })).toBeVisible();
  await expect(page.getByLabel('Таймер баннера')).toContainText('До старта');
  await page.getByRole('button', { name: 'Сейчас · 1', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Текущий genshin' })).toBeVisible();
  await expect(page.getByLabel('Таймер баннера')).toContainText('До завершения');
  const dates = await page.locator('.wish-dates').boundingBox();
  const title = await page.locator('.wish-copy h2').boundingBox();
  const tag = await page.locator('.wish-tagline').boundingBox();
  const timer = await page.locator('.wish-timer').boundingBox();
  const element = await page.locator('.wish-element').boundingBox();
  expect(dates!.y + dates!.height).toBeLessThanOrEqual(title!.y);
  expect(tag!.y + tag!.height).toBeLessThanOrEqual(timer!.y);
  expect(timer!.y + timer!.height).toBeLessThanOrEqual(element!.y);
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
  ).toBeTruthy();
  await page.screenshot({ path: `test-results/banners-${info.project.name}.png`, fullPage: true });
  await page.getByRole('button', { name: 'Календарь', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Календарь баннеров' })).toBeVisible();
  for (const [game, label] of [
    ['hsr', 'Honkai: Star Rail'],
    ['zzz', 'Zenless Zone Zero'],
    ['wuwa', 'Wuthering Waves'],
  ]) {
    await page.getByRole('button', { name: label }).click();
    await expect(page.getByRole('heading', { name: `Текущий ${game}` })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Текущий genshin' })).not.toBeVisible();
  }
});

test('banner failures and stale cache are explicit, no demo fallback', async ({ page }) => {
  let failed = true;
  await page.route('**/api/v1/banners?*', (route) =>
    failed
      ? route.fulfill({ status: 503, json: { error: { code: 'banners_unavailable' } } })
      : route.fulfill({
          json: {
            game: 'genshin',
            source: 'Test source',
            source_url: 'https://example.invalid',
            fetched_at: new Date().toISOString(),
            stale: true,
            warning: 'Источник временно недоступен. Показано последнее сохранённое расписание.',
            time_note: 'UTC',
            items: [],
          },
        }),
  );
  await page.goto('/banners');
  await expect(page.getByRole('alert')).toContainText('Источник баннеров временно недоступен');
  failed = false;
  await page.getByRole('button', { name: 'Повторить' }).click();
  await expect(page.getByText('Сохранённое расписание', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Следующие · 0' }).click();
  await expect(
    page.getByRole('heading', { name: 'Следующие баннеры пока не опубликованы источником' }),
  ).toBeVisible();
  await expect(page.getByText('Царство спокойствия')).not.toBeVisible();
});

test('full artwork switches by character and safely falls back on image failure', async ({
  page,
}) => {
  const art = 'https://enka.network/ui/zzz/IconRole68.png';
  const broken = 'https://enka.network/ui/zzz/IconRole61.png';
  const icon = 'https://enka.network/ui/zzz/IconRoleSelect68.png';
  await page.route('https://enka.network/**', (route) =>
    route.request().url() === broken
      ? route.abort()
      : route.fulfill({
          contentType: 'image/svg+xml',
          body: '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="1200"><rect width="1200" height="1200" fill="teal"/></svg>',
        }),
  );
  await page.route('**/api/v1/banners?*', (route) =>
    route.fulfill({
      json: {
        game: 'genshin',
        source: 'Fixture',
        source_url: 'https://example.invalid',
        fetched_at: new Date().toISOString(),
        stale: false,
        time_note: 'UTC',
        items: [
          {
            id: 'both',
            title: 'Два персонажа',
            pool_family: 'limited_character',
            image_kind: 'icon',
            image_url: icon,
            starts_at: new Date(Date.now() - 86400000).toISOString(),
            ends_at: new Date(Date.now() + 86400000).toISOString(),
            status: 'current',
            items: [
              {
                id: '1',
                name: 'Первый',
                kind: 'character',
                rarity: 5,
                image_url: icon,
                art_url: art,
              },
              {
                id: '2',
                name: 'Второй',
                kind: 'character',
                rarity: 5,
                image_url: icon,
                art_url: broken,
              },
            ],
          },
        ],
      },
    }),
  );
  await page.goto('/banners');
  await expect(page.locator('.wish-banner')).toHaveClass(/image-splash/);
  await expect(page.locator('.wish-art')).toHaveAttribute('src', art);
  await page
    .getByLabel('Арт персонажа', { exact: true })
    .getByRole('button', { name: 'Второй' })
    .click();
  await expect(page.locator('.wish-banner')).toHaveClass(/image-icon/);
  await expect(page.getByText('Полный арт пока недоступен')).toBeVisible();
  await expect(page.locator('.wish-art')).toHaveAttribute('src', icon);
  expect((await page.locator('.wish-art').boundingBox())!.width).toBeLessThanOrEqual(180);
  await page
    .getByLabel('Арт персонажа', { exact: true })
    .getByRole('button', { name: 'Первый' })
    .click();
  await expect(page.locator('.wish-banner')).toHaveClass(/image-splash/);
  await expect(page.getByLabel('Таймер баннера')).toContainText('До завершения');
});
