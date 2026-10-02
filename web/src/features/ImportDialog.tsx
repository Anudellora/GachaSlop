import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, errorMessage } from '../api/client';
import type { Account, Game, SyncJob } from '../api/types';
import { Button, ErrorNotice } from '../components/ui';
import { games, number } from '../lib/format';

const jobStorageKey = 'gachaslop.active-job';
function storedJob() {
  try {
    return sessionStorage.getItem(jobStorageKey) ?? '';
  } catch {
    return '';
  }
}
function persistJob(id: string) {
  try {
    if (id) sessionStorage.setItem(jobStorageKey, id);
    else sessionStorage.removeItem(jobStorageKey);
  } catch {
    /* Private browsing may disable storage. */
  }
}

export function ImportDialog({
  open,
  onOpen,
  onClose,
  initialGame,
  accounts,
  accountID,
  onImported,
}: {
  open: boolean;
  onOpen: () => void;
  onClose: () => void;
  initialGame: Game;
  accounts: Account[];
  accountID?: string;
  onImported: (job: SyncJob) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const submitting = useRef(false);
  const completed = useRef('');
  const initialized = useRef(false);
  const [game, setGame] = useState(initialGame);
  const [selectedAccount, setSelectedAccount] = useState(accountID ?? '');
  const [url, setURL] = useState('');
  const [jobID, setJobID] = useState(storedJob);
  const [sending, setSending] = useState(false);
  const [submitError, setSubmitError] = useState('');
  const client = useQueryClient();
  const job = useQuery({
    queryKey: ['sync-job', jobID],
    enabled: !!jobID,
    queryFn: ({ signal }) => api.job(jobID, signal),
    refetchInterval: (q) =>
      q.state.status === 'error' || ['completed', 'failed'].includes(q.state.data?.status ?? '')
        ? false
        : 1800,
    retry: 2,
  });
  const current = job.data;
  const active =
    !!jobID && (!current || current.status === 'queued' || current.status === 'running');

  useEffect(() => {
    if (open) {
      if (!initialized.current && !active && !sending) {
        setGame(initialGame);
        setSelectedAccount(accountID ?? '');
      }
      initialized.current = true;
      if (!dialog.current?.open) dialog.current?.showModal();
    } else {
      initialized.current = false;
      dialog.current?.close();
      setURL('');
    }
  }, [open, initialGame, accountID, active, sending]);
  useEffect(() => {
    if (current?.status === 'completed' && completed.current !== current.id) {
      completed.current = current.id;
      persistJob('');
      void client.invalidateQueries({ queryKey: ['accounts'] });
      for (const key of ['pity', 'summary', 'pulls'])
        void client.invalidateQueries({ queryKey: [key, current.account_id] });
      onImported(current);
    }
  }, [current, client, onImported]);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (submitting.current || active) return;
    let parsed: URL;
    try {
      parsed = new URL(url.trim());
      if (parsed.protocol !== 'https:') throw new Error();
    } catch {
      setSubmitError('Вставьте полный HTTPS-адрес истории из игры.');
      return;
    }
    submitting.current = true;
    setSending(true);
    setSubmitError('');
    // Credentials never enter query/mutation caches, browser storage, or a URL.
    const secret = url.trim();
    setURL('');
    try {
      const created = await api.createSync(game, secret, selectedAccount || undefined);
      client.setQueryData(['sync-job', created.id], created);
      persistJob(created.id);
      setJobID(created.id);
    } catch (error) {
      setSubmitError(errorMessage(error));
    } finally {
      submitting.current = false;
      setSending(false);
    }
  }
  function reset() {
    persistJob('');
    setJobID('');
    setSubmitError('');
    setURL('');
  }
  return (
    <>
      {!open && (active || sending) && (
        <button className="sync-float" onClick={onOpen} aria-label="Показать статус импорта">
          <span className="spinner" />
          Импорт истории
        </button>
      )}
      <dialog
        ref={dialog}
        className="import-dialog"
        aria-labelledby="import-heading"
        onCancel={onClose}
        onClose={onClose}
      >
        <div className="section-heading">
          <h2 id="import-heading">Импорт истории</h2>
          <Button
            variant="quiet"
            aria-label="Закрыть импорт"
            onClick={() => {
              dialog.current?.close();
              onClose();
            }}
          >
            ×
          </Button>
        </div>
        <p className="muted">Добавьте крутки из игры в свою коллекцию.</p>
        <div className="import-steps">
          <span>01 Игра</span>
          <span>02 Ссылка</span>
          <span className={jobID ? 'current' : ''}>03 Результат</span>
        </div>
        {!jobID ? (
          <form onSubmit={submit}>
            <label className="field">
              Выберите игру
              <select
                value={game}
                disabled={sending}
                onChange={(e) => {
                  setGame(e.target.value as Game);
                  setSelectedAccount('');
                }}
              >
                {games.map((g) => (
                  <option key={g.id} value={g.id}>
                    {g.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              Аккаунт
              <select
                value={selectedAccount}
                disabled={sending}
                onChange={(e) => setSelectedAccount(e.target.value)}
              >
                <option value="">Новый / определить по ссылке</option>
                {accounts
                  .filter((a) => a.game === game)
                  .map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.region} · UID {a.uid}
                    </option>
                  ))}
              </select>
            </label>
            <label className="field" htmlFor="history-url">
              Ссылка на историю круток
              <input
                id="history-url"
                type="password"
                autoComplete="off"
                spellCheck={false}
                required
                maxLength={32000}
                value={url}
                disabled={sending}
                onChange={(e) => setURL(e.target.value)}
                placeholder="Вставьте полную ссылку из истории игры"
                aria-describedby="url-privacy"
              />
            </label>
            <details className="import-help">
              <summary>Какую ссылку использовать?</summary>
              <p>
                Откройте историю круток в игре и скопируйте полный адрес её веб-страницы доступным
                вам способом. Нужна именно персональная ссылка с временным ключом, а не адрес сайта
                игры.
              </p>
              <p>
                HoYo: global-серверы и параметр authkey. WuWa: global/CN и параметры player_id,
                record_id, resources_id. Автоматического извлечения ссылки из клиента пока нет.
              </p>
              <p>Не публикуйте эту ссылку и не отправляйте её другим людям.</p>
            </details>
            <div className="privacy-note" id="url-privacy">
              <strong>Ссылка используется только для импорта</strong>
              <p>
                Временный ключ передаётся вашему Go-серверу. Он не сохраняется в базе и браузерном
                хранилище. Поле очищается после отправки.
              </p>
            </div>
            {submitError && (
              <p className="error-notice" role="alert">
                {submitError}
              </p>
            )}
            <p className="footnote">Повторные записи будут пропущены автоматически.</p>
            <div className="dialog-actions">
              <Button type="button" variant="secondary" onClick={onClose}>
                Закрыть
              </Button>
              <Button type="submit" disabled={sending || !url.trim()}>
                {sending ? 'Отправляем…' : 'Загрузить историю'}
              </Button>
            </div>
          </form>
        ) : (
          <div className="job-state" aria-live="polite">
            {job.isError ? (
              <>
                <ErrorNotice error={job.error} retry={() => void job.refetch()} />
                <p className="muted">
                  Ошибка проверки статуса не означает, что импорт остановился.
                </p>
                {job.error instanceof Error &&
                  'status' in job.error &&
                  job.error.status === 404 && (
                    <Button variant="secondary" onClick={reset}>
                      Начать новый импорт
                    </Button>
                  )}
              </>
            ) : current?.status === 'completed' ? (
              <>
                <span className="job-icon">✓</span>
                <h3>История на своём месте</h3>
                <strong className="job-count">+ {number(current.imported_count)}</strong>
                <p>новых записей · получено {number(current.fetched_count)}</p>
                <p className="muted">Дубликаты пропущены, счётчики обновлены.</p>
                <div className="dialog-actions">
                  <Button variant="secondary" onClick={reset}>
                    Ещё один импорт
                  </Button>
                  <Button onClick={onClose}>К коллекции</Button>
                </div>
              </>
            ) : current?.status === 'failed' ? (
              <>
                <span className="job-icon error">!</span>
                <h3>Не удалось завершить импорт</h3>
                <p role="alert">{errorMessage(current.error_code ?? 'sync_failed')}</p>
                <Button onClick={reset}>Вставить новую ссылку</Button>
              </>
            ) : (
              <>
                <span className="spinner large" />
                <h3>{current?.status === 'running' ? 'Собираем историю' : 'Ожидаем в очереди'}</h3>
                <p className="muted">
                  Проверяем доступные баннеры и сохраняем записи. Большая история может занять
                  несколько минут.
                </p>
                <p className="footnote">
                  Окно можно закрыть — импорт продолжится. Статус восстановится после перезагрузки
                  этой вкладки.
                </p>
                <Button variant="secondary" onClick={onClose}>
                  Свернуть
                </Button>
              </>
            )}
          </div>
        )}
      </dialog>
    </>
  );
}
