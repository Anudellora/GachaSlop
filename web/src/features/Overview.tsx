import { Link } from 'react-router-dom';
import { useCollection } from '../components/CollectionContext';
import { usePity, usePulls, useSummary } from '../api/queries';
import type { PoolStats } from '../api/types';
import { Button, ErrorNotice, Loading, Panel } from '../components/ui';
import { number, pityValue, poolNames, rarityLabel } from '../lib/format';
import { PullTable } from './History';

export function Overview() {
  const { account, game, openImport } = useCollection();
  const pity = usePity(account?.id);
  const summary = useSummary(account?.id);
  const pulls = usePulls(account?.id);
  if (pity.isPending || summary.isPending) return <Loading />;
  if (pity.isError || summary.isError)
    return (
      <ErrorNotice
        error={pity.error ?? summary.error}
        retry={() => {
          void pity.refetch();
          void summary.refetch();
        }}
      />
    );
  const pools = pity.data.items;
  const primary = pools.find((p) => p.pool_family === 'limited_character') ?? pools[0];
  const others = pools.filter((p) => p !== primary);
  const data = summary.data;
  const topRank = rarityLabel(game, 5);
  return (
    <>
      <div className="overview-grid">
        <Panel className="pity-hero">
          <div className="section-heading">
            <h3>{primary ? poolNames[primary.pool_family] : 'Событие персонажа'}</h3>
            <span className="eyebrow">СЧЁТЧИК {topRank}</span>
          </div>
          {primary && <small className="muted">Группа {primary.pity_group}</small>}
          <div className="pity-counter">
            <strong>{pityValue(primary)}</strong>
            <div>
              <h3>круток после последнего {topRank}</h3>
              <p>
                {!primary
                  ? 'В этом аккаунте ещё нет записей.'
                  : primary.five_star_certainty === 'exact'
                    ? `Счётчик точный · последняя ${topRank} запись есть в истории`
                    : `Нижняя граница · последней ${topRank} записи нет в доступной истории`}
              </p>
            </div>
          </div>
          <div className="pity-details">
            <span>После {rarityLabel(game, 4)} или выше</span>
            <strong>
              {primary
                ? `${primary.four_star_certainty === 'at_least' ? '≥ ' : ''}${primary.four_star_pity}`
                : '—'}
            </strong>
          </div>
          <details className="guarantee">
            <summary>
              ⓘ Гарант персонажа неизвестен <span>Почему?</span>
            </summary>
            <p>
              История выпадений не содержит полного каталога прошлых баннеров. Без него нельзя
              надёжно определить выигрыш 50/50 и обещать следующего персонажа. Счётчик не
              показывает, сколько круток осталось до гарантированного выпадения.
            </p>
          </details>
        </Panel>
        <div className="other-pools">
          {others.length ? (
            others.map((p) => <PoolCard key={p.pity_group} pool={p} />)
          ) : (
            <>
              <PoolCard label="Событие оружия" />
              <PoolCard label="Стандартный баннер" />
            </>
          )}
        </div>
      </div>
      <div className="metrics">
        {[
          ['Всего круток', number(data.total), 'в сохранённой истории'],
          [`Предметов ${topRank}`, number(data.five_star_count), 'по всем типам баннеров'],
          [`Предметов ${rarityLabel(game, 4)}`, number(data.four_star_count), 'персонажи и оружие'],
          [
            'Последний импорт',
            data.last_import ? `+ ${number(data.last_import.imported_count)}` : '—',
            'новых записей · без дубликатов',
          ],
        ].map(([label, value, hint]) => (
          <Panel key={label} className="metric">
            <p>{label}</p>
            <strong>{value}</strong>
            <small>{hint}</small>
          </Panel>
        ))}
      </div>
      <div className="overview-grid">
        <Panel>
          <div className="section-heading">
            <h2>Последние крутки</h2>
            <Link to="/history">Вся история →</Link>
          </div>
          {pulls.isPending ? (
            <Loading />
          ) : pulls.isError ? (
            <ErrorNotice error={pulls.error} retry={() => void pulls.refetch()} />
          ) : (
            <PullTable pulls={pulls.data.pages[0].items.slice(0, 6)} game={game} compact />
          )}
        </Panel>
        <div className="other-pools">
          <Panel>
            <span className="eyebrow">ВАША КОЛЛЕКЦИЯ</span>
            <h3>{data.last_import ? 'История синхронизирована' : 'История сохранена'}</h3>
            <p className="muted">
              {data.last_import
                ? `Последний импорт добавил ${number(data.last_import.imported_count)} записей. Ранее сохранённые крутки остались на месте.`
                : 'Загрузите свежую ссылку, чтобы добавить новые крутки.'}
            </p>
            <Button variant="quiet" onClick={openImport}>
              Обновить историю →
            </Button>
          </Panel>
          <Panel>
            <h3>Баннеры в игровом стиле</h3>
            <p className="muted">
              Текущие и объявленные баннеры четырёх игр. Состав, даты и время до смены фазы.
            </p>
            <Link to="/banners">Открыть баннеры →</Link>
          </Panel>
        </div>
      </div>
      <p className="footnote">
        Даты круток указаны по времени игровой истории. Импорт содержит только записи, которые ещё
        доступны в игре.
      </p>
    </>
  );
}

function PoolCard({ pool, label }: { pool?: PoolStats; label?: string }) {
  return (
    <Panel className="pool-card">
      <p>{pool ? poolNames[pool.pool_family] : label}</p>
      <strong>{pityValue(pool)}</strong>
      {pool && <small>Группа {pool.pity_group}</small>}
      <small>
        {!pool
          ? 'Нет записей этого типа'
          : pool.five_star_certainty === 'exact'
            ? 'После последней награды высшей редкости'
            : 'История неполная · это нижняя граница'}
      </small>
    </Panel>
  );
}
