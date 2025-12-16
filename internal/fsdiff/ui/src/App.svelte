<script lang="ts">
  import { onMount } from 'svelte';
  import { changes, filters, ui, filteredChanges, stats } from '$lib/stores';
  import { fetchChanges, connectSSE } from '$lib/api';
  import { createKeyboardHandler } from '$lib/utils';
  import type { Change, Filters } from '$lib/types';

  import Header from './components/Header.svelte';
  import FiltersBar from './components/Filters.svelte';
  import ChangeList from './components/ChangeList.svelte';
  import FileViewer from './components/FileViewer.svelte';

  let viewingChange = $state<Change | null>(null);

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
    filters.update((f) => ({ ...f, ...update }));
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

  onMount(() => {
    loadChanges();

    const es = connectSSE((data) => {
      changes.update((list) => [data as Change, ...list]);
    });

    es.onerror = () => {
      ui.update((s) => ({ ...s, live: false }));
    };

    es.onopen = () => {
      ui.update((s) => ({ ...s, live: true }));
    };

    return () => es.close();
  });
</script>

<svelte:window onkeydown={keyHandler} />

<div class="h-screen flex flex-col">
  <Header stats={$stats} live={$ui.live} />
  <FiltersBar filters={$filters} onUpdate={handleFilterUpdate} />

  <main class="flex-1 overflow-hidden">
    <ChangeList
      changes={$filteredChanges}
      selectedIdx={$ui.selectedIdx}
      onSelect={handleSelect}
      onOpen={handleOpen}
    />
  </main>
</div>

{#if viewingChange}
  <FileViewer change={viewingChange} onClose={handleCloseViewer} />
{/if}