import { useState } from 'react';
import { useCollection } from '../components/CollectionContext';
import { usePulls } from '../api/queries';
import type { Game, PoolFamily, Pull } from '../api/types';
import { Button, ErrorNotice, Loading, Panel } from '../components/ui';
import { poolNames, pullDate, rarityLabel } from '../lib/format';

export function History() {
  const { account, game } = useCollection();
  const [family, setFamily] = useState<PoolFamily | ''>('');
  const [rarity, setRarity] = useState(0);
  const pulls = usePulls(account?.id, {
    pool_family: family || undefined,
    rarity: rarity || undefined,
  });
  const items = pulls.data?.pages.flatMap((p) => p.items) ?? [];
  return (
    <Panel>
      <div className="section-heading">
        <h2>Все выпадения</h2>
        <span className="muted">Загружено: {items.length}</span>
      </div>
      <div className="history-filters">
        <label>
          <span className="sr-only">Тип баннера</span>
          <select value={family} onChange={(e) => setFamily(e.target.value as PoolFamily | '')}>
            <option value="">Все баннеры</option>
            {Object.entries(poolNames).map(([id, name]) => (
              <option key={id} value={id}>
                {name}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span className="sr-only">Редкость</span>
          <select value={rarity} onChange={(e) => setRarity(Number(e.target.value))}>
            <option value="0">Любая редкость</option>
            {[5, 4, 3].map((r) => (
              <option key={r} value={r}>
                {rarityLabel(game, r)}
              </option>
            ))}
          </select>
        </label>
        {(family || rarity !== 0) && (
          <Button
            variant="quiet"
            onClick={() => {
              setFamily('');
              setRarity(0);
            }}
          >
            Сбросить
          </Button>
        )}
      </div>
      {pulls.isPending ? (
        <Loading />
      ) : (
        <>
          <PullTable pulls={items} game={game} />
          {pulls.isError && (
            <ErrorNotice
              error={pulls.error}
              retry={() => {
                if (pulls.isFetchNextPageError) void pulls.fetchNextPage();
                else void pulls.refetch();
              }}
            />
          )}
          {pulls.hasNextPage && (
            <div className="load-more">
              <Button
                variant="secondary"
                disabled={pulls.isFetchingNextPage}
                onClick={() => void pulls.fetchNextPage()}
              >
                {pulls.isFetchingNextPage ? 'Загружаем…' : 'Показать ещё 50'}
              </Button>
            </div>
          )}
        </>
      )}
      <p className="footnote">
        Фильтры применяются ко всей истории на сервере, а не только к загруженной странице. Время —
        как в игровой истории.
      </p>
    </Panel>
  );
}

export function PullTable({
  pulls,
  game,
  compact = false,
}: {
  pulls: Pull[];
  game: Game;
  compact?: boolean;
}) {
  if (!pulls.length)
    return (
      <div className="table-empty">
        Здесь пока нет выпадений. Попробуйте другой фильтр или импортируйте историю.
      </div>
    );
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>РЕДКОСТЬ</th>
            <th>ПРЕДМЕТ</th>
            <th>ТИП</th>
            {!compact && <th>БАННЕР</th>}
            <th>ДАТА</th>
          </tr>
        </thead>
        <tbody>
          {pulls.map((p) => (
            <tr key={p.id}>
              <td>
                <span className={`rarity rarity-${p.rarity}`}>{rarityLabel(game, p.rarity)}</span>
              </td>
              <td className="item-name">{p.name}</td>
              <td>{p.item_type}</td>
              {!compact && (
                <td>
                  <span>{poolNames[p.pool_family]}</span>
                  <small className="group-id">{p.pity_group}</small>
                </td>
              )}
              <td>
                <time>{pullDate(p.pulled_at)}</time>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
