<script lang="ts">
  import { onMount } from 'svelte';
  import { changes, filters, ui, filteredChanges, stats } from '$lib/stores';
  import { fetchChanges, fetchConfig, fetchScans, updateConfig, connectSSE } from '$lib/api';
  import { createKeyboardHandler } from '$lib/utils';
  import { syncFiltersToURL, readFiltersFromURL } from '$lib/url';
  import type { Change, Config, Filters, Scan } from '$lib/types';

  import Header from './components/Header.svelte';
  import FiltersBar from './components/Filters.svelte';
  import ChangeList from './components/ChangeList.svelte';
  import FileViewer from './components/FileViewer.svelte';

  let viewingChange = $state<Change | null>(null);
  let config = $state<Config | null>(null);
  let scans = $state<Scan[]>([]);

  async function loadChanges() {
    ui.update((s) => ({ ...s, loading: true }));
    try {
      const resp = await fetchChanges($filters);
      changes.set(resp.changes);
    } catch (e) {
      console.error('Failed to fetch changes:', e);
    } finally {
      ui.update((s) => ({ ...s, loading: false }));
    }
  }

  function handleFilterUpdate(update: Partial<Filters>) {
    filters.update((f) => {
      const newFilters = { ...f, ...update };
      syncFiltersToURL(newFilters);
      return newFilters;
    });
    loadChanges();
  }

  function handleSelect(idx: number) {
    ui.update((s) => ({ ...s, selectedIdx: idx }));
  }

  function handleOpen(change: Change) {
    viewingChange = change;
  }

  function handleCloseViewer() {
    viewingChange = null;
  }

  function moveSelection(delta: number) {
    const list = $filteredChanges;
    if (list.length === 0) return;

    ui.update((s) => {
      const current = s.selectedIdx ?? -1;
      const next = Math.max(0, Math.min(list.length - 1, current + delta));
      return { ...s, selectedIdx: next };
    });
  }

  function openSelected() {
    const idx = $ui.selectedIdx;
    if (idx !== null && $filteredChanges[idx]) {
      viewingChange = $filteredChanges[idx];
    }
  }

  const keyHandler = createKeyboardHandler({
    j: () => moveSelection(1),
    ArrowDown: () => moveSelection(1),
    k: () => moveSelection(-1),
    ArrowUp: () => moveSelection(-1),
    Enter: () => openSelected(),
    Escape: () => handleCloseViewer(),
    '1': () => handleFilterUpdate({ priority: 'critical' }),
    '2': () => handleFilterUpdate({ priority: 'interesting' }),
    '3': () => handleFilterUpdate({ priority: 'all' }),
    r: () => loadChanges(),
    '/': () => document.querySelector<HTMLInputElement>('input[type="text"]')?.focus(),
  });

  async function loadConfig() {
    try {
      config = await fetchConfig();
    } catch (e) {
      console.error('Failed to fetch config:', e);
    }
  }

  async function loadScans() {
    try {
      scans = await fetchScans();
    } catch (e) {
      console.error('Failed to fetch scans:', e);
    }
  }

  function handleConfigChange(newConfig: Config) {
    config = newConfig;
    // Reload changes in case ignore patterns changed
    loadChanges();
  }

  function handleScanComplete() {
    // Refresh config and changes after manual scan
    loadConfig();
    loadChanges();
    loadScans();
  }

  function handleFilterByScan(scanId: number) {
    handleFilterUpdate({ scanId });
  }

  async function handleIgnore(path: string) {
    if (!config) return;
    if (config.ignorePatterns.includes(path)) return;

    try {
      const newConfig = await updateConfig({
        ignorePatterns: [...config.ignorePatterns, path],
      });
      config = newConfig;
      // Changes will be filtered on next fetch, but also filter locally for immediate feedback
      loadChanges();
    } catch (err) {
      console.error('Failed to add ignore pattern:', err);
    }
  }

  onMount(() => {
    // Initialize filters from URL params
    const urlFilters = readFiltersFromURL();
    if (Object.keys(urlFilters).length > 0) {
      filters.update((f) => ({ ...f, ...urlFilters }));
    }

    loadChanges();
    loadConfig();
    loadScans();

    const es = connectSSE((data) => {
      changes.update((list) => [data as Change, ...list]);
      loadScans();
    });

    es.onerror = () => {
      ui.update((s) => ({ ...s, live: false }));
    };

    es.onopen = () => {
      ui.update((s) => ({ ...s, live: true }));
    };

    // Poll config every 5 seconds for countdown accuracy
    const configInterval = setInterval(loadConfig, 5000);

    return () => {
      es.close();
      clearInterval(configInterval);
    };
  });
</script>

<svelte:window onkeydown={keyHandler} />

<div class="h-screen flex flex-col">
  <Header stats={$stats} live={$ui.live} {config} changes={$filteredChanges} onConfigChange={handleConfigChange} onScanComplete={handleScanComplete} />
  <FiltersBar filters={$filters} {scans} onUpdate={handleFilterUpdate} />

  <main class="flex-1 overflow-hidden">
    <ChangeList
      changes={$filteredChanges}
      selectedIdx={$ui.selectedIdx}
      onSelect={handleSelect}
      onOpen={handleOpen}
      onFilterByScan={handleFilterByScan}
    />
  </main>
</div>

{#if viewingChange}
  <FileViewer change={viewingChange} onClose={handleCloseViewer} onIgnore={handleIgnore} />
{/if}