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

  interface BulkGroup {
    id: number;
    changes: Change[];
  }

  // Group changes by priority, with bulk as sub-category of noise
  function groupByPriority(items: Change[]) {
    const critical: Change[] = [];
    const interesting: Change[] = [];
    const noise: Change[] = [];
    const bulkGroups = new Map<number, Change[]>();

    for (const c of items) {
      switch (c.priority) {
        case 'critical':
          critical.push(c);
          break;
        case 'interesting':
          interesting.push(c);
          break;
        default:
          // Separate bulk from regular noise
          if (c.bulk && c.bulk > 0) {
            const group = bulkGroups.get(c.bulk) || [];
            group.push(c);
            bulkGroups.set(c.bulk, group);
          } else {
            noise.push(c);
          }
      }
    }

    // Convert bulk map to sorted array
    const bulk: BulkGroup[] = Array.from(bulkGroups.entries())
      .map(([id, changes]) => ({ id, changes }))
      .sort((a, b) => b.id - a.id); // Most recent first

    return { critical, interesting, noise, bulk };
  }

  let groups = $derived(groupByPriority(changes));
  let showNoise = $state(false);
  let showBulk = $state(false);
  let expandedBulkGroups = $state<Set<number>>(new Set());

  function toggleBulkGroup(id: number) {
    const next = new Set(expandedBulkGroups);
    if (next.has(id)) {
      next.delete(id);
    } else {
      next.add(id);
    }
    expandedBulkGroups = next;
  }

  // Calculate total bulk changes count
  const bulkCount = $derived(groups.bulk.reduce((sum, g) => sum + g.changes.length, 0));

  // Calculate index offset for bulk items
  function getBulkItemIndex(groupIdx: number, itemIdx: number): number {
    const baseOffset = groups.critical.length + groups.interesting.length + groups.noise.length;
    let offset = baseOffset;
    for (let i = 0; i < groupIdx; i++) {
      offset += groups.bulk[i].changes.length;
    }
    return offset + itemIdx;
  }
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

  {#if groups.noise.length > 0 || groups.bulk.length > 0}
    <div class="sticky top-0 z-10 px-3 py-1.5 bg-bg-1 border-b border-bg-3 mt-2 flex items-center justify-between">
      <span class="text-muted text-xs font-medium uppercase tracking-wider">
        Noise ({groups.noise.length + bulkCount})
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
      <!-- Regular noise (non-bulk) -->
      {#if groups.noise.length > 0}
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

      <!-- Bulk groups (sub-category of noise) -->
      {#if groups.bulk.length > 0}
        <div class="px-3 py-1.5 bg-bg-2 border-y border-bg-3 flex items-center justify-between">
          <span class="text-fg-3 text-xs">
            Bulk operations ({groups.bulk.length} groups, {bulkCount} changes)
          </span>
          <button
            type="button"
            class="text-xs text-fg-3 hover:text-fg-2"
            onclick={() => (showBulk = !showBulk)}
          >
            {showBulk ? 'collapse all' : 'expand'}
          </button>
        </div>

        {#if showBulk}
          {#each groups.bulk as group, groupIdx (group.id)}
            <div class="border-b border-bg-3">
              <button
                type="button"
                class="w-full px-3 py-1.5 flex items-center justify-between bg-bg-2/50 hover:bg-bg-3 text-left"
                onclick={() => toggleBulkGroup(group.id)}
              >
                <span class="text-xs text-fg-3">
                  <span class="font-mono">Bulk #{group.id}</span>
                  <span class="text-fg-3/60 ml-2">{group.changes.length} files</span>
                </span>
                <span class="text-xs text-fg-3">
                  {expandedBulkGroups.has(group.id) ? '−' : '+'}
                </span>
              </button>

              {#if expandedBulkGroups.has(group.id)}
                <div class="flex flex-col pl-4 bg-bg-1/50">
                  {#each group.changes as change, i (change.path + change.ts)}
                    {@const idx = getBulkItemIndex(groupIdx, i)}
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
            </div>
          {/each}
        {/if}
      {/if}
    {/if}
  {/if}

  {#if changes.length === 0}
    <div class="flex-1 flex items-center justify-center text-fg-3">
      No changes detected
    </div>
  {/if}
</div>
