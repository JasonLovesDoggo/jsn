import type { APIResponse, Config, Filters, Scan } from './types';

const BASE_URL = '';

export async function fetchChanges(filters: Filters): Promise<APIResponse> {
  const params = new URLSearchParams();

  if (filters.since) {
    params.set('since', String(Math.floor(filters.since / 1000)));
  }
  if (filters.until) {
    params.set('until', String(Math.floor(filters.until / 1000)));
  }
  if (filters.priority !== 'all') {
    params.set('priority', filters.priority);
  }
  if (filters.scanId) {
    params.set('scanId', String(filters.scanId));
  }
  params.set('exclude_bulk', String(filters.excludeBulk));
  params.set('limit', '1000');

  const res = await fetch(`${BASE_URL}/api/changes?${params}`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

export async function fetchConfig(): Promise<Config> {
  const res = await fetch(`${BASE_URL}/api/config`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

export async function updateConfig(interval: number): Promise<Config> {
  const res = await fetch(`${BASE_URL}/api/config`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ interval }),
  });
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

export async function fetchScans(): Promise<Scan[]> {
  const res = await fetch(`${BASE_URL}/api/scans`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  return res.json();
}

export async function fetchContent(hash: string): Promise<string> {
  const res = await fetch(`${BASE_URL}/content/${hash}`);
  if (!res.ok) {
    throw new Error(`Content not found: ${hash}`);
  }
  return res.text();
}

export function connectSSE(onMessage: (data: unknown) => void): EventSource {
  const es = new EventSource(`${BASE_URL}/events`);

  es.addEventListener('change', (e) => {
    try {
      const data = JSON.parse(e.data);
      onMessage(data);
    } catch {
      console.error('Failed to parse SSE message');
    }
  });

  es.addEventListener('ping', () => {
    // Connection alive
  });

  return es;
}
