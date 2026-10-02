import { describe, expect, it } from 'vitest';
import { pityValue, pullDate, rarityLabel } from './format';
import type { PoolStats } from '../api/types';

describe('game-aware display', () => {
  it('does not shift wall-clock history timestamps into the browser timezone', () => {
    expect(pullDate('2026-09-14 00:03:20')).toBe('14.09.2026 · 00:03');
  });
  it('distinguishes unknown, zero, and incomplete pity', () => {
    expect(pityValue()).toBe('—');
    expect(pityValue({ five_star_pity: 0, five_star_certainty: 'exact' } as PoolStats)).toBe('0');
    expect(pityValue({ five_star_pity: 41, five_star_certainty: 'at_least' } as PoolStats)).toBe(
      '≥ 41',
    );
  });
  it('uses ranks for ZZZ', () => {
    expect(rarityLabel('zzz', 5)).toBe('S');
    expect(rarityLabel('hsr', 5)).toBe('5★');
  });
});
