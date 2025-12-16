export type ChangeType = 'added' | 'modified' | 'deleted';
export type Priority = 'critical' | 'interesting' | 'noise';

export interface Change {
  ts: string;
  path: string;
  type: ChangeType;
  hash?: string;
  size?: number;
  mode?: number;
  content?: string;
  bulk?: number;
  priority: Priority;
}

export interface APIResponse {
  changes: Change[];
  total: number;
  offset: number;
  limit: number;
}

export interface Filters {
  since: number | null;
  until: number | null;
  priority: 'critical' | 'interesting' | 'all';
  excludeBulk: boolean;
  search: string;
}

export interface UIState {
  selectedIdx: number | null;
  viewingHash: string | null;
  loading: boolean;
  live: boolean;
}

export interface Stats {
  critical: number;
  interesting: number;
  noise: number;
  bulk: number;
}
