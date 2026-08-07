<script lang="ts">
  import type { Change } from '$lib/types';
  import { formatTime, truncatePath, resolveUid, hasSetuid, hasSetgid, isExecutable } from '$lib/utils';
  import Badge from './Badge.svelte';

  interface Props {
    change: Change;
    selected?: boolean;
    onclick?: () => void;
    ondblclick?: () => void;
    onFilterByScan?: (scanId: number) => void;
  }

  let { change, selected = false, onclick, ondblclick, onFilterByScan }: Props = $props();

  function handleScanClick(e: MouseEvent) {
    e.stopPropagation();
    if (change.scanId && onFilterByScan) {
      onFilterByScan(change.scanId);
    }
  }

  const typeColors: Record<string, string> = {
    added: 'text-added',
    modified: 'text-modified',
    deleted: 'text-deleted',
  };

  const typeLabels: Record<string, string> = {
    added: 'ADD',
    modified: 'MOD',
    deleted: 'DEL',
  };

  function getBorderClass(priority: string): string {
    switch (priority) {
      case 'critical':
        return 'border-l-2 border-critical bg-critical-dim';
      case 'interesting':
        return 'border-l-2 border-warn bg-warn-dim';
      default:
        return 'border-l-2 border-transparent bg-bg-2';
    }
  }

  function getUid(): number | undefined {
    if (change.path.startsWith('/etc/') || change.path.startsWith('/usr/')) {
      return 0;
    }
    if (change.path.includes('/home/')) {
      return 1000;
    }
    return undefined;
  }
</script>

<div
  role="button"
  tabindex="0"
  class="group w-full flex items-center gap-3 px-3 py-2 text-left transition-colors
         hover:bg-bg-3 cursor-pointer {getBorderClass(change.priority)}
         {selected ? 'ring-1 ring-accent ring-inset' : ''}"
  {onclick}
  {ondblclick}
  onkeydown={(e) => e.key === 'Enter' && onclick?.()}
>
  <span class="text-fg-3 font-mono text-xs w-16 shrink-0">
    {formatTime(change.ts)}
  </span>

  <span class="font-mono text-xs w-8 shrink-0 {typeColors[change.type]}">
    {typeLabels[change.type]}
  </span>

  <span class="flex-1 font-mono text-sm text-fg-1 truncate" title={change.path}>
    {truncatePath(change.path, 60)}
  </span>

  <span class="text-fg-3 text-xs w-12 shrink-0 text-right">
    {resolveUid(getUid())}
  </span>

  <span class="flex items-center gap-1 shrink-0">
    {#if change.scanId}
      <span
        role="button"
        tabindex="0"
        class="px-1.5 py-0.5 text-xs font-mono rounded bg-bg-3 text-fg-3 hover:bg-accent hover:text-bg-1 transition-colors cursor-pointer"
        title="Filter to scan #{change.scanId}"
        onclick={handleScanClick}
        onkeydown={(e) => e.key === 'Enter' && handleScanClick(e as unknown as MouseEvent)}
      >
        #{change.scanId}
      </span>
    {/if}
    {#if hasSetuid(change.mode)}
      <Badge type="suid" />
    {/if}
    {#if hasSetgid(change.mode)}
      <Badge type="sgid" />
    {/if}
    {#if isExecutable(change.mode) && !hasSetuid(change.mode) && !hasSetgid(change.mode)}
      <Badge type="exec" />
    {/if}
  </span>

  <span class="text-fg-3 text-sm shrink-0">›</span>
</div>
