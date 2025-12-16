export type ChangeType = 'added' | 'modified' | 'deleted';
export type Priority = 'critical' | 'interesting' | 'noise';

export interface Change {
  scanId?: number;
  ts: string;
  path: string;
  type: ChangeType;
  hash?: string;
  size?: number;
  mode?: number;
  content?: string;
  bulk?: number;
  priority: Priority;
  diff?: string;
}

export interface Scan {
  id: number;
  start: string;
  end: string;
  durationMs: number;
  added: number;
  modified: number;
  deleted: number;
}

export interface Config {
  interval: number;
  lastScanTime: number;
  nextScanTime: number;
  ignorePatterns: string[];
  // Progress fields
  scanning: boolean;
  filesProcessed: number;
  totalFiles: number;
  percent: number;
  rate: number;
  scanStartedAt: number;
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
  scanId: number | null;
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
