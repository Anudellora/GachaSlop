import { createContext, useContext } from 'react';
import type { Account, Game } from '../api/types';

interface CollectionState {
  game: Game;
  account?: Account;
  accounts: Account[];
  openImport: () => void;
}

export const Collection = createContext<CollectionState | null>(null);
export function useCollection() {
  const context = useContext(Collection);
  if (!context) throw new Error('Collection provider is missing');
  return context;
}
