import type { Banner, Game } from '../api/types';

export type DisplayBanner = Banner & { sharedPhaseLineup?: boolean };

// Ennead groups simultaneous ZZZ characters into one phase. Split the view,
// not the source snapshot: dates and the original pity family stay unchanged.
export function displayBanners(game: Game, banners: Banner[]): DisplayBanner[] {
  if (game !== 'zzz') return banners;
  return banners.flatMap((banner) => {
    const featured = banner.items.filter((item) => item.kind === 'character' && item.rarity === 5);
    if (banner.pool_family !== 'limited_character' || featured.length < 2) return [banner];
    const supporting = banner.items.filter((item) => item.rarity < 5);
    return featured.map((item) => ({
      ...banner,
      id: `${banner.id}:character:${item.id}`,
      title: item.name,
      image_url: item.image_url,
      image_kind: 'icon' as const,
      items: [item, ...supporting],
      sharedPhaseLineup: supporting.length > 0,
    }));
  });
}

export function bannerStatus(b: Banner, now: number): Banner['status'] {
  const start = b.starts_at ? Date.parse(b.starts_at) : NaN;
  const end = b.ends_at ? Date.parse(b.ends_at) : NaN;
  if (Number.isFinite(end) && now >= end) return 'ended';
  if (Number.isFinite(start) && now < start) return 'upcoming';
  return Number.isFinite(start) && Number.isFinite(end) ? 'current' : 'unknown';
}
export function bannerTimer(b: Banner, now: number): string {
  const status = bannerStatus(b, now);
  if (status === 'ended') return 'Баннер завершён';
  if (status === 'unknown') return 'Точные сроки ещё не указаны';
  const target = Date.parse((status === 'upcoming' ? b.starts_at : b.ends_at)!);
  const seconds = Math.max(0, Math.ceil((target - now) / 1000));
  const d = Math.floor(seconds / 86400),
    h = Math.floor((seconds % 86400) / 3600),
    m = Math.floor((seconds % 3600) / 60),
    s = seconds % 60;
  return `${status === 'upcoming' ? 'До старта' : 'До завершения'} ${d ? `${d} дн. ` : ''}${h} ч. ${m} мин. ${s} с.`;
}
export const bannerDate = (date: string | null) =>
  date
    ? new Date(date).toLocaleString('ru-RU', {
        day: '2-digit',
        month: 'short',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      })
    : 'Дата уточняется';
export const phaseNames: Record<Banner['status'], string> = {
  current: 'Сейчас',
  upcoming: 'Следующие',
  ended: 'Завершённые',
  unknown: 'Без точных дат',
};
