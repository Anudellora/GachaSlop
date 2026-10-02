import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { api } from './client';
import type { Game, PullFilters } from './types';

export const useBanners = (game: Game) =>
  useQuery({
    queryKey: ['banners', game],
    queryFn: ({ signal }) => api.banners(game, signal),
    staleTime: 60_000,
    refetchInterval: 60_000,
    retry: false,
  });

export const useAccounts = () =>
  useQuery({ queryKey: ['accounts'], queryFn: ({ signal }) => api.accounts(signal) });
export const usePity = (id?: string) =>
  useQuery({
    queryKey: ['pity', id],
    queryFn: ({ signal }) => api.pity(id!, signal),
    enabled: !!id,
  });
export const useSummary = (id?: string) =>
  useQuery({
    queryKey: ['summary', id],
    queryFn: ({ signal }) => api.summary(id!, signal),
    enabled: !!id,
  });
export const usePulls = (id: string | undefined, filters: PullFilters = {}) =>
  useInfiniteQuery({
    queryKey: ['pulls', id, filters],
    initialPageParam: '',
    queryFn: ({ signal, pageParam }) => api.pulls(id!, filters, pageParam, signal),
    getNextPageParam: (page) => page.next_cursor || undefined,
    enabled: !!id,
  });
