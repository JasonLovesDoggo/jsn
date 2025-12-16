import type { Filters } from './types';

export function filtersToParams(f: Filters): URLSearchParams {
  const p = new URLSearchParams();

  if (f.since) p.set('since', String(f.since));
  if (f.until) p.set('until', String(f.until));
  if (f.priority !== 'all') p.set('priority', f.priority);
  if (f.scanId) p.set('scanId', String(f.scanId));
  if (f.search) p.set('search', f.search);

  return p;
}

export function paramsToFilters(p: URLSearchParams): Partial<Filters> {
  const filters: Partial<Filters> = {};

  const since = p.get('since');
  if (since) filters.since = Number(since);

  const until = p.get('until');
  if (until) filters.until = Number(until);

  const priority = p.get('priority');
  if (priority === 'critical' || priority === 'interesting') {
    filters.priority = priority;
  }

  const scanId = p.get('scanId');
  if (scanId) filters.scanId = Number(scanId);

  const search = p.get('search');
  if (search) filters.search = search;

  return filters;
}

export function syncFiltersToURL(filters: Filters): void {
  const params = filtersToParams(filters);
  const url = new URL(window.location.href);

  // Clear existing filter params
  url.searchParams.delete('since');
  url.searchParams.delete('until');
  url.searchParams.delete('priority');
  url.searchParams.delete('scanId');
  url.searchParams.delete('search');

  // Add new params
  params.forEach((value, key) => {
    url.searchParams.set(key, value);
  });

  // Update URL without reload
  window.history.replaceState({}, '', url.toString());
}

export function readFiltersFromURL(): Partial<Filters> {
  const params = new URLSearchParams(window.location.search);
  return paramsToFilters(params);
}
