import { useState } from 'react';
import { NavLink, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
import { useAccounts } from './api/queries';
import type { Game } from './api/types';
import { Collection, useCollection } from './components/CollectionContext';
import { games, dateTime } from './lib/format';
import { Button, Empty, ErrorNotice, Loading } from './components/ui';
import { Overview } from './features/Overview';
import { History } from './features/History';
import { ImportDialog } from './features/ImportDialog';
import { Banners } from './features/Banners';

export function App() {
  const [game, setGame] = useState<Game>('genshin');
  const [accountID, setAccountID] = useState('');
  const [importOpen, setImportOpen] = useState(false);
  const accounts = useAccounts();
  const list = accounts.data?.items ?? [];
  const filtered = list.filter((a) => a.game === game);
  const account = filtered.find((a) => a.id === accountID) ?? filtered[0];
  const location = useLocation();
  const navigate = useNavigate();
  const openImport = () => setImportOpen(true);
  const isBanners = location.pathname === '/banners';
  const headings: Record<string, [string, string, string]> = {
    '/': [
      'МОЙ ОБЗОР',
      'Каждая крутка на своём месте.',
      'Четыре игры. Одна история. Следите за тем, что уже выпало.',
    ],
    '/history': [
      'ЛИЧНАЯ КОЛЛЕКЦИЯ',
      'История круток',
      'Все сохранённые выпадения — без дубликатов и потерь между импортами.',
    ],
    '/accounts': [
      'ЛИЧНАЯ КОЛЛЕКЦИЯ',
      'Ваши аккаунты',
      'Разные игры и серверы. Каждая история хранится отдельно.',
    ],
    '/banners': [
      'КАЛЕНДАРЬ БАННЕРОВ',
      'Следующая встреча — в игре.',
      'Текущие и объявленные баннеры. Следите за составом и временем до смены фазы.',
    ],
  };
  const heading = headings[location.pathname] ?? [
    '404',
    'Такой страницы нет',
    'Вернитесь к своей коллекции.',
  ];
  return (
    <Collection.Provider value={{ game, account, accounts: list, openImport }}>
      <a className="skip-link" href="#content">
        К содержимому
      </a>
      <div className="app-shell">
        <aside className="sidebar">
          <NavLink className="logo" to="/">
            <span>✦</span> GachaSLop
          </NavLink>
          <div className="nav-label">ЛИЧНАЯ КОЛЛЕКЦИЯ</div>
          <nav aria-label="Главное меню">
            <NavLink to="/" end>
              Обзор
            </NavLink>
            <NavLink to="/history">История круток</NavLink>
            <NavLink to="/accounts">Аккаунты</NavLink>
            <button onClick={openImport}>Импорт данных</button>
            <div className="nav-label">БАННЕРЫ</div>
            <NavLink to="/banners">Календарь</NavLink>
          </nav>
          <div className="profile">
            <span className="avatar">П</span>
            <div>
              Путешественник<small>Локальный профиль</small>
            </div>
          </div>
          <p className="sidebar-note">История хранится в вашей SQLite-базе.</p>
        </aside>
        <main id="content">
          <div className="topline">
            <span>{heading[0]}</span>
            <span className="local-status">
              <i />
              Локальная коллекция
            </span>
          </div>
          <header className="page-heading">
            <div>
              <h1>{heading[1]}</h1>
              <p>{heading[2]}</p>
            </div>
            <Button onClick={openImport}>+ Импорт истории</Button>
          </header>
          <div className="game-tabs" aria-label="Выберите игру">
            {games.map((g) => (
              <button
                key={g.id}
                aria-pressed={game === g.id}
                onClick={() => {
                  setGame(g.id);
                  setAccountID('');
                }}
              >
                <span className={`game-symbol ${g.id}`}>{g.symbol}</span>
                <span>{g.name}</span>
              </button>
            ))}
          </div>
          {!isBanners && (
            <div className="account-toolbar">
              <h2>{games.find((g) => g.id === game)?.name}</h2>
              {account ? (
                <>
                  <label className="account-select">
                    <span className="sr-only">Аккаунт</span>
                    <select value={account.id} onChange={(e) => setAccountID(e.target.value)}>
                      {filtered.map((a) => (
                        <option key={a.id} value={a.id}>
                          {a.region || 'Сервер'} · UID {a.uid}
                        </option>
                      ))}
                    </select>
                  </label>
                  <small>Обновлено {dateTime(account.updated_at)}</small>
                </>
              ) : (
                <small>Аккаунт ещё не добавлен</small>
              )}
            </div>
          )}
          {accounts.isPending && !isBanners ? (
            <Loading />
          ) : accounts.isError && !isBanners ? (
            <ErrorNotice error={accounts.error} retry={() => void accounts.refetch()} />
          ) : (
            <Routes>
              <Route path="/" element={account ? <Overview key={account.id} /> : <FirstImport />} />
              <Route
                path="/history"
                element={account ? <History key={account.id} /> : <FirstImport />}
              />
              <Route
                path="/accounts"
                element={
                  <div className="account-grid">
                    {filtered.length ? (
                      filtered.map((a) => (
                        <section className="panel account-card" key={a.id}>
                          <span className="eyebrow">
                            {games.find((g) => g.id === a.game)?.name}
                          </span>
                          <h2>UID {a.uid}</h2>
                          <p>{a.region || 'Регион не указан'}</p>
                          <small>Последний импорт: {dateTime(a.updated_at)}</small>
                          <Button
                            variant="secondary"
                            onClick={() => {
                              setAccountID(a.id);
                              navigate('/');
                            }}
                          >
                            Открыть коллекцию →
                          </Button>
                        </section>
                      ))
                    ) : (
                      <FirstImport />
                    )}
                  </div>
                }
              />
              <Route path="/banners" element={<Banners key={game} />} />
              <Route
                path="*"
                element={
                  <Empty
                    title="Страница не найдена"
                    action={<Button onClick={() => navigate('/')}>На главную</Button>}
                  >
                    Коллекция доступна в разделе «Обзор».
                  </Empty>
                }
              />
            </Routes>
          )}
          <footer className="page-footer">
            <span>GachaSLop · Личная коллекция</span>
            <span>Не связан с HoYoverse или Kuro Games</span>
          </footer>
        </main>
      </div>
      <ImportDialog
        open={importOpen}
        onOpen={openImport}
        onClose={() => setImportOpen(false)}
        initialGame={game}
        accounts={list}
        accountID={account?.id}
        onImported={(job) => {
          setGame(job.game);
          setAccountID(job.account_id ?? '');
          navigate('/');
        }}
      />
    </Collection.Provider>
  );
}

function FirstImport() {
  const { openImport } = useCollection();
  return (
    <Empty
      title="У вашей истории будет свой дом"
      action={<Button onClick={openImport}>Импортировать первые крутки</Button>}
    >
      Добавьте ссылку на историю из игры. Мы сохраним доступные записи, разделим баннеры и посчитаем
      крутки после последней редкой награды.
    </Empty>
  );
}
