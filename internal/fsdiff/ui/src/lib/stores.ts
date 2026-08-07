import { writable, derived } from 'svelte/store';
import type { Change, Filters, UIState, Stats } from './types';

export const changes = writable<Change[]>([]);

export const filters = writable<Filters>({
  since: Date.now() - 3600_000, // last hour
  until: null,
  priority: 'all',
  excludeBulk: true,
  search: '',
  scanId: null,
});

export const ui = writable<UIState>({
  selectedIdx: null,
  viewingHash: null,
  loading: false,
  live: true,
});

export const filteredChanges = derived(
  [changes, filters],
  ([$changes, $filters]) => {
    if (!$filters.search) return $changes;
    const q = $filters.search.toLowerCase();
    return $changes.filter((c) => c.path.toLowerCase().includes(q));
  }
);

export const stats = derived(changes, ($changes) => {
  const result: Stats = { critical: 0, interesting: 0, noise: 0, bulk: 0 };
  const bulkIds = new Set<number>();

  for (const c of $changes) {
    if (c.bulk && c.bulk > 0) {
      bulkIds.add(c.bulk);
    }
    switch (c.priority) {
      case 'critical':
        result.critical++;
        break;
      case 'interesting':
        result.interesting++;
        break;
      default:
        result.noise++;
    }
  }

  result.bulk = bulkIds.size;
  return result;
});
