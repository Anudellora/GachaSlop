import { useEffect, useState, type CSSProperties } from 'react';
import { Link } from 'react-router-dom';
import { useCollection } from '../components/CollectionContext';
import { useBanners, usePity } from '../api/queries';
import type { Banner, Game } from '../api/types';
import { Button, Empty, ErrorNotice, Panel } from '../components/ui';
import { games, dateTime, pityValue, poolNames, rarityLabel } from '../lib/format';
import { bannerDate, bannerStatus, bannerTimer, displayBanners, phaseNames } from '../lib/banners';

const themes: Record<Game, CSSProperties> = {
  genshin: { '--hue': '#8b72cb', '--wash': '#e7e0f5', '--ink': '#363057' } as CSSProperties,
  hsr: { '--hue': '#758ac4', '--wash': '#e0e6f5', '--ink': '#303c57' } as CSSProperties,
  zzz: { '--hue': '#c6a353', '--wash': '#f3ead6', '--ink': '#514529' } as CSSProperties,
  wuwa: { '--hue': '#76aeb0', '--wash': '#deeeee', '--ink': '#29484b' } as CSSProperties,
};

export function Banners() {
  const { game, account } = useCollection();
  const schedule = useBanners(game);
  const pity = usePity(account?.id);
  const [selected, setSelected] = useState('');
  const [phase, setPhase] = useState<Banner['status']>('current');
  const [view, setView] = useState<'cards' | 'calendar'>('cards');
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  if (schedule.isPending)
    return (
      <div className="loading" role="status">
        <span className="spinner" />
        Получаем расписание баннеров…
      </div>
    );
  if (schedule.isError && !schedule.data)
    return <ErrorNotice error={schedule.error} retry={() => void schedule.refetch()} />;
  const data = schedule.data!;
  const displayed = displayBanners(game, data.items);
  const items = displayed.filter((b) => bannerStatus(b, now) === phase);
  const banner = items.find((b) => b.id === selected) ?? items[0];
  // Never merge independent pity groups merely because their display family matches.
  const matching = pity.data?.items.filter((p) => p.pool_family === banner?.pool_family) ?? [];
  const pool = matching.length === 1 ? matching[0] : undefined;
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  return (
    <div className="banners-page">
      <div className={`schedule-status ${data.stale ? 'is-stale' : ''}`} role="status">
        <div>
          <strong>{data.stale ? 'Сохранённое расписание' : 'Расписание подключено'}</strong>
          <small>
            Источник:{' '}
            <a href={data.source_url} target="_blank" rel="noreferrer">
              {data.source} ↗
            </a>{' '}
            · Обновлено {dateTime(data.fetched_at)}
          </small>
        </div>
        <Button
          variant="secondary"
          disabled={schedule.isFetching}
          onClick={() => void schedule.refetch()}
        >
          {schedule.isFetching ? 'Проверяем…' : 'Обновить'}
        </Button>
      </div>
      {data.warning && (
        <p className="schedule-warning" role="alert">
          {data.warning}
        </p>
      )}
      {schedule.isError && (
        <ErrorNotice error={schedule.error} retry={() => void schedule.refetch()} />
      )}
      <p className="schedule-note">
        {data.time_note} Даты ниже — в часовом поясе {timezone}.
      </p>
      <div className="section-heading banner-heading">
        <div>
          <span className="eyebrow">{games.find((g) => g.id === game)?.name.toUpperCase()}</span>
          <h2>Баннеры событий</h2>
        </div>
        <div className="segmented" aria-label="Вид баннеров">
          <button aria-pressed={view === 'cards'} onClick={() => setView('cards')}>
            Баннеры
          </button>
          <button aria-pressed={view === 'calendar'} onClick={() => setView('calendar')}>
            Календарь
          </button>
        </div>
      </div>
      {view === 'calendar' ? (
        <BannerCalendar
          items={displayed}
          now={now}
          onSelect={(b) => {
            setSelected(b.id);
            setPhase(bannerStatus(b, now));
            setView('cards');
          }}
        />
      ) : (
        <>
          <div className="phase-tabs segmented live-phases" aria-label="Период баннеров">
            {(['current', 'upcoming', 'ended', 'unknown'] as const).map((p) => {
              const count = displayed.filter((b) => bannerStatus(b, now) === p).length;
              return (
                (p === 'current' || p === 'upcoming' || count > 0) && (
                  <button
                    key={p}
                    aria-pressed={phase === p}
                    onClick={() => {
                      setPhase(p);
                      setSelected('');
                    }}
                  >
                    {phaseNames[p]} · {count}
                  </button>
                )
              );
            })}
          </div>
          {!banner ? (
            <Empty
              title={
                phase === 'upcoming'
                  ? 'Следующие баннеры пока не опубликованы источником'
                  : 'В этом периоде нет баннеров'
              }
            >
              Мы показываем только полученное расписание. Даты и состав следующей фазы не
              подставляются вручную.
            </Empty>
          ) : (
            <>
              <LiveBanner key={banner.id} banner={banner} game={game} now={now} />
              <div className="banner-selector live-selector" aria-label="Выберите баннер">
                {items.map((b) => (
                  <button
                    key={b.id}
                    aria-pressed={b.id === banner.id}
                    onClick={() => setSelected(b.id)}
                  >
                    {b.image_url && <img src={b.image_url} alt="" loading="lazy" />}
                    <strong>
                      {b.items
                        .filter((x) => x.rarity >= 5)
                        .map((x) => x.name)
                        .join(' / ') || b.title}
                    </strong>
                    <small>{poolNames[b.pool_family]}</small>
                  </button>
                ))}
              </div>
              <Panel>
                <div className="section-heading">
                  <h3>Состав баннера</h3>
                  <small>
                    {banner.version ? `Версия ${banner.version}` : 'По данным источника'}
                  </small>
                </div>
                <div className="banner-items">
                  {banner.items.map((item) => (
                    <div key={item.id} className="banner-item">
                      {item.image_url && <img src={item.image_url} alt="" loading="lazy" />}
                      <div>
                        <strong>{item.name}</strong>
                        <small className={`rarity-${item.rarity}`}>
                          {rarityLabel(game, item.rarity)} ·{' '}
                          {item.kind === 'character'
                            ? 'Персонаж'
                            : game === 'hsr'
                              ? 'Световой конус'
                              : game === 'zzz'
                                ? 'Амплификатор'
                                : 'Оружие'}
                        </small>
                      </div>
                    </div>
                  ))}
                </div>
                {banner.sharedPhaseLineup && (
                  <p className="schedule-note">
                    A-ранг указан для общей фазы: источник не разделяет его состав по персонажам.
                  </p>
                )}
                {!banner.items.length && (
                  <p className="muted">
                    Состав не указан в структурированных данных — смотрите объявление источника.
                  </p>
                )}
              </Panel>
            </>
          )}
        </>
      )}
      <div className="banner-account panel">
        <strong>{pityValue(pool)}</strong>
        <div>
          {!account
            ? 'Подключите свою историю'
            : matching.length > 1
              ? 'Для этого типа есть несколько независимых счётчиков'
              : 'Ваш счётчик этого типа баннера'}
          <small>
            {!account
              ? 'Счётчик появится после первого импорта'
              : pool
                ? `Группа ${pool.pity_group} · по вашей сохранённой истории`
                : 'Подробности и отдельные группы — в обзоре аккаунта'}
          </small>
        </div>
        <Link to="/">К коллекции →</Link>
      </div>
      {pity.isError && account && (
        <ErrorNotice error={pity.error} retry={() => void pity.refetch()} />
      )}
      <footer className="banner-footer">
        <span>Данные обновляются автоматически. Изображения © HoYoverse / Kuro Games.</span>
        <span>Нет аккаунта? Расписание всё равно доступно.</span>
      </footer>
    </div>
  );
}

