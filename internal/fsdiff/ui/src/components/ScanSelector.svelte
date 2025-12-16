<script lang="ts">
  import type { Scan } from '$lib/types';

  interface Props {
    scans: Scan[];
    selectedId: number | null;
    onSelect: (id: number | null) => void;
  }

  let { scans, selectedId, onSelect }: Props = $props();

  let open = $state(false);
  let search = $state('');
  let dropdownRef = $state<HTMLDivElement | null>(null);

  // Sort scans by ID descending (most recent first)
  const sortedScans = $derived(
    [...scans].sort((a, b) => b.id - a.id)
  );

  // Filter scans based on search
  const filteredScans = $derived.by(() => {
    if (!search) return sortedScans.slice(0, 20);
    const q = search.toLowerCase();
    return sortedScans.filter((s) => String(s.id).includes(q)).slice(0, 50);
  });

  const selectedScan = $derived(
    selectedId ? scans.find((s) => s.id === selectedId) : null
  );

  function handleClickOutside(e: MouseEvent) {
    if (open && dropdownRef && !dropdownRef.contains(e.target as Node)) {
      open = false;
      search = '';
    }
  }

  function selectScan(id: number | null) {
    onSelect(id);
    open = false;
    search = '';
  }

  function formatTime(iso: string): string {
    return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  function formatChanges(s: Scan): string {
    const parts: string[] = [];
    if (s.added > 0) parts.push(`+${s.added}`);
    if (s.modified > 0) parts.push(`~${s.modified}`);
    if (s.deleted > 0) parts.push(`-${s.deleted}`);
    return parts.join(' ') || '0 changes';
  }
</script>

<svelte:window onclick={handleClickOutside} />

<div class="relative" bind:this={dropdownRef}>
  <button
    type="button"
    class="flex items-center gap-1 px-2 py-1 text-xs rounded transition-colors
           {open ? 'bg-accent text-bg-1' : 'bg-bg-3 text-fg-2 hover:bg-bg-3/80'}"
    onclick={() => (open = !open)}
  >
    {#if selectedScan}
      <span class="font-mono">#{selectedScan.id}</span>
    {:else}
      <span>All scans</span>
    {/if}
    <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
    </svg>
  </button>

  {#if open}
    <div class="absolute left-0 top-full mt-1 w-56 bg-bg-2 border border-bg-3 rounded-lg shadow-xl z-50">
      <div class="p-2 border-b border-bg-3">
        <input
          type="text"
          placeholder="Search by scan #..."
          bind:value={search}
          class="w-full bg-bg-1 text-fg-1 text-xs px-2 py-1 rounded border border-bg-3
                 placeholder:text-fg-3 focus:border-accent focus:outline-none"
        />
      </div>
      <div class="max-h-64 overflow-y-auto">
        <button
          type="button"
          class="w-full flex items-center justify-between px-3 py-2 text-xs hover:bg-bg-3
                 {selectedId === null ? 'bg-accent/10 text-accent' : 'text-fg-2'}"
          onclick={() => selectScan(null)}
        >
          <span>All scans</span>
          {#if selectedId === null}
            <span class="text-accent">*</span>
          {/if}
        </button>

        {#each filteredScans as scan (scan.id)}
          <button
            type="button"
            class="w-full flex items-center justify-between px-3 py-2 text-xs hover:bg-bg-3
                   {selectedId === scan.id ? 'bg-accent/10 text-accent' : 'text-fg-2'}"
            onclick={() => selectScan(scan.id)}
          >
            <span class="flex items-center gap-2">
              <span class="font-mono text-fg-1">#{scan.id}</span>
              <span class="text-fg-3">{formatTime(scan.start)}</span>
            </span>
            <span class="text-fg-3">{formatChanges(scan)}</span>
          </button>
        {/each}

        {#if filteredScans.length === 0}
          <div class="px-3 py-2 text-xs text-fg-3">No scans found</div>
        {/if}
      </div>
    </div>
  {/if}
</div>
