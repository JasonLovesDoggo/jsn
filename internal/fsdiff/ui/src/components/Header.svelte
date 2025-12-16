<script lang="ts">
  import type { Stats, Config } from '$lib/types';
  import { updateConfig } from '$lib/api';

  interface Props {
    stats: Stats;
    live: boolean;
    config: Config | null;
    onConfigChange?: (config: Config) => void;
  }

  let { stats, live, config, onConfigChange }: Props = $props();

  let countdown = $state(0);
  let intervalValue = $state(30);
  let debounceTimer: ReturnType<typeof setTimeout>;

  // Sync interval from config
  $effect(() => {
    if (config) {
      intervalValue = config.interval;
    }
  });

  // Update countdown every second
  $effect(() => {
    if (!config) return;

    const updateCountdown = () => {
      const remaining = Math.max(0, config.nextScanTime - Date.now());
      countdown = Math.ceil(remaining / 1000);
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);

    return () => clearInterval(interval);
  });

  function handleIntervalChange(e: Event) {
    const value = Number((e.target as HTMLInputElement).value);
    intervalValue = value;
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(async () => {
      try {
        const newConfig = await updateConfig(value);
        onConfigChange?.(newConfig);
      } catch (err) {
        console.error('Failed to update interval:', err);
      }
    }, 500);
  }
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

  <div class="flex items-center gap-4">
    {#if config}
      <div class="flex items-center gap-2 text-sm">
        <span class="text-fg-3">Next:</span>
        <span class="text-fg-2 font-mono w-8">{countdown}s</span>
      </div>

      <div class="flex items-center gap-2">
        <span class="text-fg-3 text-xs">Interval:</span>
        <input
          type="range"
          min="5"
          max="120"
          step="5"
          value={intervalValue}
          oninput={handleIntervalChange}
          class="w-20 h-1 bg-bg-3 rounded-lg appearance-none cursor-pointer accent-accent"
        />
        <span class="text-fg-3 text-xs font-mono w-8">{intervalValue}s</span>
      </div>
    {/if}

    <span class="flex items-center gap-1.5 text-sm">
      <span class="w-2 h-2 rounded-full {live ? 'bg-added animate-pulse' : 'bg-fg-3'}"></span>
      <span class="text-fg-3">{live ? 'Live' : 'Disconnected'}</span>
    </span>
  </div>
</header>
