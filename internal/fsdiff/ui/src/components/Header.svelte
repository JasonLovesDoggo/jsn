<script lang="ts">
  import type { Stats, Config, Change } from '$lib/types';
  import { updateConfig, triggerScan } from '$lib/api';

  interface Props {
    stats: Stats;
    live: boolean;
    config: Config | null;
    changes?: Change[];
    onConfigChange?: (config: Config) => void;
    onScanComplete?: () => void;
  }

  let { stats, live, config, changes = [], onConfigChange, onScanComplete }: Props = $props();

  let countdown = $state(0);
  let intervalValue = $state(30);
  let debounceTimer: ReturnType<typeof setTimeout>;
  let showIgnorePanel = $state(false);
  let newIgnorePattern = $state('');
  let ignorePanelRef = $state<HTMLDivElement | null>(null);

  // Extract unique directory paths from changes for autocomplete
  const suggestedPaths = $derived(() => {
    const dirs = new Set<string>();
    for (const c of changes) {
      const parts = c.path.split('/');
      let path = '';
      for (let i = 1; i < parts.length - 1; i++) {
        path += '/' + parts[i];
        dirs.add(path);
      }
      dirs.add(c.path); // Also add full path
    }
    return Array.from(dirs).sort();
  });

  // Filter suggestions based on input
  const filteredSuggestions = $derived(() => {
    if (!newIgnorePattern || newIgnorePattern.length < 2) return [];
    const input = newIgnorePattern.toLowerCase();
    const existing = new Set(config?.ignorePatterns ?? []);
    return suggestedPaths()
      .filter((p) => p.toLowerCase().includes(input) && !existing.has(p))
      .slice(0, 5);
  });

  function handleClickOutside(e: MouseEvent) {
    if (showIgnorePanel && ignorePanelRef && !ignorePanelRef.contains(e.target as Node)) {
      showIgnorePanel = false;
    }
  }

  const isScanning = $derived(config?.scanning ?? false);

  function formatProgress(): string {
    if (!config || !config.scanning) return '';
    const parts: string[] = [];

    if (config.percent > 0) {
      parts.push(`${config.percent}%`);
    }

    if (config.filesProcessed > 0) {
      const files = config.filesProcessed.toLocaleString();
      parts.push(`${files} files`);
    }

    if (config.rate > 0) {
      parts.push(`${config.rate.toLocaleString()}/s`);
    }

    // Calculate ETA
    if (config.totalFiles > 0 && config.rate > 0 && config.filesProcessed < config.totalFiles) {
      const remaining = config.totalFiles - config.filesProcessed;
      const etaSeconds = Math.ceil(remaining / config.rate);
      if (etaSeconds > 0 && etaSeconds < 3600) {
        parts.push(`~${etaSeconds}s`);
      }
    }

    return parts.join(' · ');
  }

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
    if (value < 5 || value > 3600) return;
    intervalValue = value;
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(async () => {
      try {
        const newConfig = await updateConfig({ interval: value });
        onConfigChange?.(newConfig);
      } catch (err) {
        console.error('Failed to update interval:', err);
      }
    }, 500);
  }

  async function handleScanNow() {
    try {
      await triggerScan();
      onScanComplete?.();
    } catch (err) {
      console.error('Failed to trigger scan:', err);
    }
  }

  async function addIgnorePattern() {
    if (!newIgnorePattern.trim() || !config) return;
    const pattern = newIgnorePattern.trim();
    if (config.ignorePatterns.includes(pattern)) return;

    try {
      const newConfig = await updateConfig({
        ignorePatterns: [...config.ignorePatterns, pattern],
      });
      onConfigChange?.(newConfig);
      newIgnorePattern = '';
    } catch (err) {
      console.error('Failed to add ignore pattern:', err);
    }
  }

  async function removeIgnorePattern(pattern: string) {
    if (!config) return;
    try {
      const newConfig = await updateConfig({
        ignorePatterns: config.ignorePatterns.filter((p) => p !== pattern),
      });
      onConfigChange?.(newConfig);
    } catch (err) {
      console.error('Failed to remove ignore pattern:', err);
    }
  }

  function selectSuggestion(path: string) {
    newIgnorePattern = path;
  }
</script>

