<script lang="ts">
  import type { Filters, Scan } from '$lib/types';

  interface Props {
    filters: Filters;
    scans: Scan[];
    onUpdate: (filters: Partial<Filters>) => void;
  }

  let { filters, scans, onUpdate }: Props = $props();

  const timeRanges = [
    { label: '5m', ms: 5 * 60 * 1000 },
    { label: '15m', ms: 15 * 60 * 1000 },
    { label: '1h', ms: 60 * 60 * 1000 },
    { label: 'all', ms: null },
  ];

  function setTimeRange(ms: number | null) {
    onUpdate({
      since: ms ? Date.now() - ms : null,
      until: null,
    });
  }

  function isActiveRange(ms: number | null): boolean {
    if (ms === null) return filters.since === null;
    if (filters.since === null) return false;
    const diff = Date.now() - filters.since;
    return Math.abs(diff - ms) < 60000; // within 1 minute tolerance
  }

  let searchValue = $state('');
  let searchTimeout: ReturnType<typeof setTimeout>;

  // Sync searchValue when filters.search changes externally
  $effect(() => {
    searchValue = filters.search;
  });

  function handleSearch(e: Event) {
    const target = e.target as HTMLInputElement;
    searchValue = target.value;
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      onUpdate({ search: searchValue });
    }, 300);
  }
</script>

<div class="flex items-center gap-4 px-4 py-2 bg-bg-2 border-b border-bg-3">
  <div class="flex items-center gap-1">
    {#each timeRanges as range (range.label)}
      <button
        type="button"
        class="px-2 py-1 text-xs rounded transition-colors
               {isActiveRange(range.ms)
                 ? 'bg-accent text-bg-1'
                 : 'text-fg-3 hover:text-fg-2 hover:bg-bg-3'}"
        onclick={() => setTimeRange(range.ms)}
      >
        {range.label}
      </button>
    {/each}
  </div>

  <div class="flex items-center gap-1">
    <select
      class="bg-bg-3 text-fg-2 text-xs px-2 py-1 rounded border-none outline-none cursor-pointer"
      value={filters.priority}
      onchange={(e) => onUpdate({ priority: e.currentTarget.value as Filters['priority'] })}
    >
      <option value="critical">Critical only</option>
      <option value="interesting">Critical + Interesting</option>
      <option value="all">All</option>
    </select>
  </div>

  <div class="flex items-center gap-1">
    <select
      class="bg-bg-3 text-fg-2 text-xs px-2 py-1 rounded border-none outline-none cursor-pointer"
      value={filters.scanId ?? ''}
      onchange={(e) => {
        const val = e.currentTarget.value;
        onUpdate({ scanId: val ? Number(val) : null });
      }}
    >
      <option value="">All scans</option>
      {#each scans as scan (scan.id)}
        <option value={scan.id}>
          Scan #{scan.id} ({scan.added + scan.modified + scan.deleted} changes)
        </option>
      {/each}
    </select>
  </div>

  <div class="flex-1">
    <input
      type="text"
      placeholder="Filter by path..."
      class="w-full bg-bg-1 text-fg-1 text-sm px-3 py-1.5 rounded border border-bg-3
             placeholder:text-fg-3 focus:border-accent focus:outline-none"
      value={searchValue}
      oninput={handleSearch}
    />
  </div>
</div>