function LiveBanner({ banner: b, game, now }: { banner: Banner; game: Game; now: number }) {
  const [failedURLs, setFailedURLs] = useState<string[]>([]);
  const [artID, setArtID] = useState('');
  const featured = b.items.filter((x) => x.rarity >= 5);
  const characters = featured.filter((x) => x.kind === 'character');
  const character = characters.find((x) => x.id === artID) ?? characters[0];
  const art = character?.art_url;
  const hasArt = !!art && !failedURLs.includes(art);
  const preview = character?.image_url || b.image_url;
  const image = hasArt ? art : preview;
  const imageKind = hasArt ? 'splash' : b.image_kind;
  const name = featured.map((x) => x.name).join(' / ');
  const element = featured
    .map((x) => x.element)
    .filter(Boolean)
    .join(' / ');
  return (
    <section
      className={`wish-banner live-banner image-${imageKind}`}
      data-game={game}
      style={themes[game]}
      aria-label="Выбранный баннер"
    >
      <div className="wish-orbit" aria-hidden="true" />
      {image && !failedURLs.includes(image) ? (
        <img
          key={image}
          className="wish-art"
          src={image}
          alt={character?.name || name || b.title}
          onError={() => setFailedURLs((urls) => [...urls, image])}
        />
      ) : (
        <span className="art-unavailable">Изображение недоступно</span>
      )}
      <div className="wish-copy">
        <div className="wish-type">— {poolNames[b.pool_family]}</div>
        <div className="wish-dates">
          <small>Период баннера</small>
          <div>
            {bannerDate(b.starts_at)} — {bannerDate(b.ends_at)}
          </div>
        </div>
        <h2>{b.title}</h2>
        {featured.length > 0 && (
          <div className="wish-stars">{game === 'zzz' ? 'Ранг S' : '★★★★★'}</div>
        )}
        <h3>{name}</h3>
        <p className="wish-tagline">
          {b.version ? `Версия ${b.version} · ` : ''}Повышенный шанс выпадения
        </p>
        <div className="wish-timer" aria-label="Таймер баннера">
          {bannerTimer(b, now)}
        </div>
        <div className="wish-element">
          {[
            element,
            b.pool_family === 'limited_character'
              ? 'Персонажи'
              : game === 'hsr'
                ? 'Световые конусы'
                : game === 'zzz'
                  ? 'Амплификаторы'
                  : 'Оружие',
          ]
            .filter(Boolean)
            .join(' · ')}
        </div>
      </div>
      <div className="wish-notice">{phaseNames[bannerStatus(b, now)]}</div>
      {characters.length > 1 && (
        <div className="art-picker" aria-label="Арт персонажа">
          {characters.map((item) => (
            <button
              key={item.id}
              aria-pressed={character?.id === item.id}
              onClick={() => setArtID(item.id)}
            >
              {item.name}
            </button>
          ))}
        </div>
      )}
      {characters.length > 0 && !hasArt && imageKind !== 'banner' && (
        <span className="art-fallback-note">Полный арт пока недоступен</span>
      )}
    </section>
  );
}

