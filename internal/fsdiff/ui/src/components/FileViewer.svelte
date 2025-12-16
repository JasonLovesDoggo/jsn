<script lang="ts">
  import type { Change } from '$lib/types';
  import { formatTime, formatSize, formatMode, resolveUid } from '$lib/utils';
  import { fetchContent, revertFile } from '$lib/api';
  import DiffViewer from './DiffViewer.svelte';

  interface Props {
    change: Change;
    onClose: () => void;
    onRevert?: () => void;
    onIgnore?: (path: string) => void;
  }

  let { change, onClose, onRevert, onIgnore }: Props = $props();

  let content = $state<string | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let viewMode = $state<'content' | 'diff'>('content');
  let reverting = $state(false);
  let revertError = $state<string | null>(null);
  let showRevertConfirm = $state(false);
  let showIgnorePanel = $state(false);

  const hasDiff = $derived(!!change.diff);
  const canRevert = $derived(!!change.content);

  // Generate all possible paths to ignore (file + all parent directories)
  const ignorePaths = $derived.by(() => {
    const paths: { path: string; label: string; type: 'file' | 'dir' }[] = [];
    const fullPath = change.path;

    // Add the full file path
    const fileName = fullPath.split('/').pop() || fullPath;
    paths.push({ path: fullPath, label: fileName, type: 'file' });

    // Add each parent directory
    const parts = fullPath.split('/').filter(Boolean);
    let currentPath = '';
    for (let i = 0; i < parts.length - 1; i++) {
      currentPath += '/' + parts[i];
      paths.push({
        path: currentPath,
        label: currentPath,
        type: 'dir',
      });
    }

    // Reverse so deepest paths come first
    return paths.reverse();
  });

  $effect(() => {
    if (change.content) {
      loading = true;
      error = null;
      fetchContent(change.content)
        .then((text) => {
          content = text;
        })
        .catch((e) => {
          error = e.message;
        })
        .finally(() => {
          loading = false;
        });
    } else {
      loading = false;
      content = null;
    }
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      if (showIgnorePanel) {
        showIgnorePanel = false;
      } else {
        onClose();
      }
    }
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      onClose();
    }
  }

  function getLines(text: string): string[] {
    return text.split('\n');
  }

  async function handleRevert() {
    if (!change.content) return;
    reverting = true;
    revertError = null;
    try {
      await revertFile(change.path, change.content);
      showRevertConfirm = false;
      onRevert?.();
      onClose();
    } catch (e) {
      revertError = e instanceof Error ? e.message : 'Revert failed';
    } finally {
      reverting = false;
    }
  }

  function handleIgnore(path: string) {
    onIgnore?.(path);
    showIgnorePanel = false;
    onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div
  class="fixed inset-0 bg-black/80 flex items-center justify-center z-50 p-8"
  onclick={handleBackdropClick}
  onkeydown={handleKeydown}
  tabindex="-1"
  role="dialog"
  aria-modal="true"
>
  <div class="bg-bg-2 rounded-lg shadow-2xl max-w-4xl w-full max-h-full flex flex-col overflow-hidden">
    <div class="flex items-center justify-between px-4 py-3 border-b border-bg-3">
      <h2 class="text-fg-1 font-mono text-sm truncate" title={change.path}>
        {change.path}
      </h2>
      <div class="flex items-center gap-3">
        {#if hasDiff}
          <div class="flex items-center gap-1 text-xs">
            <button
              type="button"
              class="px-2 py-1 rounded transition-colors
                     {viewMode === 'content' ? 'bg-accent text-bg-1' : 'text-fg-3 hover:text-fg-2 hover:bg-bg-3'}"
              onclick={() => (viewMode = 'content')}
            >
              Content
            </button>
            <button
              type="button"
              class="px-2 py-1 rounded transition-colors
                     {viewMode === 'diff' ? 'bg-accent text-bg-1' : 'text-fg-3 hover:text-fg-2 hover:bg-bg-3'}"
              onclick={() => (viewMode = 'diff')}
            >
              Diff
            </button>
          </div>
        {/if}
        <button
          type="button"
          class="text-fg-3 hover:text-fg-1 text-sm px-2"
          onclick={onClose}
        >
          ESC
        </button>
      </div>
    </div>

    <div class="flex-1 overflow-auto bg-bg-1">
      {#if viewMode === 'diff' && hasDiff}
        <DiffViewer diff={change.diff!} />
      {:else if loading}
        <div class="flex items-center justify-center h-48 text-fg-3">
          Loading...
        </div>
      {:else if error}
        <div class="flex items-center justify-center h-48 text-critical">
          {error}
        </div>
      {:else if content === null}
        <div class="flex items-center justify-center h-48 text-fg-3">
          No content available
        </div>
      {:else}
        <div class="font-mono text-sm">
          {#each getLines(content) as line, i (i)}
            <div class="flex hover:bg-bg-2">
              <span class="w-12 px-2 py-0.5 text-right text-fg-3 select-none border-r border-bg-3 shrink-0">
                {i + 1}
              </span>
              <pre class="px-3 py-0.5 text-fg-1 whitespace-pre overflow-x-auto flex-1">{line}</pre>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="flex items-center justify-between px-4 py-2 border-t border-bg-3">
      <div class="flex items-center gap-4 text-xs text-fg-3">
        <span>Size: {formatSize(change.size)}</span>
        <span>Modified: {formatTime(change.ts)}</span>
        {#if change.mode !== undefined}
          <span>Mode: {formatMode(change.mode)}</span>
        {/if}
      </div>

      <div class="flex items-center gap-2">
        {#if onIgnore}
          <div class="relative">
            <button
              type="button"
              class="px-2 py-1 text-xs rounded transition-colors
                     {showIgnorePanel ? 'bg-critical text-white' : 'bg-bg-3 text-fg-2 hover:bg-critical/20 hover:text-critical'}"
              onclick={() => (showIgnorePanel = !showIgnorePanel)}
            >
              Ignore
            </button>

            {#if showIgnorePanel}
              <div class="absolute bottom-full right-0 mb-1 w-80 bg-bg-2 border border-bg-3 rounded-lg shadow-xl z-50">
                <div class="px-3 py-2 border-b border-bg-3">
                  <div class="text-xs text-fg-2 font-medium">Ignore this path</div>
                  <div class="text-xs text-fg-3 mt-0.5">Select what to ignore:</div>
                </div>
                <div class="max-h-48 overflow-y-auto">
                  {#each ignorePaths as item (item.path)}
                    <button
                      type="button"
                      class="w-full flex items-center gap-2 px-3 py-2 text-left hover:bg-bg-3 transition-colors"
                      onclick={() => handleIgnore(item.path)}
                    >
                      <span class="text-xs px-1.5 py-0.5 rounded {item.type === 'file' ? 'bg-accent/20 text-accent' : 'bg-warn/20 text-warn'}">
                        {item.type === 'file' ? 'file' : 'dir'}
                      </span>
                      <span class="text-xs text-fg-1 font-mono truncate flex-1" title={item.path}>
                        {item.path}
                      </span>
                    </button>
                  {/each}
                </div>
              </div>
            {/if}
          </div>
        {/if}

        {#if canRevert}
          {#if showRevertConfirm}
            <span class="text-xs text-warn">Revert to this version?</span>
            <button
              type="button"
              class="px-2 py-1 text-xs rounded bg-critical text-white hover:bg-critical/80 disabled:opacity-50"
              onclick={handleRevert}
              disabled={reverting}
            >
              {reverting ? 'Reverting...' : 'Yes, Revert'}
            </button>
            <button
              type="button"
              class="px-2 py-1 text-xs rounded bg-bg-3 text-fg-2 hover:bg-bg-3/80"
              onclick={() => (showRevertConfirm = false)}
            >
              Cancel
            </button>
          {:else}
            <button
              type="button"
              class="px-2 py-1 text-xs rounded bg-bg-3 text-fg-2 hover:bg-warn hover:text-bg-1 transition-colors"
              onclick={() => (showRevertConfirm = true)}
              title="Revert file to this stored content"
            >
              Revert
            </button>
          {/if}
          {#if revertError}
            <span class="text-xs text-critical">{revertError}</span>
          {/if}
        {/if}
      </div>
    </div>
  </div>
</div>
