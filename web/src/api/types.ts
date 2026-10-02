// HTTP DTOs. Database models and vendor history formats never cross this boundary.
export type Game = 'genshin' | 'hsr' | 'zzz' | 'wuwa';
export interface BannerItem {
  id: string;
  name: string;
  image_url?: string;
  art_url?: string;
  rarity: number;
  element?: string;
  kind: 'character' | 'weapon';
}
export interface Banner {
  id: string;
  title: string;
  version?: string;
  pool_family: PoolFamily;
  image_url?: string;
  image_kind: 'icon' | 'banner';
  items: BannerItem[];
  starts_at: string | null;
  ends_at: string | null;
  status: 'current' | 'upcoming' | 'ended' | 'unknown';
}
export interface BannerSchedule {
  game: Game;
  items: Banner[];
  source: string;
  source_url: string;
  time_note: string;
  fetched_at: string;
  stale: boolean;
  warning?: string;
}
export type PoolFamily =
  | 'beginner'
  | 'standard'
  | 'limited_character'
  | 'limited_weapon'
  | 'chronicled'
  | 'bangboo'
  | 'other';
export interface Account {
  id: string;
  game: Game;
  uid: string;
  region: string;
  language: string;
  timezone_offset: number;
  created_at: string;
  updated_at: string;
}
export interface Pull {
  id: string;
  account_id: string;
  external_id: string;
  gacha_type: string;
  source_pool: string;
  pool_family: PoolFamily;
  pity_group: string;
  name: string;
  item_type: string;
  rarity: number;
  pulled_at: string;
  created_at: string;
}
export interface PoolStats {
  pity_group: string;
  pool_family: PoolFamily;
  total: number;
  five_star_pity: number;
  five_star_certainty: 'exact' | 'at_least';
  four_star_pity: number;
  four_star_certainty: 'exact' | 'at_least';
  guarantee_state: 'unknown' | 'yes' | 'no';
}
export interface SyncJob {
  id: string;
  game: Game;
  status: 'queued' | 'running' | 'completed' | 'failed';
  account_id?: string;
  fetched_count: number;
  imported_count: number;
  error_code?: string;
  error_message?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}
export interface AccountSummary {
  total: number;
  five_star_count: number;
  four_star_count: number;
  last_import: SyncJob | null;
}
export interface PullPage {
  items: Pull[];
  next_cursor?: string;
}
export interface PullFilters {
  pool_family?: PoolFamily;
  rarity?: number;
}
