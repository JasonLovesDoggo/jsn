<script lang="ts">
  import type { Filters, Scan } from '$lib/types';
  import ScanSelector from './ScanSelector.svelte';

  interface Props {
    filters: Filters;
    scans: Scan[];
    onUpdate: (filters: Partial<Filters>) => void;
  }

  let { filters, scans, onUpdate }: Props = $props();

  const timeRanges = [
    { label: '5m', ms: 5 * 60 * 1000 },
    { label: '15m', ms: 15 * 60 * 1000 },
    { label: '30m', ms: 30 * 60 * 1000 },
    { label: '1h', ms: 60 * 60 * 1000 },
    { label: '6h', ms: 6 * 60 * 60 * 1000 },
    { label: '24h', ms: 24 * 60 * 60 * 1000 },
    { label: 'all', ms: null },
  ];

  function setTimeRange(ms: number | null) {
    onUpdate({
      since: ms ? Date.now() - ms : null,
      until: null,
    });
    showCustomTime = false;
  }

  function isActiveRange(ms: number | null): boolean {
    // If custom range is set (both since and until), no preset is active
    if (filters.until !== null) return false;
    if (ms === null) return filters.since === null;
    if (filters.since === null) return false;
    const diff = Date.now() - filters.since;
    return Math.abs(diff - ms) < 60000; // within 1 minute tolerance
  }

  // Custom time range state
  let showCustomTime = $state(false);
  let customFromValue = $state(30);
  let customFromUnit = $state<'m' | 'h' | 'd'>('m');
  let customToValue = $state(0);
  let customToUnit = $state<'m' | 'h' | 'd'>('m');
  let customTimeRef = $state<HTMLDivElement | null>(null);

  const isCustomActive = $derived(filters.until !== null);

  function unitToMs(value: number, unit: 'm' | 'h' | 'd'): number {
    switch (unit) {
      case 'm':
        return value * 60 * 1000;
      case 'h':
        return value * 60 * 60 * 1000;
      case 'd':
        return value * 24 * 60 * 60 * 1000;
    }
  }

  function applyCustomTime() {
    const now = Date.now();
    const fromMs = unitToMs(customFromValue, customFromUnit);
    const toMs = unitToMs(customToValue, customToUnit);

    onUpdate({
      since: now - fromMs,
      until: toMs > 0 ? now - toMs : null,
    });
    showCustomTime = false;
  }

  function handleClickOutside(e: MouseEvent) {
    if (showCustomTime && customTimeRef && !customTimeRef.contains(e.target as Node)) {
      showCustomTime = false;
    }
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

  function handleScanSelect(id: number | null) {
    onUpdate({ scanId: id });
  }
</script>

<svelte:window onclick={handleClickOutside} />

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

    <div class="relative" bind:this={customTimeRef}>
      <button
        type="button"
        class="px-2 py-1 text-xs rounded transition-colors
               {isCustomActive || showCustomTime
                 ? 'bg-accent text-bg-1'
                 : 'text-fg-3 hover:text-fg-2 hover:bg-bg-3'}"
        onclick={() => (showCustomTime = !showCustomTime)}
      >
        custom
      </button>

      {#if showCustomTime}
        <div class="absolute left-0 top-full mt-1 bg-bg-2 border border-bg-3 rounded-lg shadow-xl z-50 p-3 w-56">
          <div class="text-xs text-fg-3 mb-2">Time range</div>

          <div class="flex items-center gap-2 mb-2">
            <span class="text-xs text-fg-2 w-10">From:</span>
            <input
              type="number"
              min="0"
              bind:value={customFromValue}
              class="w-16 bg-bg-1 text-fg-1 text-xs px-2 py-1 rounded border border-bg-3 [appearance:textfield]
                     [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
            />
            <select
              bind:value={customFromUnit}
              class="bg-bg-1 text-fg-2 text-xs px-1 py-1 rounded border border-bg-3"
            >
              <option value="m">min</option>
              <option value="h">hr</option>
              <option value="d">day</option>
            </select>
            <span class="text-xs text-fg-3">ago</span>
          </div>

          <div class="flex items-center gap-2 mb-3">
            <span class="text-xs text-fg-2 w-10">To:</span>
            <input
              type="number"
              min="0"
              bind:value={customToValue}
              class="w-16 bg-bg-1 text-fg-1 text-xs px-2 py-1 rounded border border-bg-3 [appearance:textfield]
                     [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
            />
            <select
              bind:value={customToUnit}
              class="bg-bg-1 text-fg-2 text-xs px-1 py-1 rounded border border-bg-3"
            >
              <option value="m">min</option>
              <option value="h">hr</option>
              <option value="d">day</option>
            </select>
            <span class="text-xs text-fg-3">ago</span>
          </div>

          <button
            type="button"
            class="w-full px-2 py-1 text-xs rounded bg-accent text-bg-1 hover:bg-accent/80"
            onclick={applyCustomTime}
          >
            Apply
          </button>
        </div>
      {/if}
    </div>
  </div>

  <select
    class="bg-bg-3 text-fg-2 text-xs px-2 py-1 rounded border-none outline-none cursor-pointer"
    value={filters.priority}
    onchange={(e) => onUpdate({ priority: e.currentTarget.value as Filters['priority'] })}
  >
    <option value="critical">Critical only</option>
    <option value="interesting">Critical + Interesting</option>
    <option value="all">All</option>
  </select>

  <ScanSelector scans={scans} selectedId={filters.scanId} onSelect={handleScanSelect} />

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
