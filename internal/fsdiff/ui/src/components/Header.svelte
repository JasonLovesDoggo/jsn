<script lang="ts">
  import type { Stats } from '$lib/types';

  interface Props {
    stats: Stats;
    live: boolean;
  }

  let { stats, live }: Props = $props();
</script>

<header class="flex items-center justify-between px-4 py-3 bg-bg-2 border-b border-bg-3">
  <div class="flex items-center gap-4">
    <h1 class="text-fg-1 font-semibold">fsdvr</h1>

    <div class="flex items-center gap-3 text-sm">
      <span class="flex items-center gap-1.5">
        <span class="w-2 h-2 rounded-full bg-critical"></span>
        <span class="text-fg-2">{stats.critical} critical</span>
      </span>

      <span class="text-fg-3">·</span>

      <span class="flex items-center gap-1.5">
        <span class="w-2 h-2 rounded-full bg-warn"></span>
        <span class="text-fg-2">{stats.interesting} interesting</span>
      </span>

      {#if stats.bulk > 0}
        <span class="text-fg-3">·</span>
        <span class="flex items-center gap-1.5">
          <span class="w-2 h-2 rounded-full bg-muted"></span>
          <span class="text-fg-3">{stats.bulk} bulk groups</span>
        </span>
      {/if}
    </div>
  </div>

  <div class="flex items-center gap-2">
    <span class="flex items-center gap-1.5 text-sm">
      <span class="w-2 h-2 rounded-full {live ? 'bg-added animate-pulse' : 'bg-fg-3'}"></span>
      <span class="text-fg-3">{live ? 'Live' : 'Disconnected'}</span>
    </span>
  </div>
</header>
