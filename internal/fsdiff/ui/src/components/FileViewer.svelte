<script lang="ts">
  import type { Change } from '$lib/types';
  import { formatTime, formatSize, formatMode, resolveUid } from '$lib/utils';
  import { fetchContent } from '$lib/api';
  import DiffViewer from './DiffViewer.svelte';

  interface Props {
    change: Change;
    onClose: () => void;
  }

  let { change, onClose }: Props = $props();

  let content = $state<string | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let viewMode = $state<'content' | 'diff'>('content');

  const hasDiff = $derived(!!change.diff);

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
      onClose();
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

    <div class="flex items-center gap-4 px-4 py-2 border-t border-bg-3 text-xs text-fg-3">
      <span>Size: {formatSize(change.size)}</span>
      <span>Modified: {formatTime(change.ts)}</span>
      {#if change.mode !== undefined}
        <span>Mode: {formatMode(change.mode)}</span>
      {/if}
    </div>
  </div>
</div>