function BannerCalendar({
  items,
  now,
  onSelect,
}: {
  items: Banner[];
  now: number;
  onSelect: (b: Banner) => void;
}) {
  const dated = items.flatMap((b) =>
    b.starts_at && b.ends_at ? [Date.parse(b.starts_at), Date.parse(b.ends_at)] : [],
  );
  const start = dated.length ? Math.min(...dated) : now;
  const end = dated.length ? Math.max(...dated) : now;
  const span = Math.max(1, end - start);
  return (
    <Panel className="live-calendar">
      <div className="section-heading">
        <h3>Календарь баннеров</h3>
        <small>
          {dated.length
            ? `${new Date(start).toLocaleDateString('ru-RU')} — ${new Date(end).toLocaleDateString('ru-RU')}`
            : 'Точные даты пока не указаны'}
        </small>
      </div>
      {items.map((b) => (
        <button key={b.id} className="calendar-row" onClick={() => onSelect(b)}>
          <span>
            {b.title}
            <small>{phaseNames[bannerStatus(b, now)]}</small>
          </span>
          <span className="calendar-track">
            <span
              style={
                b.starts_at && b.ends_at
                  ? {
                      marginLeft: `${((Date.parse(b.starts_at) - start) / span) * 100}%`,
                      width: `${((Date.parse(b.ends_at) - Date.parse(b.starts_at)) / span) * 100}%`,
                    }
                  : undefined
              }
            >
              {b.starts_at ? new Date(b.starts_at).toLocaleDateString('ru-RU') : '?'} —{' '}
              {b.ends_at ? new Date(b.ends_at).toLocaleDateString('ru-RU') : '?'}
            </span>
          </span>
          <span>Открыть →</span>
        </button>
      ))}
      {!items.length && <p className="muted">Источник пока не опубликовал расписание.</p>}
    </Panel>
  );
}
