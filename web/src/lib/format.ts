import type { Game, PoolFamily, PoolStats } from '../api/types';

export const games: { id: Game; name: string; short: string; symbol: string }[] = [
  { id: 'genshin', name: 'Genshin Impact', short: 'Genshin', symbol: '✧' },
  { id: 'hsr', name: 'Honkai: Star Rail', short: 'Star Rail', symbol: '✦' },
  { id: 'zzz', name: 'Zenless Zone Zero', short: 'ZZZ', symbol: '◈' },
  { id: 'wuwa', name: 'Wuthering Waves', short: 'WuWa', symbol: '≋' },
];
export const poolNames: Record<PoolFamily, string> = {
  limited_character: 'Событие персонажа',
  limited_weapon: 'Событие оружия',
  standard: 'Стандартный баннер',
  beginner: 'Баннер новичка',
  chronicled: 'Молитва хроник',
  bangboo: 'Банбу',
  other: 'Другой баннер',
};
export const number = (n: number) => new Intl.NumberFormat('ru-RU').format(n);
export const dateTime = (s: string) =>
  new Date(s).toLocaleString('ru-RU', { dateStyle: 'short', timeStyle: 'short' });
// Upstream timestamps are wall-clock strings, not UTC. Do not shift their timezone.
export function pullDate(s: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}:\d{2})/.exec(s);
  return match ? `${match[3]}.${match[2]}.${match[1]} · ${match[4]}` : s;
}
export const pityValue = (p?: PoolStats) =>
  p ? `${p.five_star_certainty === 'at_least' ? '≥ ' : ''}${number(p.five_star_pity)}` : '—';
export const rarityLabel = (game: Game, rarity: number) =>
  game === 'zzz' ? ({ 5: 'S', 4: 'A', 3: 'B' }[rarity] ?? String(rarity)) : `${rarity}★`;
