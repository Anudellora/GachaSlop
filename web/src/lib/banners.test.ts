import { describe, expect, it } from 'vitest';
import type { Banner } from '../api/types';
import { bannerStatus, bannerTimer, displayBanners } from './banners';
const b = { starts_at: '2026-10-01T00:00:00Z', ends_at: '2026-10-02T00:00:00Z' } as Banner;
describe('live banner dates', () => {
  it('changes phase at exact boundaries', () => {
    expect(bannerStatus(b, Date.parse(b.starts_at!) - 1)).toBe('upcoming');
    expect(bannerStatus(b, Date.parse(b.starts_at!))).toBe('current');
    expect(bannerStatus(b, Date.parse(b.ends_at!))).toBe('ended');
  });
  it('does not invent unknown periods', () => {
    expect(bannerStatus({ ...b, starts_at: null }, Date.parse('2026-10-01T12:00:00Z'))).toBe(
      'unknown',
    );
  });
  it('counts down without negative values', () => {
    expect(bannerTimer(b, Date.parse(b.ends_at!) - 1000)).toBe('До завершения 0 ч. 0 мин. 1 с.');
    expect(bannerTimer(b, Date.parse(b.ends_at!) + 1)).toBe('Баннер завершён');
  });
});

describe('ZZZ character cards', () => {
  const grouped: Banner = {
    ...b,
    id: 'phase',
    title: 'First / Second',
    image_kind: 'icon',
    status: 'current',
    pool_family: 'limited_character',
    items: [
      {
        id: '1',
        name: 'First',
        kind: 'character',
        rarity: 5,
        image_url: '/first.png',
        art_url: '/first-art.png',
      },
      {
        id: '2',
        name: 'Second',
        kind: 'character',
        rarity: 5,
        image_url: '/second.png',
        art_url: '/second-art.png',
      },
      { id: '3', name: 'Support', kind: 'character', rarity: 4 },
    ],
  };
  it('makes distinct cards with their own artwork and the same source period', () => {
    const cards = displayBanners('zzz', [grouped]);
    expect(cards.map((card) => card.title)).toEqual(['First', 'Second']);
    expect(new Set(cards.map((card) => card.id)).size).toBe(2);
    for (const [index, card] of cards.entries()) {
      expect(card.starts_at).toBe(grouped.starts_at);
      expect(card.ends_at).toBe(grouped.ends_at);
      expect(card.pool_family).toBe(grouped.pool_family);
      expect(card.items.filter((item) => item.rarity === 5)).toEqual([grouped.items[index]]);
      expect(card.image_url).toBe(grouped.items[index].image_url);
      expect(card.sharedPhaseLineup).toBe(true);
    }
    expect(grouped.items).toHaveLength(3);
    expect(displayBanners('zzz', cards)).toEqual(cards);
  });
  it('leaves other games and weapon banners intact', () => {
    expect(displayBanners('genshin', [grouped])).toEqual([grouped]);
    const weapon = { ...grouped, pool_family: 'limited_weapon' as const };
    expect(displayBanners('zzz', [weapon])).toEqual([weapon]);
  });
});
