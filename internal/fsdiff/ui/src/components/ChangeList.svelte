<script lang="ts">
  import type { Change } from '$lib/types';
  import ChangeRow from './ChangeRow.svelte';

  interface Props {
    changes: Change[];
    selectedIdx: number | null;
    onSelect: (idx: number) => void;
    onOpen: (change: Change) => void;
    onFilterByScan?: (scanId: number) => void;
  }

  let { changes, selectedIdx, onSelect, onOpen, onFilterByScan }: Props = $props();

  // Group changes by priority
  function groupByPriority(items: Change[]) {
    const critical: Change[] = [];
    const interesting: Change[] = [];
    const noise: Change[] = [];

    for (const c of items) {
      switch (c.priority) {
        case 'critical':
          critical.push(c);
          break;
        case 'interesting':
          interesting.push(c);
          break;
        default:
          noise.push(c);
      }
    }

    return { critical, interesting, noise };
  }

  let groups = $derived(groupByPriority(changes));
  let showNoise = $state(false);
</script>

<div class="flex flex-col h-full overflow-y-auto">
  {#if groups.critical.length > 0}
    <div class="sticky top-0 z-10 px-3 py-1.5 bg-bg-1 border-b border-bg-3">
      <span class="text-critical text-xs font-medium uppercase tracking-wider">
        Critical ({groups.critical.length})
      </span>
    </div>
    <div class="flex flex-col">
      {#each groups.critical as change, i (change.path + change.ts)}
        <ChangeRow
          {change}
          selected={selectedIdx === i}
          onclick={() => onSelect(i)}
          ondblclick={() => onOpen(change)}
          {onFilterByScan}
        />
      {/each}
    </div>
  {/if}

  {#if groups.interesting.length > 0}
    <div class="sticky top-0 z-10 px-3 py-1.5 bg-bg-1 border-b border-bg-3 mt-2">
      <span class="text-warn text-xs font-medium uppercase tracking-wider">
        Interesting ({groups.interesting.length})
      </span>
    </div>
    <div class="flex flex-col">
      {#each groups.interesting as change, i (change.path + change.ts)}
        {@const idx = groups.critical.length + i}
        <ChangeRow
          {change}
          selected={selectedIdx === idx}
          onclick={() => onSelect(idx)}
          ondblclick={() => onOpen(change)}
          {onFilterByScan}
        />
      {/each}
    </div>
  {/if}

  {#if groups.noise.length > 0}
    <div class="sticky top-0 z-10 px-3 py-1.5 bg-bg-1 border-b border-bg-3 mt-2 flex items-center justify-between">
      <span class="text-muted text-xs font-medium uppercase tracking-wider">
        Noise ({groups.noise.length})
      </span>
      <button
        type="button"
        class="text-xs text-fg-3 hover:text-fg-2"
        onclick={() => (showNoise = !showNoise)}
      >
        {showNoise ? 'hide' : 'show'}
      </button>
    </div>
    {#if showNoise}
      <div class="flex flex-col">
        {#each groups.noise as change, i (change.path + change.ts)}
          {@const idx = groups.critical.length + groups.interesting.length + i}
          <ChangeRow
            {change}
            selected={selectedIdx === idx}
            onclick={() => onSelect(idx)}
            ondblclick={() => onOpen(change)}
            {onFilterByScan}
          />
        {/each}
      </div>
    {/if}
  {/if}

  {#if changes.length === 0}
    <div class="flex-1 flex items-center justify-center text-fg-3">
      No changes detected
    </div>
  {/if}
</div>