<svelte:window onclick={handleClickOutside} />

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
        {#if isScanning}
          <div class="flex items-center gap-2">
            <div class="w-24 h-1.5 bg-bg-3 rounded-full overflow-hidden">
              <div
                class="h-full bg-accent transition-all duration-300"
                style="width: {config?.percent ?? 0}%"
              ></div>
            </div>
            <span class="text-accent font-mono text-xs">{formatProgress() || 'Scanning...'}</span>
          </div>
        {:else}
          <span class="text-fg-3">Next:</span>
          <span class="text-fg-2 font-mono w-8">{countdown}s</span>
        {/if}
      </div>

      <button
        type="button"
        class="px-2 py-1 text-xs rounded transition-colors
               {isScanning
                 ? 'bg-bg-3 text-fg-3 cursor-not-allowed'
                 : 'bg-accent text-bg-1 hover:bg-accent/80'}"
        disabled={isScanning}
        onclick={handleScanNow}
      >
        Scan Now
      </button>

      <div class="flex items-center gap-2">
        <span class="text-fg-3 text-xs">Interval:</span>
        <input
          type="number"
          min="5"
          max="3600"
          step="5"
          value={intervalValue}
          oninput={handleIntervalChange}
          class="w-16 bg-bg-3 text-fg-2 text-xs px-2 py-1 rounded border-none outline-none
                 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
        />
        <span class="text-fg-3 text-xs">s</span>
      </div>

      <div class="relative" bind:this={ignorePanelRef}>
        <button
          type="button"
          class="px-2 py-1 text-xs rounded transition-colors
                 {showIgnorePanel
                   ? 'bg-accent text-bg-1'
                   : 'bg-bg-3 text-fg-2 hover:bg-bg-3/80'}"
          onclick={() => (showIgnorePanel = !showIgnorePanel)}
        >
          Ignore ({config.ignorePatterns.length})
        </button>

        {#if showIgnorePanel}
          <div
            class="absolute right-0 top-full mt-1 w-80 bg-bg-2 border border-bg-3 rounded-lg shadow-xl z-50"
          >
            <div class="p-3 border-b border-bg-3">
              <div class="flex gap-2">
                <input
                  type="text"
                  placeholder="/path/to/ignore"
                  bind:value={newIgnorePattern}
                  onkeydown={(e) => e.key === 'Enter' && addIgnorePattern()}
                  class="flex-1 bg-bg-1 text-fg-1 text-sm px-2 py-1 rounded border border-bg-3
                         placeholder:text-fg-3 focus:border-accent focus:outline-none"
                />
                <button
                  type="button"
                  class="px-2 py-1 text-xs rounded bg-accent text-bg-1 hover:bg-accent/80"
                  onclick={addIgnorePattern}
                >
                  Add
                </button>
              </div>
              {#if filteredSuggestions().length > 0}
                <div class="mt-2 border border-bg-3 rounded bg-bg-1 max-h-32 overflow-y-auto">
                  {#each filteredSuggestions() as suggestion (suggestion)}
                    <button
                      type="button"
                      class="block w-full px-2 py-1 text-xs text-left text-fg-2 font-mono truncate hover:bg-bg-3"
                      onclick={() => selectSuggestion(suggestion)}
                    >
                      {suggestion}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
            <div class="max-h-48 overflow-y-auto">
              {#if config.ignorePatterns.length === 0}
                <div class="p-3 text-sm text-fg-3">No patterns</div>
              {:else}
                {#each config.ignorePatterns as pattern (pattern)}
                  <div class="flex items-center justify-between px-3 py-2 hover:bg-bg-3 group">
                    <span class="text-sm text-fg-2 font-mono truncate">{pattern}</span>
                    <button
                      type="button"
                      class="text-fg-3 hover:text-critical text-xs opacity-0 group-hover:opacity-100 transition-opacity"
                      onclick={() => removeIgnorePattern(pattern)}
                    >
                      remove
                    </button>
                  </div>
                {/each}
              {/if}
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <span class="flex items-center gap-1.5 text-sm">
      <span class="w-2 h-2 rounded-full {live ? 'bg-added animate-pulse' : 'bg-fg-3'}"></span>
      <span class="text-fg-3">{live ? 'Live' : 'Disconnected'}</span>
    </span>
  </div>
</header>
