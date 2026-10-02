import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, APIError, errorMessage } from './client';

afterEach(() => vi.unstubAllGlobals());
describe('HTTP boundary', () => {
  it('sends filters and pagination to the server', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response('{"items":[]}'));
    vi.stubGlobal('fetch', fetch);
    await api.pulls('account/id', { rarity: 5, pool_family: 'limited_character' }, 'cursor+/=');
    const path = fetch.mock.calls[0][0] as string;
    expect(path).toContain('/accounts/account%2Fid/pulls?');
    expect(new URL('http://local' + path).searchParams.get('cursor')).toBe('cursor+/=');
    expect(path).toContain('rarity=5');
  });
  it('keeps the secret in the POST body, never the request URL', async () => {
    const fetch = vi.fn().mockResolvedValue(new Response('{"id":"job"}', { status: 202 }));
    vi.stubGlobal('fetch', fetch);
    await api.createSync('genshin', 'https://example.invalid/?authkey=PRIVATE');
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/sync-jobs');
    expect(JSON.parse(fetch.mock.calls[0][1].body).source_url).toContain('PRIVATE');
  });
  it('does not echo server error messages with credentials', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response('{"error":{"code":"source_expired","message":"authkey=PRIVATE"}}', {
          status: 400,
        }),
      ),
    );
    await expect(api.accounts()).rejects.toEqual(new APIError('source_expired', 400));
    expect(errorMessage(new APIError('source_expired', 400))).not.toContain('PRIVATE');
  });
});
