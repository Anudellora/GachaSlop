import type {
  Account,
  AccountSummary,
  Game,
  PoolStats,
  PullFilters,
  PullPage,
  SyncJob,
  BannerSchedule,
} from './types';

export class APIError extends Error {
  constructor(
    public code: string,
    public status: number,
  ) {
    super(code);
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    ...init,
    headers: { Accept: 'application/json', ...init.headers },
  });
  if (!response.ok) {
    const body = await response.json().catch(() => null);
    // Never echo arbitrary server responses: they could contain a private history URL.
    throw new APIError(body?.error?.code ?? 'request_failed', response.status);
  }
  return response.json() as Promise<T>;
}

export const api = {
  banners: (game: Game, signal?: AbortSignal) =>
    request<BannerSchedule>(`/banners?game=${game}`, { signal }),
  accounts: (signal?: AbortSignal) => request<{ items: Account[] }>('/accounts', { signal }),
  pity: (id: string, signal?: AbortSignal) =>
    request<{ items: PoolStats[] }>(`/accounts/${encodeURIComponent(id)}/pity`, { signal }),
  summary: (id: string, signal?: AbortSignal) =>
    request<AccountSummary>(`/accounts/${encodeURIComponent(id)}/summary`, { signal }),
  pulls: (id: string, filters: PullFilters = {}, cursor = '', signal?: AbortSignal) => {
    const params = new URLSearchParams({ limit: '50' });
    if (filters.pool_family) params.set('pool_family', filters.pool_family);
    if (filters.rarity) params.set('rarity', String(filters.rarity));
    if (cursor) params.set('cursor', cursor);
    return request<PullPage>(`/accounts/${encodeURIComponent(id)}/pulls?${params}`, { signal });
  },
  createSync: (game: Game, sourceURL: string, accountID?: string) =>
    request<SyncJob>('/sync-jobs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        game,
        source_url: sourceURL,
        ...(accountID ? { account_id: accountID } : {}),
      }),
    }),
  job: (id: string, signal?: AbortSignal) =>
    request<SyncJob>(`/sync-jobs/${encodeURIComponent(id)}`, { signal }),
};

const messages: Record<string, string> = {
  banners_unavailable:
    'Источник баннеров временно недоступен, а сохранённого расписания пока нет. Попробуйте через минуту.',
  source_expired:
    'Ссылка недействительна или истекла. Откройте историю в игре заново и вставьте свежую ссылку.',
  invalid_source:
    'Эта ссылка не поддерживается. Нужен полный адрес игровой истории с временным ключом доступа.',
  account_mismatch:
    'Ссылка принадлежит другому аккаунту или игре. Выберите правильный аккаунт либо импорт нового.',
  queue_full: 'Очередь импорта заполнена. Попробуйте чуть позже.',
  worker_interrupted: 'Импорт прерван перезапуском сервера. Повторите его со свежей ссылкой.',
  not_found: 'Данные не найдены. Возможно, база данных была изменена.',
  invalid_input: 'Проверьте игру и ссылку на историю.',
  sync_failed:
    'Не удалось загрузить историю из игры. Проверьте соединение и попробуйте свежую ссылку.',
};
export function errorMessage(error: unknown): string {
  const code = error instanceof APIError ? error.code : typeof error === 'string' ? error : '';
  return (
    messages[code] ??
    'Не удалось связаться с API. Проверьте, что Go-сервер запущен, и повторите попытку.'
  );
}
